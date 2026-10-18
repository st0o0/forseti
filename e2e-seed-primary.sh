#!/usr/bin/env bash
# Seed primary Pi-hole (alpha) with test data for sync E2E tests.
# Run AFTER Pi-hole containers are healthy, BEFORE starting Forseti in sync mode.
# Uses -f (fail silently on 4xx) but ignores duplicates via || true.
set -uo pipefail

ALPHA_URL="${1:-http://localhost:18080}"
ALPHA_PW="${2:-alpha-dev-password}"

SID=$(curl -sf -X POST "$ALPHA_URL/api/auth" \
  -H 'Content-Type: application/json' \
  -d "{\"password\":\"$ALPHA_PW\"}" | python -c "import sys,json; print(json.load(sys.stdin)['session']['sid'])")

H="X-FTL-SID: $SID"
CT="Content-Type: application/json"

seed() {
  local label="$1"; shift
  local result
  result=$(curl -s -o /dev/null -w "%{http_code}" "$@" 2>&1)
  case "$result" in
    2*) echo "  $label: created" ;;
    *)  echo "  $label: ok (already exists or $result)" ;;
  esac
}

echo "Seeding primary ($ALPHA_URL) ..."

seed "group: e2e-sync-group" -X POST "$ALPHA_URL/api/groups" -H "$H" -H "$CT" \
  -d '{"name":"e2e-sync-group","comment":"seed for sync","enabled":true}'

seed "adlist: StevenBlack" -X POST "$ALPHA_URL/api/lists?type=block" -H "$H" -H "$CT" \
  -d '{"address":"https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts","comment":"seed blocklist","enabled":true}'

seed "allowlist: anudeepND" -X POST "$ALPHA_URL/api/lists?type=allow" -H "$H" -H "$CT" \
  -d '{"address":"https://raw.githubusercontent.com/anudeepND/whitelist/master/domains/whitelist.txt","comment":"seed allowlist","enabled":true}'

seed "deny: sync-deny.example.com" -X POST "$ALPHA_URL/api/domains/deny/exact" -H "$H" -H "$CT" \
  -d '{"domain":"sync-deny.example.com","comment":"seed deny","enabled":true}'

seed "deny regex" -X POST "$ALPHA_URL/api/domains/deny/regex" -H "$H" -H "$CT" \
  -d '{"domain":".*tracking\\.sync\\..*","comment":"seed deny regex","enabled":true}'

seed "allow: sync-allow.example.com" -X POST "$ALPHA_URL/api/domains/allow/exact" -H "$H" -H "$CT" \
  -d '{"domain":"sync-allow.example.com","comment":"seed allow","enabled":true}'

seed "dns: sync-nas.lan" -X PUT "$ALPHA_URL/api/config/dns/hosts/$(python -c 'import urllib.parse; print(urllib.parse.quote("192.168.10.1 sync-nas.lan"))')" -H "$H"

seed "dns: sync-printer.lan" -X PUT "$ALPHA_URL/api/config/dns/hosts/$(python -c 'import urllib.parse; print(urllib.parse.quote("192.168.10.2 sync-printer.lan"))')" -H "$H"

seed "cname: app.sync.lan" -X PUT "$ALPHA_URL/api/config/dns/cnameRecords/$(python -c 'import urllib.parse; print(urllib.parse.quote("app.sync.lan,server.sync.lan"))')" -H "$H"

seed "cname: web.sync.lan" -X PUT "$ALPHA_URL/api/config/dns/cnameRecords/$(python -c 'import urllib.parse; print(urllib.parse.quote("web.sync.lan,lb.sync.lan"))')" -H "$H"

seed "client: 10.0.0.0/8" -X POST "$ALPHA_URL/api/clients" -H "$H" -H "$CT" \
  -d '{"client":"10.0.0.0/8","comment":"seed client"}'

echo "Primary seeded."
