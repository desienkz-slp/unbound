# Walkthrough — Refactor Pre Fresh-Install

**Date:** 2026-10-22
**Scope:** 5 refactor tasks executed on the `unbound` (Netora DNS) codebase

---

## Changes Summary

### Task 1: Graceful Shutdown (Safety)
**File:** `cmd/netora-api/main.go`

Shutdown sequence sekarang ordered:
1. HTTP server stop (drain in-flight requests, timeout 10s)
2. Cancel background goroutines (stats collector, query log tailer, heartbeat, blocklist scheduler)
3. Wait 500ms for goroutine drain
4. Close database connections

Sebelumnya: `defer` based (unordered), timeout hanya 5s.

### Task 2: NOC `policy:read` RBAC
**Files:** `pkg/auth/auth.go`, `pkg/auth/auth_test.go`

Role NOC sekarang punya `policy:read` — bisa lihat policies, filters, blocklist status, dan allowlist via API tanpa write access. Test ditambahkan.

### Task 3: Hapus Hardcoded Seed Block Rules
**File:** `cmd/netora-api/main.go`

3 baris `AddBlockRule` dihapus dari `main.go`. Redundan — blocklist manager (`SyncAll` + `StandardCoreFeeds`) sudah handle loading ratusan ribu domain dari feed files.

### Task 4: Configurable Blocklist Feeds
**Files:** `pkg/model/config.go`, `pkg/blocklist/manager.go`

- `FeedConfig` struct baru di model
- `BlocklistFeeds` field baru di `DesiredState` (JSON: `blocklist_feeds`)
- `ResolveFeeds()` method di blocklist manager — pakai config dari `DesiredState` jika ada, fallback ke `StandardCoreFeeds()`
- Non-breaking: existing deployments tanpa `blocklist_feeds` tetap pakai default feeds

### Task 5: Split server.go (1810 → 5 files)
**Files:** `cmd/netora-api/server.go` + 4 file baru

| File | Content | Lines |
|------|---------|-------|
| `server.go` | Server struct, NewServer, RegisterRoutes, middleware | ~270 |
| `handler_auth.go` | Login, Logout, Me, UpdateProfile | ~230 |
| `handler_node.go` | Nodes, NodeSubroutes (config/apply/rollback/stats), Logrotate | ~380 |
| `handler_policy.go` | Policies, Filters, FilterSubroutes, SyncBlocklists, Allowlist | ~340 |
| `handler_ops.go` | SystemHealth, SystemUpdate, AuditLogs, Diagnose, Alerts, Cluster, Hermes, DatabaseBackups | ~350 |

Zero logic changes — murni file split.

---

## Verification

```
✅ go build ./cmd/netora-api/  — PASS
✅ go test ./...               — ALL PASS (18 packages)
```

## Fresh-server install review (2026-10-22)

- Verdict: not production-ready until the blockers in the [review report](file:///C:/Users/desie/.gemini/antigravity-ide/brain/e3ce5228-55a9-4bcb-b160-61c6996f0cf3/review_report.md) are fixed and tested on clean Ubuntu/Debian.
- Main blockers: weak default admin/DB passwords; PostgreSQL failure silently falls back to volatile memory; installer overwrites system DNS config; open-resolver ACL behavior lacks proof; DB credentials are interpolated unsafely.
- `go test ./...` passed locally. Fresh VM installation, IPv4/IPv6 ACL isolation, reboot persistence, DNSSEC, and uninstall restoration were not tested.
- No project code was modified in this review.
