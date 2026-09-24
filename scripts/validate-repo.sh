#!/usr/bin/env bash
set -euo pipefail

echo "== Netora DNS repository validation =="

command -v git >/dev/null && echo "[OK] git" || echo "[WARN] git missing"
command -v docker >/dev/null && echo "[OK] docker" || echo "[WARN] docker missing"
command -v go >/dev/null && echo "[OK] go" || echo "[WARN] go missing"
command -v node >/dev/null && echo "[OK] node" || echo "[WARN] node missing"

if command -v unbound-checkconf >/dev/null 2>&1; then
  echo "[OK] unbound-checkconf"
else
  echo "[INFO] unbound-checkconf not installed on this machine"
fi

echo "Repository instructions:"
echo "  AGENTS.md"
echo "  .agents/00-bootstrap.md"
echo "  .agents/rules/"
echo "  .agents/skills/"
echo "  .agents/architecture/"
echo "  .agents/mcp/"
