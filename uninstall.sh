#!/usr/bin/env bash
# ==============================================================================
# Netora DNS — Platform Uninstaller Script
# GitHub: https://github.com/desienkz-slp/unbound
# ==============================================================================

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

if [[ $EUID -ne 0 ]]; then
  echo -e "${RED}Error: This script must be run as root. (sudo bash $0)${NC}"
  exit 1
fi

echo -e "${RED}${BOLD}"
echo "╔═══════════════════════════════════════════════════════════════════╗"
echo "║          NETORA DNS PLATFORM — UNINSTALLER                        ║"
echo "╚═══════════════════════════════════════════════════════════════════╗"
echo -e "${NC}"

echo -e "${YELLOW}PERINGATAN: Tindakan ini akan menghentikan dan menghapus:${NC}"
echo "  1. Layanan Unbound DNS & Netora Control Plane API"
echo "  2. Systemd timer pembaruan otomatis (netora-dns-updater)"
echo "  3. Berkas konfigurasi di /etc/netora-dns & /etc/unbound"
echo "  4. Web UI NOC Dashboard di /var/www/netora-ui"
echo "  5. Perintah CLI /usr/local/bin/netora"
echo ""

read -p "Apakah Anda yakin ingin menghapus Netora DNS? (Ketik 'HAPUS' untuk konfirmasi): " -r confirm </dev/tty || confirm=""
if [[ "$confirm" != "HAPUS" ]]; then
  echo -e "${CYAN}Uninstaller dibatalkan. Tidak ada berkas yang diubah.${NC}"
  exit 0
fi

echo -e "\n${CYAN}1. Menghentikan Layanan...${NC}"
systemctl stop netora-dns-updater.timer 2>/dev/null || true
systemctl disable netora-dns-updater.timer 2>/dev/null || true
systemctl stop netora-api 2>/dev/null || true
systemctl disable netora-api 2>/dev/null || true
systemctl stop unbound 2>/dev/null || true
systemctl disable unbound 2>/dev/null || true

echo -e "${CYAN}2. Menghapus Systemd Service & Timers...${NC}"
rm -f /etc/systemd/system/netora-api.service
rm -f /etc/systemd/system/netora-dns-updater.service
rm -f /etc/systemd/system/netora-dns-updater.timer
rm -f /etc/systemd/system/unbound.service.d/override.conf
rm -f /etc/sysctl.d/99-netora-dns-tuning.conf
systemctl daemon-reload

echo -e "${CYAN}3. Mengembalikan Resolver Sistem (systemd-resolved)...${NC}"
rm -f /etc/systemd/resolved.conf.d/netora-disable-stub.conf
systemctl restart systemd-resolved 2>/dev/null || true

if [[ -f /etc/resolv.conf.netora.bak ]]; then
  cp /etc/resolv.conf.netora.bak /etc/resolv.conf 2>/dev/null || true
fi

echo -e "${CYAN}4. Menghapus Binaries & CLI Helpers...${NC}"
rm -f /usr/local/bin/netora
rm -f /usr/local/bin/netora-api
rm -f /usr/local/bin/netora-cli
rm -f /usr/local/bin/netora-reset-password
rm -f /usr/local/bin/netora-bench
rm -f /usr/local/bin/netora-mcp
rm -f /usr/local/sbin/netora-dns-update

echo -e "${CYAN}5. Menghapus Direktori Web UI & Netora Base...${NC}"
rm -rf /var/www/netora-ui
rm -rf /etc/netora-dns
rm -rf /var/lib/netora-dns
rm -rf /var/log/netora-dns
rm -rf /opt/netora-dns

echo -e "\n${GREEN}${BOLD}✔ Netora DNS berhasil dihapus dari sistem.${NC}"
echo -e "${YELLOW}Catatan: Paket PostgreSQL dan database 'netora' dibiarkan utuh demi keamanan data. Untuk menghapusnya manual, gunakan: sudo -u postgres psql -c 'DROP DATABASE netora;'${NC}\n"
