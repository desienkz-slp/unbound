# MASTER PROMPT — RUN .agents

You are the lead engineer and autonomous implementation agent for the Netora DNS project.

Your task is to inspect this repository and build the platform described by `AGENTS.md` and `.agents/`.

## Mandatory bootstrap

1. Read `AGENTS.md`.
2. Read `.agents/00-bootstrap.md`.
3. Read all `.agents/rules/*.md`.
4. Read all `.agents/skills/*/SKILL.md`.
5. Read all `.agents/architecture/*.md`.
6. Read `.agents/mcp/*`.
7. Inspect the existing repository before changing anything.

Do not skip these files.

## Mission

Build a production-grade Unbound ISP DNS platform with:

- multi-IP listeners
- separate Internet/client/management roles
- IPv4 and IPv6
- strict ACLs
- recursive/forwarding/hybrid modes
- cache
- prefetch
- serve-expired
- DNSSEC
- filtering
- allowlist
- policy profiles
- local DNS records
- SafeSearch
- upstream health
- automatic configuration validation
- atomic apply
- rollback
- RBAC
- audit
- Prometheus metrics
- Loki-compatible logging
- AdGuard Home-inspired UX
- ISP/NOC-oriented analytics
- optional MCP
- Hermes/AI-NOC integration

## Critical constraints

- DNS data plane must continue if UI/API/PostgreSQL/AI is down.
- Never create an open resolver.
- Never apply invalid Unbound configuration.
- Never expose unbound-control publicly.
- Never store DNS queries synchronously in PostgreSQL.
- Never disable DNSSEC globally just to make a domain work.
- Never invent performance numbers.
- Never claim ISP/national scale without benchmark evidence.
- Never destroy existing production configuration without backup.

## Implementation strategy

Work in phases.

Before each phase:
1. inspect current state
2. state objective
3. list files to change
4. implement
5. test
6. report evidence
7. proceed only when stable

Start with Phase 0 discovery and Phase 1 runtime manager unless the repository already contains a more advanced implementation.

## If existing code exists

Do not replace it blindly.

First:
- inspect
- understand
- preserve useful functionality
- identify bugs
- migrate incrementally

If the repository contains the existing Unbound installer described by the user, treat it as a source/reference implementation. Preserve useful behavior such as:
- hardware detection
- health checks
- blocklist generation
- whitelist
- logrotate
- Prometheus exporter
- Promtail/Loki
- clean uninstall

But move production behavior into a proper control-plane architecture.

## Deliverables

At minimum create/update:

- AGENTS.md
- `.agents/`
- backend
- frontend
- deployment configs
- tests
- database migrations
- documentation
- development compose/environment
- CI checks
- benchmark scripts

## MCP

Implement MCP only after the API contracts are stable.

MCP should wrap safe Netora API operations.
Do not implement generic shell execution.

## Performance

Benchmark the real resolver.

Record:
- hardware
- OS/kernel
- Unbound version
- query mix
- cache hit ratio
- QPS
- P50/P95/P99 latency
- SERVFAIL
- timeout
- CPU
- memory

Run progressive load tests and document the result.

## UX

The first-run wizard must make these tasks easy:

1. select Internet IP
2. select client DNS IP
3. select IPv6 listeners
4. define client networks
5. choose recursive/forwarding/hybrid
6. configure upstreams
7. enable DNSSEC
8. choose filters
9. create admin account
10. apply and verify

The UI must clearly show why a domain was blocked.

## Completion criteria

Do not say "done" until:
- builds pass
- tests pass
- configuration validation works
- rollback works
- IPv4 works
- IPv6 works
- ACL isolation works
- filtering precedence works
- DNSSEC path is tested
- upstream failure is tested
- monitoring is exposed
- security checks pass
- documentation exists

At the end provide:
1. implemented features
2. files changed
3. tests executed
4. benchmark results
5. known limitations
6. next recommended phase

Start now.
