#!/bin/bash
# Seed the development database with inventory and teams.
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

echo "Clearing existing seed data..."
docker compose exec -T db psql -U utrustning -d utrustning -c "
  DELETE FROM article_events;
  DELETE FROM booking_events;
  DELETE FROM booking_items;
  DELETE FROM bookings;
  DELETE FROM product_images;
  DELETE FROM articles;
  DELETE FROM team_claim_mappings;
  DELETE FROM teams;
  DELETE FROM group_settings;
  DELETE FROM categories;
  DELETE FROM locations;
  DELETE FROM users;
  DELETE FROM groups;
" || echo "Warning: cleanup had errors, continuing..."
# Clear image files (dev mode uses local mount)
rm -rf data/images/*.webp 2>/dev/null || true
echo "Cleared."

echo "Bootstrapping groups..."
# In dev mode (air), binary is at ./tmp/server; in production, /bin/server
SERVER_BIN=$(docker compose exec -T api sh -c 'test -f ./tmp/server && echo ./tmp/server || echo /bin/server')
docker compose exec -T api $SERVER_BIN init-group \
  --group-id 766 --group-name "Mälarscouterna" \
  --manager-claim "group:766:material_responsible" --team-name "Utrustningsgruppen" \
  --seed-locations

docker compose exec -T api $SERVER_BIN init-group \
  --group-id 999 --group-name "Testkåren" \
  --manager-claim "group:999:admin" --team-name "Admin"

# Check that the API is in dev mode (X-Dev-Role-Override must work)
HTTP_CODE=$(curl -s -o /dev/null -w '%{http_code}' "$API/api/v0/locations" -H "$HEADER")
if [ "$HTTP_CODE" != "200" ]; then
  echo "ERROR: API rejected dev persona header (HTTP $HTTP_CODE)."
  echo "       The seed script requires DEV_MODE=true on the API."
  echo "       Run with: docker compose up --build"
  exit 1
fi

echo "Importing articles from: $CSV"
RESULT=$(curl -sf -X POST "$API/api/v0/articles/import" \
  -H "$HEADER" \
  -F "file=@$CSV")
echo "$RESULT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(f'Imported: {d[\"imported\"]}, skipped: {d[\"skipped\"]}')"

echo "Creating teams..."
# Troops (book level)
for entry in "Yggdrasil:troop:17443:book" "Spindlarna:troop:9109:book" "Valarna:troop:19260:book" "Flaskpostorné:troop:20956:book"; do
  IFS=: read NAME TYPE CLAIM_ID ACCESS <<< "$entry"
  curl -sf -X POST "$API/api/v0/teams" \
    -H "$HEADER" -H "Content-Type: application/json" \
    -d "{\"name\":\"$NAME\",\"type\":\"$TYPE\",\"access_level\":\"$ACCESS\",\"claim_scope\":\"troop\",\"claim_id\":\"$CLAIM_ID\"}" > /dev/null && echo "  Created: $NAME ($ACCESS)" || echo "  Exists: $NAME"
done
# Roles with specific access levels (Utrustningsgruppen already created by init-group)
for entry in "IT-gruppen:role:it_manager:manager" "Valborgskommittén:role:walpurgis_committee:trusted" "Läger:role:group_camp_committee:trusted"; do
  IFS=: read NAME TYPE CLAIM_ID ACCESS <<< "$entry"
  curl -sf -X POST "$API/api/v0/teams" \
    -H "$HEADER" -H "Content-Type: application/json" \
    -d "{\"name\":\"$NAME\",\"type\":\"$TYPE\",\"access_level\":\"$ACCESS\",\"claim_scope\":\"group\",\"claim_id\":\"$CLAIM_ID\"}" > /dev/null && echo "  Created: $NAME ($ACCESS)" || echo "  Exists: $NAME"
done
# Test group team
curl -sf -X POST "$API/api/v0/teams" \
  -H "X-Dev-Role-Override: other-kar-leader" -H "Content-Type: application/json" \
  -d '{"name":"Avdelning 1","type":"troop","access_level":"book","claim_scope":"troop","claim_id":"99901"}' > /dev/null && echo "  Created: Avdelning 1 (book, group 999)" || echo "  Exists: Avdelning 1"

# Report issues on some quantity-tracked articles to demo status mix
echo ""
echo "Uploading product images..."
SEED_IMG_DIR="docs/seed-images"
if [ -d "$SEED_IMG_DIR" ]; then
  # Get all article groups and compute upload plan in one Python pass
  ARTICLES_JSON=$(curl -s "$API/api/v0/articles" -H "$HEADER")
  UPLOAD_PLAN=$(ARTICLES_JSON="$ARTICLES_JSON" SEED_IMG_DIR="$SEED_IMG_DIR" python3 << 'PYEOF'
import json, os, re, unicodedata, glob, sys

def simplify(s):
    s = unicodedata.normalize("NFD", s.lower())
    return "".join(c for c in s if unicodedata.category(c) != "Mn")

articles = json.loads(os.environ["ARTICLES_JSON"])
img_dir = os.environ["SEED_IMG_DIR"]

# Build article groups: simplified_name -> [{name, location_id, location_name}]
groups = {}
for a in articles:
    key = (a["commercial_name"], a["location_id"])
    if key not in groups:
        groups[key] = {"name": a["commercial_name"], "location_id": a["location_id"], "location_name": a["location_name"]}
by_name = {}
for g in groups.values():
    by_name.setdefault(simplify(g["name"]), []).append(g)
for v in by_name.values():
    v.sort(key=lambda x: x["location_name"])

descriptions = ["{name} ute i verkligheten", "{name} i skogen"]
group_idx = {}

files = sorted(glob.glob(os.path.join(img_dir, "*")))
for f in files:
    ext = os.path.splitext(f)[1].lower()
    if ext not in (".jpg", ".jpeg", ".png", ".webp", ".heic"):
        continue
    base = os.path.splitext(os.path.basename(f))[0]
    m = re.match(r"^(.+?)(?:-(\d+))?$", base.lower())
    file_key, file_num = m.group(1), int(m.group(2) or 1)
    if file_key not in by_name:
        continue
    locations = by_name[file_key]
    # Multi-location split: first half of numbered images → first location, rest → second
    if len(locations) > 1:
        target = locations[0 if file_num <= 2 else min(1, len(locations) - 1)]
    else:
        target = locations[0]
    gkey = (target["name"], target["location_id"])
    idx = group_idx.get(gkey, 0)
    group_idx[gkey] = idx + 1
    desc = descriptions[idx % 2].format(name=target["name"])
    print(json.dumps({"file": f, "name": target["name"], "location_id": target["location_id"], "title": target["name"], "description": desc}))
PYEOF
  )

  ATTRIBUTION="Teo, Mälarscouterna"
  TEO="X-Dev-Role-Override: leader-team-it"
  echo "$UPLOAD_PLAN" | while IFS= read -r LINE; do
    [ -z "$LINE" ] && continue
    file=$(echo "$LINE" | python3 -c "import json,sys; print(json.load(sys.stdin)['file'])")
    name=$(echo "$LINE" | python3 -c "import json,sys; print(json.load(sys.stdin)['name'])")
    location_id=$(echo "$LINE" | python3 -c "import json,sys; print(json.load(sys.stdin)['location_id'])")
    title=$(echo "$LINE" | python3 -c "import json,sys; print(json.load(sys.stdin)['title'])")
    description=$(echo "$LINE" | python3 -c "import json,sys; print(json.load(sys.stdin)['description'])")
    BASENAME=$(basename "$file")
    curl -sf -X POST "$API/api/v0/images/product" \
      -H "$TEO" \
      -F "file=@$file" \
      -F "commercial_name=$name" \
      -F "location_id=$location_id" \
      -F "title=$title" \
      -F "description=$description" \
      -F "attribution=$ATTRIBUTION" > /dev/null && echo "  $name → $BASENAME" || echo "  FAILED: $name → $BASENAME"
  done
else
  echo "  Skipped (no images in $SEED_IMG_DIR)"
fi

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
# Manager investigates, sets under repair
curl -sf -X PUT "$API/api/v0/articles/$LAMP2/status" \
  -H "$HEADER" -H "Content-Type: application/json" \
  -d '{"status":"under_repair","comment":"Skickat för reparation"}' > /dev/null
# Repaired, back to ok
curl -sf -X PUT "$API/api/v0/articles/$LAMP2/status" \
  -H "$HEADER" -H "Content-Type: application/json" \
  -d '{"status":"ok","comment":"Reparerad, fungerar igen"}' > /dev/null
# Breaks again
curl -sf -X PUT "$API/api/v0/articles/$LAMP2/status" \
  -H "$LEADER" -H "Content-Type: application/json" \
  -d '{"status":"reported_unusable","comment":"Trasig igen, samma problem"}' > /dev/null
echo "  Tältlampa LED: 1 reported unusable (trasig, reparerad, trasig igen)"

curl -sf -X PUT "$API/api/v0/articles/$LAMP3/status" \
  -H "$HEADER" -H "Content-Type: application/json" \
  -d '{"status":"under_repair","comment":"Skickad för batteribyte","expected_available_date":"2026-07-10"}' > /dev/null
echo "  Tältlampa LED: 1 under repair (beräknas klar 10 jul)"

# Lykta: archive one
LYKTA_IDS=$(curl -s "$API/api/v0/articles?search=Lykta" -H "$HEADER" | python3 -c "
import json,sys
ids = [a['id'] for a in json.load(sys.stdin)]
for i in ids: print(i)
")
LYKTA1=$(echo "$LYKTA_IDS" | sed -n '1p')
curl -sf -X PUT "$API/api/v0/articles/$LYKTA1/status" \
  -H "$HEADER" -H "Content-Type: application/json" \
  -d '{"status":"archived","comment":"Uttjänt, ersätts inte"}' > /dev/null
echo "  Lykta: 1 archived"

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

# Stormkök 7 (Östergården): incoming with expected date
STORM_OG_IDS=$(curl -s "$API/api/v0/articles?search=Stormk%C3%B6k" -H "$HEADER" | python3 -c "
import json,sys
for a in json.load(sys.stdin):
    if a['common_name'] == 'Stormkök 7': print(a['id']); break
")
if [ -n "$STORM_OG_IDS" ]; then
  curl -sf -X PUT "$API/api/v0/articles/$STORM_OG_IDS/status" \
    -H "$HEADER" -H "Content-Type: application/json" \
    -d '{"status":"incoming","comment":"Beställd, leverans väntas","expected_available_date":"2026-08-01"}' > /dev/null
  echo "  Stormkök 7 (Östergården): incoming (beräknas 1 aug)"
fi

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

TEAM_ID=$(curl -s "$API/api/v0/teams" -H "$LEADER" | python3 -c "import json,sys; print([u['id'] for u in json.load(sys.stdin) if u['name']=='Yggdrasil'][0])")

# ─── Booking 1: Active booking in picked_up state (checked out right now) ───
echo ""
TODAY=$(date +%Y-%m-%d)
END_5D=$(date -d "+5 days" +%Y-%m-%d 2>/dev/null || date -v+5d +%Y-%m-%d)
echo "Creating booking 1 (active, picked_up, $TODAY to $END_5D)..."
BOOKING_ID=$(curl -s -X POST "$API/api/v0/bookings" \
  -H "$LEADER" -H "Content-Type: application/json" \
  -d "{\"start_date\":\"$TODAY\",\"end_date\":\"$END_5D\",\"used_by_team_id\":\"$TEAM_ID\",\"notes\":\"Hajk med Yggdrasil\"}" | python3 -c "import json,sys; print(json.load(sys.stdin)['id'])")
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
# Sibley is low-approval, so manager needs to approve before pickup
curl -sf -X POST "$API/api/v0/bookings/$BOOKING_ID/approve" \
  -H "$HEADER" -H "Content-Type: application/json" \
  -d '{"message":"Godkänt, ha en bra hajk!"}' > /dev/null
curl -sf -X POST "$API/api/v0/bookings/$BOOKING_ID/pickup" -H "$LEADER" > /dev/null
echo "  Status: picked_up"

# Pick up everything
ITEMS_JSON=$(curl -s "$API/api/v0/bookings/$BOOKING_ID" -H "$LEADER")
echo "$ITEMS_JSON" | python3 -c "
import json,sys
d = json.load(sys.stdin)
for i in d['items']:
    print(i['id'], i['common_name'])
" | while read ID NAME; do
  curl -sf -X PUT "$API/api/v0/bookings/$BOOKING_ID/items/$ID/pickup" \
    -H "$LEADER" -H "Content-Type: application/json" \
    -d '{"pickup_status":"picked_up"}' > /dev/null
done
echo "  Picked up all items"

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

# Jägarpanna: reported usable
JAGAR_ID=$(curl -s "$API/api/v0/articles?search=J%C3%A4garpanna" -H "$HEADER" | python3 -c "import json,sys; arts=json.load(sys.stdin); print(arts[0]['id'] if arts else '')")
if [ -n "$JAGAR_ID" ]; then
  curl -sf -X PUT "$API/api/v0/articles/$JAGAR_ID/status" \
    -H "$LEADER" -H "Content-Type: application/json" \
    -d '{"status":"reported_usable","comment":"Rostfläckar på pannan"}' > /dev/null
  echo "  Jägarpanna: reported usable (rostfläckar)"
fi

# ─── Booking 2: Confirmed, reserved for next week (not yet picked up) ───
echo ""
START_7D=$(date -d "+7 days" +%Y-%m-%d 2>/dev/null || date -v+7d +%Y-%m-%d)
END_12D=$(date -d "+12 days" +%Y-%m-%d 2>/dev/null || date -v+12d +%Y-%m-%d)
echo "Creating booking 2 (confirmed, $START_7D to $END_12D)..."
BOOKING2_ID=$(curl -s -X POST "$API/api/v0/bookings" \
  -H "$LEADER" -H "Content-Type: application/json" \
  -d "{\"start_date\":\"$START_7D\",\"end_date\":\"$END_12D\",\"used_by_team_id\":\"$TEAM_ID\",\"notes\":\"Sommarläger vid Karsvik, 12 utmanare + 3 ledare\"}" | python3 -c "import json,sys; print(json.load(sys.stdin)['id'])")

curl -sf -X POST "$API/api/v0/bookings/$BOOKING2_ID/items" \
  -H "$LEADER" -H "Content-Type: application/json" \
  -d '{"commercial_name":"Brandfilt","quantity":2}' > /dev/null
curl -sf -X POST "$API/api/v0/bookings/$BOOKING2_ID/items" \
  -H "$LEADER" -H "Content-Type: application/json" \
  -d '{"commercial_name":"Primus","quantity":1}' > /dev/null
curl -sf -X POST "$API/api/v0/bookings/$BOOKING2_ID/items" \
  -H "$LEADER" -H "Content-Type: application/json" \
  -d '{"commercial_name":"Matk\u00e5sa","quantity":2}' > /dev/null
curl -sf -X POST "$API/api/v0/bookings/$BOOKING2_ID/submit" \
  -H "$LEADER" -H "Content-Type: application/json" \
  -d '{"message":"Behöver hämta på fredag kväll, går det bra?"}' > /dev/null
echo "  Booking 2 (confirmed): 2x Brandfilt, 1x Primus, 2x Matkåsa"

# ─── Booking 3: Submitted, waiting for approval (leader booked Sibley = low) ───
echo ""
FLASKPOST="X-Dev-Role-Override: leader-flaskpost"
FLASK_TEAM_ID=$(curl -s "$API/api/v0/teams" -H "$FLASKPOST" | python3 -c "import json,sys; print([u['id'] for u in json.load(sys.stdin) if u['name']=='Flaskpostorné'][0])")
START_14D=$(date -d "+14 days" +%Y-%m-%d 2>/dev/null || date -v+14d +%Y-%m-%d)
END_16D=$(date -d "+16 days" +%Y-%m-%d 2>/dev/null || date -v+16d +%Y-%m-%d)
echo "Creating booking 3 (submitted, awaiting approval, $START_14D to $END_16D)..."
BOOKING3_ID=$(curl -s -X POST "$API/api/v0/bookings" \
  -H "$FLASKPOST" -H "Content-Type: application/json" \
  -d "{\"start_date\":\"$START_14D\",\"end_date\":\"$END_16D\",\"used_by_team_id\":\"$FLASK_TEAM_ID\",\"notes\":\"Helgutflykt med Flaskpostorné, övernattning vid sjön\"}" | python3 -c "import json,sys; print(json.load(sys.stdin)['id'])")

curl -sf -X POST "$API/api/v0/bookings/$BOOKING3_ID/items" \
  -H "$FLASKPOST" -H "Content-Type: application/json" \
  -d '{"commercial_name":"Sibley","quantity":1}' > /dev/null
curl -sf -X POST "$API/api/v0/bookings/$BOOKING3_ID/items" \
  -H "$FLASKPOST" -H "Content-Type: application/json" \
  -d "{\"commercial_name\":\"Stormkök\",\"quantity\":2}" > /dev/null
curl -sf -X POST "$API/api/v0/bookings/$BOOKING3_ID/submit" \
  -H "$FLASKPOST" -H "Content-Type: application/json" \
  -d '{"message":"Vi är 8 scouter och 2 ledare, behöver ett stort tält för samling"}' > /dev/null
echo "  Booking 3 (submitted): 1x Sibley (low), 2x Stormkök — waiting for approval"

# ─── Booking 4: Trusted team booking (auto-confirmed despite low approval) ───
echo ""
PROJECT_LEADER="X-Dev-Role-Override: project-unit-leader"
VALBORG_ID=$(curl -s "$API/api/v0/teams" -H "$PROJECT_LEADER" | python3 -c "import json,sys; print([u['id'] for u in json.load(sys.stdin) if u['name']=='Valborgskommittén'][0])")
START_21D=$(date -d "+21 days" +%Y-%m-%d 2>/dev/null || date -v+21d +%Y-%m-%d)
END_22D=$(date -d "+22 days" +%Y-%m-%d 2>/dev/null || date -v+22d +%Y-%m-%d)
echo "Creating booking 4 (trusted team, auto-confirmed, $START_21D to $END_22D)..."
BOOKING4_ID=$(curl -s -X POST "$API/api/v0/bookings" \
  -H "$PROJECT_LEADER" -H "Content-Type: application/json" \
  -d "{\"start_date\":\"$START_21D\",\"end_date\":\"$END_22D\",\"used_by_team_id\":\"$VALBORG_ID\",\"notes\":\"Valborg 2026 — uppställning och fest\"}" | python3 -c "import json,sys; print(json.load(sys.stdin)['id'])")

curl -sf -X POST "$API/api/v0/bookings/$BOOKING4_ID/items" \
  -H "$PROJECT_LEADER" -H "Content-Type: application/json" \
  -d '{"commercial_name":"Sibley","quantity":1}' > /dev/null
curl -sf -X POST "$API/api/v0/bookings/$BOOKING4_ID/items" \
  -H "$PROJECT_LEADER" -H "Content-Type: application/json" \
  -d '{"commercial_name":"Brandfilt","quantity":3}' > /dev/null
curl -sf -X POST "$API/api/v0/bookings/$BOOKING4_ID/submit" -H "$PROJECT_LEADER" > /dev/null
echo "  Booking 4 (confirmed): 1x Sibley (low, auto-approved via trusted team), 3x Brandfilt"

# ─── Booking 5: Returned booking from two weeks ago ───
echo ""
START_PAST=$(date -d "-14 days" +%Y-%m-%d 2>/dev/null || date -v-14d +%Y-%m-%d)
END_PAST=$(date -d "-10 days" +%Y-%m-%d 2>/dev/null || date -v-10d +%Y-%m-%d)
echo "Creating booking 5 (returned, $START_PAST to $END_PAST)..."
BOOKING5_ID=$(curl -s -X POST "$API/api/v0/bookings" \
  -H "$LEADER" -H "Content-Type: application/json" \
  -d "{\"start_date\":\"$START_PAST\",\"end_date\":\"$END_PAST\",\"used_by_team_id\":\"$TEAM_ID\",\"notes\":\"Helgövning i skogen\"}" | python3 -c "import json,sys; print(json.load(sys.stdin)['id'])")

curl -sf -X POST "$API/api/v0/bookings/$BOOKING5_ID/items" \
  -H "$LEADER" -H "Content-Type: application/json" \
  -d '{"commercial_name":"Vindskydd","quantity":2}' > /dev/null
curl -sf -X POST "$API/api/v0/bookings/$BOOKING5_ID/items" \
  -H "$LEADER" -H "Content-Type: application/json" \
  -d '{"commercial_name":"Presenning","quantity":1}' > /dev/null
curl -sf -X POST "$API/api/v0/bookings/$BOOKING5_ID/submit" -H "$LEADER" > /dev/null
curl -sf -X POST "$API/api/v0/bookings/$BOOKING5_ID/pickup" -H "$LEADER" > /dev/null

ITEMS5_JSON=$(curl -s "$API/api/v0/bookings/$BOOKING5_ID" -H "$LEADER")
echo "$ITEMS5_JSON" | python3 -c "
import json,sys
d = json.load(sys.stdin)
for i in d['items']:
    print(i['id'])
" | while read ID; do
  curl -sf -X PUT "$API/api/v0/bookings/$BOOKING5_ID/items/$ID/pickup" \
    -H "$LEADER" -H "Content-Type: application/json" \
    -d '{"pickup_status":"picked_up"}' > /dev/null
  curl -sf -X PUT "$API/api/v0/bookings/$BOOKING5_ID/items/$ID/return" \
    -H "$LEADER" -H "Content-Type: application/json" \
    -d '{"return_status":"returned_ok"}' > /dev/null
done
curl -sf -X POST "$API/api/v0/bookings/$BOOKING5_ID/return" -H "$LEADER" > /dev/null
echo "  Booking 5 (returned): 2x Vindskydd, 1x Presenning"

# ─── Booking 6: Manager's own booking (confirmed) ───
echo ""
MANAGER="X-Dev-Role-Override: manager-equipment"
START_10D=$(date -d "+10 days" +%Y-%m-%d 2>/dev/null || date -v+10d +%Y-%m-%d)
END_11D=$(date -d "+11 days" +%Y-%m-%d 2>/dev/null || date -v+11d +%Y-%m-%d)
echo "Creating booking 6 (manager's own, confirmed, $START_10D to $END_11D)..."
BOOKING6_ID=$(curl -s -X POST "$API/api/v0/bookings" \
  -H "$MANAGER" -H "Content-Type: application/json" \
  -d "{\"start_date\":\"$START_10D\",\"end_date\":\"$END_11D\",\"notes\":\"Inventering av Hajkförrådet\"}" | python3 -c "import json,sys; print(json.load(sys.stdin)['id'])")

curl -sf -X POST "$API/api/v0/bookings/$BOOKING6_ID/items" \
  -H "$MANAGER" -H "Content-Type: application/json" \
  -d '{"commercial_name":"Lykta","quantity":2}' > /dev/null
curl -sf -X POST "$API/api/v0/bookings/$BOOKING6_ID/submit" -H "$MANAGER" > /dev/null
echo "  Booking 6 (confirmed): 2x Lykta (manager's personal booking)"

# ─── Booking 7: Rejected → edited → resubmitted (approval conversation) ───
echo ""
START_25D=$(date -d "+25 days" +%Y-%m-%d 2>/dev/null || date -v+25d +%Y-%m-%d)
END_27D=$(date -d "+27 days" +%Y-%m-%d 2>/dev/null || date -v+27d +%Y-%m-%d)
echo "Creating booking 7 (rejected then resubmitted, $START_25D to $END_27D)..."
BOOKING7_ID=$(curl -s -X POST "$API/api/v0/bookings" \
  -H "$LEADER" -H "Content-Type: application/json" \
  -d "{\"start_date\":\"$START_25D\",\"end_date\":\"$END_27D\",\"used_by_team_id\":\"$TEAM_ID\",\"notes\":\"Hajk med Yggdrasil — behöver tält och tarp\"}" | python3 -c "import json,sys; print(json.load(sys.stdin)['id'])")

curl -sf -X POST "$API/api/v0/bookings/$BOOKING7_ID/items" \
  -H "$LEADER" -H "Content-Type: application/json" \
  -d '{"commercial_name":"Sibley","quantity":2}' > /dev/null
curl -sf -X POST "$API/api/v0/bookings/$BOOKING7_ID/items" \
  -H "$LEADER" -H "Content-Type: application/json" \
  -d '{"commercial_name":"Tarp","quantity":2}' > /dev/null

# Leader submits with explanation
curl -sf -X POST "$API/api/v0/bookings/$BOOKING7_ID/submit" \
  -H "$LEADER" -H "Content-Type: application/json" \
  -d '{"message":"Vi ska på hajk med 15 utmanare, behöver 2 Sibley för att alla ska få plats. Tarparna behövs som regnskydd."}' > /dev/null
echo "  Submitted with message"

# Manager rejects
curl -sf -X POST "$API/api/v0/bookings/$BOOKING7_ID/reject" \
  -H "$HEADER" -H "Content-Type: application/json" \
  -d '{"message":"Sibley 2 är under reparation just nu, boka bara 1 Sibley + 1 Vindskydd istället. Tarparna är ok."}' > /dev/null
echo "  Manager rejected with feedback"

# Leader removes one Sibley, adds a Vindskydd
curl -sf -X POST "$API/api/v0/bookings/$BOOKING7_ID/items" \
  -H "$LEADER" -H "Content-Type: application/json" \
  -d '{"commercial_name":"Vindskydd","quantity":1}' > /dev/null

SIBLEY_ITEM=$(curl -s "$API/api/v0/bookings/$BOOKING7_ID" -H "$LEADER" | python3 -c "
import json,sys
d = json.load(sys.stdin)
for i in d['items']:
    if i['commercial_name'] == 'Sibley':
        print(i['id'])
        break
")
curl -sf -X DELETE "$API/api/v0/bookings/$BOOKING7_ID/items/$SIBLEY_ITEM" -H "$LEADER" > /dev/null

# Leader resubmits with response
curl -sf -X POST "$API/api/v0/bookings/$BOOKING7_ID/submit" \
  -H "$LEADER" -H "Content-Type: application/json" \
  -d '{"message":"Ändrat till 1x Sibley + 1x Vindskydd som du föreslog, tack!"}' > /dev/null
echo "  Leader edited and resubmitted"
echo "  Booking 7 (submitted): 1x Sibley, 2x Tarp, 1x Vindskydd — approval conversation with 3 events"

# ─── Booking 8: Force-approval on freely bookable items ───
echo ""
START_30D=$(date -d "+30 days" +%Y-%m-%d 2>/dev/null || date -v+30d +%Y-%m-%d)
END_32D=$(date -d "+32 days" +%Y-%m-%d 2>/dev/null || date -v+32d +%Y-%m-%d)
echo "Creating booking 8 (force-approval, $START_30D to $END_32D)..."
BOOKING8_ID=$(curl -s -X POST "$API/api/v0/bookings" \
  -H "$LEADER" -H "Content-Type: application/json" \
  -d "{\"start_date\":\"$START_30D\",\"end_date\":\"$END_32D\",\"used_by_team_id\":\"$TEAM_ID\",\"notes\":\"Prova-på-dag för nya scouter\"}" | python3 -c "import json,sys; print(json.load(sys.stdin)['id'])")

curl -sf -X POST "$API/api/v0/bookings/$BOOKING8_ID/items" \
  -H "$LEADER" -H "Content-Type: application/json" \
  -d "{\"commercial_name\":\"Stormkök\",\"quantity\":3}" > /dev/null
curl -sf -X POST "$API/api/v0/bookings/$BOOKING8_ID/items" \
  -H "$LEADER" -H "Content-Type: application/json" \
  -d '{"commercial_name":"Brandfilt","quantity":2}' > /dev/null
curl -sf -X POST "$API/api/v0/bookings/$BOOKING8_ID/submit" \
  -H "$LEADER" -H "Content-Type: application/json" \
  -d '{"message":"Första gången vi gör detta, vill gärna att ni kollar att vi bokar rätt grejer?","force_approval":true}' > /dev/null
echo "  Booking 8 (submitted, force-approval): 3x Stormkök, 2x Brandfilt — leader asked for review"

echo ""
echo "Done."
