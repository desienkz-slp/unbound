# Walkthrough: Sistem List Terpadu untuk Seluruh Filter & Auto-Update Otonom (Netora DNS)

Dokumen ini mendokumentasikan implementasi arsitektur **Sistem List Terpadu untuk Seluruh Filter**, integrasi sumber multi-tier (**Private Repository & Upstream Mirrors**), mesin **Auto-Update Otonom (Scheduler & Cron)**, serta pembaruan antarmuka **Netora Web UI** pada DNS Server Netora (`172.18.20.145`).

---

## 1. Ringkasan Eksekutif

Sesuai permintaan Anda untuk:
1. **Menjadikan sistem list untuk seluruh kategori filter** (baik di repository privat Anda maupun sumber redundan lainnya).
2. **Memastikan sistem melakukan auto-update secara berkala dan otomatis**.
3. **Menjaga privasi penuh** (URL dan kredensial repository privat tidak pernah terekspos ke publik atau UI).

Sistem kini telah ditingkatkan dengan modul `pkg/blocklist` mandiri berkinerja tinggi, scheduler background otomatis tiap 12 jam, redundansi cron OS-level, dan antarmuka pemantauan status sinkronisasi real-time pada Web UI.

---

## 2. Struktur Sistem List 6 Kategori Terpadu

Seluruh filter distandardisasi ke dalam format file baris domain bersih (`blocklist/*.txt`) di dalam repository dan disk server:

| Kategori Filter | File List di Repo / Server | ID Feed Internal | Target Proteksi / Fungsi | Fallback Mirror Upstream |
| :--- | :--- | :--- | :--- | :--- |
| **18+ Dewasa** | `blocklist/adult-domains.txt` | `feed-adult` | Pornografi, konten dewasa, cam sites | StevenBlack Porn / OISD NSFP |
| **Judi Online** | `blocklist/gambling-domains.txt` | `feed-gambling` | Judi online, slot gacor, taruhan daring | StevenBlack Gambling / HaGeZi Gambling |
| **Iklan & Tracker** | `blocklist/ads.txt` | `feed-ads` | Banner iklan agresif, tracking, pop-ups | AdGuard DNS Filter / HaGeZi Multi-PRO |
| **Malware & Phishing** | `blocklist/malware.txt` | `feed-malware` | Phishing, C2 Botnet, situs ransomware | URLhaus / HaGeZi Threat Intelligence |
| **Custom ISP Blacklist** | `blocklist/custom.txt` | `feed-custom` | Domain ad-hoc khusus ISP / manual | Daftar internal ISP |
| **Whitelist Pengecualian** | `blocklist/whitelist.txt` | `feed-whitelist` | Pengecualian global (anti false-positive) | Pengecualian domain esensial |

> **Catatan Whitelist:** Setiap domain yang dimasukkan ke dalam `blocklist/whitelist.txt` otomatis **dieliminasi dari seluruh daftar pemblokiran**, menjamin domain vital pelanggan (seperti portal perbankan, CDN penting, atau aplikasi pemerintah) tidak akan pernah terblokir tanpa sengaja.

---

## 3. Arsitektur Multi-Tier Resolution & Sanitasi Domain

Modul `pkg/blocklist/manager.go` dirancang dengan arsitektur multi-tingkat yang sangat tangguh:

```
[Trigger Update] (Scheduler 12 Jam / Cron / Web UI / Boot)
        │
        ▼
[Tier 1: Private Repo / Local Disk] ──── Berhasil? ───► [Sanitasi & Parsing]
        │ (Gagal / Tidak Ada)                                   │
        ▼                                                       │
[Tier 2: Upstream Mirrors Terpercaya] ── Berhasil? ───►         │
        │ (Offline / Timeout)                                   │
        ▼                                                       │
[Tier 3: Internal Seed Fallback] ─────────────────────►         │
                                                                ▼
                                                [Global Whitelist Subtraction]
                                                                │
                                                                ▼
                                                [Compile /etc/unbound/conf.d/*.conf]
                                                                │
                                                                ▼
                                                [unbound-checkconf Verification]
                                                                │
                                                                ▼
                                                [Atomic Zero-Downtime Reload]
```

### Keunggulan Engine Sanitasi (`CleanDomain`):
- **Universal Format Parser:** Mampu memproses format mentah Adblock Plus (`||domain.com^`), format hosts (`0.0.0.0 domain.com` atau `127.0.0.1 domain.com`), maupun plain text satu domain per baris.
- **Safety Filter:** Secara otomatis membuang baris komentar (`#`, `!`), baris kosong, port number (`:8080`), alamat IP dummy, dan domain loopback (`localhost`, `local`) sehingga tidak merusak resolusi internal server.
- **Punycode & IDN Normalizer:** Mendukung normalisasi karakter domain internasional.

---

## 4. Mekanisme Auto-Update Otonom

Sistem auto-update diimplementasikan secara berlapis (*defense-in-depth*) untuk menjamin kehandalan 24/7:

### A. Scheduler Otonom di Daemon Netora API (Go Engine)
- Berjalan di background service `netora-api.service`.
- **Interval Eksekusi:** Setiap **12 jam** sekali secara otomatis.
- **Cold-Start Trigger:** Sinkronisasi awal dijalankan otomatis **10 detik setelah daemon boot/start**, memastikan server selalu memiliki data terbaru setelah reboot.
- **Atomic Zero-Downtime:** Menggunakan `unbound-control reload`. Resolusi DNS klien ISP tidak terhenti maupun terputus 1 detik pun saat proses update berlangsung.

### B. Redundansi OS-Level Cron di Server Linux
Sebagai pengaman jika daemon API di-restart atau mengalami jeda, scheduler OS-level aktif di:
- **Lokasi File:** `/etc/cron.d/netora-blocklist-autoupdate`
- **Jadwal:** Pukul **03:00 WIB** dan **15:00 WIB** setiap hari.
- **Aksi:** Memanggil script `/usr/local/bin/netora-api-sync-blocklists.sh` yang memicu endpoint internal API dan memverifikasi kesehatan Unbound.

---

## 5. Pembaruan Web UI (Policy Filtering)

Tampilan menu **Policy Filtering** di Web UI Netora (`http://172.18.20.145:8080`) kini dilengkapi:

1. **Kartu Status Auto-Update Otonom:**
   - **Badge Status:** `AKTIF - Tiap 12 Jam` dengan indikator lampu hijau berkedip (*pulsing green dot*).
   - **Statistik Total Domain:** Menampilkan jumlah total domain aktif terblokir di seluruh kategori secara real-time (**257.036+ Domain**).
   - **Waktu Sinkronisasi Terakhir:** Menampilkan timestamp presisi kapan sinkronisasi terakhir berhasil dijalankan.
   - **Jadwal Update Berikutnya:** Menampilkan estimasi waktu putaran auto-update selanjutnya.
2. **Tombol "Sync Seluruh Filter Sekarang":**
   - Disediakan tombol manual on-demand dengan animasi loading spinner dan alert konfirmasi sukses berwarna hijau.
3. **Privasi Penuh (Zero-Leak):**
   - URL repository privat, nama akun, maupun token GitHub **tidak ditampilkan** pada antarmuka web maupun metadata browser.
4. **Saklar Toggle Independen untuk Setiap Kategori:**
   - SafeSearch VIP (Mesin Pencari)
   - 18+ Konten Dewasa
   - Judi Online & Taruhan
   - Iklan & Pelacak Agresif
   - Malware & Phishing

---

## 6. Bukti Verifikasi Pengujian Nyata di Server (172.18.20.145)

Pengujian end-to-end telah dilakukan langsung pada server produksi `172.18.20.145`:

### 1. Hasil Sinkronisasi Real-World (257.036 Domain)
```bash
=== Status Sinkronisasi Seluruh Feed Filter ===
Total Domain Terblokir : 257,036 domain
- Dewasa (Adult 18+)   : 46,466 domain
- Judi Online / Slot   : 83,533 domain
- Iklan & Pelacak      : 50,522 domain
- Malware & Phishing   : 76,508 domain
- Custom ISP Blacklist : 7 domain
- Whitelist Terlindungi: 3 domain
Waktu Eksekusi         : 2026-09-26 01:21:40 WIB
Status Unbound         : Berhasil divalidasi & di-reload atomik tanpa error
```

### 2. Validasi Konfigurasi Unbound
```bash
root@dns-dev:~# unbound-checkconf /etc/unbound/unbound.conf
unbound-checkconf: no errors in /etc/unbound/unbound.conf
```

### 3. Pengujian Resolusi DNS Tiap Kategori
- **Judi Online (Saklar ON):**
  ```cmd
  C:\Users\desie>nslookup sbobet.com 172.18.20.145
  *** UnKnown can't find sbobet.com: Non-existent domain
  ```
  *(Status: Diblokir seketika dengan NXDOMAIN)*

- **Iklan Agresif (Saklar ON):**
  ```cmd
  C:\Users\desie>nslookup adservice.google.com 172.18.20.145
  *** UnKnown can't find adservice.google.com: Non-existent domain
  ```
  *(Status: Diblokir seketika dengan NXDOMAIN)*

- **Dewasa 18+ (Dinamis: Diuji saat ON vs OFF):**
  - Saat Toggle ON: `xhamster.com` -> `NXDOMAIN` (Diblokir).
  - Saat Toggle OFF: `xhamster.com` -> `104.16.4.81` (Dapat diakses normal tanpa error `Server failed`).

- **SafeSearch VIP (Saklar ON):**
  ```cmd
  C:\Users\desie>nslookup www.google.com 172.18.20.145
  Name:    www.google.com
  Address:  216.239.38.120
  ```
  *(Status: Terkunci di VIP SafeSearch)*

- **Whitelist Protection:**
  ```cmd
  C:\Users\desie>nslookup google.com 172.18.20.145
  Address: 142.250.198.46
  ```
  *(Status: Domain penting dan aman tidak pernah terblokir)*

---

## 7. Panduan Operasional Pengelolaan List Domain

### A. Menambahkan Domain Secara Privat di Repository Anda:
1. Buka repository Anda di GitHub/Git:
   - Tambah domain judi ke: `blocklist/gambling-domains.txt`
   - Tambah domain dewasa ke: `blocklist/adult-domains.txt`
   - Tambah domain iklan ke: `blocklist/ads.txt`
   - Tambah domain malware ke: `blocklist/malware.txt`
   - Tambah pengecualian ke: `blocklist/whitelist.txt`
2. Simpan dan commit/push perubahan Anda.
3. Pada jadwal berikutnya (tiap 12 jam) atau saat Anda menekan tombol **"Sync Seluruh Filter Sekarang"** di Web UI, server akan otomatis menerapkan daftar domain baru tersebut.

### B. Menambahkan Domain Langsung di Server (Opsional):
Jika Anda ingin menambahkan domain darurat langsung di server Linux:
1. Edit file terkait di `/etc/unbound/blocklist/` (misal: `/etc/unbound/blocklist/custom.txt`).
2. Tekan tombol **"Sync Seluruh Filter Sekarang"** di Web UI, atau jalankan perintah:
   ```bash
   curl -X POST http://localhost:8080/api/v1/blocklists/sync -H "Authorization: Bearer <token>"
   ```
3. Konfigurasi Unbound akan ter-reload seketika tanpa perlu restart service.

---

## 8. Penyelesaian Masalah Saklar Ganda (Double Action) & UI Bersih (Single Control)

Menanggapi kendala saklar ganda (*double action*) yang Anda temukan antara Kartu Atas dan Baris Tabel:

1. **Akar Masalah yang Ditemukan:**
   - Sebelumnya, terdapat 2 titik kontrol: Kartu KPI di atas (*Master Policy*) dan saklar di baris tabel bawah (*Per-feed file list*).
   - Hal ini menyebabkan kebingungan dan status yang tampak kontradiktif (misal: di atas `ALLOWED` namun di tabel bawah `ENABLED`).

2. **Solusi yang Diterapkan Sesuai Instruksi Anda:**
   - **Tabel Raw Blocklists Disembunyikan Penuh:** Seluruh daftar filter (18+ Dewasa, Judi Online, Iklan, dan Malware) sudah tertanam di sistem, sehingga tabel baris bawah yang membingungkan telah disembunyikan secara default.
   - **Single Point of Control (5 Master Policy Cards):** Kontrol saklar kini terpusat secara bersih di kartu bagian atas:
     1. **SafeSearch VIP** (Google, Bing, YT) [Toggle ON/OFF]
     2. **18+ Dewasa** (Situs Dewasa & Pornografi) [Toggle ON/OFF]
     3. **Judi Online** (Slot Gacor, Taruhan Online) [Toggle ON/OFF]
     4. **Iklan & Pelacak** (Banner Iklan, Tracker & Pop-ups) [Toggle ON/OFF]
     5. **Malware Shield** (Phishing, Ransomware & Botnet) [Toggle ON/OFF]
   - **Tabel Pengecualian Allowlist Langsung & Rapi:** Tepat di bawah kartu, antarmuka langsung menyajikan tabel **Whitelisted Domain Rules (Exceptions)** lengkap dengan tombol **`+ Add Allowlist Exception`**, sehingga mempermudah NOC jika ada domain pelanggan darurat yang perlu di-whitelist.
   - **Alur Update Super Praktis:** Anda cukup mengedit file daftar domain di repository GitHub Anda (`blocklist/adult-domains.txt`, `blocklist/gambling-domains.txt`, dll.). Auto-Update Engine akan menyinkronkannya otomatis setiap 12 jam, atau Anda dapat menekan tombol **"Sync Seluruh Filter Sekarang"** untuk pembaruan instan.
