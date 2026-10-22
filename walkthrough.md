# Walkthrough — NVR-Style Auto-Purge Arsip Log (.gz)

## Fitur Baru
Sistem auto-purge yang memantau disk usage setiap 5 menit dan otomatis menghapus arsip `.gz` tertua saat threshold tercapai — seperti NVR CCTV yang menghapus rekaman lama saat HDD penuh.

## Komponen Perubahan

### Backend (Go)
| File | Perubahan |
|---|---|
| `pkg/model/config.go` | +2 field: `auto_purge`, `auto_purge_threshold` |
| `pkg/logrotate/purger.go` | Core purge logic: collect archives oldest-first, delete with rate limit & safety |
| `pkg/logrotate/purger_linux.go` | Disk usage via `syscall.Statfs` |
| `pkg/logrotate/purger_other.go` | Non-Linux stub (simulated 40%) |
| `pkg/logrotate/manager.go` | Background goroutine lifecycle, PurgeStatus tracking |
| `pkg/logrotate/model.go` | +`PurgeStatus` struct, +`auto_purge_status` di report |
| `pkg/logrotate/engine.go` | AI profiles include auto-purge: STORAGE_SAVER=50%, BALANCED=75%, ISP_AUDIT=90% |

### Frontend (TypeScript/React)
| File | Perubahan |
|---|---|
| `web/src/types.ts` | +`PurgeStatus` interface, auto_purge fields di `LogrotatePolicy` & `LogrotateReport` |
| `web/src/views/PerformanceView.tsx` | Toggle auto-purge + 3 tombol threshold (50/75/90%) + live status indicator |

## Safety Mechanisms
- Minimal 1 arsip selalu dipertahankan
- Max 5 file dihapus per cycle (rate limit)
- Re-check disk setelah setiap deletion
- Graceful shutdown via context cancellation
- Hanya file pattern `unbound.log.*.gz` yang eligible

## Build & Test Status
- `go build ./...` ✅
- `go test ./pkg/logrotate/` ✅ (6/6 pass)
- `npx tsc --noEmit` ✅

## Deploy
Build binary & deploy ke server, lalu apply logrotate config via UI untuk mengaktifkan auto-purge.

---

# Walkthrough — PTR Auto-Inject Tidak Muncul di Server (Binary Lama Terdeploy)

## Masalah
Setelah update, DNS Rewrite node `dns-srnk-1` hanya punya A record. PTR tidak otomatis ditambahkan.

## Akar Masalah
`scripts/netora-dns-update.sh` menyalin `deploy/bin/netora-api-linux-amd64` ke `/usr/local/bin/netora-api`.
File itu bertanggal 17 Okt, sebelum fix PTR. Binary dengan fix ada di `deploy/netora-api`, yang tidak dipakai script.

Bukti:
- Binary di server `16059405` byte, sama ukurannya dengan `deploy/bin/netora-api-linux-amd64` lama.
- `-node-id dns-srnk-1` cocok dengan `nodes.id`, jadi bukan masalah nama node.
- Journal tidak punya log `Synchronized node ...`.
- DB `nodes.config_json.local_records` hanya berisi A (TTL 300), tanpa PTR.

## Perubahan
- Build ulang `deploy/bin/netora-api-linux-amd64` (linux/amd64, CGO off, `-trimpath -ldflags="-s -w"`).
- Commit `7ea0870`, sudah di-push ke `main`.
- Kode PTR tidak diubah.

## Verifikasi
- `go test ./pkg/model/... ./pkg/configengine/... ./pkg/storage/...` lulus.
- SHA256 binary baru: `47B726FDB5F0F3F2D8BE1F86CA38CB17AABB42E913F9035406DA8F05F759EF79`.
- Belum diverifikasi di server: update node, restart service, dan munculnya PTR di UI.

## Langkah di Server
```bash
# 1. Update lewat mekanisme yang ada (UI "Update Platform" atau script update)
# 2. Cek hash harus sama dengan hash di atas
sha256sum /usr/local/bin/netora-api
# 3. Cek log startup
journalctl -u netora-api --no-pager | grep -E "Synchronized|failed saving|failed to persist"
# 4. Cek DB
sudo -u postgres psql -d netora -c "select jsonb_pretty(config_json->'local_records') from nodes;"
```
Hasil yang benar: log `Synchronized node 'dns-srnk-1' DNS rewrites ...`, dan DB punya A + PTR untuk `172.18.20.146`.

## Risiko
- Dua binary di `deploy/` (`netora-api` dan `bin/netora-api-linux-amd64`) rawan salah pakai. Build ke `deploy/bin/netora-api-linux-amd64` setiap kali kode berubah.
- Footer UI `git:<hash>` berasal dari `version.json`/`update-state.json`, bukan dari binary API. Jangan dipakai sebagai bukti versi binary.
- Aturan proyek melarang menimpa setting lokal. Fix ini hanya menambah PTR yang hilang. A record dan rewrite lain (`isolir.sornongko.net`) tidak disentuh.
