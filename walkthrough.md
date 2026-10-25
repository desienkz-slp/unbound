# Walkthrough — Client-Tier Tuning System

**Date:** 2026-10-25
**Scope:** pkg/model/config.go

---

## Ringkasan Perubahan

Sistem tuning sebelumnya hanya berbasis **hardware** (CPU cores + RAM).
Sekarang ditambahkan layer baru: **client tier** berdasarkan jumlah subscriber IP yang dilayani node.

---

## Tier System

| Tier | Batas Client | TIER Constant |
|---|---|---|
| Small | 0 - 500 client | TierSmall = TIER_SMALL |
| Medium | 501 - 2.000 client | TierMedium = TIER_MEDIUM |
| Large | > 2.000 client | TierLarge = TIER_LARGE |

---

## Parameter yang Dikontrol per Tier

| Parameter Unbound | Small (<=500) | Medium (<=2K) | Large (>2K) |
|---|---|---|---|
| num-queries-per-thread | 1024 | 4096 | 8192 |
| outgoing-range | 4096 | 8192 | 32768 |
| incoming-num-tcp | 256 | 512 | 2048 |
| so-rcvbuf / so-sndbuf | 2m | 4m | 8m |
| cache-min-ttl (detik) | 60 | 45 | 30 |
| ip-ratelimit (QPS/IP) | 200 | 500 | 1000 |
| ratelimit (QPS/domain) | 1000 | 2000 | 5000 |
| ip-ratelimit-size | 4m | 8m | 16m |
| infra-host-ttl (detik) | 900 | 900 | 1800 |

Thread count & cache size tetap dikontrol oleh hardware (cores/RAM).
Tier hanya override buffer, queue, TCP, dan rate-limit parameters.

---

## File yang Diubah

pkg/model/config.go satu-satunya file yang dimodifikasi.

### Penambahan baru:
1. ClientTier type + konstanta TierSmall, TierMedium, TierLarge
2. ClientTierFromCount(n int) ClientTier - mapping jumlah client ke tier
3. ClientTierTuning struct - container semua parameter per tier
4. clientTierTable map - SSOT tabel nilai tuning per tier
5. GetClientTierTuning(n int) ClientTierTuning - lookup dari jumlah client
6. AutoTuneByClientCount(clientCount, cores, ramMB int) - entry point utama
7. DesiredState.ClientCount int field baru (JSON: client_count, opsional)
8. DesiredState.ApplyClientTier(clientCount, cores, ramMB int) - patch non-destruktif

### Update:
NewDefaultDesiredState -> menggunakan AutoTuneByClientCount(0, cores, ramMB) + rate limit dari TierMedium

---

## Cara Penggunaan

### Buat state baru dengan tier:
  state := model.NewDefaultDesiredState(nodeID, cores, ramMB)
  state.ApplyClientTier(1500, cores, ramMB) // 1500 client -> TierMedium

### Patch state eksisting dari database (non-destruktif):
  state.ApplyClientTier(clientCount, cores, ramMB)

### Query tier tanpa apply:
  tier   := model.ClientTierFromCount(300)      // -> TierSmall
  tuning := model.GetClientTierTuning(300)      // -> ClientTierTuning{...}

---

## Jaminan Non-Destruktif

ApplyClientTier() TIDAK menyentuh:
- Upstreams, ClientNetworks, Listeners
- DNSSEC, LocalRecords, BlocklistFeeds
- SafeSearch, ParentalControl, GamblingBlock
- Performance.NumThreads dan semua *CacheSlabs

---

## Test Results

  go test ./pkg/model/... -> PASS (11 tests, 0.415s)
  go build ./...           -> OK (exit 0)
