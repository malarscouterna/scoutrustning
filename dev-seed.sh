#!/bin/bash
# Seed the development database with inventory and units.
# Usage: ./dev-seed.sh [path-to-csv]
#
# Requires the API to be running (docker compose up).

set -e

API="http://localhost:8080"
HEADER="X-Dev-Role-Override: equipment-manager"
CSV="${1:-docs/Utrustningsregister MS.xlsx - data.csv}"

echo "Waiting for API..."
until curl -sf "$API/api/health" > /dev/null 2>&1; do
  sleep 1
done
echo "API ready."

echo "Clearing existing seed data..."
curl -sf -X POST "$API/api/v0/articles/import" \
  -H "$HEADER" \
  -F "file=@/dev/null" > /dev/null 2>&1 || true
# Delete all existing articles, bookings will cascade
docker compose exec -T db psql -U utrustning -d utrustning -c "
  DELETE FROM booking_items;
  DELETE FROM bookings;
  DELETE FROM articles;
  DELETE FROM units;
" > /dev/null 2>&1
echo "Cleared."

echo "Importing articles from: $CSV"
RESULT=$(curl -sf -X POST "$API/api/v0/articles/import" \
  -H "$HEADER" \
  -F "file=@$CSV")
echo "$RESULT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(f'Imported: {d[\"imported\"]}, skipped: {d[\"skipped\"]}')"

echo "Creating units..."
for UNIT in Yggdrasil Ornéerna; do
  curl -sf -X POST "$API/api/v0/units" \
    -H "$HEADER" \
    -H "Content-Type: application/json" \
    -d "{\"name\":\"$UNIT\"}" > /dev/null && echo "  Created: $UNIT" || echo "  Exists: $UNIT"
done

# Create quantity-tracked test articles (Tältlampa LED)
# First delete the individually-tracked one from CSV import, then create 5 quantity-tracked
echo "Creating quantity-tracked test articles..."
LOC_ID=$(curl -s "$API/api/v0/locations" -H "$HEADER" | python3 -c "import json,sys; print([l['id'] for l in json.load(sys.stdin) if l['name']=='Hajkförrådet'][0])")
CAT_ID=$(curl -s "$API/api/v0/categories" -H "$HEADER" | python3 -c "import json,sys; print([c['id'] for c in json.load(sys.stdin) if c['name']=='Sova'][0])")
OLD_ID=$(curl -s "$API/api/v0/articles?search=T%C3%A4ltlampor" -H "$HEADER" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d[0]['id'] if d else '')")
if [ -n "$OLD_ID" ]; then
  curl -sf -X DELETE "$API/api/v0/articles/$OLD_ID" -H "$HEADER" > /dev/null
fi
for i in 1 2 3 4 5; do
  curl -sf -X POST "$API/api/v0/articles" \
    -H "$HEADER" \
    -H "Content-Type: application/json" \
    -d "{\"commercial_name\":\"Tältlampa LED\",\"common_name\":\"Tältlampa LED\",\"category_id\":\"$CAT_ID\",\"location_id\":\"$LOC_ID\",\"individually_tracked\":false,\"requires_approval\":false,\"place\":\"Hylla 2\"}" > /dev/null
done
echo "  Created: 5x Tältlampa LED (quantity-tracked)"

echo "Done."
