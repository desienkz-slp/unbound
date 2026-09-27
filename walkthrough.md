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

## Status Build

- Backend Go: PASS (go build ./...)
- Frontend TypeScript: PASS (tsc -b && vite build)
