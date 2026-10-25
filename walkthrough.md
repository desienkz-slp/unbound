# Walkthrough — Client-Tier Tuning & Release v1.1.17

**Date:** 2026-10-25  
**Version:** `v1.1.17` (Build 167)  
**Scope:** `pkg/model/config.go`, `deploy/bin/`, `deploy/ui/`, `version.json`, `web/scripts/update-version.cjs`

---

## 1. Ringkasan Perubahan

### A. Client-Tier Tuning System
Sistem tuning sebelumnya hanya berbasis **hardware** (CPU cores + RAM).  
Sekarang ditambahkan layer baru: **client tier** berdasarkan jumlah subscriber IP yang dilayani node:

| Tier | Batas Client | TIER Constant |
|---|---|---|
| **Small** | 0 – 500 client | `TierSmall` = `TIER_SMALL` |
| **Medium** | 501 – 2.000 client | `TierMedium` = `TIER_MEDIUM` |
| **Large** | > 2.000 client | `TierLarge` = `TIER_LARGE` |

#### Parameter yang Dikontrol per Tier
| Parameter Unbound | Small (<=500) | Medium (<=2K) | Large (>2K) |
|---|---|---|---|
| `num-queries-per-thread` | 1024 | 4096 | 8192 |
| `outgoing-range` | 4096 | 8192 | 32768 |
| `incoming-num-tcp` | 256 | 512 | 2048 |
| `so-rcvbuf` / `so-sndbuf` | 2m | 4m | 8m |
| `cache-min-ttl` (detik) | 60 | 45 | 30 |
| `ip-ratelimit` (QPS/IP) | 200 | 500 | 1000 |
| `ratelimit` (QPS/domain) | 1000 | 2000 | 5000 |
| `ip-ratelimit-size` | 4m | 8m | 16m |
| `infra-host-ttl` (detik) | 900 | 900 | 1800 |

*Hardware thread count dan cache slabs tetap dikontrol oleh CPU/RAM. Client tier hanya meng-override buffer, queue, TCP, dan rate limits secara non-destruktif.*

---

## 2. Release & Version Control Synchronization (v1.1.17)

### A. Version Standardization
Semua file versi telah diselaraskan ke **`v1.1.17`**:
- `version.json` (Root SSOT)
- `web/src/version.json` (Frontend)
- `web/public/version.json` (Public assets)
- `deploy/ui/version.json` (Production static)
- `cmd/netora-api/version.go` (Go control plane fallback)

### B. Anti-Drift Version Script (`web/scripts/update-version.cjs`)
Script build frontend diperbaiki agar membaca dan memprioritaskan versi eksplisit dari `version.json` (atau env `VERSION`). Hal ini mencegah versi melompat secara tak terduga saat `npm run build` dijalankan.

### C. Target Build Binary Produksi
Server production auto-updater (`/usr/local/sbin/netora-dns-update`) meng-copy binary dari folder `deploy/bin/`.
Binary Linux amd64 telah di-compile langsung ke:
- `deploy/bin/netora-api-linux-amd64` (11.08 MB)
- `deploy/bin/netora-cli-linux-amd64` (8.61 MB)

### D. Web UI Production Build
Frontend Vite telah di-build dan di-sinkronisasikan ke `deploy/ui/` (`index.html`, bundle assets, dan `version.json`).

---

## 3. Verifikasi & Pengujian
- **Go Unit Tests:** `go test ./...` → **PASS** (semua package lolos tanpa error).
- **Frontend Build:** `npm run build` → **PASS** (bundle dihasilkan dan tersinkronisasi ke `deploy/ui`).
- **Binary Architecture:** Linux amd64 ELF stripped (`-s -w`).

---

## 4. Mekanisme Penerapan ke Node Server
Setelah commit dan push ke `origin main`:
1. **Auto-Update (Otomatis):** `netora-dns-updater.timer` di Node 1 (172.18.20.146) dan Node 2 (172.18.20.147) berjalan setiap 5 menit. Updater akan menarik commit terbaru, menyalin binary `deploy/bin/netora-api-linux-amd64`, memperbarui static assets `/var/www/netora-ui`, dan me-restart service `netora-api`.
2. **Manual Update (Instan):** Operator dapat menekan tombol **"Update Platform"** di footer UI web pada masing-masing server.
