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

echo "Done."
