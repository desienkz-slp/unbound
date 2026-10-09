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
USE_UNBOUND_PPA="false"
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

render_progress_bar() {
  local pct="${1:-0}"
  local info="${2:-}"
  local width=32
  local num=$(( pct * width / 100 ))
  local bar=""
  for ((i=0; i<num; i++)); do bar+="█"; done
  for ((i=num; i<width; i++)); do bar+="░"; done
  printf "\r  ${CYAN}[%s]${NC} ${BOLD}%3d%%${NC} %-35s" "$bar" "$pct" "$info"
}

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
  --unbound-ppa             (Ubuntu only) Install latest Unbound from NLnet Labs PPA
                            (ppa:nlnetlabs/unbound) to enable full serve-stale features
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
    --unbound-ppa)        USE_UNBOUND_PPA="true"; shift ;;
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
    
    # Ensure local resolv.conf points to loopback.
    # Preserve the ORIGINAL resolv.conf only on first run; re-running the installer
    # must never overwrite the genuine backup with the already-modified loopback copy.
    if [[ -f /etc/resolv.conf && ! -f /etc/resolv.conf.netora.bak ]]; then
      cp /etc/resolv.conf /etc/resolv.conf.netora.bak 2>/dev/null || true
    fi
    rm -f /etc/resolv.conf
    echo "nameserver 127.0.0.1" > /etc/resolv.conf
    echo "nameserver 1.1.1.1" >> /etc/resolv.conf
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

# Optional: pull the latest Unbound from NLnet Labs PPA (Ubuntu only) so that
# modern serve-stale keywords (serve-expired-client-timeout, >=1.10.0) are available.
if [[ "$USE_UNBOUND_PPA" == "true" ]]; then
  if [[ "${ID:-}" == "ubuntu" ]]; then
    log_info "Enabling NLnet Labs Unbound PPA (ppa:nlnetlabs/unbound)..."
    apt-get update -y -q || true
    apt-get install -y -q software-properties-common || true
    if add-apt-repository -y ppa:nlnetlabs/unbound; then
      log_success "NLnet Labs Unbound PPA enabled."
    else
      log_warn "Failed to add Unbound PPA; falling back to distro package."
    fi
  else
    log_warn "--unbound-ppa is Ubuntu-only; current OS is '${ID:-unknown}'. Using distro package."
  fi
fi

apt-get update -y -q
apt-get install -y -q \
  unbound \
  unbound-anchor \
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
  unattended-upgrades \
  postgresql \
  postgresql-contrib

log_success "Base system packages installed."

# ------------------------------------------------------------------------------
# Automatic Security Updates (unattended-upgrades, security channel only)
# ------------------------------------------------------------------------------
# Keeps Unbound and all OS packages patched against security vulnerabilities
# WITHOUT performing risky full-version upgrades that could break the resolver.
# Only the distribution's security pocket is enabled.
log_step "Configuring Automatic Security Updates"

if command -v unattended-upgrade >/dev/null 2>&1 || command -v unattended-upgrades >/dev/null 2>&1; then
  # Determine the correct origin codename token (Ubuntu vs Debian)
  if [[ "${ID:-}" == "debian" ]]; then
    SECURITY_ORIGIN='"origin=Debian,codename=${distro_codename}-security,label=Debian-Security";'
  else
    SECURITY_ORIGIN='"${distro_id}:${distro_codename}-security";'
  fi

  cat > /etc/apt/apt.conf.d/50netora-unattended-upgrades <<EOF
// Managed by Netora DNS installer — security updates only.
Unattended-Upgrade::Allowed-Origins {
    ${SECURITY_ORIGIN}
    "\${distro_id}ESMApps:\${distro_codename}-apps-security";
    "\${distro_id}ESM:\${distro_codename}-infra-security";
};

// Keep DNS resolver stable: never auto-reboot a production DNS node.
Unattended-Upgrade::Automatic-Reboot "false";

// Clean up obsolete dependencies and old kernels conservatively.
Unattended-Upgrade::Remove-Unused-Kernel-Packages "true";
Unattended-Upgrade::Remove-Unused-Dependencies "true";

// Email disabled by default; configure MTA separately if desired.
Unattended-Upgrade::Mail "";
EOF

  cat > /etc/apt/apt.conf.d/20netora-auto-upgrades <<EOF
// Managed by Netora DNS installer.
APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Download-Upgradeable-Packages "1";
APT::Periodic::AutocleanInterval "7";
APT::Periodic::Unattended-Upgrade "1";
EOF

  systemctl enable --now unattended-upgrades.service >/dev/null 2>&1 || true
  log_success "Automatic security updates enabled (security pocket only; no auto-reboot)."
else
  log_warn "unattended-upgrades not available; skipping automatic security updates."
fi

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
SCRIPT_SOURCE_DIR=""
if [[ -n "${BASH_SOURCE[0]:-}" ]]; then
  SCRIPT_SOURCE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" 2>/dev/null && pwd || echo "")"
fi

if [[ -n "$SCRIPT_SOURCE_DIR" && -f "${SCRIPT_SOURCE_DIR}/conf/unbound-base.conf" && -d "${SCRIPT_SOURCE_DIR}/pkg" ]]; then
  log_info "Detected local Netora source repository at: ${SCRIPT_SOURCE_DIR}"
  mkdir -p "$REPO_DIR"
  cp -r "${SCRIPT_SOURCE_DIR}/"* "$REPO_DIR/" 2>/dev/null || true
else
  log_info "Downloading Netora Core Package from GitHub (${GIT_BRANCH})..."
  AUTH_CLONE_URL="$REPO_URL"
  if [[ -n "$GITHUB_TOKEN" ]]; then
    AUTH_CLONE_URL="https://x-access-token:${GITHUB_TOKEN}@github.com/desienkz-slp/unbound.git"
  fi

  rm -rf "$REPO_DIR"
  CLONE_LOG=$(mktemp /tmp/netora-clone.XXXXXX)

  render_progress_bar 0 "Connecting to GitHub..."

  (
    set +e
    GIT_TERMINAL_PROMPT=0 git clone --depth=1 --progress --branch "$GIT_BRANCH" "$AUTH_CLONE_URL" "$REPO_DIR" 2>&1
  ) | tee "$CLONE_LOG" | tr "\r" "\n" | while IFS= read -r line; do
    if [[ "$line" =~ Receiving[[:space:]]+objects:[[:space:]]*([0-9]+)%([^\,]*)(,\ (.*))? ]]; then
      pct="${BASH_REMATCH[1]}"
      extra="${BASH_REMATCH[4]:-}"
      render_progress_bar "$pct" "$extra"
    elif [[ "$line" =~ Resolving[[:space:]]+deltas:[[:space:]]*([0-9]+)% ]]; then
      render_progress_bar 100 "Resolving deltas..."
    fi
  done || true

  if [[ -d "$REPO_DIR/.git" ]]; then
    render_progress_bar 100 "Complete! (18.5 MB)"
    echo ""
    log_success "Repository cloned to: ${REPO_DIR}"
    rm -f "$CLONE_LOG"
  else
    echo ""
    log_error "Failed to clone repository from GitHub. If this is a private repo, make sure to provide: --github-token <TOKEN>"
    echo -e "${RED}[ERROR LOG]${NC}"
    cat "$CLONE_LOG"
    rm -f "$CLONE_LOG"
    exit 1
  fi
fi
ln -sfn "${REPO_DIR}" "${INSTALL_DIR}/current"

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
  wget -qO /var/lib/unbound/root.hints https://www.internic.net/domain/named.cache 2>/dev/null || true
fi

# Run unbound-anchor if available
if command -v unbound-anchor >/dev/null 2>&1; then
  unbound-anchor -a /var/lib/unbound/root.key || true
elif [[ -x /usr/sbin/unbound-anchor ]]; then
  /usr/sbin/unbound-anchor -a /var/lib/unbound/root.key || true
fi

# Trust Anchor Fallback: If root.key is missing or empty, inject official IANA root DNSSEC trust anchor
if [[ ! -f /var/lib/unbound/root.key ]] || [[ ! -s /var/lib/unbound/root.key ]]; then
  cat > /var/lib/unbound/root.key <<'EOF'
.	3600	IN	DNSKEY	257 3 8 AwEAAa96jeuknZlaeSrvyAJj6ZHv28hhOKkx3rLGXVaC6rXTsDc449/cidltpkyGwCJNnOAlFNKF2jBosZBU5eeHspaQWOmOElZsjICMQMC3aeHbGiShvZsx4wMYSjH8e7Vrhbu6irwCzVBApESjbUdpWWmEnhathWu1jo+siFUiRAAxm9qyJNg/wOZqqzL/dL/q8PkcRU5oUKEpUge71M3ej2/7CPqpdVwuMoTvoB+ZOT4YeGyxMvHmbrxlFzGOHOijtzN+u1TQNatX2XBuzZNQ1K+s2CXkPIZo7s6JgZyvaBevYtxPvYLw4z9mR7K2vaF18UYH9Z9GNUUeayffKC73PYc= ;{id = 38696 (ksk), size = 2048b} ;;state=2 [  VALID  ]
.	3600	IN	DNSKEY	257 3 8 AwEAAaz/tAm8yTn4Mfeh5eyI96WSVexTBAvkMgJzkKTOiW1vkIbzxeF3+/4RgWOq7HrxRixHlFlExOLAJr5emLvN7SWXgnLh4+B5xQlNVz8Og8kvArMtNROxVQuCaSnIDdD5LKyWbRd2n9WGe2R8PzgCmr3EgVLrjyBxWezF0jLHwVN8efS3rCj/EWgvIWgb9tarpVUDK/b58Da+sqqls3eNbuv7pr+eoZG+SrDK6nWeL3c6H5Apxz7LjVc1uTIdsIXxuOLYA4/ilBmSVIzuDWfdRUfhHdY6+cn8HFRm+2hM8AnXGXws9555KrUB5qihylGa8subX2Nn6UwNR1AkUTV74bU= ;{id = 20326 (ksk), size = 2048b} ;;state=2 [  VALID  ]
EOF
fi

# Ensure Unbound daemon has full read/write access to /var/lib/unbound
chown -R unbound:unbound /var/lib/unbound 2>/dev/null || true
chmod 755 /var/lib/unbound
chmod 644 /var/lib/unbound/root.hints 2>/dev/null || true
chmod 644 /var/lib/unbound/root.key 2>/dev/null || true

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

# Build ACL Directives (always explicitly allow localhost loopbacks)
ACL_DIRECTIVES="  access-control: 127.0.0.0/8 allow"$'\n'"  access-control: ::1 allow"$'\n'
for sub in $CLIENT_SUBNETS; do
  if [[ "$sub" != "127.0.0.0/8" && "$sub" != "::1" ]]; then
    ACL_DIRECTIVES+="  access-control: ${sub} allow"$'\n'
  fi
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

# ------------------------------------------------------------------------------
# Version-aware directive guards (keywords differ across Unbound releases)
# ------------------------------------------------------------------------------
# serve-expired-client-timeout was introduced in Unbound 1.10.0. Older distro
# packages (e.g. Ubuntu 18.04 ships 1.6.7, Ubuntu 20.04 ships 1.9.4) reject this
# keyword, causing unbound-checkconf to fail. Emit it only when supported.
UNBOUND_VERSION="$(unbound-checkconf -h 2>&1 | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -n1 || true)"
if [[ -z "$UNBOUND_VERSION" ]]; then
  UNBOUND_VERSION="$(unbound -V 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -n1 || true)"
fi
log_info "Detected Unbound version: ${UNBOUND_VERSION:-unknown}"

version_ge() { [[ "$(printf '%s\n%s\n' "$2" "$1" | sort -V | head -n1)" == "$2" ]]; }

SERVE_EXPIRED_CLIENT_TIMEOUT_DIRECTIVE=""
if [[ -n "$UNBOUND_VERSION" ]] && version_ge "$UNBOUND_VERSION" "1.10.0"; then
  SERVE_EXPIRED_CLIENT_TIMEOUT_DIRECTIVE="  serve-expired-client-timeout: 1800"
else
  log_warn "Unbound < 1.10.0 detected; skipping 'serve-expired-client-timeout' (unsupported keyword)."
fi

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
${SERVE_EXPIRED_CLIENT_TIMEOUT_DIRECTIVE}

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

# ------------------------------------------------------------------------------
# Default DNS Rewrites for Node (1 IP = 1 rewrite, 2 IPs = 2 rewrites)
# ------------------------------------------------------------------------------
SERVER_IPS=()
if command -v ip >/dev/null 2>&1; then
  while IFS= read -r s_ip; do
    [[ -n "$s_ip" && "$s_ip" != 127.* ]] && SERVER_IPS+=("$s_ip")
  done < <(ip -4 -o addr show scope global | awk '{print $4}' | cut -d/ -f1 | sort -u)
fi
if [[ ${#SERVER_IPS[@]} -eq 0 ]] && command -v hostname >/dev/null 2>&1; then
  for h_ip in $(hostname -I 2>/dev/null || true); do
    [[ -n "$h_ip" && "$h_ip" != 127.* && ! "$h_ip" =~ : ]] && SERVER_IPS+=("$h_ip")
  done
fi

if [[ ${#SERVER_IPS[@]} -gt 0 ]]; then
  log_info "Detected ${#SERVER_IPS[@]} active IP address(es) for node '${NODE_ID}': ${SERVER_IPS[*]}"
  cat > /etc/unbound/conf.d/local-records.conf <<EOF
# Netora DNS — Local Authoritative Records for ${NODE_ID}
# Automatically configured: ${#SERVER_IPS[@]} IP(s) -> ${#SERVER_IPS[@]} rewrite(s)
server:
EOF
  for s_ip in "${SERVER_IPS[@]}"; do
    echo "  local-data: \"${NODE_ID} 300 A ${s_ip}\"" >> /etc/unbound/conf.d/local-records.conf
    echo "  local-data-ptr: \"${s_ip} ${NODE_ID}\"" >> /etc/unbound/conf.d/local-records.conf
  done
  log_success "Generated ${#SERVER_IPS[@]} default DNS rewrite(s) for node '${NODE_ID}' in /etc/unbound/conf.d/local-records.conf"
fi

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

# Apply database schema migrations
MIGRATION_DIR=""
if [[ -d "${REPO_DIR}/pkg/storage/migrations" ]]; then
  MIGRATION_DIR="${REPO_DIR}/pkg/storage/migrations"
elif [[ -d "/opt/netora-dns/repository/pkg/storage/migrations" ]]; then
  MIGRATION_DIR="/opt/netora-dns/repository/pkg/storage/migrations"
fi

if [[ -n "$MIGRATION_DIR" && -d "$MIGRATION_DIR" ]]; then
  log_info "Applying database schema migrations..."
  for sql_file in $(find "$MIGRATION_DIR" -name "*.sql" | sort); do
    log_info "Executing migration: $(basename "$sql_file")"
    cat "$sql_file" | (cd /tmp && sudo -u postgres psql -q -d "${DB_NAME}") || true
  done
  (cd /tmp && sudo -u postgres psql -q -d "${DB_NAME}" -c "
    DO \$\$
    DECLARE r RECORD;
    BEGIN
      FOR r IN (SELECT tablename FROM pg_tables WHERE schemaname = 'public') LOOP
        EXECUTE 'ALTER TABLE ' || quote_ident(r.tablename) || ' OWNER TO ${DB_USER}';
      END LOOP;
    END \$\$;
    GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO ${DB_USER};
    GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO ${DB_USER};
    ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO ${DB_USER};
  ") || true
  log_success "Database schema migrations applied and ownership granted."
elif [[ -f "${REPO_DIR}/pkg/storage/migrations/001_initial_schema.sql" ]]; then
  log_info "Applying initial database schema migrations..."
  cat "${REPO_DIR}/pkg/storage/migrations/001_initial_schema.sql" | (cd /tmp && sudo -u postgres psql -q -d "${DB_NAME}") || true
  (cd /tmp && sudo -u postgres psql -q -d "${DB_NAME}" -c "GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO ${DB_USER}; GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO ${DB_USER}; ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO ${DB_USER};") || true
  log_success "Initial database schema applied and privileges granted."
fi

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

# 2. Deploy Web UI static files (POSIX-safe directory copy avoids glob failure under set -euo pipefail)
if [[ -d "${REPO_DIR}/deploy/ui" && "$(ls -A "${REPO_DIR}/deploy/ui")" ]]; then
  cp -r "${REPO_DIR}/deploy/ui/." /var/www/netora-ui/
  log_success "Deployed Web UI distribution from deploy/ui to /var/www/netora-ui"
elif [[ -d "${REPO_DIR}/web/dist" && "$(ls -A "${REPO_DIR}/web/dist")" ]]; then
  cp -r "${REPO_DIR}/web/dist/." /var/www/netora-ui/
  log_success "Deployed Web UI distribution from web/dist to /var/www/netora-ui"
fi

# 2b. Seed version.json to state directory for backend API (/api/v1/system/version)
if [[ -f "${REPO_DIR}/deploy/ui/version.json" ]]; then
  cp -f "${REPO_DIR}/deploy/ui/version.json" /var/lib/netora-dns/version.json
  log_success "Seeded version.json to /var/lib/netora-dns/version.json"
elif [[ -f "${REPO_DIR}/version.json" ]]; then
  cp -f "${REPO_DIR}/version.json" /var/lib/netora-dns/version.json
  log_success "Seeded version.json to /var/lib/netora-dns/version.json"
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

# Install systemd updater service (ProtectSystem=false + /var/www/netora-ui in ReadWritePaths for full update scope)
if [[ -f "${REPO_DIR}/deploy/systemd/netora-dns-updater.service" ]]; then
  cp "${REPO_DIR}/deploy/systemd/netora-dns-updater.service" /etc/systemd/system/
  log_success "Installed netora-dns-updater.service (sandbox: ProtectSystem=false, full ReadWritePaths)"
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

log_info "Warming up DNS recursive resolution..."
DNS_TEST_RESULT=""
for attempt in 1 2 3 4 5; do
  if DNS_TEST_RESULT="$(dig @127.0.0.1 -p "$DNS_LISTEN_PORT" google.com +short +time=3 +tries=2 2>/dev/null)" && [[ -n "$DNS_TEST_RESULT" ]]; then
    break
  fi
  sleep 1
done

if [[ -n "$DNS_TEST_RESULT" ]]; then
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
