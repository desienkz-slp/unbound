# AGENTS.md — Netora DNS

You are an AI software engineering agent working on Netora DNS.

## Mission

Build a professional ISP DNS platform around Unbound. Optimize for correctness, resilience, security, observability, operability, and maintainability before cosmetic features.

## Architecture

### Data plane
- Unbound
- DNS cache
- Recursive resolution / forwarding / hybrid resolution
- DNSSEC validation
- Filtering
- Local records
- ACLs
- Multi-interface IPv4/IPv6 listeners

### Control plane
- Go service
- REST API
- configuration model
- config generator
- validator
- atomic apply
- rollback
- health verification
- audit log
- RBAC
- scheduler
- filter manager

### Storage
- PostgreSQL for control-plane state
- filesystem for generated Unbound configuration
- Prometheus/Loki for telemetry; do not store every DNS query synchronously in PostgreSQL

### Frontend
- React/TypeScript
- Tailwind
- responsive NOC/admin UI
- UX inspired by AdGuard Home, not a copy

## Critical invariants

1. Never make DNS depend on the web UI.
2. Never require PostgreSQL for Unbound to answer queries.
3. Never apply an unvalidated configuration.
4. Every configuration change must be versioned and reversible.
5. Failed apply must preserve the last known-good configuration.
6. Never expose `unbound-control` publicly.
7. Never create an open resolver accidentally.
8. Never disable DNSSEC globally to hide a domain-specific problem.
9. Never log every DNS query synchronously into PostgreSQL.
10. IPv4 and IPv6 must be first-class.
11. Internet-facing listener and client-facing listener are separate concepts.
12. Secrets must never be committed.
13. Every destructive or security-sensitive action needs explicit authorization in the API layer.
14. Tests must include failure and rollback scenarios, not only happy paths.
15. Performance claims must be benchmarked, not guessed.
16. ISP recursive resolvers must prioritize real-world CDN compatibility (e.g. disable strict qname-minimisation if upstream authoritative CDNs return non-compliant NXDOMAIN on intermediate labels).
17. Production node updates (including server 172.18.20.145 / dns-dev) must always be deployed by pulling from the canonical GitHub repository after committing and pushing changes, never via ad-hoc unversioned manual file transfers.
18. Multi-Node IP Config Invariance: Auto-update releases must never distribute hardcoded node IP listeners, unbound interface bindings, or overwrite local database configurations. Each node's unique IP topology and PostgreSQL state must remain immutable across updates.
19. Environment Access Boundary: Respect access tiers — dev nodes (e.g., 172.18.20.145) allow SSH root access for end-to-end automation, whereas production nodes (e.g., 172.18.20.146) are strictly updated via Web UI / HTTP API. Even if SSH access is available for diagnostics/investigation, deployment of updates MUST NEVER be done via SCP or manual CLI commands directly on the server.
20. Server 146 Web UI Footer Update Mandate: Updates on server 172.18.20.146 must ALWAYS be triggered using the "Update" button in the Web UI footer (or via browser subagent / `POST /api/v1/system/update`) after committing and pushing code to GitHub. Never update server 146 via SCP or direct shell commands.

## Coding behavior

- Inspect existing code before modifying it.
- Prefer small, reviewable commits.
- Do not rewrite working systems without evidence.
- Preserve backward compatibility where practical.
- Add tests for every non-trivial behavior.
- Validate configuration with `unbound-checkconf` before reload.
- Use atomic file replacement.
- Reload before restart when possible.
- Use structured logs.
- Return actionable errors.
- For DNS validation, use browser subagent testing against real media-heavy websites (e-commerce, video) to verify CDN and streaming asset delivery alongside CLI benchmarks.
- For server updates on target nodes (especially 172.18.20.146), always commit and push changes to GitHub first, then trigger the update exclusively via the Web UI footer update button (or `POST /api/v1/system/update`), never via SCP or direct CLI commands.
- Document operational assumptions.

## Definition of done

A feature is not done until:
- implementation exists
- unit/integration tests exist
- security implications are considered
- metrics/logging exist where appropriate
- API/UI behavior is documented
- rollback behavior is verified when applicable
- build/test/lint passes
