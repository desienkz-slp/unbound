# Netora DNS — Rollback Architecture & Disaster Recovery

## 1. Automatic Rollback Triggers

Netora DNS activates automatic rollback when ANY of the following occurs:

| Failure Mode | Detection Point | Automated Action |
|---|---|---|
| **Syntax / Validation Failure** | `validate_candidate_config` | **Immediate Abort:** Candidate files are wiped from staging. Active configuration is never touched. Service is never reloaded. |
| **Open Resolver Ingestion** | Regex audit for `0.0.0.0/0 allow` | **Immediate Abort:** Critical security violation logged. |
| **Daemon Reload Failure** | `unbound-control reload` exit code != 0 | **Automatic Rollback:** Pre-activation backup from `/opt/netora-dns/backups/` is restored, and Unbound is reloaded to last-known-good. |
| **Post-Activation Health Failure** | `action_health` probe failure (< 3 passes) | **Automatic Rollback:** Live queries failed to resolve or DNSSEC broke. Immediate fallback to previous backup. |

## 2. Rollback Mechanism

When a rollback is triggered:
1. The updater acquires the `/run/lock/netora-dns-update.lock` file.
2. The latest verified backup directory in `/opt/netora-dns/backups/<prev_sha>_<timestamp>/` is located.
3. The previous `/etc/unbound/` configuration tree is restored atomically.
4. `unbound-control reload` is called (with fallback to `systemctl restart unbound` if daemon is stuck).
5. Comprehensive DNS health checks are re-executed to confirm recovery.
6. The failed candidate release is preserved under `/opt/netora-dns/releases/<failed_sha>/` for post-mortem analysis.
7. `/var/lib/netora-dns/update-state.json` is updated with `update_status: "ROLLED_BACK"`.

## 3. Manual Rollback SOP

An operator can manually revert to any previous backup at any time:

```bash
# View available backups
ls -la /opt/netora-dns/backups/

# Rollback to the most recent backup
netora-dns-update rollback

# Rollback to a specific named backup point
netora-dns-update rollback /opt/netora-dns/backups/8a71b6c_20260925_143000
```
