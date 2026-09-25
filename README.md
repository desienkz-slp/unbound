<div align="center">
  <img src="assets/logo.png" alt="Netora DNS Logo" width="150" />
  <h1>Netora DNS — Enterprise Carrier-Grade ISP Platform</h1>
  <p><strong>Autonomous High-Speed Unbound DNS Resolver with Real-Time NOC Telemetry, Suffix-Trie Filtering, and Zero-Downtime CI/CD Fleet Auto-Updates.</strong></p>

  <p>
    <img src="https://img.shields.io/badge/Ubuntu-20.04%20|%2022.04%20|%2024.04-orange?style=flat-square&logo=ubuntu" alt="Ubuntu" />
    <img src="https://img.shields.io/badge/Debian-11%20|%2012-red?style=flat-square&logo=debian" alt="Debian" />
    <img src="https://img.shields.io/badge/Unbound-1.13+-blue?style=flat-square&logo=dns" alt="Unbound" />
    <img src="https://img.shields.io/badge/Throughput-30K+%20QPS-brightgreen?style=flat-square" alt="QPS" />
    <img src="https://img.shields.io/badge/Rollback-Atomic%20%3C150ms-emerald?style=flat-square" alt="Atomic Rollback" />
    <img src="https://img.shields.io/badge/License-Proprietary-purple?style=flat-square" alt="License" />
  </p>
</div>

---

## ⚡ Quick Start — Auto-Install Script

Deploy a production-ready Netora DNS node on any fresh Ubuntu or Debian server in under 2 minutes:

### 1. One-Line Standard Installation (Interactive)

```bash
curl -sSL https://raw.githubusercontent.com/desienkz-slp/unbound/main/install.sh | sudo bash
```
*(Or using `wget`: `wget -qO- https://raw.githubusercontent.com/desienkz-slp/unbound/main/install.sh | sudo bash`)*

### 2. Non-Interactive / Automated Fleet Installation (Silent Mode)

For automated provisioning (Cloud-Init, Ansible, or PXE boot), pass `-y` and configuration flags:

```bash
curl -sSL https://raw.githubusercontent.com/desienkz-slp/unbound/main/install.sh | sudo bash -s -- \
  --silent \
  --node-id "dns-jakarta-01" \
  --mode recursive \
  --web-port 8080 \
  --admin-pass "SuperSecretNocPass123"
```

### 3. Private GitHub Repository Installation

If your repository is private, provide your GitHub Personal Access Token (PAT):

```bash
export GITHUB_TOKEN="ghp_yourPersonalAccessTokenHere"
curl -H "Authorization: token $GITHUB_TOKEN" -sSL https://raw.githubusercontent.com/desienkz-slp/unbound/main/install.sh | sudo -E bash
```

---

## 🖥️ NOC Web Dashboard & Default Credentials

Once the installation finishes, open your browser and navigate to:

* **URL:** `http://<YOUR_SERVER_IP>:8080/`
* **Default Username:** `admin`
* **Default Password:** `admin123` *(or the custom password provided during installation)*
* **Prometheus Telemetry:** `http://<YOUR_SERVER_IP>:8080/metrics`
* **API Health Check:** `http://<YOUR_SERVER_IP>:8080/api/v1/health`

---

## 📋 System Requirements & Port Matrix

| Requirement | Minimum | Recommended for High-Traffic ISP |
|:---|:---|:---|
| **Operating System** | Ubuntu 20.04 / 22.04 / 24.04 LTS, Debian 11 / 12 | Ubuntu 22.04 LTS (Kernel 5.15+) |
| **CPU Architecture** | 1 vCPU (x86_64 / amd64 or arm64) | 4+ vCPUs |
| **Memory (RAM)** | 1 GB RAM | 4 GB - 16 GB RAM (Auto-tuned cache) |
| **Disk Storage** | 5 GB SSD / NVMe | 20 GB+ NVMe SSD |
| **Network Interfaces** | 1 Gbps Ethernet | 2.5 Gbps - 10 Gbps Ethernet |

### Network Port Allocation
* **`53/UDP` & `53/TCP`:** Unbound DNS Data Plane (Client & Upstream Queries)
* **`8080/TCP`:** Netora Control Plane REST API & React NOC Web UI
* **`5432/TCP`:** PostgreSQL Database (`127.0.0.1` loopback only)
* **`8953/TCP`:** Unbound Remote Control (`127.0.0.1` loopback with TLS certificates)

---

## ⚙️ Installer Command-Line Flags

| Option | Default | Description |
|:---|:---|:---|
| `-y`, `--silent` | `false` | Run without interactive prompts, accepting all default values. |
| `--mode <mode>` | `recursive` | DNS resolution engine: `recursive` (IANA Root Hints) or `forwarder` (Cloudflare/Google). |
| `--node-id <id>` | *hostname* | Unique cluster node identifier displayed in NOC telemetry. |
| `--web-port <port>` | `8080` | TCP port for NOC Web UI Dashboard and REST API. |
| `--dns-port <port>` | `53` | UDP/TCP port for DNS query answering. |
| `--admin-pass <pass>`| `admin123` | Master password for operator authentication in the Web UI. |
| `--db-pass <pass>` | `netora_dev_password` | Dedicated PostgreSQL password for the `netora` role. |
| `--subnets "<subnets>"`| `10.0.0.0/8 172.16.0.0/12 192.168.0.0/16 127.0.0.0/8` | Space-separated list of allowed client CIDR ranges. |
| `--channel <chan>` | `development` | Release channel for updates: `development` or `production`. |
| `--github-token <tok>`| *empty* | GitHub Token for pulling private repositories securely. |
| `--no-auto-update` | `false` | Disable the autonomous systemd auto-update timer. |
| `-h`, `--help` | — | Display help information and exit. |

---

## 🚀 Architectural Invariants & Key Capabilities

```
                              NETORA DNS PLATFORM
                              
               ┌────────────────────────────────────────────────┐
               │                 CONTROL PLANE                  │
               │  • Web UI (React/TS/Tailwind - Dark NOC Theme) │
               │  • REST API (cmd/netora-api :8080)             │
               │  • PostgreSQL (Database on 127.0.0.1:5432)     │
               │  • Cluster Manager (Multi-node HA)             │
               │  • Telemetry & Prometheus Exporter (/metrics)  │
               │  • Model Context Protocol (cmd/netora-mcp)     │
               │  • Hermes AI-NOC Investigator                  │
               └───────────────────────┬────────────────────────┘
                                       │ 
                 Atomic Configuration  │ Staging, Validation,
                 & Health Verification │ & Reload (<150ms)
                                       ▼
               ┌────────────────────────────────────────────────┐
               │                  DATA PLANE                    │
               │             NLnet Labs UNBOUND                 │
               │  • High-Speed DNS Cache (Auto-tuned to RAM)    │
               │  • Cryptographic DNSSEC Validation             │
               │  • Suffix-Trie Policy Filter (358ns / query)   │
               │  • Carrier CDN Optimization (Anti-Bengong)     │
               │  • Zero Open Resolver Semantic Guardrails       │
               │  • Recursive / Forwarding / Hybrid Modes       │
               └────────────────────────────────────────────────┘
```

1. **Total Data Plane Independence (Anti-Bengong):**  
   Unbound resolves DNS queries directly from flat, atomically validated configurations. If PostgreSQL, the REST API, or the Web UI crashes, **Unbound continues answering 100% of customer DNS queries without missing a single packet**.
2. **Verified High-Throughput Performance:**  
   Benchmarked at **30,445+ QPS** on a modest 2 vCPU server with **1.22 ms** median latency (P50).
3. **Carrier CDN Compatibility (Critical Invariant #16):**  
   Strict `qname-minimisation: no` is enforced to prevent non-RFC-compliant CDN authoritative nameservers (such as Baishan Cloud / Shopee CDN / TikTok / Akamai) from returning incorrect `NXDOMAIN` on intermediate empty non-terminals.
4. **Versions & Atomic 1-Click Rollback:**  
   Every configuration change generates a versioned snapshot with SHA256 verification. If an operator misconfigures a rule, 1-click rollback restores the previous state in **< 150 ms** with zero dropped packets.
5. **Zero Open Resolver Guarantee:**  
   The configuration engine rejects arbitrary internet client access (`0.0.0.0/0 allow`) at the semantic validation level.

---

## 🛠️ NOC Operator CLI Management (`netora`)

The installer places the convenient `netora` CLI tool into `/usr/local/bin/netora`:

```bash
# 1. Check overall system health, Unbound status, and updater state
netora status

# 2. Run instant DNS resolution and API health checks
netora health

# 3. Pull and atomically apply the latest revision from GitHub
netora update

# 4. Rollback to the previous stable configuration snapshot
netora rollback

# 5. Gracefully restart Unbound and Netora API
netora restart

# 6. Stream live service logs
netora logs unbound
netora logs api
netora logs updater

# 7. Print access endpoints, IP topology, and operator credentials
netora info
```

---

## 🔄 Autonomous CI/CD Auto-Update Engine

Netora DNS uses a decentralized pull-based deployment model:
* **Background Timer:** `/etc/systemd/system/netora-dns-updater.timer` runs every 5 minutes with randomized jitter to prevent server thundering herds.
* **Sandboxed Staging:** Candidates from GitHub are assembled in `/opt/netora-dns/staging/` and pre-validated using `unbound-checkconf`.
* **Atomic Swap:** Symlink `/opt/netora-dns/current` is atomically updated and Unbound is reloaded via `unbound-control reload`.
* **Auto-Rollback on Failure:** If reload fails or post-activation health checks fail, the previous verified configuration is restored within seconds.

---

## 🗑️ Uninstallation Runbook

To completely remove Netora DNS and restore the default system resolver (`systemd-resolved`):

```bash
curl -sSL https://raw.githubusercontent.com/desienkz-slp/unbound/main/uninstall.sh | sudo bash
```
*(Or run `sudo ./uninstall.sh` from the repository root)*

---

## 📚 Technical Documentation & References

* [Final Engineering Walkthrough & Audit Report](walkthrough.md)
* [Autonomous CI/CD Engine SOP](docs/auto-update.md)
* [Carrier Rollback & Recovery Guide](docs/rollback.md)
* [Multi-Node Cluster Deployment](docs/deployment.md)
* [DNS Validation & CDN Compatibility Skill](.agents/skills/dns-validation/SKILL.md)

---

<div align="center">
  <small>© Netora DNS Platform • Engineered for High-Reliability Internet Service Providers.</small>
</div>
