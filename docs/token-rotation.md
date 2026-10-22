# GitHub Token Rotation Guide

The GitHub PAT is stored at `/etc/netora-dns/github.token` (mode `600`, root-only). It's used by the auto-updater, systemd timer, and Web UI 1-click updates.

## When to Rotate

- Token expired (Fine-Grained PATs have configurable expiry: 30/60/90 days)
- Security policy rotation schedule
- Symptoms: `netora update` fails with `Authentication failed`, Web UI update button shows error

## Token Requirements

Create at **GitHub → Settings → Developer Settings → Fine-grained tokens**:

- **Repository access:** Only select this repository
- **Permissions:** Contents: Read-only, Metadata: Read-only

## Update Methods

### Method 1: CLI (Recommended)

```bash
sudo netora set-token "github_pat_newTokenHere"
```

Interactive mode (token hidden from shell history):

```bash
sudo netora set-token
```

### Method 2: One-Liner

```bash
echo "github_pat_newTokenHere" | sudo tee /etc/netora-dns/github.token > /dev/null && sudo chmod 600 /etc/netora-dns/github.token
```

### Method 3: Manual Edit

```bash
sudo nano /etc/netora-dns/github.token
# Replace content with new token (single line, no trailing whitespace)
sudo chmod 600 /etc/netora-dns/github.token
```

## Verification

> **Zero-downtime:** No service restart needed. Token is read dynamically on each update check.

```bash
# Verify Git access
TOKEN=$(sudo cat /etc/netora-dns/github.token)
git ls-remote "https://${TOKEN}@github.com/desienkz-slp/unbound.git" refs/heads/main

# Test update
sudo netora update

# API check
curl -s http://127.0.0.1:8080/api/v1/system/version | jq .
```
