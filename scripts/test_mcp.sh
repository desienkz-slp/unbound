#!/bin/bash
set -e

REQ='{"jsonrpc":"2.0","id":1,"method":"tools/list"}'
echo "$REQ" | /usr/local/bin/netora-mcp | jq .
