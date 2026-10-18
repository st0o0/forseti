#!/usr/bin/env bash
# Seed primary Pi-hole (alpha) with test data for sync E2E tests.
# Run AFTER Pi-hole containers are healthy, BEFORE starting Forseti in sync mode.
set -euo pipefail

ALPHA_URL="${1:-http://localhost:18080}"
ALPHA_PW="${2:-alpha-dev-password}"

SID=$(curl -sf -X POST "$ALPHA_URL/api/auth" \
  -H 'Content-Type: application/json' \
  -d "{\"password\":\"$ALPHA_PW\"}" | python -c "import sys,json; print(json.load(sys.stdin)['session']['sid'])")

H="X-FTL-SID: $SID"
CT="Content-Type: application/json"

echo "Seeding primary ($ALPHA_URL) ..."

# Groups
echo "  groups ..."
curl -sf -X POST "$ALPHA_URL/api/groups" -H "$H" -H "$CT" \
  -d '{"name":"e2e-sync-group","comment":"seed for sync","enabled":true}' > /dev/null

# Adlists (block)
echo "  adlists (block) ..."
curl -sf -X POST "$ALPHA_URL/api/lists?type=block" -H "$H" -H "$CT" \
  -d '{"address":"https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts","comment":"seed blocklist","enabled":true}' > /dev/null

# Allowlists
echo "  allowlists ..."
curl -sf -X POST "$ALPHA_URL/api/lists?type=allow" -H "$H" -H "$CT" \
  -d '{"address":"https://raw.githubusercontent.com/anudeepND/whitelist/master/domains/whitelist.txt","comment":"seed allowlist","enabled":true}' > /dev/null

# Deny domains
echo "  deny domains ..."
curl -sf -X POST "$ALPHA_URL/api/domains/deny/exact" -H "$H" -H "$CT" \
  -d '{"domain":"sync-deny.example.com","comment":"seed deny","enabled":true}' > /dev/null
curl -sf -X POST "$ALPHA_URL/api/domains/deny/regex" -H "$H" -H "$CT" \
  -d '{"domain":".*tracking\\.sync\\..*","comment":"seed deny regex","enabled":true}' > /dev/null

# Allow domains
echo "  allow domains ..."
curl -sf -X POST "$ALPHA_URL/api/domains/allow/exact" -H "$H" -H "$CT" \
  -d '{"domain":"sync-allow.example.com","comment":"seed allow","enabled":true}' > /dev/null

# Local DNS
echo "  local DNS ..."
curl -sf -X PUT "$ALPHA_URL/api/config/dns/hosts/$(python -c 'import urllib.parse; print(urllib.parse.quote("192.168.10.1 sync-nas.lan"))')" \
  -H "$H" > /dev/null
curl -sf -X PUT "$ALPHA_URL/api/config/dns/hosts/$(python -c 'import urllib.parse; print(urllib.parse.quote("192.168.10.2 sync-printer.lan"))')" \
  -H "$H" > /dev/null

# CNAME records
echo "  CNAME records ..."
curl -sf -X PUT "$ALPHA_URL/api/config/dns/cnameRecords/$(python -c 'import urllib.parse; print(urllib.parse.quote("app.sync.lan,server.sync.lan"))')" \
  -H "$H" > /dev/null
curl -sf -X PUT "$ALPHA_URL/api/config/dns/cnameRecords/$(python -c 'import urllib.parse; print(urllib.parse.quote("web.sync.lan,lb.sync.lan"))')" \
  -H "$H" > /dev/null

# Clients
echo "  clients ..."
curl -sf -X POST "$ALPHA_URL/api/clients" -H "$H" -H "$CT" \
  -d '{"client":"10.0.0.0/8","comment":"seed client"}' > /dev/null

echo "Primary seeded."
