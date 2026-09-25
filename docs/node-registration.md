# Netora DNS — Node Registration Guide

Follow this guide to provision and enroll a new carrier DNS server into the Netora autonomous fleet.

---

## 1. Quick Registration (One-Liner)

Run the registration script as `root` on the new server:

```bash
# Syntax: ./register-node.sh <NODE_ID> <CHANNEL> <GITHUB_TOKEN>
curl -fsSL https://raw.githubusercontent.com/desienkz-slp/unbound/main/scripts/register-node.sh | \
  bash -s -- "dns-jakarta-01" "production" "ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
```

---

## 2. Manual Step-by-Step Registration

If registering manually or integrating into Ansible / Cloud-Init:

### Step 1: Install Dependencies
```bash
apt-get update -qq
apt-get install -y -qq git jq curl bind9-dnsutils unbound
```

### Step 2: Establish File System Hierarchy
```bash
mkdir -p /etc/netora-dns/overlays \
         /opt/netora-dns/releases \
         /opt/netora-dns/backups \
         /opt/netora-dns/staging \
         /var/lib/netora-dns \
         /var/log/netora-dns

chmod 750 /etc/netora-dns /opt/netora-dns /var/lib/netora-dns /var/log/netora-dns
```

### Step 3: Configure Node Identity & Security
```bash
echo "dns-jakarta-01" > /etc/netora-dns/node-id
chmod 644 /etc/netora-dns/node-id

# Secure the GitHub token (Permissions MUST be 600)
echo "ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx" > /etc/netora-dns/github.token
chmod 600 /etc/netora-dns/github.token
```

### Step 4: Write `/etc/netora-dns/updater.conf`
```bash
cat <<EOF > /etc/netora-dns/updater.conf
NODE_ID="dns-jakarta-01"
CHANNEL="production"
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
```

### Step 5: Install Updater & Enable Timer
```bash
cp scripts/netora-dns-update.sh /usr/local/sbin/netora-dns-update
chmod 755 /usr/local/sbin/netora-dns-update

cp deploy/systemd/netora-dns-updater.service /etc/systemd/system/
cp deploy/systemd/netora-dns-updater.timer /etc/systemd/system/

systemctl daemon-reload
systemctl enable --now netora-dns-updater.timer
```

### Step 6: Verify Registration
```bash
netora-dns-update status
netora-dns-update check
```
