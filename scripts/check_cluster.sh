#!/bin/bash
set -e

TOKEN=$(curl -s -X POST http://127.0.0.1:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | jq -r .token)

echo "Authenticated with token: ${TOKEN:0:15}..."
echo "--- CLUSTER STATUS ---"
curl -s http://127.0.0.1:8080/api/v1/cluster/status \
  -H "Authorization: Bearer $TOKEN" | jq .
