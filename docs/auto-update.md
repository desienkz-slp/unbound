# Netora DNS — Server-Side Auto-Update Agent

## 1. Directory Layout

The updater adheres to the canonical `/opt/netora-dns/` layout:

```text
/opt/netora-dns/
├── repository/         # Local git cache (if maintained)
├── releases/           # Immutable release packages keyed by Commit SHA
│   └── <sha>/          # Complete snapshot of candidate release
├── current             # Symlink pointing to active release (/opt/netora-dns/releases/<sha>)
├── backups/            # Pre-activation snapshots (retained up to BACKUP_RETENTION)
│   └── <sha>_<time>/   # Complete backup of /etc/unbound and state
├── staging/            # Temporary compilation directory for candidates
└── state/              # Local persistent state
    └── update-state.json
```

## 2. Configuration & State

### 2.1. Configuration: `/etc/netora-dns/updater.conf`
```bash
NODE_ID="dns-dev"
CHANNEL="development"
REPO_URL="https://github.com/desienkz-slp/unbound.git"
GIT_BRANCH="main"
TOKEN_FILE="/etc/netora-dns/github.token"
AUTO_UPDATE_ENABLED="true"
MAINTENANCE_MODE="false"
BACKUP_RETENTION=5
HEALTH_CHECK_TIMEOUT=5
MAX_RELOAD_WAIT=10
OVERLAYS_DIR="/etc/netora-dns/overlays"
```

### 2.2. State File: `/var/lib/netora-dns/update-state.json`
```json
{
  "node_id": "dns-dev",
  "channel": "development",
  "current_revision": "413e1a17937402636fa09930f7604313eb1947b1",
  "previous_revision": "27d583487f3b89ce7cf372134ed728b7a481c629",
  "update_time": "2026-09-25T15:20:00Z",
  "update_status": "SUCCESS",
  "update_duration_sec": 4,
  "last_error": "none",
  "health_status": "PASS",
  "auto_update_enabled": true,
  "maintenance_mode": false
}
```

## 3. CLI Command Reference

The command `/usr/local/sbin/netora-dns-update` provides full operator control:

```bash
# Display node status, current vs latest revision, and health
netora-dns-update status

# Query upstream repository for updates without downloading
netora-dns-update check

# Trigger an immediate update check and activation
netora-dns-update update

# Force update to a specific git commit SHA
netora-dns-update update --revision <sha> --force

# Restore previous working configuration
netora-dns-update rollback

# Execute DNS data-plane health check
netora-dns-update health

# Display updater version
netora-dns-update version

# Maintenance controls (suppresses auto-updates during emergency windows)
netora-dns-update maintenance on
netora-dns-update maintenance off

# Temporarily pause / resume background updater timer
netora-dns-update pause
netora-dns-update resume
```

## 4. Systemd Timer & Service

The updater is scheduled via systemd timer `/etc/systemd/system/netora-dns-updater.timer`:
- **Frequency:** Every 5 minutes (`OnUnitActiveSec=5min`).
- **Jitter / Desynchronization:** `RandomizedDelaySec=30s` prevents all fleet nodes from hammering the GitHub API simultaneously.
- **Service Type:** `oneshot` unit running with root capabilities bounded by systemd security filters.
