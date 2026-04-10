#!/bin/bash
# SSR smoke test — verifies every page renders without 500 errors.
# Requires docker compose to be running and seeded (./dev-seed.sh).
#
# Usage: ./smoke-test.sh
#
# Catches: SSR crashes from uninitialized state, broken load functions,
# template errors during server-side rendering.

set -e

WEB="${WEB:-http://localhost:3000}"
API="${API:-http://localhost:8080}"
LEADER_COOKIE="Cookie: dev-persona=leader-yggdrasil"
MANAGER_COOKIE="Cookie: dev-persona=manager-equipment"

PASS=0
FAIL=0
FAILURES=""

check() {
  local label="$1" url="$2" cookie="$3" expect="${4:-200}"
  local code
  code=$(curl -s -o /dev/null -w '%{http_code}' "$url" -H "$cookie" --max-time 10 2>/dev/null)
  if [ "$code" = "$expect" ]; then
    PASS=$((PASS + 1))
  else
    FAIL=$((FAIL + 1))
    FAILURES="${FAILURES}\n  FAIL: ${label} — expected ${expect}, got ${code}"
  fi
}

echo "Waiting for web server..."
until curl -sf "$WEB" -H "$LEADER_COOKIE" -o /dev/null 2>/dev/null; do
  sleep 1
done
echo "Web server ready."

# Fetch real IDs for dynamic routes
BOOKING_ID=$(curl -s "$API/api/v0/bookings" -H "X-Dev-Role-Override: leader-yggdrasil" \
  | python3 -c "import json,sys; bs=json.load(sys.stdin); print(bs[0]['id'] if bs else '')" 2>/dev/null)
ARTICLE_ID=$(curl -s "$API/api/v0/articles" -H "X-Dev-Role-Override: manager-equipment" \
  | python3 -c "import json,sys; arts=json.load(sys.stdin); print(arts[0]['id'] if arts else '')" 2>/dev/null)

echo "Using booking_id=${BOOKING_ID:-<none>}, article_id=${ARTICLE_ID:-<none>}"
echo ""

# --- Static pages (leader) ---
echo "Testing static pages (leader)..."
check "Home"       "$WEB/"          "$LEADER_COOKIE"
check "Browse"     "$WEB/browse"    "$LEADER_COOKIE"
check "Book"       "$WEB/book"      "$LEADER_COOKIE"
check "Bookings"   "$WEB/bookings"  "$LEADER_COOKIE"
check "Issues"     "$WEB/issues"    "$LEADER_COOKIE"
check "Guide"      "$WEB/guide"     "$LEADER_COOKIE"
check "Profile"    "$WEB/profile"   "$LEADER_COOKIE"
check "Login"      "$WEB/login"     "$LEADER_COOKIE"

# --- Static pages (manager) ---
echo "Testing static pages (manager)..."
check "Browse (mgr)"     "$WEB/browse"        "$MANAGER_COOKIE"
check "Issues (mgr)"     "$WEB/issues"        "$MANAGER_COOKIE"
check "Profile (mgr)"    "$WEB/profile"       "$MANAGER_COOKIE"
check "New article"      "$WEB/articles/new"   "$MANAGER_COOKIE"

# --- Dynamic pages ---
if [ -n "$BOOKING_ID" ]; then
  echo "Testing booking detail..."
  check "Booking detail (leader)"  "$WEB/bookings/$BOOKING_ID"  "$LEADER_COOKIE"
  check "Booking detail (mgr)"    "$WEB/bookings/$BOOKING_ID"  "$MANAGER_COOKIE"
fi

if [ -n "$ARTICLE_ID" ]; then
  echo "Testing article pages..."
  check "Article detail (leader)"  "$WEB/articles/$ARTICLE_ID"       "$LEADER_COOKIE"
  check "Article detail (mgr)"    "$WEB/articles/$ARTICLE_ID"       "$MANAGER_COOKIE"
  check "Article edit (mgr)"      "$WEB/articles/$ARTICLE_ID/edit"  "$MANAGER_COOKIE"
fi

# --- Access control: manager-only pages redirect for leaders ---
echo "Testing access control redirects..."
check "New article (leader→redirect)"     "$WEB/articles/new"  "$LEADER_COOKIE"  "302"
if [ -n "$ARTICLE_ID" ]; then
  check "Article edit (leader→redirect)"  "$WEB/articles/$ARTICLE_ID/edit"  "$LEADER_COOKIE"  "302"
fi

# --- Results ---
echo ""
echo "Results: $PASS passed, $FAIL failed"
if [ $FAIL -gt 0 ]; then
  echo -e "$FAILURES"
  exit 1
fi
