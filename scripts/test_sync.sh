#!/bin/bash
set -e

LOGIN_RES=$(curl -s -X POST http://127.0.0.1:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}')

TOKEN=$(echo "$LOGIN_RES" | grep -o '"token":"[^"]*' | cut -d'"' -f4)

echo "Logged in successfully. Token length: ${#TOKEN}"

echo ""
echo "=== 1. Auto-Update Status ==="
curl -s -H "Authorization: Bearer $TOKEN" http://127.0.0.1:8080/api/v1/blocklists/auto-update-status

echo ""
echo ""
echo "=== 2. Triggering Full Blocklists Sync ==="
curl -s -X POST -H "Authorization: Bearer $TOKEN" http://127.0.0.1:8080/api/v1/blocklists/sync

echo ""
echo ""
echo "=== 3. List Filters in DB ==="
curl -s -H "Authorization: Bearer $TOKEN" http://127.0.0.1:8080/api/v1/filters | grep -o '"id":"[^"]*' | head -n 10

echo ""
echo "=== 4. Checking Unbound Configuration Syntax ==="
unbound-checkconf /etc/unbound/unbound.conf
echo "Unbound syntax check passed!"

echo ""
echo ""
echo "=== 5. Testing DNS Resolution Before Toggle ==="
echo "sbobet.com (Gambling): $(dig @127.0.0.1 -p 53 sbobet.com +short || echo 'BLOCKED')"
echo "xhamster.com (Adult before toggle): $(dig @127.0.0.1 -p 53 xhamster.com +short || echo 'BLOCKED')"

echo ""
echo "=== 6. Toggling Adult Feed (feed-adult) ON ==="
curl -s -X PUT -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"enabled":true}' \
  http://127.0.0.1:8080/api/v1/filters/feed-adult

sleep 2
echo ""
echo "xhamster.com after toggle ON (should be BLOCKED / empty):"
dig @127.0.0.1 -p 53 xhamster.com +short || true
dig @127.0.0.1 -p 53 xhamster.com | grep "status:"

echo ""
echo "=== 7. Toggling AdBlock Feed (feed-ads) ON ==="
curl -s -X PUT -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"enabled":true}' \
  http://127.0.0.1:8080/api/v1/filters/feed-ads

sleep 2
echo ""
echo "adservice.google.com after toggle ON (should be BLOCKED / empty):"
dig @127.0.0.1 -p 53 adservice.google.com +short || true
dig @127.0.0.1 -p 53 adservice.google.com | grep "status:"

