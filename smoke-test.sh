#!/bin/bash
# SSR smoke test - verifies every page renders without 500 errors.
# Requires docker compose to be running and seeded (./dev-seed.sh).
#
# Usage: ./smoke-test.sh
#
# Detects dev vs demo mode from .env and tests expected behavior for each:
# - Dev: persona cookies work without OIDC, auto-fallback to default persona
# - Demo: persona cookies require OIDC, unauthenticated users redirected to login

set -e

# Load mode from .env
if [ -f .env ]; then
  eval "$(grep -E '^(DEV_MODE|DEMO_MODE)=' .env)"
fi
DEV_MODE="${DEV_MODE:-true}"
DEMO_MODE="${DEMO_MODE:-false}"

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
    FAILURES="${FAILURES}\n  FAIL: ${label} - expected ${expect}, got ${code}"
  fi
}

# Check that a page body does NOT contain a string
check_not_contains() {
  local label="$1" url="$2" cookie="$3" forbidden="$4"
  local body
  body=$(curl -s "$url" -H "$cookie" --max-time 10 2>/dev/null)
  if echo "$body" | grep -q "$forbidden"; then
    FAIL=$((FAIL + 1))
    FAILURES="${FAILURES}\n  FAIL: ${label} - page contains '${forbidden}'"
  else
    PASS=$((PASS + 1))
  fi
}

echo "Mode: DEV_MODE=$DEV_MODE, DEMO_MODE=$DEMO_MODE"
echo "Waiting for web server..."
if [ "$DEMO_MODE" = "true" ]; then
  # In demo mode, unauthenticated requests redirect - wait for login page
  until curl -sf "$WEB/login" -o /dev/null 2>/dev/null; do sleep 1; done
else
  until curl -sf "$WEB" -H "$LEADER_COOKIE" -o /dev/null 2>/dev/null; do sleep 1; done
fi
echo "Web server ready."

# Fetch real IDs for dynamic routes
BOOKING_ID=$(curl -s "$API/api/v0/bookings" -H "X-Dev-Role-Override: leader-yggdrasil" \
  | python3 -c "import json,sys; bs=json.load(sys.stdin); print(bs[0]['id'] if bs else '')" 2>/dev/null)
ARTICLE_ID=$(curl -s "$API/api/v0/articles" -H "X-Dev-Role-Override: manager-equipment" \
  | python3 -c "import json,sys; arts=json.load(sys.stdin); print(arts[0]['id'] if arts else '')" 2>/dev/null)
ISSUE_ID=$(curl -s "$API/api/v0/issues" -H "X-Dev-Role-Override: manager-equipment" \
  | python3 -c "import json,sys; issues=json.load(sys.stdin); print(issues[0]['id'] if issues else '')" 2>/dev/null)

echo "Using booking_id=${BOOKING_ID:-<none>}, article_id=${ARTICLE_ID:-<none>}, issue_id=${ISSUE_ID:-<none>}"
echo ""

if [ "$DEMO_MODE" = "true" ]; then
  # ============================================================
  # DEMO MODE: unauthenticated users must be redirected to login
  # Persona cookies without OIDC session must not grant access
  # ============================================================

  echo "Testing demo mode: unauthenticated access blocked..."
  NO_COOKIE="Cookie: "
  check "Home (no auth->login)"     "$WEB/"        "$NO_COOKIE"  "302"
  check "Browse (no auth->login)"   "$WEB/browse"  "$NO_COOKIE"  "302"
  check "Book (no auth->login)"     "$WEB/book"    "$NO_COOKIE"  "302"
  check "Login page (public)"       "$WEB/login"   "$NO_COOKIE"  "200"

  echo "Testing demo mode: persona cookie without OIDC blocked..."
  check "Home (persona no OIDC->login)"    "$WEB/"        "$LEADER_COOKIE"   "302"
  check "Browse (persona no OIDC->login)"  "$WEB/browse"  "$MANAGER_COOKIE"  "302"

  echo "Testing demo mode: login page has no persona switcher..."
  check_not_contains "Login (no switcher)" "$WEB/login" "$NO_COOKIE" "Dev persona"

else
  # ============================================================
  # DEV MODE: persona cookies work, auto-fallback to default
  # ============================================================

  # --- Static pages (leader) ---
  echo "Testing static pages (leader)..."
  check "Home"       "$WEB/"          "$LEADER_COOKIE"
  check "Browse"     "$WEB/browse"    "$LEADER_COOKIE"
  check "Book"       "$WEB/book"      "$LEADER_COOKIE"
  check "Bookings"   "$WEB/bookings"  "$LEADER_COOKIE"
  check "Issues"     "$WEB/issues"    "$LEADER_COOKIE"
  check "Issues new" "$WEB/issues/new" "$LEADER_COOKIE"
  check "Guide"      "$WEB/guide"     "$LEADER_COOKIE"
  check "Profile"    "$WEB/profile"   "$LEADER_COOKIE"
  check "Login"      "$WEB/login"     "$LEADER_COOKIE"

  # --- Static pages (manager) ---
  echo "Testing static pages (manager)..."
  check "Browse (mgr)"     "$WEB/browse"        "$MANAGER_COOKIE"
  check "Issues (mgr)"     "$WEB/issues"        "$MANAGER_COOKIE"
  check "Issues new (mgr)" "$WEB/issues/new"    "$MANAGER_COOKIE"
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

  if [ -n "$ISSUE_ID" ]; then
    echo "Testing issue detail..."
    check "Issue detail (leader)"  "$WEB/issues/$ISSUE_ID"  "$LEADER_COOKIE"
    check "Issue detail (mgr)"    "$WEB/issues/$ISSUE_ID"  "$MANAGER_COOKIE"
  fi

  # --- Access control: manager-only pages redirect for leaders ---
  echo "Testing access control redirects..."
  check "New article (leader->redirect)"     "$WEB/articles/new"  "$LEADER_COOKIE"  "302"
  if [ -n "$ARTICLE_ID" ]; then
    check "Article edit (leader->redirect)"  "$WEB/articles/$ARTICLE_ID/edit"  "$LEADER_COOKIE"  "302"
  fi

  # --- View-only persona ---
  echo "Testing view-only persona..."
  VIEW_COOKIE="Cookie: dev-persona=view-only"
  check "Browse (view-only)"   "$WEB/browse"    "$VIEW_COOKIE"
  check "Issues (view-only)"   "$WEB/issues"    "$VIEW_COOKIE"
  check "Profile (view-only)"  "$WEB/profile"   "$VIEW_COOKIE"

  # --- Invalid persona cookie should not crash ---
  echo "Testing invalid persona..."
  check "Home (bad persona)"  "$WEB/"  "Cookie: dev-persona=nonexistent"
fi

# --- Results ---
echo ""
echo "Results: $PASS passed, $FAIL failed"
if [ $FAIL -gt 0 ]; then
  echo -e "$FAILURES"
  exit 1
fi
