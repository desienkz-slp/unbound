# Netora DNS — Fleet Deployment & Rolling Upgrade Strategy

## 1. Multi-Node Deployment Topology

The Netora fleet scales across independent regional PoPs:

```
[GitHub Repo: main]
       │
       ▼
 ┌─────────────┐
 │   dns-dev   │ (Phase 1: Canary node / Continuous deployment)
 └──────┬──────┘
        │ Passes CI & Automated Health Probes
        ▼
 ┌─────────────┐
 │   Staging   │ (Phase 2: Validation PoP)
 └──────┬──────┘
        │ Manual or Tag-based promotion
        ▼
 ┌─────────────┐
 │  Prod Node  │ (Phase 3: Rolling across regional PoPs)
 └─────────────┘
  ├── dns-jakarta-01
  ├── dns-jakarta-02
  └── dns-kediri-01
```

## 2. Deployment Channels

1. **`development` (Continuous Delivery):**
   * Configured on `dns-dev`.
   * Directly follows `main` branch.
   * Updates trigger automatically every 5 minutes.
2. **`staging` (Integration Testing):**
   * Follows `staging` branch or release candidates (`v*.*.*-rc*`).
3. **`production` (Controlled Carrier Release):**
   * Follows immutable release tags (`v1.0.0`, `v1.0.1`).
   * Canary rolling update with staggered timer delays to ensure zero global outage risk.

## 3. Node-Specific Configuration Overlays

Node-specific settings (listener IP addresses, management subnets, local interface bindings) must NEVER be stored in the root git repository.

Instead, each server maintains its overlays in `/etc/netora-dns/overlays/`:
- `/etc/netora-dns/overlays/conf.d/interfaces.conf`
- `/etc/netora-dns/overlays/conf.d/local-acls.conf`

During the staging phase, the updater merges the repository's base templates with the local overlays before running `unbound-checkconf`. This guarantees that repository updates never wipe out node-specific IP configurations.
