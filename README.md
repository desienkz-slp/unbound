# 🛡️ Unbound DNS — Anti-Bengong Installer

Automated Unbound DNS resolver installer for Ubuntu Server with DoH forwarding, recursive resolution, blocklist management, and monitoring add-ons.

## ✨ Features

- **Interactive Installer** — Choose between DoH forwarding (via cloudflared) or full recursive resolution
- **Auto-Tuning** — Automatically detects CPU/RAM and optimizes Unbound for your server
- **Serve-Expired** — Zero-downtime DNS with stale cache serving
- **Blocklist Engine** — Aggregated ad/tracker/malware blocking with custom whitelist support
- **Adult Content Scheduling** — Time-based adult domain blocking via cron
- **Monitoring Ready** — Optional Prometheus exporter & Promtail (Loki) integration
- **AppArmor Compatible** — Proper security profile configuration
- **Zero-Downtime Logrotate** — USR1 signal-based log rotation

## 📁 Structure

```
unbound/
├── install-dns.sh          # Main interactive installer
├── remove-dns.sh           # Clean uninstaller
├── lib/
│   └── helpers.sh          # Shared utility functions
├── conf/
│   ├── unbound-base.conf   # Base Unbound configuration template
│   ├── local-records.conf  # Local DNS records (A/PTR)
│   ├── domain-insecure.conf # DNSSEC-insecure domains
│   └── logrotate-unbound.conf
├── blocklist/
│   ├── gen-blocklist.sh    # Blocklist generator & aggregator
│   ├── update-lists.sh     # Cron-friendly blocklist updater
│   ├── block_mine.txt      # Custom block entries
│   ├── adult-domains.txt   # Adult domain sources
│   └── whitelist.txt       # Whitelist overrides
└── addons/
    ├── install-exporter.sh # Prometheus unbound_exporter
    ├── install-promtail.sh # Promtail log shipper
    └── setup-jadwal.sh     # Adult-content schedule manager
```

## 🚀 Quick Start

```bash
# Clone the repository
git clone https://github.com/<YOUR_USERNAME>/unbound.git
cd unbound

# Run the installer
sudo bash install-dns.sh
```

## 📋 Requirements

- Ubuntu 20.04+ / Debian 11+
- Root or sudo access
- Internet connectivity

## 🗑️ Uninstall

```bash
sudo bash remove-dns.sh
```

## 📜 License

MIT
