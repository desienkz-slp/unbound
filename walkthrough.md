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

## Investigasi & Perbaikan Versioning

### Akar Masalah Versi Tetap di `v1.0.69`:
1. Pada commit sebelumnya (`e63bbba`), build frontend dan binary Go sudah ter-compile dan masuk ke dalam `deploy/`.
2. Namun, file `version.json` di root repositori belum ter-stage/ter-commit ke git sehingga di server GitHub `origin/main` file `version.json` masih tercatat di `v1.0.69`.
3. Script update di server (`netora-dns-update.sh`) membaca `version.json` dari root repositori yang di-clone dengan shallow `--depth=1`. Akibatnya, server tetap menyalin label `v1.0.69` meskipun git commit SHA berhasil diperbarui ke `e63bbba`.
4. Kode fitur **DNS Rewrites** sebenarnya **sudah aktif dan terpasang** di server (karena SHA git yang berjalan sudah sesuai).

### Solusi & Langkah yang Dilakukan:
1. Sinkronisasi metadata versi: `version.json`, `web/src/version.json`, `web/public/version.json`, dan `deploy/ui/version.json` diperbarui ke `v1.0.73`.
2. Peningkatan ketahanan pada `scripts/netora-dns-update.sh` agar membaca versi dari `deploy/ui/version.json` jika lebih baru dibanding root `version.json`.
3. Semua artefak sudah di-commit (`b2b42e2`) dan di-push ke GitHub `origin/main`.
4. Pengguna cukup menekan tombol **"Perbarui Platform Sekarang"** satu kali lagi di modal UI untuk memperbarui label versi ke `v1.0.73`.

