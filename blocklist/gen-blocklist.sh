#!/bin/bash
# ============================================================
#  gen-blocklist.sh — Generate semua blocklist conf untuk Unbound
#  Menggabungkan: gen-block.conf.sh + gen-adult-block.sh
#  + whitelist processing
# ============================================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BLOCKLIST_DIR="/etc/unbound/blocklist"

source "${SCRIPT_DIR}/../lib/helpers.sh" 2>/dev/null || {
  log_info()    { echo "[INFO] $1"; }
  log_success() { echo "[✅] $1"; }
  log_warn()    { echo "[⚠️] $1"; }
  log_error()   { echo "[❌] $1"; }
}

# === Konfigurasi ===
ADS_SRC="${BLOCKLIST_DIR}/block-ads.txt"
ABPINDO_SRC="${BLOCKLIST_DIR}/block-abpindo.txt"
ADS_TXT_SRC="${BLOCKLIST_DIR}/ads.txt"
MAL_SRC="${BLOCKLIST_DIR}/block-malware.txt"
MINE_SRC="${BLOCKLIST_DIR}/block-mine.txt"
ADULT_SRC="${BLOCKLIST_DIR}/adult-domains.txt"
WHITELIST_SRC="${BLOCKLIST_DIR}/whitelist.txt"

OUT_ADBLOCK="${BLOCKLIST_DIR}/ad-malware-block.conf"
OUT_ADULT="${BLOCKLIST_DIR}/adult-redirect.conf"
OUT_WHITELIST="${BLOCKLIST_DIR}/whitelist.conf"

TMP="/tmp/unbound-blocklist-$$.tmp"
TMP_WL="/tmp/unbound-whitelist-$$.tmp"

# Ambil REDIRECT_IP dari env atau gunakan default
REDIRECT_IP="${ADULT_REDIRECT_IP:-172.18.20.234}"

# ============================================================
#  BAGIAN 1: Generate Ad + Malware Block
# ============================================================
log_info "🔁 Generating ad-malware blocklist..."

> "$TMP"

# Proses ABP format (iklan)
if [[ -f "$ADS_SRC" ]]; then
  log_info "  Memproses blocklist iklan (ABP format)..."
  grep '^||' "$ADS_SRC" 2>/dev/null \
    | sed -E 's/^\|\|([^\/\^\$\*]+).*/\1/' \
    | grep -Ev '^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$' \
    | grep -Ev '[^a-zA-Z0-9.-]' \
    | sort -u \
    | awk '{print "local-zone: \"" $1 "\" static"}' >> "$TMP"
else
  log_warn "  File $ADS_SRC tidak ditemukan, skip."
fi

# Proses ABPindo blocklist (ABP format)
if [[ -f "$ABPINDO_SRC" ]]; then
  log_info "  Memproses blocklist ABPindo (Indonesia)..."
  grep '^||' "$ABPINDO_SRC" 2>/dev/null \
    | sed -E 's/^\|\|([^\/\^\$\*]+).*/\1/' \
    | grep -Ev '^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$' \
    | grep -Ev '[^a-zA-Z0-9.-]' \
    | sort -u \
    | awk '{print "local-zone: \"" $1 "\" static"}' >> "$TMP"
fi

# Proses ads.txt master (plain domain list)
if [[ -f "$ADS_TXT_SRC" ]]; then
  log_info "  Memproses blocklist ads.txt master..."
  grep -Ev '^[#|0-9]' "$ADS_TXT_SRC" 2>/dev/null \
    | grep -Ev '[^a-zA-Z0-9.-]' \
    | sort -u \
    | awk '{print "local-zone: \"" $1 "\" static"}' >> "$TMP"
fi

# Proses custom blocklist (ABP format)
if [[ -f "$MINE_SRC" ]]; then
  log_info "  Memproses custom blocklist..."
  grep '^||' "$MINE_SRC" 2>/dev/null \
    | sed -E 's/^\|\|([^\/\^\$\*]+).*/\1/' \
    | grep -Ev '^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$' \
    | grep -Ev '[^a-zA-Z0-9.-]' \
    | sort -u \
    | awk '{print "local-zone: \"" $1 "\" static"}' >> "$TMP"
else
  log_warn "  File $MINE_SRC tidak ditemukan, skip."
fi

# Proses hosts format (malware)
if [[ -f "$MAL_SRC" ]]; then
  log_info "  Memproses blocklist malware (hosts format)..."
  grep -E '^(0\.0\.0\.0|127\.0\.0\.1)' "$MAL_SRC" 2>/dev/null \
    | awk '{print $2}' \
    | grep -Ev '^(0\.0\.0\.0|127\.0\.0\.1|localhost)$' \
    | grep -Ev '^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$' \
    | grep -Ev '[^a-zA-Z0-9.-]' \
    | sort -u \
    | awk '{print "local-zone: \"" $1 "\" static"}' >> "$TMP"
else
  log_warn "  File $MAL_SRC tidak ditemukan, skip."
fi

# ============================================================
#  BAGIAN 2: Proses Whitelist (exclude dari blocklist)
# ============================================================
if [[ -f "$WHITELIST_SRC" ]] && [[ -s "$WHITELIST_SRC" ]]; then
  log_info "  Memproses whitelist..."
  > "$TMP_WL"

  # Extract domain dari whitelist (format @@||domain^)
  grep '^@@||' "$WHITELIST_SRC" 2>/dev/null \
    | sed -E 's/^@@\|\|([^\/\^\$\*]+).*/\1/' \
    | sort -u > "$TMP_WL"

  if [[ -s "$TMP_WL" ]]; then
    # Hapus domain whitelist dari blocklist
    local_count_before=$(wc -l < "$TMP")
    while IFS= read -r wl_domain; do
      sed -i "/\"${wl_domain}\"/d" "$TMP"
    done < "$TMP_WL"
    local_count_after=$(wc -l < "$TMP")
    log_info "  Whitelist: $((local_count_before - local_count_after)) domain diexclude"

    # Generate whitelist.conf (always allow)
    > "$OUT_WHITELIST"
    while IFS= read -r wl_domain; do
      echo "local-zone: \"${wl_domain}\" transparent" >> "$OUT_WHITELIST"
    done < "$TMP_WL"
    log_success "  Whitelist conf: $OUT_WHITELIST"
  fi

  rm -f "$TMP_WL"
else
  # Kosongkan whitelist conf
  > "$OUT_WHITELIST" 2>/dev/null || true
fi

# Finalize ad-malware blocklist
sort -u "$TMP" > "$OUT_ADBLOCK"
rm -f "$TMP"

local_count=$(wc -l < "$OUT_ADBLOCK")
log_success "Ad-malware blocklist: ${local_count} domain diblokir → $OUT_ADBLOCK"

# ============================================================
#  BAGIAN 3: Generate Adult Redirect
# ============================================================
if [[ -f "$ADULT_SRC" ]] && [[ -s "$ADULT_SRC" ]]; then
  log_info "🔁 Generating adult redirect (→ $REDIRECT_IP)..."
  > "$OUT_ADULT"

  while IFS= read -r domain || [[ -n "$domain" ]]; do
    # Skip empty lines dan comments
    [[ -z "$domain" ]] && continue
    [[ "$domain" =~ ^# ]] && continue
    # Bersihkan whitespace
    domain=$(echo "$domain" | tr -d '[:space:]')
    [[ -z "$domain" ]] && continue

    echo "local-data: \"$domain A $REDIRECT_IP\"" >> "$OUT_ADULT"
    echo "local-data: \"www.$domain A $REDIRECT_IP\"" >> "$OUT_ADULT"
  done < "$ADULT_SRC"

  local_adult_count=$(grep -c "local-data" "$OUT_ADULT" 2>/dev/null || echo 0)
  log_success "Adult redirect: $((local_adult_count / 2)) domain → $OUT_ADULT"
else
  > "$OUT_ADULT" 2>/dev/null || true
  log_warn "File $ADULT_SRC tidak ditemukan atau kosong, adult redirect dilewati."
fi

# ============================================================
#  BAGIAN 4: Validasi Config
# ============================================================
log_info "🔍 Mengecek konfigurasi Unbound..."
if command -v unbound-checkconf &>/dev/null; then
  if unbound-checkconf 2>/dev/null; then
    log_success "Konfigurasi Unbound valid!"
  else
    log_error "Konfigurasi Unbound error! Cek output di atas."
    log_info "Coba: unbound-checkconf untuk detail"
    exit 1
  fi
else
  log_warn "unbound-checkconf tidak ditemukan, skip validasi."
fi

log_success "Semua blocklist berhasil di-generate!"
