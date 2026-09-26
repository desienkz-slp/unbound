#!/usr/bin/env bash
# ==============================================================================
# Netora DNS — Enterprise Carrier-Grade ISP Unbound Platform
# Autonomous One-Line Installer & Bootstrap Script
# GitHub: https://github.com/desienkz-slp/unbound
# ==============================================================================

set -euo pipefail

# Visual Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

# Installation Defaults
INSTALL_DIR="/opt/netora-dns"
REPO_DIR="${INSTALL_DIR}/repository"
REPO_URL="https://github.com/desienkz-slp/unbound.git"
GIT_BRANCH="main"

DNS_LISTEN_PORT="53"
WEB_LISTEN_PORT="8080"
NODE_ID="$(hostname -s 2>/dev/null || echo "dns-node-01")"
RESOLVER_MODE="recursive"
ADMIN_USER="admin"
ADMIN_PASS="admin123"
DB_USER="netora"
DB_PASS="netora_dev_password"
DB_NAME="netora"
DB_PORT="5432"
CLIENT_SUBNETS="10.0.0.0/8 172.16.0.0/12 192.168.0.0/16 127.0.0.0/8"
CHANNEL="development"
ENABLE_AUTO_UPDATE="true"
SILENT_MODE="false"
GITHUB_TOKEN="${GITHUB_TOKEN:-}"

# ------------------------------------------------------------------------------
# Helpers & Banner
# ------------------------------------------------------------------------------
log_info()    { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[✔]${NC} $1"; }
log_warn()    { echo -e "${YELLOW}[!]${NC} $1"; }
log_error()   { echo -e "${RED}[✖]${NC} $1"; }
log_step()    { echo -e "\n${CYAN}${BOLD}=== $1 ===${NC}"; }

show_banner() {
  echo -e "${CYAN}${BOLD}"
  echo "╔═══════════════════════════════════════════════════════════════════╗"
  echo "║   _   _      _                    ____  _   _ ____                ║"
  echo "║  | \ | | ___| |_ ___  _ __ __ _  |  _ \| \ | / ___|               ║"
  echo "║  |  \| |/ _ \ __/ _ \| '__/ _\` | | | | |  \| \___ \               ║"
  echo "║  | |\  |  __/ || (_) | | | (_| | | |_| | |\  |___) |              ║"
  echo "║  |_| \_|\___|\__\___/|_|  \__,_| |____/|_| \_|____/               ║"
  echo "║                                                                   ║"
  echo "║   Netora DNS — Autonomous Carrier-Grade ISP Unbound Platform      ║"
  echo "║   Anti-Bengong Engine • Real-Time NOC UI • Autonomous CI/CD       ║"
  echo "╚═══════════════════════════════════════════════════════════════════╝"
  echo -e "${NC}"
}

print_help() {
  cat <<EOF
Netora DNS — Enterprise Installer

Usage:
  curl -sSL https://raw.githubusercontent.com/desienkz-slp/unbound/main/install.sh | sudo bash
  sudo ./install.sh [options]

Options:
  -y, --silent              Non-interactive installation using defaults
  --mode <mode>             Resolution mode: 'recursive' (default) or 'forwarder'
  --node-id <name>          Cluster Node Identifier (default: hostname)
  --web-port <port>         NOC Web UI & REST API port (default: 8080)
  --dns-port <port>         DNS listener UDP/TCP port (default: 53)
  --admin-pass <pass>       Default operator password for Web UI (default: admin123)
  --db-pass <pass>          PostgreSQL password for netora role
  --subnets "<cidrs>"       Allowed client CIDR subnets (space-separated)
  --channel <channel>       Update channel: 'production' or 'development' (default)
  --github-token <token>    GitHub Personal Access Token for private repo access
  --no-auto-update          Disable autonomous background updater timer
  -h, --help                Show this help message and exit

Environment Variables:
  GITHUB_TOKEN              GitHub token for private repository
EOF
  exit 0
}

# ------------------------------------------------------------------------------
# Argument Parsing
# ------------------------------------------------------------------------------
while [[ $# -gt 0 ]]; do
  case "$1" in
    -y|--silent)          SILENT_MODE="true"; shift ;;
    --mode)               RESOLVER_MODE="$2"; shift 2 ;;
    --node-id)            NODE_ID="$2"; shift 2 ;;
    --web-port)           WEB_LISTEN_PORT="$2"; shift 2 ;;
    --dns-port)           DNS_LISTEN_PORT="$2"; shift 2 ;;
    --admin-pass)         ADMIN_PASS="$2"; shift 2 ;;
    --db-pass)            DB_PASS="$2"; shift 2 ;;
    --subnets)            CLIENT_SUBNETS="$2"; shift 2 ;;
    --channel)            CHANNEL="$2"; shift 2 ;;
    --github-token)       GITHUB_TOKEN="$2"; shift 2 ;;
    --no-auto-update)     ENABLE_AUTO_UPDATE="false"; shift ;;
    -h|--help)            print_help ;;
    *)                    log_warn "Unknown parameter: $1"; shift ;;
  esac
done

# ------------------------------------------------------------------------------
# Pre-Flight Verifications
# ------------------------------------------------------------------------------
show_banner

if [[ $EUID -ne 0 ]]; then
  log_error "This script must be run as root. Please run with: sudo bash $0"
  exit 1
fi

if [[ ! -f /etc/os-release ]]; then
  log_error "Cannot detect operating system (/etc/os-release missing)."
  exit 1
fi

# shellcheck disable=SC1091
source /etc/os-release
if [[ "$ID" != "ubuntu" && "$ID" != "debian" ]]; then
  log_warn "Detected OS: $PRETTY_NAME. Netora DNS is engineered and tested for Ubuntu 20.04/22.04/24.04 and Debian 11/12."
  if [[ "$SILENT_MODE" != "true" ]]; then
    read -p "Continue anyway? (y/N): " -r confirm </dev/tty || confirm="n"
    [[ "${confirm,,}" != "y" ]] && exit 1
  fi
fi

log_success "Target Environment: $PRETTY_NAME ($(uname -m))"

# Check RAM
RAM_TOTAL_MB="$(free -m | awk '/^Mem:/{print $2}')"
if [[ "$RAM_TOTAL_MB" -lt 900 ]]; then
  log_warn "Detected RAM: ${RAM_TOTAL_MB}MB. Recommended minimum is 1024MB for full NOC stack."
else
  log_info "Detected RAM: ${RAM_TOTAL_MB}MB | CPU Cores: $(nproc)"
fi

# ------------------------------------------------------------------------------
# Interactive Setup (if not silent)
# ------------------------------------------------------------------------------
if [[ "$SILENT_MODE" != "true" && -t 0 ]] || [[ "$SILENT_MODE" != "true" && -e /dev/tty ]]; then
  log_step "Configuration Options"
  
  echo -e "Pilih mode operasi DNS Resolver:"
  echo -e "  [1] ${CYAN}Recursive Resolver${NC} (via IANA Root Hints + DNSSEC) — Standar ISP, Tercepat & Independen"
  echo -e "  [2] ${CYAN}DoH Forwarder${NC} (via Cloudflare / Google) — Forwarding terenkripsi"
  read -p "Pilihan Anda [1/2] (Default: 1): " -r mode_sel </dev/tty || mode_sel="1"
  if [[ "$mode_sel" == "2" ]]; then
    RESOLVER_MODE="forwarder"
  else
    RESOLVER_MODE="recursive"
  fi

  read -p "Node Identifier [${NODE_ID}]: " -r node_input </dev/tty || node_input=""
  [[ -n "$node_input" ]] && NODE_ID="$node_input"

  read -p "NOC Web UI Port [${WEB_LISTEN_PORT}]: " -r web_port_input </dev/tty || web_port_input=""
  [[ -n "$web_port_input" ]] && WEB_LISTEN_PORT="$web_port_input"

  read -p "Operator Admin Password [${ADMIN_PASS}]: " -r pass_input </dev/tty || pass_input=""
  [[ -n "$pass_input" ]] && ADMIN_PASS="$pass_input"

  if [[ -z "$GITHUB_TOKEN" ]]; then
    read -p "GitHub Token (Optional, for private repo auto-updates): " -r token_input </dev/tty || token_input=""
    [[ -n "$token_input" ]] && GITHUB_TOKEN="$token_input"
  fi
fi

log_info "Configured Mode:    ${RESOLVER_MODE}"
log_info "Node ID:            ${NODE_ID}"
log_info "Web UI Port:        ${WEB_LISTEN_PORT}"
log_info "DNS Listen Port:    ${DNS_LISTEN_PORT}"

# ------------------------------------------------------------------------------
# Resolve Port 53 Conflicts (systemd-resolved)
# ------------------------------------------------------------------------------
log_step "Checking Port 53 Conflicts"
if ss -tulnp | grep -q ":${DNS_LISTEN_PORT} "; then
  occupant="$(ss -tulnp | grep ":${DNS_LISTEN_PORT} " | awk '{print $NF}' | head -n1)"
  log_warn "Port ${DNS_LISTEN_PORT} is currently in use by: ${occupant}"

  if echo "$occupant" | grep -q "systemd-resolve"; then
    log_info "Safely disabling systemd-resolved stub listener..."
    mkdir -p /etc/systemd/resolved.conf.d/
    cat > /etc/systemd/resolved.conf.d/netora-disable-stub.conf <<EOF
[Resolve]
DNSStubListener=no
EOF
    systemctl restart systemd-resolved || true
    
    # Ensure local resolv.conf points to loopback
    if [[ -f /etc/resolv.conf ]]; then
      cp /etc/resolv.conf /etc/resolv.conf.netora.bak 2>/dev/null || true
      rm -f /etc/resolv.conf
      echo "nameserver 127.0.0.1" > /etc/resolv.conf
      echo "nameserver 1.1.1.1" >> /etc/resolv.conf
    fi
    log_success "systemd-resolved stub listener disabled."
  else
    log_warn "Port ${DNS_LISTEN_PORT} occupied by non-systemd service. Continuing with installation..."
  fi
fi

# ------------------------------------------------------------------------------
# Install Base Packages
# ------------------------------------------------------------------------------
log_step "Installing Required Packages"
export DEBIAN_FRONTEND=noninteractive
apt-get update -y -q
apt-get install -y -q \
  unbound \
  dnsutils \
  curl \
  wget \
  git \
  jq \
  logrotate \
  apparmor-utils \
  sudo \
  bc \
  net-tools \
  postgresql \
  postgresql-contrib

log_success "Base system packages installed."

# ------------------------------------------------------------------------------
# Extreme OS & Kernel Tuning (Anti-Bengong & Carrier Scale)
# ------------------------------------------------------------------------------
log_step "Applying High-Throughput OS Kernel Tuning"

cat > /etc/sysctl.d/99-netora-dns-tuning.conf <<EOF
# Netora DNS Extreme Throughput Kernel Parameters
net.core.rmem_max = 16777216
net.core.wmem_max = 16777216
net.core.rmem_default = 16777216
net.core.wmem_default = 16777216
net.core.optmem_max = 16777216

# UDP memory buffers
net.ipv4.udp_mem = 16777216 16777216 16777216
net.ipv4.udp_rmem_min = 131072
net.ipv4.udp_wmem_min = 131072

# Connection tracking & Ephemeral port range
net.netfilter.nf_conntrack_max = 2000000
net.ipv4.ip_local_port_range = 1024 65535
EOF

sysctl --system >/dev/null 2>&1 || true

# Systemd Limit Overrides for Unbound
mkdir -p /etc/systemd/system/unbound.service.d/
cat > /etc/systemd/system/unbound.service.d/override.conf <<EOF
[Service]
LimitNOFILE=1048576
LimitNPROC=1048576
EOF

log_success "OS kernel and systemd limits tuned for carrier loads."

# ------------------------------------------------------------------------------
# Setup Netora Working Directories & Source Checkout
# ------------------------------------------------------------------------------
log_step "Setting Up Netora Workspace Hierarchy"

mkdir -p "$INSTALL_DIR" \
  "${INSTALL_DIR}/releases" \
  "${INSTALL_DIR}/backups" \
  "${INSTALL_DIR}/staging" \
  /etc/netora-dns \
  /var/lib/netora-dns \
  /var/log/netora-dns \
  /var/www/netora-ui

chmod 750 "$INSTALL_DIR" /etc/netora-dns /var/lib/netora-dns /var/log/netora-dns

# Determine Source Location: Local running directory or Git Clone
SCRIPT_SOURCE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" 2>/dev/null && pwd || echo "")"

if [[ -f "${SCRIPT_SOURCE_DIR}/conf/unbound-base.conf" && -d "${SCRIPT_SOURCE_DIR}/pkg" ]]; then
  log_info "Detected local Netora source repository at: ${SCRIPT_SOURCE_DIR}"
  mkdir -p "$REPO_DIR"
  cp -r "${SCRIPT_SOURCE_DIR}/"* "$REPO_DIR/" 2>/dev/null || true
else
  log_info "Cloning Netora repository from GitHub (${GIT_BRANCH})..."
  AUTH_CLONE_URL="$REPO_URL"
  if [[ -n "$GITHUB_TOKEN" ]]; then
    AUTH_CLONE_URL="https://${GITHUB_TOKEN}@github.com/desienkz-slp/unbound.git"
  fi

  rm -rf "$REPO_DIR"
  if ! git clone --depth=1 --branch "$GIT_BRANCH" "$AUTH_CLONE_URL" "$REPO_DIR" 2>/dev/null; then
    log_error "Failed to clone repository from GitHub. If this is a private repo, provide --github-token <TOKEN>."
    exit 1
  fi
  log_success "Repository cloned to: ${REPO_DIR}"
fi

# ------------------------------------------------------------------------------
# Configure Unbound Data Plane
# ------------------------------------------------------------------------------
log_step "Configuring Unbound DNS Resolver"

# Setup Remote Control Certificates
unbound-control-setup >/dev/null 2>&1 || true

# Setup Root Hints & Trust Anchor
mkdir -p /var/lib/unbound
if [[ ! -f /var/lib/unbound/root.hints ]] || [[ ! -s /var/lib/unbound/root.hints ]]; then
  log_info "Downloading latest IANA root.hints..."
  wget -qO /var/lib/unbound/root.hints https://www.internic.net/domain/named.cache || true
fi

unbound-anchor -a /var/lib/unbound/root.key || true

# Calculate Optimal Cache Sizing based on RAM
CORES="$(nproc 2>/dev/null || echo 1)"
THREADS=1
while [[ $THREADS -lt $CORES ]]; do THREADS=$((THREADS * 2)); done
[[ $THREADS -gt $CORES ]] && THREADS=$((THREADS / 2))
[[ $THREADS -lt 1 ]] && THREADS=1

if [[ $RAM_TOTAL_MB -le 1024 ]]; then
  MSG_CACHE="64m"; RRSET_CACHE="128m"; KEY_CACHE="32m"; NEG_CACHE="32m"
elif [[ $RAM_TOTAL_MB -le 2048 ]]; then
  MSG_CACHE="128m"; RRSET_CACHE="256m"; KEY_CACHE="64m"; NEG_CACHE="64m"
elif [[ $RAM_TOTAL_MB -le 4096 ]]; then
  MSG_CACHE="256m"; RRSET_CACHE="512m"; KEY_CACHE="128m"; NEG_CACHE="128m"
else
  MSG_CACHE="512m"; RRSET_CACHE="1024m"; KEY_CACHE="256m"; NEG_CACHE="256m"
fi

# Build ACL Directives
ACL_DIRECTIVES=""
for sub in $CLIENT_SUBNETS; do
  ACL_DIRECTIVES+="  access-control: ${sub} allow"$'\n'
done

# Mode Specific Config
FORWARD_DIRECTIVE=""
ROOT_DIRECTIVE=""
if [[ "$RESOLVER_MODE" == "forwarder" ]]; then
  FORWARD_DIRECTIVE="forward-zone:
  name: \".\"
  forward-addr: 1.1.1.1
  forward-addr: 1.0.0.1
  forward-addr: 8.8.8.8"
else
  ROOT_DIRECTIVE="  root-hints: \"/var/lib/unbound/root.hints\"
  auto-trust-anchor-file: \"/var/lib/unbound/root.key\"
  val-clean-additional: yes
  val-permissive-mode: no
  harden-dnssec-stripped: yes
  ignore-cd-flag: no
  module-config: \"validator iterator\""
fi

mkdir -p /etc/unbound/conf.d
cat > /etc/unbound/unbound.conf <<EOF
# Netora DNS — Autonomous Carrier-Grade ISP Unbound Configuration
# Generated at: $(date -u +"%Y-%m-%dT%H:%M:%SZ")

server:
  verbosity: 1
  num-threads: ${THREADS}
  interface: 0.0.0.0
  interface: ::0
  port: ${DNS_LISTEN_PORT}

  # Security & Access Control
  hide-identity: yes
  hide-version: yes
  harden-glue: yes
  harden-dnssec-stripped: yes
  use-caps-for-id: no

  # CRITICAL INVARIANT: Disable strict QNAME minimisation for ISP CDN compatibility
  qname-minimisation: no

  # Buffer Sizing
  so-rcvbuf: 8m
  so-sndbuf: 8m
  msg-buffer-size: 65552

  # Cache Sizing
  msg-cache-size: ${MSG_CACHE}
  rrset-cache-size: ${RRSET_CACHE}
  key-cache-size: ${KEY_CACHE}
  neg-cache-size: ${NEG_CACHE}

  # Prefetch & Optimistic Caching (Serve Expired)
  prefetch: yes
  prefetch-key: yes
  serve-expired: yes
  serve-expired-ttl: 86400
  serve-expired-client-timeout: 1800

  # Access Control List
${ACL_DIRECTIVES}

  # Mode Directives
${ROOT_DIRECTIVE}

${FORWARD_DIRECTIVE}

remote-control:
  control-enable: yes
  control-interface: 127.0.0.1
  control-port: 8953
  server-key-file: "/etc/unbound/unbound_server.key"
  server-cert-file: "/etc/unbound/unbound_server.pem"
  control-key-file: "/etc/unbound/unbound_control.key"
  control-cert-file: "/etc/unbound/unbound_control.pem"

include: "/etc/unbound/conf.d/*.conf"
EOF

# Pre-flight syntax validation
if ! unbound-checkconf /etc/unbound/unbound.conf; then
  log_error "unbound-checkconf syntax validation failed! Please inspect /etc/unbound/unbound.conf."
  exit 1
fi

systemctl daemon-reload
systemctl enable unbound
systemctl restart unbound
log_success "Unbound DNS service configured and running."

# ------------------------------------------------------------------------------
# Provision PostgreSQL Database
# ------------------------------------------------------------------------------
log_step "Provisioning PostgreSQL Database"
systemctl enable --now postgresql

sudo -u postgres psql -q <<EOF
DO
\$do\$
BEGIN
   IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = '${DB_USER}') THEN
      CREATE ROLE ${DB_USER} WITH LOGIN PASSWORD '${DB_PASS}';
   ELSE
      ALTER ROLE ${DB_USER} WITH PASSWORD '${DB_PASS}';
   END IF;
END
\$do\$;

SELECT 'CREATE DATABASE ${DB_NAME} OWNER ${DB_USER}'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = '${DB_NAME}')\gexec

GRANT ALL PRIVILEGES ON DATABASE ${DB_NAME} TO ${DB_USER};
EOF

log_success "PostgreSQL database '${DB_NAME}' provisioned for user '${DB_USER}'."

# ------------------------------------------------------------------------------
# Install Netora Control Plane API & NOC Web UI
# ------------------------------------------------------------------------------
log_step "Deploying Control Plane API & NOC Web UI"

# 1. Deploy netora-api binary
API_BIN_SOURCE=""
if [[ -f "${REPO_DIR}/deploy/bin/netora-api-linux-amd64" ]]; then
  API_BIN_SOURCE="${REPO_DIR}/deploy/bin/netora-api-linux-amd64"
elif [[ -f "${REPO_DIR}/bin/netora-api-linux" ]]; then
  API_BIN_SOURCE="${REPO_DIR}/bin/netora-api-linux"
elif [[ -f "/usr/local/bin/netora-api" ]]; then
  API_BIN_SOURCE="/usr/local/bin/netora-api"
fi

if [[ -n "$API_BIN_SOURCE" && -f "$API_BIN_SOURCE" ]]; then
  cp "$API_BIN_SOURCE" /usr/local/bin/netora-api
  chmod +x /usr/local/bin/netora-api
  log_success "Installed netora-api binary to /usr/local/bin/netora-api"
elif command -v go >/dev/null 2>&1; then
  log_info "Building netora-api from Go source..."
  (cd "$REPO_DIR" && go build -o /usr/local/bin/netora-api cmd/netora-api/main.go)
  chmod +x /usr/local/bin/netora-api
  log_success "Built netora-api from source."
else
  log_warn "Precompiled netora-api binary not found and Go is not installed."
  log_info "Installing Go runtime to compile netora-api..."
  apt-get install -y -q golang-go 2>/dev/null || true
  if command -v go >/dev/null 2>&1; then
    (cd "$REPO_DIR" && go build -o /usr/local/bin/netora-api cmd/netora-api/main.go)
    chmod +x /usr/local/bin/netora-api
    log_success "Compiled netora-api successfully."
  fi
fi

# 2. Deploy Web UI static files
if [[ -d "${REPO_DIR}/deploy/ui" && "$(ls -A "${REPO_DIR}/deploy/ui")" ]]; then
  cp -r "${REPO_DIR}/deploy/ui/"* /var/www/netora-ui/
  log_success "Deployed Web UI distribution from deploy/ui to /var/www/netora-ui"
elif [[ -d "${REPO_DIR}/web/dist" && "$(ls -A "${REPO_DIR}/web/dist")" ]]; then
  cp -r "${REPO_DIR}/web/dist/"* /var/www/netora-ui/
  log_success "Deployed Web UI distribution from web/dist to /var/www/netora-ui"
fi

# 3. Configure API Environment & Systemd Service
cat > /etc/netora-dns/api.env <<EOF
NETORA_LISTEN=:${WEB_LISTEN_PORT}
NETORA_DB_URL=postgres://${DB_USER}:${DB_PASS}@127.0.0.1:${DB_PORT}/${DB_NAME}?sslmode=disable
NETORA_NODE_ID=${NODE_ID}
NETORA_ADMIN_USER=${ADMIN_USER}
NETORA_ADMIN_PASS=${ADMIN_PASS}
EOF
chmod 600 /etc/netora-dns/api.env

cat > /etc/systemd/system/netora-api.service <<EOF
[Unit]
Description=Netora DNS Control Plane API & NOC Server
Documentation=https://github.com/desienkz-slp/unbound
After=network.target postgresql.service unbound.service
Wants=postgresql.service unbound.service

[Service]
Type=simple
User=root
EnvironmentFile=-/etc/netora-dns/api.env
ExecStart=/usr/local/bin/netora-api -listen :${WEB_LISTEN_PORT} -db-url postgres://${DB_USER}:${DB_PASS}@127.0.0.1:${DB_PORT}/${DB_NAME}?sslmode=disable -node-id ${NODE_ID}
Restart=always
RestartSec=5
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
if [[ -x /usr/local/bin/netora-api ]]; then
  systemctl enable --now netora-api.service
  log_success "Netora API service active on port :${WEB_LISTEN_PORT}"
fi

# ------------------------------------------------------------------------------
# Setup Autonomous GitHub CI/CD Auto-Updater
# ------------------------------------------------------------------------------
log_step "Configuring Autonomous GitHub CI/CD Updater"

# Install update agent script
if [[ -f "${REPO_DIR}/scripts/netora-dns-update.sh" ]]; then
  cp "${REPO_DIR}/scripts/netora-dns-update.sh" /usr/local/sbin/netora-dns-update
  chmod +x /usr/local/sbin/netora-dns-update
fi

# Write node identity & updater configuration
echo "$NODE_ID" > /etc/netora-dns/node-id

cat > /etc/netora-dns/updater.conf <<EOF
NODE_ID="${NODE_ID}"
CHANNEL="${CHANNEL}"
AUTO_UPDATE_ENABLED="${ENABLE_AUTO_UPDATE}"
REPO_URL="https://github.com/desienkz-slp/unbound.git"
GIT_BRANCH="${GIT_BRANCH}"
TOKEN_FILE="/etc/netora-dns/github.token"
BACKUP_RETENTION=5
EOF

if [[ -n "$GITHUB_TOKEN" ]]; then
  echo "$GITHUB_TOKEN" > /etc/netora-dns/github.token
  chmod 600 /etc/netora-dns/github.token
fi

# Install systemd timer
if [[ -f "${REPO_DIR}/deploy/systemd/netora-dns-updater.service" ]]; then
  cp "${REPO_DIR}/deploy/systemd/netora-dns-updater.service" /etc/systemd/system/
fi
if [[ -f "${REPO_DIR}/deploy/systemd/netora-dns-updater.timer" ]]; then
  cp "${REPO_DIR}/deploy/systemd/netora-dns-updater.timer" /etc/systemd/system/
fi

systemctl daemon-reload
if [[ "$ENABLE_AUTO_UPDATE" == "true" ]]; then
  systemctl enable --now netora-dns-updater.timer
  log_success "Autonomous auto-update timer scheduled (every 5 minutes)."
fi

# ------------------------------------------------------------------------------
# Install Netora CLI Helper & Password Reset Utilities
# ------------------------------------------------------------------------------
if [[ -f "${REPO_DIR}/scripts/netora-cli.sh" ]]; then
  cp "${REPO_DIR}/scripts/netora-cli.sh" /usr/local/bin/netora
  chmod +x /usr/local/bin/netora
  log_success "Installed Netora CLI to /usr/local/bin/netora"
fi

if [[ -f "${REPO_DIR}/scripts/netora-reset-password.sh" ]]; then
  cp "${REPO_DIR}/scripts/netora-reset-password.sh" /usr/local/bin/netora-reset-password
  chmod +x /usr/local/bin/netora-reset-password
  log_success "Installed password reset script to /usr/local/bin/netora-reset-password"
fi

CLI_BIN_SOURCE=""
if [[ -f "${REPO_DIR}/deploy/bin/netora-cli-linux-amd64" ]]; then
  CLI_BIN_SOURCE="${REPO_DIR}/deploy/bin/netora-cli-linux-amd64"
elif [[ -f "${REPO_DIR}/bin/netora-cli-linux" ]]; then
  CLI_BIN_SOURCE="${REPO_DIR}/bin/netora-cli-linux"
fi

if [[ -n "$CLI_BIN_SOURCE" && -f "$CLI_BIN_SOURCE" ]]; then
  cp "$CLI_BIN_SOURCE" /usr/local/bin/netora-cli
  chmod +x /usr/local/bin/netora-cli
  log_success "Installed netora-cli binary to /usr/local/bin/netora-cli"
elif command -v go >/dev/null 2>&1; then
  log_info "Building netora-cli from Go source..."
  (cd "$REPO_DIR" && go build -o /usr/local/bin/netora-cli cmd/netora-cli/main.go) 2>/dev/null && chmod +x /usr/local/bin/netora-cli && log_success "Built netora-cli from source." || true
fi

# ------------------------------------------------------------------------------
# Post-Installation Health Verification
# ------------------------------------------------------------------------------
log_step "Executing Post-Installation Health Checks"

sleep 1
DNS_TEST_RESULT=""
if DNS_TEST_RESULT="$(dig @127.0.0.1 -p "$DNS_LISTEN_PORT" google.com +short +time=3 2>/dev/null)" && [[ -n "$DNS_TEST_RESULT" ]]; then
  log_success "DNS Data Plane Resolution: PASS (google.com -> ${DNS_TEST_RESULT})"
else
  log_warn "DNS resolution test on 127.0.0.1:${DNS_LISTEN_PORT} returned no record. Check 'systemctl status unbound'."
fi

API_TEST_RESULT=""
if API_TEST_RESULT="$(curl -sf "http://127.0.0.1:${WEB_LISTEN_PORT}/api/v1/health" 2>/dev/null)"; then
  log_success "Control Plane REST API:    PASS (${API_TEST_RESULT})"
else
  log_warn "API health check failed on :${WEB_LISTEN_PORT}. Service may still be initializing."
fi

# ------------------------------------------------------------------------------
# Summary Presentation
# ------------------------------------------------------------------------------
SERVER_IP="$(hostname -I | awk '{print $1}' 2>/dev/null || echo "127.0.0.1")"

echo -e "\n${GREEN}${BOLD}═══════════════════════════════════════════════════════════════════${NC}"
echo -e "${GREEN}${BOLD}       NETORA DNS PLATFORM INSTALLED SUCCESSFULLY!                 ${NC}"
echo -e "${GREEN}${BOLD}═══════════════════════════════════════════════════════════════════${NC}"
echo -e "  ${BOLD}NOC Web Dashboard:${NC}   ${CYAN}http://${SERVER_IP}:${WEB_LISTEN_PORT}/${NC}"
echo -e "  ${BOLD}Prometheus Metrics:${NC}  ${CYAN}http://${SERVER_IP}:${WEB_LISTEN_PORT}/metrics${NC}"
echo -e "  ${BOLD}DNS Server Listener:${NC} ${CYAN}${SERVER_IP}:${DNS_LISTEN_PORT}${NC} & ${CYAN}127.0.0.1:${DNS_LISTEN_PORT}${NC}"
echo -e "  ${BOLD}Cluster Node ID:${NC}     ${YELLOW}${NODE_ID}${NC}"
echo ""
echo -e "  ${BOLD}Web UI Credentials:${NC}"
echo -e "    Username:          ${GREEN}${ADMIN_USER}${NC}"
echo -e "    Password:          ${GREEN}${ADMIN_PASS}${NC}"
echo ""
echo -e "  ${BOLD}CLI Operations Cheatsheet:${NC}"
echo -e "    Status & Health:   ${CYAN}netora status${NC}  or  ${CYAN}netora health${NC}"
echo -e "    Trigger Update:    ${CYAN}netora update${NC}"
echo -e "    1-Click Rollback:  ${CYAN}netora rollback${NC}"
echo -e "    Follow Logs:       ${CYAN}netora logs [unbound|api|updater]${NC}"
echo -e "${GREEN}${BOLD}═══════════════════════════════════════════════════════════════════${NC}\n"
