#!/bin/bash
# Seed the development database with inventory and units.
# Usage: ./dev-seed.sh [path-to-csv]
#
# Requires the API to be running (docker compose up).

set -e

API="${API:-http://localhost:8080}"
HEADER="X-Dev-Role-Override: manager-equipment"
LEADER="X-Dev-Role-Override: leader-yggdrasil"
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
  DELETE FROM article_events;
  DELETE FROM booking_items;
  DELETE FROM bookings;
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

# Report issues on some quantity-tracked articles to demo status mix
echo ""
echo "Reporting issues on quantity-tracked articles..."

# Tältlampa LED: report 2 with issues
LAMP_IDS=$(curl -s "$API/api/v0/articles?search=T%C3%A4ltlampa%20LED" -H "$HEADER" | python3 -c "
import json,sys
ids = [a['id'] for a in json.load(sys.stdin)]
for i in ids: print(i)
")
LAMP1=$(echo "$LAMP_IDS" | sed -n '1p')
LAMP2=$(echo "$LAMP_IDS" | sed -n '2p')
LAMP3=$(echo "$LAMP_IDS" | sed -n '3p')

curl -sf -X PUT "$API/api/v0/articles/$LAMP1/status" \
  -H "$LEADER" -H "Content-Type: application/json" \
  -d '{"status":"reported_usable","comment":"Blinkar ibland"}' > /dev/null
echo "  Tältlampa LED: 1 reported usable (blinkar)"

curl -sf -X PUT "$API/api/v0/articles/$LAMP2/status" \
  -H "$LEADER" -H "Content-Type: application/json" \
  -d '{"status":"reported_unusable","comment":"Helt trasig, lyser inte alls"}' > /dev/null
echo "  Tältlampa LED: 1 reported unusable (trasig)"

curl -sf -X PUT "$API/api/v0/articles/$LAMP3/status" \
  -H "$HEADER" -H "Content-Type: application/json" \
  -d '{"status":"under_repair","comment":"Skickad för batteribyte","expected_available_date":"2026-07-10"}' > /dev/null
echo "  Tältlampa LED: 1 under repair (beräknas klar 10 jul)"

# Pannlampa: archive one
PANN_IDS=$(curl -s "$API/api/v0/articles?search=Pannlampa" -H "$HEADER" | python3 -c "
import json,sys
ids = [a['id'] for a in json.load(sys.stdin)]
for i in ids: print(i)
")
PANN1=$(echo "$PANN_IDS" | sed -n '1p')
curl -sf -X PUT "$API/api/v0/articles/$PANN1/status" \
  -H "$HEADER" -H "Content-Type: application/json" \
  -d '{"status":"archived","comment":"Uttjänt, ersätts inte"}' > /dev/null
echo "  Pannlampa: 1 archived"

# Stormkök: report issues on a couple
STORM_IDS=$(curl -s "$API/api/v0/articles?search=Stormk%C3%B6k" -H "$HEADER" | python3 -c "
import json,sys
ids = [a['id'] for a in json.load(sys.stdin) if a['individually_tracked']]
for i in ids: print(i)
")
STORM4=$(echo "$STORM_IDS" | sed -n '4p')
STORM5=$(echo "$STORM_IDS" | sed -n '5p')

curl -sf -X PUT "$API/api/v0/articles/$STORM4/status" \
  -H "$LEADER" -H "Content-Type: application/json" \
  -d '{"status":"reported_usable","comment":"Brännaren flämtar lite"}' > /dev/null
echo "  Stormkök 4: reported usable (flämtar)"

curl -sf -X PUT "$API/api/v0/articles/$STORM5/status" \
  -H "$LEADER" -H "Content-Type: application/json" \
  -d '{"status":"reported_unusable","comment":"Läcker bränsle, farligt"}' > /dev/null
echo "  Stormkök 5: reported unusable (läcker)"

# Helper: find booking item ID by article common_name
find_item() {
  local BOOKING=$1 NAME=$2 PERSONA=$3
  curl -s "$API/api/v0/bookings/$BOOKING" -H "X-Dev-Role-Override: $PERSONA" | python3 -c "
import json,sys
d = json.load(sys.stdin)
for i in d['items']:
    if i['common_name'] == '$NAME':
        print(i['id'])
        break
"
}

# Helper: find article ID by common_name
find_article() {
  local NAME=$1
  curl -s "$API/api/v0/articles?search=$(python3 -c "import urllib.parse; print(urllib.parse.quote('$NAME'))")" -H "$HEADER" | python3 -c "
import json,sys
for a in json.load(sys.stdin):
    if a['common_name'] == '$NAME':
        print(a['id'])
        break
"
}

UNIT_ID=$(curl -s "$API/api/v0/units" -H "$LEADER" | python3 -c "import json,sys; print([u['id'] for u in json.load(sys.stdin) if u['name']=='Yggdrasil'][0])")

# ─── Booking 1: Active booking in picked_up state (checked out right now) ───
echo ""
TODAY=$(date +%Y-%m-%d)
END_5D=$(date -d "+5 days" +%Y-%m-%d 2>/dev/null || date -v+5d +%Y-%m-%d)
echo "Creating booking 1 (active, picked_up, $TODAY to $END_5D)..."
BOOKING_ID=$(curl -s -X POST "$API/api/v0/bookings" \
  -H "$LEADER" -H "Content-Type: application/json" \
  -d "{\"start_date\":\"$TODAY\",\"end_date\":\"$END_5D\",\"used_by_unit_id\":\"$UNIT_ID\",\"notes\":\"Hajk med Yggdrasil\"}" | python3 -c "import json,sys; print(json.load(sys.stdin)['id'])")
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
curl -sf -X POST "$API/api/v0/bookings/$BOOKING_ID/items" \
  -H "$LEADER" -H "Content-Type: application/json" \
  -d '{"commercial_name":"Brandfilt","quantity":2}' > /dev/null
echo "  Added: 2x Sibley, 2x Stormkök, 3x Tältlampa LED, 2x Brandfilt"

curl -sf -X POST "$API/api/v0/bookings/$BOOKING_ID/submit" -H "$LEADER" > /dev/null
curl -sf -X POST "$API/api/v0/bookings/$BOOKING_ID/pickup" -H "$LEADER" > /dev/null
echo "  Status: picked_up"

# Pick up everything except Stormkök (left for pickup testing)
ITEMS_JSON=$(curl -s "$API/api/v0/bookings/$BOOKING_ID" -H "$LEADER")
echo "$ITEMS_JSON" | python3 -c "
import json,sys
d = json.load(sys.stdin)
for i in d['items']:
    if not i['common_name'].startswith('Stormkök'):
        print(i['id'], i['common_name'])
" | while read ID NAME; do
  curl -sf -X PUT "$API/api/v0/bookings/$BOOKING_ID/items/$ID/pickup" \
    -H "$LEADER" -H "Content-Type: application/json" \
    -d '{"pickup_status":"picked_up"}' > /dev/null
done
echo "  Picked up all except Stormkök"

# Return Sibley 1 as OK
SIBLEY1_ITEM=$(find_item "$BOOKING_ID" "Sibley 1" "leader-yggdrasil")
curl -sf -X PUT "$API/api/v0/bookings/$BOOKING_ID/items/$SIBLEY1_ITEM/return" \
  -H "$LEADER" -H "Content-Type: application/json" \
  -d '{"return_status":"returned_ok"}' > /dev/null
echo "  Sibley 1: returned OK"

# Return Sibley 2 as reported_usable (creates picked_up + issue_reported events)
SIBLEY2_ITEM=$(find_item "$BOOKING_ID" "Sibley 2" "leader-yggdrasil")
curl -sf -X PUT "$API/api/v0/bookings/$BOOKING_ID/items/$SIBLEY2_ITEM/return" \
  -H "$LEADER" -H "Content-Type: application/json" \
  -d '{"return_status":"reported_usable","notes":"Liten reva i duken"}' > /dev/null
echo "  Sibley 2: returned with issue (reva i duken)"

# Manager acknowledges Sibley 2 issue → under_repair
SIBLEY2_ART=$(find_article "Sibley 2")
curl -sf -X PUT "$API/api/v0/articles/$SIBLEY2_ART/status" \
  -H "$HEADER" -H "Content-Type: application/json" \
  -d '{"status":"under_repair","comment":"Skickat till lagning"}' > /dev/null
echo "  Sibley 2: manager set under_repair"

# Manager resolves Sibley 2 → ok
curl -sf -X PUT "$API/api/v0/articles/$SIBLEY2_ART/status" \
  -H "$HEADER" -H "Content-Type: application/json" \
  -d '{"status":"ok","comment":"Lagad, redo att användas igen"}' > /dev/null
echo "  Sibley 2: manager resolved → ok"

# Leader reports new issue on Sibley 2 → reported_unusable
curl -sf -X PUT "$API/api/v0/articles/$SIBLEY2_ART/status" \
  -H "$LEADER" -H "Content-Type: application/json" \
  -d '{"status":"reported_unusable","comment":"Revan har öppnat sig igen, går inte att använda"}' > /dev/null
echo "  Sibley 2: leader re-reported as unusable"

# Manager sets under_repair again
curl -sf -X PUT "$API/api/v0/articles/$SIBLEY2_ART/status" \
  -H "$HEADER" -H "Content-Type: application/json" \
  -d '{"status":"under_repair","comment":"Ny lagning behövs"}' > /dev/null
echo "  Sibley 2: manager set under_repair again"

# Manager resolves again
curl -sf -X PUT "$API/api/v0/articles/$SIBLEY2_ART/status" \
  -H "$HEADER" -H "Content-Type: application/json" \
  -d '{"status":"ok","comment":"Dubbelsydd, borde hålla nu"}' > /dev/null
echo "  Sibley 2: manager resolved again → ok"

# Leader reports a third time
curl -sf -X PUT "$API/api/v0/articles/$SIBLEY2_ART/status" \
  -H "$LEADER" -H "Content-Type: application/json" \
  -d '{"status":"reported_unusable","comment":"Sömmarna har gått upp igen, behöver bytas ut"}' > /dev/null
echo "  Sibley 2: leader reported unusable a third time (8 events total)"

# ─── Standalone issue reports (not from bookings) ───
echo ""
echo "Creating standalone issue reports..."

# Bryne: reported usable
BRYNE_ID=$(curl -s "$API/api/v0/articles?search=Bryne" -H "$HEADER" | python3 -c "import json,sys; arts=[a for a in json.load(sys.stdin) if a['location_name']=='Hajkförrådet']; print(arts[0]['id'] if arts else '')")
if [ -n "$BRYNE_ID" ]; then
  curl -sf -X PUT "$API/api/v0/articles/$BRYNE_ID/status" \
    -H "$LEADER" -H "Content-Type: application/json" \
    -d '{"status":"reported_usable","comment":"Slitet och behöver slipas"}' > /dev/null
  echo "  Bryne: reported usable (slitet)"
fi

# ─── Booking 2: Confirmed, reserved for next week (not yet picked up) ───
echo ""
START_7D=$(date -d "+7 days" +%Y-%m-%d 2>/dev/null || date -v+7d +%Y-%m-%d)
END_12D=$(date -d "+12 days" +%Y-%m-%d 2>/dev/null || date -v+12d +%Y-%m-%d)
echo "Creating booking 2 (confirmed, $START_7D to $END_12D)..."
BOOKING2_ID=$(curl -s -X POST "$API/api/v0/bookings" \
  -H "$LEADER" -H "Content-Type: application/json" \
  -d "{\"start_date\":\"$START_7D\",\"end_date\":\"$END_12D\",\"used_by_unit_id\":\"$UNIT_ID\",\"notes\":\"Sommarläger\"}" | python3 -c "import json,sys; print(json.load(sys.stdin)['id'])")

curl -sf -X POST "$API/api/v0/bookings/$BOOKING2_ID/items" \
  -H "$LEADER" -H "Content-Type: application/json" \
  -d '{"commercial_name":"Brandfilt","quantity":2}' > /dev/null
curl -sf -X POST "$API/api/v0/bookings/$BOOKING2_ID/items" \
  -H "$LEADER" -H "Content-Type: application/json" \
  -d '{"commercial_name":"Primus","quantity":1}' > /dev/null
curl -sf -X POST "$API/api/v0/bookings/$BOOKING2_ID/items" \
  -H "$LEADER" -H "Content-Type: application/json" \
  -d '{"commercial_name":"Liggunderlag","quantity":4}' > /dev/null
curl -sf -X POST "$API/api/v0/bookings/$BOOKING2_ID/submit" -H "$LEADER" > /dev/null
echo "  Booking 2 (confirmed, next week): 2x Brandfilt, 1x Primus, 4x Liggunderlag"

echo ""
echo "Done."
