#!/bin/bash
# ============================================================
#  update-lists.sh — Download semua blocklist dari sumber
# ============================================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BLOCKLIST_DIR="/etc/unbound/blocklist"

source "${SCRIPT_DIR}/../lib/helpers.sh" 2>/dev/null || {
  # Fallback jika helpers tidak ada
  log_info()    { echo "[INFO] $1"; }
  log_success() { echo "[✅] $1"; }
  log_error()   { echo "[❌] $1"; }
  safe_download() {
    curl -fsSL --connect-timeout 10 --max-time 60 "$1" -o "$2" 2>/dev/null
  }
}

log_info "🔁 Mengupdate semua blocklist..."

# === 1. Blocklist Iklan (ABP format) ===
safe_download \
  "https://raw.githubusercontent.com/hagezi/dns-blocklists/main/adblock/popupads.txt" \
  "${BLOCKLIST_DIR}/block-ads.txt" \
  "Blocklist Iklan (Hagezi)"

# === 2. Blocklist Malware (hosts format) ===
safe_download \
  "https://raw.githubusercontent.com/hagezi/dns-blocklists/main/hosts/light.txt" \
  "${BLOCKLIST_DIR}/block-malware.txt" \
  "Blocklist Malware (Hagezi)"

# === 3. Blocklist Domain Dewasa ===
safe_download \
  "https://raw.githubusercontent.com/alexsannikov/adguardhome-filters/refs/heads/master/porn.txt" \
  "${BLOCKLIST_DIR}/adult-domains.txt" \
  "Blocklist Domain Dewasa"

# === 4. Custom Blocklist ===
safe_download \
  "https://raw.githubusercontent.com/desienkz-slp/DNS/refs/heads/main/block_mine.txt" \
  "${BLOCKLIST_DIR}/block-mine.txt" \
  "Custom Blocklist"

log_success "Semua blocklist diperbarui!"
