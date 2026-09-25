<div align="center">
  <img src="assets/logo.png" alt="Netora DNS Logo" width="140" />
  <h1>Netora DNS — Enterprise Carrier-Grade ISP Platform</h1>
  <p>Autonomous carrier-grade Unbound DNS platform with real-time NOC analytics, zero open resolver guardrails, and automated CI/CD.</p>
</div>

---

- Unbound = DNS data plane
- Netora API/control service = control plane
- PostgreSQL = configuration/state/audit metadata
- Prometheus = metrics
- Loki/Promtail = logs
- Web UI = operator experience inspired by AdGuard Home, but designed for ISP/NOC workflows
- Hermes/AI-NOC = optional diagnostics/automation consumer
- MCP = optional integration layer for AI agents; never required for DNS data-plane availability

Core non-negotiable principle:
DATA PLANE MUST KEEP RUNNING IF UI, API, DATABASE, OR AI LAYER IS DOWN.

## Autonomous CI/CD & Auto-Update Engine

Netora DNS nodes maintain complete operational autonomy using an asynchronous pull-based deployment agent:

* **Decentralized Polling:** Every node runs `netora-dns-update` via systemd timer to pull validated releases from GitHub.
* **Pre-Activation Staging:** Candidate releases are staged and tested with `unbound-checkconf` before touching production.
* **Anti-Open Resolver Guardrail:** Semantic checks prevent accidental exposure of `0.0.0.0/0 allow`.
* **Zero Downtime Reload:** Unbound uses atomic file replacement and `unbound-control reload`.
* **Automated Post-Activation Health Check:** Real DNS queries (localhost, DNSSEC, CDNs) are verified immediately after activation.
* **Auto-Rollback:** If reload fails or post-activation health checks fail, the previous verified configuration is restored instantly.

### Quick Commands

```bash
# View update status & health
netora-dns-update status

# Query upstream repository for updates
netora-dns-update check

# Trigger an immediate safe update
netora-dns-update update

# Revert to last-known-good backup
netora-dns-update rollback
```

For full documentation, see:
* [CI/CD Architecture](docs/cicd.md)
* [Server Update Agent](docs/auto-update.md)
* [Fleet Deployment & Overlays](docs/deployment.md)
* [Rollback & Recovery SOP](docs/rollback.md)
* [Node Registration Guide](docs/node-registration.md)

Read `.agents/00-bootstrap.md` first, then follow `.agents/rules/*`, `.agents/skills/*`, and `.agents/mcp/*`.
