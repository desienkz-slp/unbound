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

# === 1b. Blocklist Iklan Lokal Indonesia (ABPindo) ===
safe_download \
  "https://raw.githubusercontent.com/ABPindo/indonesianadblockrules/master/subscriptions/aghome.txt" \
  "${BLOCKLIST_DIR}/block-abpindo.txt" \
  "Blocklist Iklan Indonesia (ABPindo)"

# === 1c. Blocklist Iklan Netora Master (ads.txt) ===
safe_download \
  "https://raw.githubusercontent.com/desienkz-slp/unbound/main/blocklist/ads.txt" \
  "${BLOCKLIST_DIR}/ads.txt" \
  "Blocklist Iklan Netora Master (ads.txt)"

# === 2. Blocklist Malware (hosts format) ===
safe_download \
  "https://raw.githubusercontent.com/hagezi/dns-blocklists/main/hosts/light.txt" \
  "${BLOCKLIST_DIR}/block-malware.txt" \
  "Blocklist Malware (Hagezi)"

# === 3. Blocklist Domain Dewasa (18+) ===
safe_download \
  "https://raw.githubusercontent.com/desienkz-slp/unbound/main/blocklist/adult-domains.txt" \
  "${BLOCKLIST_DIR}/adult-domains.txt" \
  "Blocklist Domain Dewasa (GitHub)"

# === 4. Blocklist Judi Online & Taruhan ===
safe_download \
  "https://raw.githubusercontent.com/desienkz-slp/unbound/main/blocklist/gambling-domains.txt" \
  "${BLOCKLIST_DIR}/gambling-domains.txt" \
  "Blocklist Judi Online (GitHub)"

# === 5. Custom Blocklist ===
safe_download \
  "https://raw.githubusercontent.com/desienkz-slp/DNS/refs/heads/main/block_mine.txt" \
  "${BLOCKLIST_DIR}/block-mine.txt" \
  "Custom Blocklist"

log_success "Semua blocklist diperbarui dari GitHub!"
