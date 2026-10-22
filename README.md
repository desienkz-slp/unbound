<div align="center">
  <img src="assets/logo.png" alt="Netora DNS Logo" width="150" />
  <h1>Netora DNS</h1>
  <p><strong>Carrier-Grade Autonomous DNS Resolver Platform for ISPs</strong></p>
  <p>
    <img src="https://img.shields.io/badge/Release-v1.1.7-cyan?style=flat-square" alt="Release" />
    <img src="https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat-square&logo=go" alt="Go" />
    <img src="https://img.shields.io/badge/React-18+-61DAFB?style=flat-square&logo=react" alt="React" />
    <img src="https://img.shields.io/badge/PostgreSQL-15+-336791?style=flat-square&logo=postgresql" alt="PostgreSQL" />
    <img src="https://img.shields.io/badge/Ubuntu-20.04%20|%2022.04%20|%2024.04-E95420?style=flat-square&logo=ubuntu" alt="Ubuntu" />
    <img src="https://img.shields.io/badge/Debian-11%20|%2012-A81D33?style=flat-square&logo=debian" alt="Debian" />
    <img src="https://img.shields.io/badge/Throughput-30K+%20QPS-brightgreen?style=flat-square" alt="QPS" />
    <img src="https://img.shields.io/badge/License-Proprietary-purple?style=flat-square" alt="License" />
  </p>
</div>

---

Netora DNS is a production-grade DNS management platform built on top of [NLnet Labs Unbound](https://nlnetlabs.nl/projects/unbound/about/). It provides a REST API control plane, real-time NOC web dashboard, policy-based filtering with suffix-trie lookups, multi-node cluster management, and autonomous zero-downtime fleet updates — all designed for ISP and carrier network operations.

## Table of Contents

- [Key Features](#key-features)
- [Architecture](#architecture)
- [Project Structure](#project-structure)
- [Quick Start](#quick-start)
- [System Requirements](#system-requirements)
- [Installer Flags](#installer-flags)
- [NOC Web Dashboard](#noc-web-dashboard)
- [CLI Reference](#cli-reference)
- [Development](#development)
- [Operational Guides](#operational-guides)
- [License](#license)

---

## Key Features

| Feature | Description |
|:---|:---|
| **High-Throughput DNS** | 30,000+ QPS on 2 vCPU, 1.22 ms P50 latency, auto-tuned caches to available RAM |
| **Suffix-Trie Policy Filter** | 358 ns/query domain blocking — adult, gambling, ads, malware, custom blocklists |
| **Atomic Config & Rollback** | Every change versioned with SHA256. 1-click rollback in < 150 ms, zero dropped packets |
| **Multi-Node HA Cluster** | Heartbeat, health evaluation, and config sync across DNS fleet |
| **Carrier CDN Compatibility** | `qname-minimisation: no` enforced to prevent NXDOMAIN from non-RFC CDN nameservers (Shopee/TikTok/Akamai) |
| **DNS Rewrites** | A, AAAA, CNAME, PTR, TXT with wildcard subdomain support via Unbound native `local-zone redirect` |
| **Autonomous Fleet Updates** | Pull-based CI/CD: sandbox staging → `unbound-checkconf` validation → atomic swap → auto-rollback on failure |
| **Zero Open Resolver Guard** | Semantic validation rejects `0.0.0.0/0 allow` at the config engine level |
| **Database Disaster Recovery** | Point-in-time PostgreSQL snapshots with download, restore, and deletion via API |
| **Hermes AI-NOC Diagnostics** | Deep per-domain investigation: policy trace, live resolution probe, blocklist match explanation |
| **Prometheus Metrics** | `/metrics` endpoint for Grafana integration — queries, cache, latency, blocked counts |

---

## Architecture

```
                          NETORA DNS PLATFORM

           ┌──────────────────────────────────────────────┐
           │                CONTROL PLANE                  │
           │  Web UI (React + TypeScript + Tailwind)       │
           │  REST API (Go, net/http)          :8080       │
           │  PostgreSQL                       :5432       │
           │  Cluster Manager (Multi-Node HA)              │
           │  Telemetry & Prometheus Exporter  /metrics    │
           │  MCP Server (cmd/netora-mcp)                  │
           │  Hermes AI-NOC Investigator                   │
           └────────────────────┬─────────────────────────┘
                                │
              Atomic Config     │  Staging → Validation
              & Health Check    │  → Reload (<150ms)
                                ▼
           ┌──────────────────────────────────────────────┐
           │                 DATA PLANE                    │
           │            NLnet Labs UNBOUND                 │
           │  DNS Cache (auto-tuned to RAM)                │
           │  DNSSEC Validation                            │
           │  Suffix-Trie Policy Filter                    │
           │  Recursive / Forwarding / Hybrid              │
           └──────────────────────────────────────────────┘
```

**Data plane independence**: If PostgreSQL, the API, or the Web UI crashes, Unbound continues resolving DNS queries from its flat, atomically validated config files. Zero customer impact.

---

## Project Structure

```
.
├── cmd/
│   ├── netora-api/          # REST API server (control plane)
│   │   ├── main.go          # Entrypoint, graceful shutdown
│   │   ├── server.go        # Server struct, routes, middleware
│   │   ├── handler_auth.go  # Login, logout, profile
│   │   ├── handler_node.go  # Node config, apply, rollback, stats
│   │   ├── handler_policy.go# Policies, filters, blocklists, allowlist
│   │   ├── handler_ops.go   # Health, update, diagnostics, cluster, backups
│   │   └── version.go       # Version & update status endpoints
│   ├── netora-cli/          # CLI for password reset, user management
│   ├── netora-mcp/          # Model Context Protocol server
│   └── netora-bench/        # DNS benchmark tooling
├── pkg/
│   ├── auth/                # JWT + bcrypt + RBAC (SUPER_ADMIN, ADMIN, NOC, OPERATOR)
│   ├── backup/              # PostgreSQL snapshot disaster recovery
│   ├── blocklist/           # Feed sync, scheduler, domain aggregation
│   ├── cluster/             # Multi-node heartbeat, sync, HA state machine
│   ├── configengine/        # Atomic config generation, apply, rollback
│   ├── hermes/              # AI-NOC deep diagnostic investigation
│   ├── logrotate/           # Host log rotation management
│   ├── model/               # Domain models, DesiredState, validation
│   ├── policy/              # Suffix-trie domain filter engine
│   ├── runtime/             # DNS health checker, system detection
│   ├── security/            # Input sanitization, security utilities
│   ├── storage/             # PostgreSQL store interface & implementation
│   └── telemetry/           # Prometheus metrics, query log tailer, alerts
├── web/                     # React + TypeScript + Tailwind NOC dashboard
├── scripts/                 # Operational scripts (CLI wrapper, updater)
├── install.sh               # Universal 1-line installer
├── uninstall.sh             # Clean removal script
└── deploy/                  # Build artifacts & deployment assets
```

---

## Quick Start

### One-Line Install (Interactive)

```bash
curl -sSL https://raw.githubusercontent.com/desienkz-slp/unbound/main/install.sh | sudo bash
```

### Silent Fleet Provisioning

```bash
curl -sSL https://raw.githubusercontent.com/desienkz-slp/unbound/main/install.sh | sudo bash -s -- \
  --silent \
  --node-id "dns-jakarta-01" \
  --mode recursive \
  --web-port 8080 \
  --admin-pass "YourSecurePassword"
```

### Private Repository

```bash
TOKEN="github_pat_yourFineGrainedTokenHere"

curl -H "Authorization: token $TOKEN" \
  -sSL https://raw.githubusercontent.com/desienkz-slp/unbound/main/install.sh \
  | sudo bash -s -- \
  --silent \
  --node-id "dns-jakarta-01" \
  --admin-pass "YourSecurePassword" \
  --github-token "$TOKEN"
```

> [!IMPORTANT]
> Use a **Fine-Grained PAT** scoped to this repository with **Contents: Read-only** and **Metadata: Read-only** permissions.

### Unbound Version Compatibility

The installer auto-detects Unbound version (1.6+). For full serve-stale support, add `--unbound-ppa` to install the latest from NLnet Labs PPA (Ubuntu only, opt-in).

---

## System Requirements

| Requirement | Minimum | Recommended (ISP) |
|:---|:---|:---|
| **OS** | Ubuntu 20.04+ / Debian 11+ | Ubuntu 22.04 LTS |
| **CPU** | 1 vCPU (x86_64 / arm64) | 4+ vCPUs |
| **RAM** | 1 GB | 4–16 GB (cache auto-tuned) |
| **Disk** | 5 GB SSD | 20 GB+ NVMe |
| **Network** | 1 Gbps | 2.5–10 Gbps |

### Port Allocation

| Port | Protocol | Service |
|:---|:---|:---|
| `53` | UDP/TCP | Unbound DNS (data plane) |
| `8080` | TCP | REST API + Web UI (control plane) |
| `5432` | TCP | PostgreSQL (loopback only) |
| `8953` | TCP | Unbound remote control (loopback, TLS) |

---

## Installer Flags

| Flag | Default | Description |
|:---|:---|:---|
| `-y`, `--silent` | `false` | Non-interactive mode |
| `--mode <mode>` | `recursive` | `recursive` (IANA root hints) or `forwarder` (Cloudflare/Google) |
| `--node-id <id>` | hostname | Cluster node identifier |
| `--web-port <port>` | `8080` | Control plane port |
| `--dns-port <port>` | `53` | DNS port |
| `--admin-pass <pw>` | `admin123` | Web UI admin password |
| `--db-pass <pw>` | auto-generated | PostgreSQL password |
| `--subnets "<cidrs>"` | RFC1918 ranges | Allowed client CIDRs (space-separated) |
| `--github-token <tok>` | — | PAT for private repo access |
| `--no-auto-update` | `false` | Disable systemd auto-update timer |
| `--unbound-ppa` | `false` | Install latest Unbound from NLnet Labs PPA (Ubuntu) |

---

## NOC Web Dashboard

After installation, access the dashboard at `http://<SERVER_IP>:8080/`.

**Default credentials:** `admin` / `admin123` (or custom `--admin-pass`).

### Dashboard Capabilities

- **Overview** — Real-time query counts, cache hit ratio, top clients, top queried/blocked domains, upstream latency
- **DNS Configuration** — Listeners, upstream DoT presets, cache tuning, DNS rewrites (wildcard support)
- **Policy & Filtering** — Blocklist feeds (auto-update every 12h), adult/gambling/ads/malware toggles, custom allowlist
- **Fleet Management** — Version badge with Git SHA, 1-click update trigger, update status monitor
- **Audit & Diagnostics** — Domain resolution inspector, full audit log trail, Hermes deep diagnostics

### API Endpoints

| Endpoint | Description |
|:---|:---|
| `GET /api/v1/health` | Health check |
| `GET /metrics` | Prometheus telemetry |
| `POST /api/v1/auth/login` | JWT authentication |
| `GET /api/v1/nodes/{id}/config` | Node desired state |
| `POST /api/v1/nodes/{id}/apply` | Apply configuration |
| `POST /api/v1/nodes/{id}/rollback` | Rollback to version |
| `GET /api/v1/filters` | List blocklist feeds |
| `POST /api/v1/blocklists/sync` | Force blocklist sync |
| `POST /api/v1/diagnose` | Domain diagnostics |
| `POST /api/v1/hermes/investigate` | Deep AI-NOC investigation |
| `GET /api/v1/cluster/status` | Cluster health & nodes |
| `GET /api/v1/audit` | Audit logs |

---

## CLI Reference

The installer places `netora` at `/usr/local/bin/netora`:

```bash
netora status              # System health, Unbound status, updater state
netora health              # DNS resolution & API health checks
netora update              # Pull & apply latest from GitHub
netora rollback            # Restore previous config snapshot
netora restart             # Graceful restart Unbound + API
netora logs unbound|api    # Stream live logs
netora info                # Access endpoints, IP topology, credentials
netora reset-password      # Interactive password reset
netora users               # List registered operators
netora set-token <pat>     # Update GitHub PAT (auto-verifies access)
```

For direct database operations (disaster recovery):

```bash
netora-cli -mode reset-password -username admin -new-password "NewPass"
netora-cli -mode list-users
netora-cli -mode create-user -username noc_op -new-password "Pass" -role OPERATOR
```

---

## Development

### Prerequisites

- Go 1.21+
- Node.js 18+ (for web UI)
- PostgreSQL 15+ (or use the installer for automated setup)

### Build

```bash
# Backend
go build -o netora-api ./cmd/netora-api/
go build -o netora-cli ./cmd/netora-cli/

# Frontend
cd web && npm install && npm run build

# Run tests
go test ./...
```

### RBAC Roles

| Role | Permissions |
|:---|:---|
| `SUPER_ADMIN` | Full access |
| `ADMIN` | All except user management |
| `NOC` | Read nodes, diagnostics, audit, policy:read |
| `OPERATOR` | Read-only node status |

---

## Operational Guides

Detailed guides are maintained in `docs/`:

- [Autonomous CI/CD Engine SOP](docs/auto-update.md) — Update engine internals, staging, rollback
- [Carrier Rollback & Recovery](docs/rollback.md) — Atomic rollback procedure
- [Multi-Node Cluster Deployment](docs/deployment.md) — HA fleet setup
- [Password Reset Guide](docs/password-reset.md) — CLI, interactive, and emergency fallback methods
- [GitHub Token Rotation](docs/token-rotation.md) — PAT renewal & verification

### Auto-Update Engine

The platform uses decentralized pull-based deployment:

1. **Systemd timer** runs every 5 minutes with randomized jitter
2. Candidates are staged in `/opt/netora-dns/staging/`
3. Validated with `unbound-checkconf` before activation
4. Atomic symlink swap at `/opt/netora-dns/current`
5. Auto-rollback if health check fails post-activation
6. Node IP topology and local configuration are preserved across updates

---

## License

Proprietary. © Netora DNS Platform — Engineered for High-Reliability Internet Service Providers.
