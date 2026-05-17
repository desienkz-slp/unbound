#!/bin/bash
# ============================================================
#  install-dns.sh — Installer Utama DNS (Unbound)
#  Anti-Bengong Edition (Menggabungkan 3 script instalasi lama)
# ============================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/lib/helpers.sh" 2>/dev/null || { echo "ERROR: lib/helpers.sh tidak ditemukan!"; exit 1; }

check_root
check_os
show_banner

# === Variabel Default ===
DNS_LISTEN_IP="0.0.0.0"
DNS_LISTEN_PORT="53"
SUBNETS="192.168.0.0/16 172.16.0.0/12 10.0.0.0/8"
ADULT_REDIRECT_IP="172.18.20.234"
ENABLE_SAFESEARCH="yes"

# === 1. Pre-flight Checks ===
if ! check_port_free $DNS_LISTEN_PORT; then
  log_error "Instalasi tidak dapat dilanjutkan karena port $DNS_LISTEN_PORT dipakai."
  exit 1
fi

# === 2. Konfigurasi Interaktif ===
log_step "Konfigurasi DNS Server"

echo -e "Pilih mode operasi DNS:"
echo -e "  [1] ${CYAN}DoH Forwarder${NC} (via Cloudflare/Google) - Terbaik untuk bypass sensor"
echo -e "  [2] ${CYAN}Recursive Resolver${NC} (via Root Hints) - Lebih privat, independen"
read -p "Pilihan Anda [1/2] (Default: 1): " MODE_CHOICE
MODE_CHOICE=${MODE_CHOICE:-1}

# === 3. Install Packages ===
log_step "Install Packages"
apt-get update
apt-get install -y unbound curl wget dnsutils logrotate apparmor-utils

# === 4. Setup Mode Operasi ===
if [[ "$MODE_CHOICE" == "1" ]]; then
  log_info "Mode Terpilih: DoH Forwarder"
  
  # Install Cloudflared
  if ! command -v cloudflared &>/dev/null; then
    log_info "Menginstall cloudflared..."
    wget -qO cloudflared.deb https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64.deb
    dpkg -i cloudflared.deb
    rm -f cloudflared.deb
  fi
  
  mkdir -p /etc/cloudflared
  
  cat > /etc/cloudflared/cloudflare-doh.yml <<EOF
proxy-dns: true
proxy-dns-port: 5053
upstream:
 - https://1.1.1.1/dns-query
 - https://1.0.0.1/dns-query
EOF

  cat > /etc/cloudflared/google-doh.yml <<EOF
proxy-dns: true
proxy-dns-port: 5353
upstream:
 - https://dns.google/dns-query
EOF

  cat > /etc/systemd/system/cloudflared-cloudflare.service <<EOF
[Unit]
Description=cloudflared DoH - Cloudflare
After=network.target

[Service]
ExecStart=/usr/local/bin/cloudflared --config /etc/cloudflared/cloudflare-doh.yml
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

  cat > /etc/systemd/system/cloudflared-google.service <<EOF
[Unit]
Description=cloudflared DoH - Google
After=network.target

[Service]
ExecStart=/usr/local/bin/cloudflared --config /etc/cloudflared/google-doh.yml
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

  systemctl daemon-reload
  systemctl enable --now cloudflared-cloudflare.service
  systemctl enable --now cloudflared-google.service
  
  RECURSIVE_CONFIG=""
  FORWARD_CONFIG="forward-zone:
  name: \".\"
  forward-addr: 127.0.0.1@5053
  forward-addr: 127.0.0.1@5353"

else
  log_info "Mode Terpilih: Recursive Resolver"
  
  log_info "Mendownload root.hints terbaru..."
  wget -qO /var/lib/unbound/root.hints https://www.internic.net/domain/named.cache
  
  log_info "Setup DNSSEC anchor..."
  unbound-anchor -a /var/lib/unbound/root.key || true
  
  RECURSIVE_CONFIG="  root-hints: \"/var/lib/unbound/root.hints\"
  auto-trust-anchor-file: \"/var/lib/unbound/root.key\"
  val-clean-additional: yes
  val-permissive-mode: no
  harden-dnssec-stripped: yes
  ignore-cd-flag: no
  module-config: \"validator iterator\""
  
  FORWARD_CONFIG=""
fi

# === 5. Setup Config Template ===
log_step "Generate Unbound Configuration"

# Anti-bengong tuning
NUM_THREADS=$(detect_cpu_cores)
calc_cache_sizes
log_info "Tuning: Threads=$NUM_THREADS"

# Build ACL
ACL_BLOCK=""
for sub in $SUBNETS; do
  ACL_BLOCK+="  access-control: $sub allow\n"
done

# SafeSearch
if [[ "$ENABLE_SAFESEARCH" == "yes" ]]; then
  SS_CONFIG="  local-zone: \"www.google.com.\" redirect
  local-data: \"www.google.com. A 216.239.38.120\"
  local-zone: \"www.bing.com.\" redirect
  local-data: \"www.bing.com. A 204.79.197.220\""
else
  SS_CONFIG=""
fi

# Parse Template
CONF_DEST="/etc/unbound/unbound.conf"
backup_file "$CONF_DEST"

cp "${SCRIPT_DIR}/conf/unbound-base.conf" "$CONF_DEST"

sed -i "s|{{DNS_LISTEN_IP}}|$DNS_LISTEN_IP|g" "$CONF_DEST"
sed -i "s|{{DNS_LISTEN_PORT}}|$DNS_LISTEN_PORT|g" "$CONF_DEST"
sed -i "s|{{ACL_SUBNETS}}|$ACL_BLOCK|g" "$CONF_DEST"
sed -i "s|{{NUM_THREADS}}|$NUM_THREADS|g" "$CONF_DEST"
sed -i "s|{{MSG_CACHE}}|$MSG_CACHE|g" "$CONF_DEST"
sed -i "s|{{RRSET_CACHE}}|$RRSET_CACHE|g" "$CONF_DEST"
sed -i "s|{{KEY_CACHE}}|$KEY_CACHE|g" "$CONF_DEST"
sed -i "s|{{NEG_CACHE}}|$NEG_CACHE|g" "$CONF_DEST"
sed -i "s|{{RECURSIVE_CONFIG}}|$RECURSIVE_CONFIG|g" "$CONF_DEST"
sed -i "s|{{SAFESEARCH_CONFIG}}|$SS_CONFIG|g" "$CONF_DEST"
sed -i "s|{{FORWARD_CONFIG}}|$FORWARD_CONFIG|g" "$CONF_DEST"

# Setup bypass config (copy if exists)
mkdir -p /etc/unbound/unbound.conf.d /etc/unbound/conf
if [[ -f "${SCRIPT_DIR}/domain-insecure.conf" ]]; then
  cp "${SCRIPT_DIR}/domain-insecure.conf" /etc/unbound/unbound.conf.d/
fi

if [[ -f "${SCRIPT_DIR}/conf/local-records.conf" ]]; then
  cp "${SCRIPT_DIR}/conf/local-records.conf" /etc/unbound/conf/
fi

# === 6. Blocklists & Whitelists ===
log_step "Setup Blocklists"

export ADULT_REDIRECT_IP
bash "${SCRIPT_DIR}/blocklist/update-lists.sh"
bash "${SCRIPT_DIR}/blocklist/gen-blocklist.sh"

# === 7. AppArmor, Log & Control ===
log_step "Setup Logging & Security"

# Remote control certs
unbound-control-setup >/dev/null 2>&1 || true

# Logs
mkdir -p /var/log/unbound
touch /var/log/unbound/unbound.log
chown unbound:unbound /var/log/unbound/unbound.log

# AppArmor
APPARMOR_LOCAL="/etc/apparmor.d/local/usr.sbin.unbound"
mkdir -p "$(dirname "$APPARMOR_LOCAL")"

if ! grep -q "/var/log/unbound/" "$APPARMOR_LOCAL" 2>/dev/null; then
  cat >> "$APPARMOR_LOCAL" <<EOF
/var/log/unbound/ rw,
/var/log/unbound/** rwk,
EOF
  log_info "Rule AppArmor ditambahkan."
fi
apparmor_parser -r /etc/apparmor.d/usr.sbin.unbound 2>/dev/null || true

# Logrotate
cp "${SCRIPT_DIR}/conf/logrotate-unbound.conf" "/etc/logrotate.d/unbound"

# === 8. Disable systemd-resolved ===
log_step "Ganti systemd-resolved ke Unbound"

backup_file "/etc/resolv.conf"
systemctl stop systemd-resolved 2>/dev/null || true
systemctl disable systemd-resolved 2>/dev/null || true

rm -f /etc/resolv.conf
echo "nameserver 127.0.0.1" > /etc/resolv.conf

# === 8.5. Extreme Performance Tuning (OS Level) ===
log_step "Tuning OS untuk Jutaan RPS (Extreme Mode)"

# Bypass System limits untuk jumlah koneksi (file descriptor)
mkdir -p /etc/systemd/system/unbound.service.d/
cat > /etc/systemd/system/unbound.service.d/override.conf <<EOF
[Service]
LimitNOFILE=1048576
LimitNPROC=1048576
EOF

# Network Sysctls untuk DNS high throughput
cat > /etc/sysctl.d/99-unbound-tuning.conf <<EOF
# Maximize network buffer space
net.core.rmem_max = 16777216
net.core.wmem_max = 16777216
net.core.rmem_default = 16777216
net.core.wmem_default = 16777216
net.core.optmem_max = 16777216

# Maximize UDP buffer sizes
net.ipv4.udp_mem = 16777216 16777216 16777216
net.ipv4.udp_rmem_min = 131072
net.ipv4.udp_wmem_min = 131072

# Connection tracking
net.netfilter.nf_conntrack_max = 2000000

# Local port range
net.ipv4.ip_local_port_range = 1024 65535
EOF

sysctl --system >/dev/null 2>&1 || true

# === 9. Start Services ===
log_step "Starting Unbound"

systemctl daemon-reload
systemctl enable unbound
systemctl restart unbound

# === 10. Health Check ===
if health_check_dns "127.0.0.1"; then
  log_success "Unbound berhasil diinstal dan berjalan dengan normal!"
else
  log_error "Unbound gagal melakukan DNS resolving! Cek log dengan: journalctl -u unbound"
fi

# === 11. Crontabs ===
log_step "Setup Crontabs"

# Update blocklists jam 3 pagi
add_crontab_safe "0 3 * * * export ADULT_REDIRECT_IP=${ADULT_REDIRECT_IP}; ${SCRIPT_DIR}/blocklist/update-lists.sh && ${SCRIPT_DIR}/blocklist/gen-blocklist.sh && unbound-control reload"

# Update root.hints jam 2 pagi tiap 3 hari (hanya jika recursive)
if [[ "$MODE_CHOICE" == "2" ]]; then
  cat > /usr/local/bin/update-unbound-roothints.sh <<EOF
#!/bin/bash
wget -qO /var/lib/unbound/root.hints https://www.internic.net/domain/named.cache
unbound-control reload
EOF
  chmod +x /usr/local/bin/update-unbound-roothints.sh
  add_crontab_safe "0 2 */3 * * /usr/local/bin/update-unbound-roothints.sh"
fi

# === 12. Addons (Opsional) ===
echo -e "\nApakah Anda ingin menginstall komponen tambahan?"
read -p "Install unbound_exporter (Prometheus metrics)? [y/N]: " ans_exp
if [[ "${ans_exp,,}" == "y" ]]; then
  bash "${SCRIPT_DIR}/addons/install-exporter.sh"
fi

read -p "Install Promtail (Kirim log ke Loki)? [y/N]: " ans_prom
if [[ "${ans_prom,,}" == "y" ]]; then
  bash "${SCRIPT_DIR}/addons/install-promtail.sh"
fi

read -p "Setup otomatis Jadwal Block/Unblock Dewasa? [y/N]: " ans_jadwal
if [[ "${ans_jadwal,,}" == "y" ]]; then
  bash "${SCRIPT_DIR}/addons/setup-jadwal.sh"
fi

log_step "Instalasi Selesai"
echo -e "DNS Anti-Bengong Aktif!"
echo -e "Untuk cek statistik (jika belum ada exporter): ${CYAN}unbound-control stats${NC}"
echo -e "Untuk tail logs: ${CYAN}tail -f /var/log/unbound/unbound.log${NC}"
