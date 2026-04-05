#!/bin/bash
# Seed the development database with inventory and units.
# Usage: ./dev-seed.sh [path-to-csv]
#
# Requires the API to be running (docker compose up).

set -e

API="${API:-http://localhost:8080}"
HEADER="X-Dev-Role-Override: manager-it"
CSV="${1:-docs/import-example.csv}"

echo "Waiting for API..."
until curl -sf "$API/api/health" > /dev/null 2>&1; do
  sleep 1
done
echo "API ready."

# Check that the API is in dev mode (X-Dev-Role-Override must work)
HTTP_CODE=$(curl -s -o /dev/null -w '%{http_code}' "$API/api/v0/locations" -H "$HEADER")
if [ "$HTTP_CODE" != "200" ]; then
  echo "ERROR: API rejected dev persona header (HTTP $HTTP_CODE)."
  echo "       The seed script requires DEV_MODE=true on the API."
  echo "       Run with: docker compose up --build"
  exit 1
fi

echo "Clearing existing seed data..."
docker compose exec -T db psql -U utrustning -d utrustning -c "
  DELETE FROM audit_log;
  DELETE FROM issue_reports;
  DELETE FROM booking_items;
  DELETE FROM bookings;
  DELETE FROM package_items;
  DELETE FROM packages;
  DELETE FROM articles;
  DELETE FROM units;
" || echo "Warning: cleanup had errors, continuing..."
echo "Cleared."

echo "Importing articles from: $CSV"
RESULT=$(curl -sf -X POST "$API/api/v0/articles/import" \
  -H "$HEADER" \
  -F "file=@$CSV")
echo "$RESULT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(f'Imported: {d[\"imported\"]}, skipped: {d[\"skipped\"]}')"

echo "Creating units and projects..."
for UNIT in Yggdrasil Spindlarna Valarna Flaskpostorné; do
  curl -sf -X POST "$API/api/v0/units" \
    -H "$HEADER" \
    -H "Content-Type: application/json" \
    -d "{\"name\":\"$UNIT\",\"type\":\"unit\"}" > /dev/null && echo "  Created unit: $UNIT" || echo "  Exists: $UNIT"
done
for PROJECT in Valborgskommittén Läger Utrustningsgruppen IT-gruppen; do
  curl -sf -X POST "$API/api/v0/units" \
    -H "$HEADER" \
    -H "Content-Type: application/json" \
    -d "{\"name\":\"$PROJECT\",\"type\":\"project\"}" > /dev/null && echo "  Created project: $PROJECT" || echo "  Exists: $PROJECT"
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

# Create a test booking in picked_up state with items in various states
echo "Creating test booking..."
LEADER="X-Dev-Role-Override: leader-yggdrasil"

UNIT_ID=$(curl -s "$API/api/v0/units" -H "$LEADER" | python3 -c "import json,sys; print([u['id'] for u in json.load(sys.stdin) if u['name']=='Yggdrasil'][0])")

BOOKING_ID=$(curl -s -X POST "$API/api/v0/bookings" \
  -H "$LEADER" -H "Content-Type: application/json" \
  -d "{\"start_date\":\"2026-06-01\",\"end_date\":\"2026-06-05\",\"used_by_unit_id\":\"$UNIT_ID\",\"notes\":\"Testbokning\"}" | python3 -c "import json,sys; print(json.load(sys.stdin)['id'])")
echo "  Booking: $BOOKING_ID"

curl -sf -X POST "$API/api/v0/bookings/$BOOKING_ID/items" \
  -H "$LEADER" -H "Content-Type: application/json" \
  -d '{"commercial_name":"Sibley","quantity":2}' > /dev/null
curl -sf -X POST "$API/api/v0/bookings/$BOOKING_ID/items" \
  -H "$LEADER" -H "Content-Type: application/json" \
  -d '{"commercial_name":"Stormk\u00f6k","quantity":2}' > /dev/null
curl -sf -X POST "$API/api/v0/bookings/$BOOKING_ID/items" \
  -H "$LEADER" -H "Content-Type: application/json" \
  -d '{"commercial_name":"T\u00e4ltlampa LED","quantity":3}' > /dev/null
echo "  Added items: 2x Sibley, 2x Stormkök, 3x Tältlampa LED"

curl -sf -X POST "$API/api/v0/bookings/$BOOKING_ID/submit" -H "$LEADER" > /dev/null
curl -sf -X POST "$API/api/v0/bookings/$BOOKING_ID/pickup" -H "$LEADER" > /dev/null
echo "  Status: picked_up"

# Mark some items with different pickup/return statuses
ITEMS=$(curl -s "$API/api/v0/bookings/$BOOKING_ID" -H "$LEADER" | python3 -c "
import json,sys
d = json.load(sys.stdin)
for i in d['items']:
    print(i['id'], i['common_name'])
")
echo "  Items:"
echo "$ITEMS" | while read ID NAME; do echo "    $NAME ($ID)"; done

# Pick up most items, leave some Stormkök unmarked for pickup testing
ITEM_COUNT=0
echo "$ITEMS" | while read ID NAME; do
  ITEM_COUNT=$((ITEM_COUNT + 1))
  case "$NAME" in
    Stormk*) ;; # skip — leave for pickup testing
    *)
      curl -sf -X PUT "$API/api/v0/bookings/$BOOKING_ID/items/$ID/pickup" \
        -H "$LEADER" -H "Content-Type: application/json" \
        -d '{"pickup_status":"picked_up"}' > /dev/null
      ;;
  esac
done
echo "  Picked up all except Stormkök (left for pickup testing)"

# Return first Sibley as OK, second as reported_usable
FIRST_SIBLEY=$(echo "$ITEMS" | head -1 | cut -d' ' -f1)
SECOND_SIBLEY=$(echo "$ITEMS" | sed -n '2p' | cut -d' ' -f1)
curl -sf -X PUT "$API/api/v0/bookings/$BOOKING_ID/items/$FIRST_SIBLEY/return" \
  -H "$LEADER" -H "Content-Type: application/json" \
  -d '{"return_status":"returned_ok"}' > /dev/null
echo "  Sibley 1: returned OK"

curl -sf -X PUT "$API/api/v0/bookings/$BOOKING_ID/items/$SECOND_SIBLEY/return" \
  -H "$LEADER" -H "Content-Type: application/json" \
  -d '{"return_status":"reported_usable","notes":"Liten reva i duken"}' > /dev/null
echo "  Sibley 2: reported usable (reva)"

# Report an issue on Bryne directly (not via return)
BRYNE_ID=$(curl -s "$API/api/v0/articles?search=Bryne" -H "$LEADER" | python3 -c "import json,sys; print(json.load(sys.stdin)[0]['id'])")
curl -sf -X PUT "$API/api/v0/articles/$BRYNE_ID/status" \
  -H "$LEADER" -H "Content-Type: application/json" \
  -d '{"status":"reported_usable","comment":"Slitet och beh\u00f6ver slipas"}' > /dev/null
echo "  Bryne: reported usable (slitet)"

# Archive Pannlampa via manager status update
PANN_ID=$(curl -s "$API/api/v0/articles?search=Pannlampa" -H "$HEADER" | python3 -c "import json,sys; print(json.load(sys.stdin)[0]['id'])")
curl -sf -X PUT "$API/api/v0/articles/$PANN_ID/status" \
  -H "$HEADER" -H "Content-Type: application/json" \
  -d '{"status":"archived","comment":"Uttjänt, ersätts inte"}' > /dev/null
echo "  Pannlampa: archived"

# Create a second booking in confirmed state (ready for pickup testing)
BOOKING2_ID=$(curl -s -X POST "$API/api/v0/bookings" \
  -H "$LEADER" -H "Content-Type: application/json" \
  -d "{\"start_date\":\"2026-07-01\",\"end_date\":\"2026-07-05\",\"used_by_unit_id\":\"$UNIT_ID\",\"notes\":\"Sommarläger\"}" | python3 -c "import json,sys; print(json.load(sys.stdin)['id'])")
curl -sf -X POST "$API/api/v0/bookings/$BOOKING2_ID/items" \
  -H "$LEADER" -H "Content-Type: application/json" \
  -d '{"commercial_name":"Brandfilt","quantity":2}' > /dev/null
curl -sf -X POST "$API/api/v0/bookings/$BOOKING2_ID/items" \
  -H "$LEADER" -H "Content-Type: application/json" \
  -d '{"commercial_name":"Primus","quantity":1}' > /dev/null
curl -sf -X POST "$API/api/v0/bookings/$BOOKING2_ID/submit" -H "$LEADER" > /dev/null
echo "  Booking 2 (confirmed): 2x Brandfilt, 1x Primus"

echo "Done."
