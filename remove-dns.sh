#!/bin/bash
# ============================================================
#  remove-dns.sh — Uninstaller DNS (Unbound)
#  Membersihkan seluruh file instalasi, logs, config, service
# ============================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/lib/helpers.sh" 2>/dev/null || {
  log_info()    { echo "[INFO] $1"; }
  log_success() { echo "[✅] $1"; }
  log_warn()    { echo "[⚠️] $1"; }
  log_error()   { echo "[❌] $1"; }
  log_step()    { echo "=== $1 ==="; }
}

check_root
show_banner

log_warn "PERINGATAN: Script ini akan MENGHAPUS:"
echo "- Unbound DNS Resolver & konfigurasinya"
echo "- Cloudflared DoH Proxy"
echo "- Unbound Exporter (Prometheus)"
echo "- Konfigurasi crontab terkait DNS"
echo "- Log file Unbound"
echo ""

read -p "Apakah Anda YAKIN ingin melanjutkan? (Ketik 'HAPUS' untuk lanjut): " konfirmasi
if [[ "$konfirmasi" != "HAPUS" ]]; then
  log_info "Uninstaller dibatalkan."
  exit 0
fi

# === 1. Hentikan Services ===
log_step "Menghentikan Services"
systemctl stop unbound 2>/dev/null || true
systemctl disable unbound 2>/dev/null || true

systemctl stop cloudflared-cloudflare 2>/dev/null || true
systemctl disable cloudflared-cloudflare 2>/dev/null || true

systemctl stop cloudflared-google 2>/dev/null || true
systemctl disable cloudflared-google 2>/dev/null || true

systemctl stop unbound_exporter 2>/dev/null || true
systemctl disable unbound_exporter 2>/dev/null || true

# Promtail dibiarkan karena bisa jadi dipakai komponen lain
# Tapi kita hapus config unbound-nya jika ingin bersih

# === 2. Hapus Packages & Files ===
log_step "Menghapus Package & Binary"
apt-get purge --autoremove unbound -y >/dev/null 2>&1 || true

rm -f /usr/local/bin/cloudflared
rm -f /usr/local/bin/dns-block-adult.sh
rm -f /usr/local/bin/dns-unblock-adult.sh
rm -f /usr/local/bin/update-unbound-roothints.sh

# === 3. Hapus Konfigurasi ===
log_step "Menghapus Konfigurasi"
rm -rf /etc/unbound/
rm -rf /var/lib/unbound/
rm -rf /etc/cloudflared/

rm -f /etc/systemd/system/cloudflared-*.service
rm -f /etc/systemd/system/unbound_exporter.service
rm -f /etc/logrotate.d/unbound

# === 4. Bersihkan Crontab ===
log_step "Membersihkan Crontab"
remove_crontab "update-lists.sh"
remove_crontab "dns-block-adult.sh"
remove_crontab "dns-unblock-adult.sh"
remove_crontab "update-unbound-roothints.sh"

# === 5. Restore resolv.conf ===
log_step "Restore systemd-resolved"
systemctl enable systemd-resolved 2>/dev/null || true
systemctl start systemd-resolved 2>/dev/null || true

# Coba cari file backup
backup_file=$(ls -t /etc/resolv.conf.bak.* 2>/dev/null | head -n1 || true)
if [[ -n "$backup_file" ]]; then
  log_info "Mengembalikan resolv.conf dari $backup_file"
  rm -f /etc/resolv.conf
  cp "$backup_file" /etc/resolv.conf
else
  # Fallback default
  rm -f /etc/resolv.conf
  ln -s /run/systemd/resolve/resolv.conf /etc/resolv.conf
fi

# Reload systemd
systemctl daemon-reload

log_success "Unbound dan semua komponen terkait telah berhasil dihapus!"
log_info "Disarankan untuk melakukan reboot server."
