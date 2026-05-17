#!/bin/bash
# ============================================================
#  install-exporter.sh — Install unbound_exporter untuk Prometheus
#  Addon opsional untuk monitoring DNS metrics
# ============================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/../lib/helpers.sh" 2>/dev/null || {
  log_info()    { echo "[INFO] $1"; }
  log_success() { echo "[✅] $1"; }
  log_error()   { echo "[❌] $1"; }
  log_step()    { echo "=== $1 ==="; }
}

GO_VERSION="1.24.5"
GO_TAR="go${GO_VERSION}.linux-amd64.tar.gz"
GO_URL="https://go.dev/dl/${GO_TAR}"
GOBIN="/root/go/bin"

check_root

# === 1. Install Go (jika belum ada) ===
log_step "Install Go ${GO_VERSION}"

if command -v go &>/dev/null; then
  CURRENT_GO=$(go version | awk '{print $3}' | sed 's/go//')
  log_info "Go sudah terinstall: v${CURRENT_GO}"
  if [[ "$CURRENT_GO" != "$GO_VERSION" ]]; then
    log_info "Mengupdate Go ke v${GO_VERSION}..."
    rm -rf /usr/local/go
  else
    log_info "Versi Go sudah sesuai, skip install."
    GO_SKIP=true
  fi
fi

if [[ "${GO_SKIP:-}" != "true" ]]; then
  cd /tmp
  safe_download "$GO_URL" "$GO_TAR" "Go ${GO_VERSION}"
  tar -C /usr/local -xzf "$GO_TAR"
  rm -f "$GO_TAR"

  # Setup PATH
  cat > /etc/profile.d/go.sh <<'GOEOF'
export PATH=$PATH:/usr/local/go/bin
GOEOF
  chmod 644 /etc/profile.d/go.sh
  export PATH=$PATH:/usr/local/go/bin
fi

# Verifikasi Go
go version || { log_error "Go tidak berhasil diinstal."; exit 1; }

# === 2. Install unbound_exporter ===
log_step "Install unbound_exporter"
go install github.com/letsencrypt/unbound_exporter@latest

# === 3. Buat systemd service ===
log_step "Setup systemd service"
cat > /etc/systemd/system/unbound_exporter.service <<EOF
[Unit]
Description=Unbound Exporter for Prometheus
After=network.target unbound.service

[Service]
User=root
ExecStart=${GOBIN}/unbound_exporter \\
  -unbound.host=tcp://127.0.0.1:8953 \\
  -unbound.key="" \\
  -unbound.cert="" \\
  -unbound.ca=""
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

# === 4. Jalankan service ===
systemctl daemon-reload
systemctl enable --now unbound_exporter

log_success "unbound_exporter aktif di http://localhost:9167/metrics"
