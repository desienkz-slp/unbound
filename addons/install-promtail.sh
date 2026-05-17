#!/bin/bash
# ============================================================
#  install-promtail.sh — Install Promtail untuk kirim log ke Loki
#  Addon opsional untuk centralized logging
# ============================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/../lib/helpers.sh" 2>/dev/null || {
  log_info()    { echo "[INFO] $1"; }
  log_success() { echo "[✅] $1"; }
  log_error()   { echo "[❌] $1"; }
  log_step()    { echo "=== $1 ==="; }
  safe_download() { curl -fsSL --connect-timeout 10 "$1" -o "$2"; }
}

PROMTAIL_VERSION="3.5.3"
ARCH="amd64"
LOKI_URL="${LOKI_URL:-http://172.18.20.243:3100}"

check_root

# === 1. Download & Install Promtail ===
log_step "Install Promtail v${PROMTAIL_VERSION}"

apt-get install -y unzip >/dev/null 2>&1

cd /tmp
safe_download \
  "https://github.com/grafana/loki/releases/download/v${PROMTAIL_VERSION}/promtail-linux-${ARCH}.zip" \
  "promtail.zip" \
  "Promtail v${PROMTAIL_VERSION}"

unzip -o promtail.zip >/dev/null
rm -f promtail.zip
mv -f "promtail-linux-${ARCH}" /usr/local/bin/promtail
chmod +x /usr/local/bin/promtail

# === 2. Konfigurasi ===
log_step "Setup Konfigurasi Promtail"
mkdir -p /etc/promtail /var/log/promtail

cat > /etc/promtail/promtail.yaml <<EOF
server:
  http_listen_port: 9080
  grpc_listen_port: 0

positions:
  filename: /var/log/promtail/positions-unbound.yaml

clients:
  - url: ${LOKI_URL}/loki/api/v1/push

scrape_configs:
  - job_name: unbound
    static_configs:
      - targets:
          - localhost
        labels:
          job: unbound
          host: $(hostname)
          __path__: /var/log/unbound/*.log
EOF

# === 3. Systemd service ===
log_step "Setup systemd service"
cat > /etc/systemd/system/promtail.service <<EOF
[Unit]
Description=Promtail Service (Unbound Logs)
After=network.target

[Service]
ExecStart=/usr/local/bin/promtail -config.file=/etc/promtail/promtail.yaml
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

# === 4. Aktifkan ===
systemctl daemon-reload
systemctl enable --now promtail

log_success "Promtail v${PROMTAIL_VERSION} aktif! Logs dikirim ke ${LOKI_URL}"
systemctl status promtail --no-pager || true
