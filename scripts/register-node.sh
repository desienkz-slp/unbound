#!/usr/bin/env bash
# ==============================================================================
# Netora DNS — Server Node Registration & Updater Installer
# Path: scripts/register-node.sh
# ==============================================================================

set -euo pipefail

NODE_ID="${1:-$(hostname -s 2>/dev/null || echo "dns-node")}"
CHANNEL="${2:-development}"
TOKEN="${3:-}"

echo "================================================================================"
echo "          Registering Netora DNS Node: ${NODE_ID} (${CHANNEL})                  "
echo "================================================================================"

if [[ $EUID -ne 0 ]]; then
    echo "Error: This script must be run as root." >&2
    exit 1
fi

# Ensure required packages exist
for pkg in git jq curl bind9-dnsutils; do
    if ! dpkg -s "$pkg" >/dev/null 2>&1; then
        echo "Installing required dependency: $pkg"
        apt-get update -qq && apt-get install -y -qq "$pkg"
    fi
done

# Setup directories
mkdir -p /etc/netora-dns/overlays /opt/netora-dns/releases /opt/netora-dns/backups /var/lib/netora-dns /var/log/netora-dns
chmod 750 /etc/netora-dns /opt/netora-dns /var/lib/netora-dns /var/log/netora-dns

# Set Node ID
echo "$NODE_ID" > /etc/netora-dns/node-id
chmod 644 /etc/netora-dns/node-id

# Set Git Token if provided
if [[ -n "$TOKEN" ]]; then
    echo "$TOKEN" > /etc/netora-dns/github.token
    chmod 600 /etc/netora-dns/github.token
fi

# Setup updater.conf
cat <<EOF > /etc/netora-dns/updater.conf
# Netora DNS Node Configuration
NODE_ID="${NODE_ID}"
CHANNEL="${CHANNEL}"
REPO_URL="https://github.com/desienkz-slp/unbound.git"
GIT_BRANCH="main"
TOKEN_FILE="/etc/netora-dns/github.token"
AUTO_UPDATE_ENABLED="true"
MAINTENANCE_MODE="false"
BACKUP_RETENTION=5
HEALTH_CHECK_TIMEOUT=5
MAX_RELOAD_WAIT=10
OVERLAYS_DIR="/etc/netora-dns/overlays"
EOF
chmod 640 /etc/netora-dns/updater.conf

# Install Updater Binary
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [[ -f "${SCRIPT_DIR}/netora-dns-update.sh" ]]; then
    cp "${SCRIPT_DIR}/netora-dns-update.sh" /usr/local/sbin/netora-dns-update
elif [[ -f "/usr/local/sbin/netora-dns-update" ]]; then
    echo "Updater script already installed in /usr/local/sbin/netora-dns-update"
else
    curl -fsSL https://raw.githubusercontent.com/desienkz-slp/unbound/main/scripts/netora-dns-update.sh -o /usr/local/sbin/netora-dns-update
fi
chmod +x /usr/local/sbin/netora-dns-update

# Install Systemd Units
if [[ -f "${SCRIPT_DIR}/../deploy/systemd/netora-dns-updater.service" ]]; then
    cp "${SCRIPT_DIR}/../deploy/systemd/netora-dns-updater.service" /etc/systemd/system/
    cp "${SCRIPT_DIR}/../deploy/systemd/netora-dns-updater.timer" /etc/systemd/system/
fi

systemctl daemon-reload
systemctl enable --now netora-dns-updater.timer

echo "Node registered successfully. Initial status:"
/usr/local/sbin/netora-dns-update status
