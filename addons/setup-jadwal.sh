#!/bin/bash
# ============================================================
#  setup-jadwal.sh — Jadwal otomatis block/unblock konten dewasa
#  Default: Block pukul 06:00, Unblock pukul 21:00
# ============================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/../lib/helpers.sh" 2>/dev/null || {
  log_info()    { echo "[INFO] $1"; }
  log_success() { echo "[✅] $1"; }
  log_step()    { echo "=== $1 ==="; }
  add_crontab_safe() {
    local job="$1"
    (crontab -l 2>/dev/null | grep -vF "$job"; echo "$job") | crontab -
  }
}

# Konfigurasi (bisa di-override via env)
BLOCK_HOUR="${ADULT_BLOCK_START:-6}"
UNBLOCK_HOUR="${ADULT_BLOCK_END:-21}"
CONF="/etc/unbound/unbound.conf"
INCLUDE_LINE='include: "/etc/unbound/blocklist/adult-redirect.conf"'

check_root

log_step "Setup Jadwal Block/Unblock Konten Dewasa"
log_info "Block: pukul ${BLOCK_HOUR}:00 | Unblock: pukul ${UNBLOCK_HOUR}:00"

# === Buat script blokir ===
cat > /usr/local/bin/dns-block-adult.sh <<BLOCKEOF
#!/bin/bash
CONF="${CONF}"
INCLUDE_LINE='${INCLUDE_LINE}'
if ! grep -qF "\$INCLUDE_LINE" "\$CONF"; then
    # Tambahkan sebelum baris remote-control
    sed -i "/^remote-control:/i \$INCLUDE_LINE" "\$CONF"
    systemctl reload unbound 2>/dev/null || systemctl restart unbound
    echo "\$(date): Adult filter ENABLED" >> /var/log/unbound/schedule.log
fi
BLOCKEOF

# === Buat script unblock ===
cat > /usr/local/bin/dns-unblock-adult.sh <<UNBLOCKEOF
#!/bin/bash
CONF="${CONF}"
INCLUDE_LINE='${INCLUDE_LINE}'
if grep -qF "\$INCLUDE_LINE" "\$CONF"; then
    sed -i "\|^\${INCLUDE_LINE}|d" "\$CONF"
    systemctl reload unbound 2>/dev/null || systemctl restart unbound
    echo "\$(date): Adult filter DISABLED" >> /var/log/unbound/schedule.log
fi
UNBLOCKEOF

chmod +x /usr/local/bin/dns-block-adult.sh /usr/local/bin/dns-unblock-adult.sh

# === Setup crontab ===
add_crontab_safe "0 ${BLOCK_HOUR} * * * /usr/local/bin/dns-block-adult.sh"
add_crontab_safe "0 ${UNBLOCK_HOUR} * * * /usr/local/bin/dns-unblock-adult.sh"

log_success "Jadwal aktif! Block ${BLOCK_HOUR}:00 — Unblock ${UNBLOCK_HOUR}:00"
