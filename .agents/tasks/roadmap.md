# Build Roadmap

## Phase 0 — Discovery
- inspect repo
- identify OS/build constraints
- define interfaces/IPs
- define expected QPS and query mix
- define deployment topology

## Phase 1 — Unbound Runtime Manager
- install/check Unbound
- node model
- multi-listener
- ACL
- config generator
- validate/apply/rollback

## Phase 2 — Control API
- Go API
- auth/RBAC
- PostgreSQL migrations
- desired state
- audit
- config versions

## Phase 3 — Filtering
- blocklists
- allowlist
- policy profiles
- local rewrites
- SafeSearch
- scheduled policy

## Phase 4 — UI
- setup wizard
- overview
- network
- upstream
- filters
- policies
- config history
- audit

## Phase 5 — Observability
- Prometheus
- Grafana
- Loki
- alerts

## Phase 6 — HA
- multi-node
- health
- synchronization
- failover/anycast-ready design

## Phase 7 — AI/MCP
- read-only diagnostics
- controlled writes
- Hermes integration

## Phase 8 — Benchmark & hardening
- failure testing
- load testing
- security testing
- long-run soak tests
- documented capacity
