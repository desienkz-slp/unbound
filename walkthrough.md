# Walkthrough — Verifikasi & Penguatan Fitur AI Auto, Tuning, Profile, dan Push ke GitHub

## Ringkasan Eksekusi
Telah diselesaikan pemeriksaan, penyempurnaan, serta pengujian end-to-end untuk seluruh fitur **Tuning**, **Hardware Profiles**, dan **AI Auto-Optimization** pada platform Netora DNS Unbound. Seluruh tombol interaktif, kalkulasi profil AI, dan mutasi backend telah diverifikasi berfungsi dengan normal.

---

## 1. Fitur AI Auto & Profil Hardware yang Diperkuat

### A. Fitur Baru: ✨ AI Full Auto-Tune (One-Click Optimization)
* Disediakan tombol **✨ AI Full Auto-Tune** pada header navigasi halaman *Tuning & Profiles*.
* Menjalankan siklus optimasi otomatis dalam satu kali klik:
  1. **Deteksi Hardware**: Menganalisis CPU Cores, kapasitas RAM, dan ruang penyimpanan disk host secara real-time.
  2. **Profil Komputasi Unbound**: Memilih profil hardware paling presisi (`SMALL`, `MEDIUM`, `LARGE`, atau `ISP`) dengan alokasi thread, cache slabs, socket buffer `so-rcvbuf`/`so-sndbuf`, dan ukuran cache yang optimal.
  3. **Carrier Resiliency Engine (RFC 8767)**: Mengaktifkan prefetch rekaman domain populer, prefetch kunci DNSSEC, serta fallback *serve-expired* (tahan saat upstream internet mati).
  4. **Subnet Anti-DDoS Shield**: Mengonfigurasi *Response Rate Limiting (RRL)* level kernel Unbound untuk melindungi resolver dari DNS amplification attack.
  5. **AI Logrotate & Storage Guard**: Menghitung dan menyinkronkan kebijakan rotasi log Unbound sesuai kapasitas disk agar disk host tidak pernah penuh (mencegah galat *No space left on device*).
  6. **Penerapan Atomik**: Menyimpan seluruh konfigurasi dan mengaktifkannya ke node server secara atomik.

### B. AI Hardware Profile Recommendation Box
* Menghitung profil hardware ideal (`recommendedId`) berdasarkan spesifikasi node.
* Dilengkapi 2 pilihan aksi interaktif:
  * **`⚡ Auto-Apply AI Profile ({recommendedId})`**: Menerapkan dan langsung menyimpan profil rekomendasi AI ke server seketika.
  * **`Terapkan ke Form`**: Mengisi form editor dengan nilai rekomendasi untuk ditinjau secara manual.

### C. AI Logrotate & Storage Guard
* **Hero Card & Warning Banner**: Dilengkapi tombol **`⚡ Auto-Tune & Simpan ke Host`** serta **`Terapkan ke Form`**.
* **Engine Fallback**: Menyediakan kalkulator AI lokal di frontend sehingga rekomendasi rotasi log selalu dapat dihitung meskipun laporan telemetri server sedang dalam proses *loading*.

---

## 2. Verifikasi Fungsi Seluruh Tombol & Kontrol

Semua tombol dan elemen interaktif di halaman *Tuning & Profiles* (`PerformanceView.tsx`) telah diperiksa dan dipastikan berfungsi:

| Bagian | Tombol / Elemen | Status | Fungsi & Aksi |
| :--- | :--- | :--- | :--- |
| **Header Toolbar** | `✨ AI Full Auto-Tune` | **Berfungsi** | Otomatis tuning hardware, resiliency, DDoS, dan logrotate sekali klik. |
| **Header Toolbar** | `Apply Tuning` | **Berfungsi** | Menyimpan perubahan konfigurasi lokal ke server via `api.applyConfig`. |
| **Navigasi Tab** | Tab *Semua*, *CPU & Cache*, *Resiliency*, *Logrotate* | **Berfungsi** | Memfilter tampilan section tuning sesuai kategori. |
| **Profil Hardware** | Kartu Profil (`SMALL`, `MEDIUM`, `LARGE`, `ISP`) | **Berfungsi** | Memilih dan mengisi form konfigurasi sesuai profil yang diklik. |
| **Arsitektur Disk** | Kartu *Minimum*, *Recommended*, *Carrier Grade* | **Berfungsi** | Memilih preset profil yang sesuai dengan kapasitas disk. |
| **Carrier Resiliency**| `Terapkan Preset Resiliency ISP` | **Berfungsi** | Mengaktifkan prefetch domain, DNSKEY, serve-expired, dan TTL reset. |
| **Anti-DDoS Presets** | `Off`, `Strict`, `Standard ISP`, `Enterprise` | **Berfungsi** | Menyetel parameter *ip-ratelimit*, *domain-ratelimit*, dan *factor*. |
| **Logrotate Presets** | `Ultra Storage Saver`, `Balanced`, `Carrier Audit` | **Berfungsi** | Menyetel frekuensi, retensi, batas ukuran log, dan kompresi gzip. |
| **Operasi Logrotate** | `Uji Validasi (Dry Run)` | **Berfungsi** | Menjalankan simulasi `logrotate -d` dan menampilkan output di terminal. |
| **Operasi Logrotate** | `Rotasi Sekarang (Paksa & Kosongkan)` | **Berfungsi** | Meminta konfirmasi dan mengeksekusi `logrotate -f` untuk mengosongkan disk. |
| **Operasi Logrotate** | `Terapkan Konfigurasi Logrotate` | **Berfungsi** | Menulis dan memvalidasi konfigurasi `/etc/logrotate.d/unbound`. |
| **Operasi Logrotate** | `Refresh Status` | **Berfungsi** | Memuat ulang telemetri ukuran log Unbound dan status rotasi host. |
| **Konsol Terminal** | `Tutup Konsol` | **Berfungsi** | Menutup jendela output logrotate. |

---

## 3. Hasil Pengujian & Kompilasi

1. **Frontend TypeScript & Vite Build**:
   * Perintah: `npm run build`
   * Hasil: `✓ built in 1.08s`, nol kesalahan kompilasi (`tsc -b` lulus tanpa error).
   * Aset terdistribusi otomatis ke direktori [`deploy/ui/`](file:///c:/xampp/htdocs/auto-install/unbound/deploy/ui/).

2. **Backend Go Test Suite**:
   * Perintah: `go test ./...`
   * Hasil: `PASS` pada seluruh paket (`cmd/netora-api`, `pkg/model`, `pkg/logrotate`, `pkg/storage`, dll.).

3. **Git Synchronization**:
   * Commit dan push ke GitHub siap dieksekusi secara bersih.
