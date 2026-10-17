# Walkthrough — Integrasi Filter AdBlock ABPindo (Indonesian AdBlock Rules) & Auto-Update Server

## Ringkasan Perubahan

Telah berhasil diintegrasikan daftar filter iklan dan pelacak lokal Indonesia dari repository resmi [ABPindo (Indonesian AdBlock Rules)](https://github.com/ABPindo/indonesianadblockrules) ke dalam platform Unbound Netora DNS, serta telah di-commit dan di-push ke GitHub branch `main`. Seluruh server DNS di jaringan yang menjalankan auto-update akan secara otomatis menerima pembaruan ini.

---

## 1. Analisis & Pemilihan Format DNS ABPindo

ABPindo menyediakan dua variasi aturan utama:
1. **Aturan Ekstensi Browser (`subscriptions/abpindo.txt`)**: Memiliki lebih dari 10.000 baris yang sebagian besar berisi *cosmetic filters* (CSS selectors seperti `##.ad-banner`, `##div[id^="ad"]`) dan path URL (`/ads/*`). Aturan kosmetik ini **tidak kompatibel** untuk DNS resolver karena dapat menyebabkan *false positive* (misal: domain induk berita seperti `detik.com` atau `kompas.com` ikut terblokir jika diparsing mentah).
2. **Aturan Khusus Pemblokir DNS (`subscriptions/domain.txt` & `subscriptions/aghome.txt`)**: Versi resmi ABPindo khusus untuk DNS (Pi-hole, AdGuard Home, Unbound). Hanya berisi domain ad server, injection portal ISP (Telkomsel, Indosat, XL, dsb.), pop-up, dan tracker lokal terkonfirmasi tanpa aturan kosmetik.

**Keputusan Arsitektur**:
* Menggunakan sumber DNS resmi ABPindo (`subscriptions/domain.txt` dan `aghome.txt`).
* Memperkuat fungsi `CleanDomain()` di `pkg/configengine/generator.go` agar secara otomatis menolak sintaks filter kosmetik (`##`, `#?#`, `#$#`, `#@#`) sebagai pengaman lapis ganda.

---

## 2. File & Komponen yang Diperbarui

### A. Repositori Blocklist
1. **`blocklist/ads.txt`**:
   * Menggabungkan 61 domain iklan global sebelumnya dengan 125 domain iklan, pelacak, dan portal injeksi ISP dari ABPindo.
   * Total domain unik terblokir di `ads.txt` kini menjadi **186 domain**.
2. **`blocklist/abpindo.txt`**:
   * Dibuat file khusus terisolasi berisi 125 domain resmi ABPindo untuk referensi independen dan download modular.

### B. Otomasi Script Crontab (Server Standalone / Legacy)
3. **`blocklist/update-lists.sh`**:
   * Menambahkan pengunduhan aman (`safe_download`) untuk ABPindo (`block-abpindo.txt`) dan master `ads.txt` langsung dari GitHub.
4. **`blocklist/gen-blocklist.sh`**:
   * Menambahkan ekstraksi domain dari `block-abpindo.txt` dan `ads.txt` ke dalam file konfigurasi `ad-malware-block.conf` Unbound dengan deduplikasi otomatis.

### C. Backend Engine & API Netora (`netora-api`)
5. **`pkg/blocklist/manager.go`**:
   * Menambahkan URL RAW ABPindo ke dalam daftar `FallbackURLs` pada feed kategori `feed-ads` (ADS_TRACKING).
   * Memperbarui label feed menjadi `Iklan, Pop-up & Pelacak (Global & ABPindo)`.
6. **`pkg/configengine/generator.go`**:
   * Memperbarui `DefaultAdsBlockedDomains` agar domain iklan ISP Indonesia (Telkomsel, Indosat, XL, Uzone, dsb.) langsung aktif di memori bahkan sebelum sinkronisasi disk terjadi.
   * Menambahkan sanitasi terhadap selector CSS browser pada fungsi `CleanDomain()`.
7. **`cmd/netora-api/server.go`**:
   * Menambahkan `feed-abpindo` ke dalam seed awal filter basis data.
   * Mendaftarkan ID `feed-abpindo` ke dalam handler switch-case toggle AdBlock.

### D. Distribusi Otomatis Lintas Server
8. **`scripts/netora-dns-update.sh`**:
   * Menambahkan penyalinan direktori `${target_release_dir}/blocklist/` ke `/etc/unbound/blocklist/` pada setiap siklus update, sehingga setiap server DNS yang menjalankan agen updater langsung menerima file `ads.txt` dan `abpindo.txt` terbaru.
9. **`install.sh`**:
   * Menambahkan langkah seeding awal direktori `blocklist` ke `/etc/unbound/blocklist` pada fresh installation.

### E. Frontend NOC Dashboard & UI Presets
10. **`web/src/views/FilteringView.tsx`**:
    * Menambahkan preset `ABPindo — Filter Iklan & Pelacak Indonesia (Recommended)` ke dalam daftar `PRESET_FEEDS`.
11. **`deploy/ui/` & `deploy/bin/netora-api-linux-amd64`**:
    * Membangun ulang bundle Web UI Vite (`npm run build`) dan mengompilasi binary Linux 64-bit terbaru untuk disebar ke server.

---

## 3. Bukti Verifikasi & Pengujian

### A. Pengujian Unit Go & Frontend Build
* **Go Unit Tests**: Seluruh package lulus tanpa error:
  ```
  ok netora-dns/cmd/netora-api 7.986s
  ok netora-dns/pkg/blocklist  0.976s
  ok netora-dns/pkg/configengine 4.042s
  ok netora-dns/pkg/hermes     0.338s
  ...
  PASS
  ```
* **Frontend Build**: `npm run build` sukses, modul terkompilasi ke `dist/` dan tersinkronisasi ke `deploy/ui/`.
* **Linux Cross-Compile**: Binary `deploy/bin/netora-api-linux-amd64` berhasil dikompilasi (16 MB).

### B. Commit & Push Git
* **Commit**: `6bddd0a` — `feat(blocklist): integrate ABPindo Indonesian adblock rules and automated server distribution`
* **Push**: Sukses terkirim ke `https://github.com/desienkz-slp/unbound.git` pada branch `main`.

### C. Status Server Produksi (172.18.20.146 / `netora-dns1`)
* Koneksi SSH dan respons API port `8080` aktif normal.
* Layanan `netora-dns-updater.timer` aktif berjalan di server dan menarik pembaruan repositori secara berkala ke host.
