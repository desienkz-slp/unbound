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
