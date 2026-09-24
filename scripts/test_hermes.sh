#!/bin/bash
set -e

TOKEN=$(curl -s -X POST http://127.0.0.1:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | jq -r .token)

echo "=== 1. HERMES REST API INVESTIGATION: doubleclick.net ==="
curl -s -X POST http://127.0.0.1:8080/api/v1/hermes/investigate \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"domain":"doubleclick.net","client_ip":"172.18.20.100","policy_group":"DEFAULT"}' | jq .

echo ""
echo "=== 2. HERMES REST API INVESTIGATION: google.com ==="
curl -s -X POST http://127.0.0.1:8080/api/v1/hermes/investigate \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"domain":"google.com","client_ip":"172.18.20.100","policy_group":"DEFAULT"}' | jq .
