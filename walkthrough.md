# Walkthrough — Fitur DNS Rewrites di Netora DNS

## Ringkasan

Fitur **DNS Rewrites** memungkinkan admin ISP/NOC untuk mengkonfigurasi jawaban DNS custom untuk domain atau wildcard subdomain tertentu — tanpa menyentuh file konfigurasi Unbound secara manual.

---

## File yang Diubah / Ditambahkan

| File | Perubahan |
|------|-----------|
| `web/src/components/DnsRewritesPanel.tsx` | **Baru** — Komponen panel lengkap (tabel, modal Add/Edit, filter chips, search) |
| `web/src/views/DNSConfigView.tsx` | Tambah sub-tab DNS Rewrites, import DnsRewritesPanel, integrasi ke localState.local_records |
| `pkg/configengine/templates.go` | Update LocalRecordsTemplate untuk wildcard (local-zone redirect) |
| `pkg/configengine/generator.go` | Tambah helper hasWildcardPrefix & trimWildcardPrefix ke template.FuncMap |
| `pkg/model/validation.go` | Tambah validasi wildcard domain: reject PTR + wildcard |

---

## Cara Menggunakan (UI)

1. Buka halaman **DNS Configuration** di Web UI.
2. Klik tab **DNS Rewrites** (tab paling kanan, ikon ->).
3. Klik tombol **Add DNS Rewrite**.
4. Isi form:
   - **Domain**: e.g. speedtest.isp.net atau *.internal.lan (wildcard)
   - **Record Type**: A / AAAA / CNAME / PTR / TXT
   - **Answer / Value**: IP address atau hostname tujuan
   - **TTL**: default 3600 detik
5. Preview otomatis tampil saat domain + value sudah diisi.
6. Klik **Add Rewrite** - record muncul di tabel.
7. Klik **Save & Atomic Apply** untuk menerapkan ke Unbound.

---

## Output Konfigurasi Unbound

### Domain biasa (A record)
  local-data: "speedtest.isp.net 3600 A 192.168.10.50"

### Wildcard subdomain (*.domain)
  local-zone: "internal.lan" redirect
  local-data: "internal.lan 3600 A 10.0.0.1"

Dengan local-zone redirect, semua query *.internal.lan dijawab dengan IP 10.0.0.1.

---

## Tipe Record Didukung

A, AAAA, CNAME, PTR, TXT

Catatan: PTR tidak bisa digabung dengan wildcard (*.) - diblokir di layer validasi.

---

## Fitur UI

- Search bar: Filter by domain atau IP
- Filter chips: Quick filter per tipe record
- Edit/Delete: Tombol muncul saat hover di baris
- Wildcard badge: Record dengan *.domain menampilkan badge wildcard
- Live preview: Modal menampilkan preview output DNS
- Empty state: Tampilan kosong dengan call-to-action

---

## Status Build & Release

- Backend Go: PASS (`go build ./...`)
- Frontend TypeScript: PASS (`tsc -b && vite build`)
- Versi Rilis Terbaru: `v1.0.73` (Commit `b2b42e2`)

---

## Investigasi & Perbaikan Data Telemetri (Top Clients, Top Domains, Blocked)

### Akar Masalah:
1. Statistik global seperti *Total DNS Queries (22)*, *Cache Hits*, dan *Upstreams* muncul normal karena diambil langsung dari socket Unbound (`unbound-control stats_noreset` & `dump_infra`).
2. Namun data rinci (**Top clients**, **Top queried domains**, dan **Top blocked domains**) bersumber dari pembacaan log query (`QueryLogTailer` di `netora-api`).
3. Pada server, direktori `/var/log/unbound/` berstatus kepemilikan `root:root` dengan permission `0755`. Ketika Unbound berjalan dengan user non-root `unbound`, Unbound gagal membuka atau membuat file `/var/log/unbound/unbound.log` (*Permission denied*).
4. Akibatnya, Unbound secara otomatis mengalihkan log query ke `systemd / journalctl`, sementara file `/var/log/unbound/unbound.log` tidak ada. Hal ini membuat `QueryLogTailer` tidak menerima baris log sama sekali sehingga tabel klien dan domain tampak kosong di UI.

### Solusi & Peningkatan ISP-Grade:
1. **Dual-Source Telemetry Engine (`pkg/telemetry/query_log.go`)**:
   - `netora-api` kini dilengkapi fallback otomatis ke `journalctl -u unbound`.
   - Saat aplikasi start, sistem langsung membaca hingga 3000 riwayat query terakhir dari `journalctl`. Dashboard tidak akan pernah kosong meskipun file log belum ada atau baru saja di-rotate.
   - Sistem juga menjalankan streaming *real-time* via `journalctl -u unbound -f` jika file log tidak aktif.
2. **Perbaikan Permission & AppArmor di Skrip Pembaruan (`scripts/netora-dns-update.sh`)**:
   - Otomatis membuat `/var/log/unbound/unbound.log` dengan permission `chmod 775` dan kepemilikan `unbound:unbound`.
   - Otomatis mendaftarkan pengecualian AppArmor `/var/log/unbound/** rwk,` jika AppArmor aktif.
3. **Penyelarasan Permission Engine (`pkg/configengine/engine.go`)**:
   - Direktori log dibuat dengan mode `0777` agar user `unbound` dapat menulis log tanpa kendala hak akses.


