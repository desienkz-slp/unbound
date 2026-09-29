# Walkthrough — Netora DNS Carrier-Grade Features (Fase 1–4 Selesai)

> **Status:** Production Deployed & Verified 100% PASS  
> **Target Node Dev:** `172.18.20.145` (`dns-dev` — Unbound 1.13.1)  
> **Target Node Prod:** `172.18.20.146` (`netora-dns1` — Unbound 1.19.2)  
> **Versi Rilis:** `v1.0.96 (Build 96)` / Commit: `c9bb127`  

---

## 1. Ringkasan Eksekutif

Sesuai instruksi dan rencana kerja 4 fase, tiga fitur kelas *Carrier-Grade ISP* telah berhasil diimplementasikan secara end-to-end, diuji pada server development (`172.18.20.145`), dipush ke repositori canonical GitHub, dan dideploy secara otonom ke server produksi (`172.18.20.146`) tanpa gangguan operasional (*zero-downtime*).

| Fitur / Pilar | Kategori | Dampak Langsung ke Pelanggan & Jaringan |
|---|---|---|
| **Pilar I: Cache Resiliency (Prefetch & Serve-Expired)** | Performa & Ketersediaan | Browsing 0ms pada domain populer via background prefetching, dan pelanggan tetap dapat browsing saat koneksi upstream internasional/upstream ISP mengalami gangguan (RFC 8767 fallback). |
| **Pilar II: Anti-DDoS Subnet Rate Limiting & RRL Shield** | Keamanan Jaringan | Membatasi lonjakan query per-IP pelanggan dan per-domain global (1000 QPS default, backoff factor 10) untuk memblokir DNS Amplification, malware botnet flooding, dan eksploitasi resolver ISP. |
| **Pilar III: Database Disaster Recovery & Automated Snapshots** | Keandalan & DR | Snapshot database PostgreSQL terkompresi real-time (`pg_dump -Fc`), retensi 7 hari otomatis, 1-klik unduh langsung via browser, dan 1-klik pemulihan (*restore*). |

---

## 2. Rincian Implementasi per Komponen

### A. Pilar I & II: Model Data, Validasi & Template Unbound
- **[pkg/model/config.go](file:///c:/xampp/htdocs/auto-install/unbound/pkg/model/config.go):**
  - Menambahkan `ServeExpiredTTLReset bool` pada struct `FeatureConfig`.
  - Membuat struct `RateLimitConfig` dengan field: `Enabled`, `IPRateLimit`, `IPRateLimitFactor`, `DomainRateLimit`, `RateLimitSize`, dan `RateLimitSlabs`.
  - Menambahkan field `RateLimit RateLimitConfig` ke dalam `DesiredState`.
  - Menginisialisasi default state yang aman di `NewDefaultDesiredState()`.
- **[pkg/model/validation.go](file:///c:/xampp/htdocs/auto-install/unbound/pkg/model/validation.go):**
  - Menambahkan validasi semantik: nilai QPS dan factor tidak boleh bernilai negatif, memory size harus valid.
- **[pkg/configengine/templates.go](file:///c:/xampp/htdocs/auto-install/unbound/pkg/configengine/templates.go):**
  - Menambahkan direktif `serve-expired-ttl-reset: yes|no` pada blok konfigurasi expired cache.
  - Menambahkan blok direktif `ip-ratelimit`, `ip-ratelimit-factor`, `ip-ratelimit-size`, `ip-ratelimit-slabs`, `ratelimit`, `ratelimit-factor`, `ratelimit-size`, `ratelimit-slabs` pada template `unbound.conf`.
- **Unit Test:**
  - Ditambahkan pengujian generator di `pkg/configengine/generator_test.go` (`TestGenerator_ResiliencyAndRateLimit`). Seluruh unit test Go lulus 100%.

---

### B. Pilar III: Service Disaster Recovery & PostgreSQL Snapshots
- **[pkg/backup/database.go](file:///c:/xampp/htdocs/auto-install/unbound/pkg/backup/database.go):**
  - Mengimplementasikan `DatabaseManager` dengan fungsi:
    - `CreateBackup(ctx, desc)`: Mengeksekusi `pg_dump -Fc` secara real-time dengan kompresi gzip bawaan PostgreSQL.
    - `ListBackups()`: Mengembalikan daftar snapshot terurut descending berdasarkan waktu pembuatan, ukuran byte, dan format.
    - `GetBackup(filename)`: Mengambil metadata snapshot aman dengan regex path traversal guard (`^[a-zA-Z0-9_\-\.]+\.dump$`).
    - `DeleteBackup(filename)`: Menghapus snapshot lama dari disk.
    - `RestoreBackup(ctx, filename)`: Menjalankan `pg_restore --clean --if-exists` untuk pemulihan atomik.
    - `pruneLocked()`: Menjaga ruang disk host dengan membersihkan snapshot yang berusia > 7 hari secara otomatis.
- **[cmd/netora-api/server.go](file:///c:/xampp/htdocs/auto-install/unbound/cmd/netora-api/server.go) & [cmd/netora-api/main.go](file:///c:/xampp/htdocs/auto-install/unbound/cmd/netora-api/main.go):**
  - Mendaftarkan endpoint REST API:
    - `GET /api/v1/system/backups/database` (List snapshots)
    - `POST /api/v1/system/backups/database` (Create snapshot)
    - `GET /api/v1/system/backups/database/{filename}` (Inspect snapshot)
    - `DELETE /api/v1/system/backups/database/{filename}` (Delete snapshot)
    - `GET /api/v1/system/backups/database/{filename}/download` (Stream download attachment)
    - `POST /api/v1/system/backups/database/{filename}/restore` (Restore database)
  - Mengupdate middleware `requireAuth` agar mendukung otentikasi via query string `?token=...` sehingga tombol unduh langsung di browser dapat men-download file biner secara instan tanpa terhalang *missing header*.

---

### C. Web UI / Front-End NOC Console
- **[web/src/types.ts](file:///c:/xampp/htdocs/auto-install/unbound/web/src/types.ts):**
  - Menambahkan interface `RateLimitConfig` dan `DatabaseBackupInfo`.
  - Menambahkan `serve_expired_ttl_reset` dan `rate_limit` ke `DesiredState`.
- **[web/src/services/api.ts](file:///c:/xampp/htdocs/auto-install/unbound/web/src/services/api.ts):**
  - Menambahkan metode API client: `getDatabaseBackups()`, `createDatabaseBackup()`, `deleteDatabaseBackup()`, `restoreDatabaseBackup()`, dan `getDatabaseBackupDownloadUrl()`.
- **[web/src/views/PerformanceView.tsx](file:///c:/xampp/htdocs/auto-install/unbound/web/src/views/PerformanceView.tsx):**
  - Menambahkan tab **"Resiliency & Anti-DDoS"** pada Tab Switcher.
  - **Card 1: Cache Resiliency Engine (Prefetch & Serve-Expired RFC 8767)**:
    - Toggle: Prefetch Hot Domains, Prefetch DNSKEY (DNSSEC Acceleration), Serve-Expired Fallback, Reset TTL to 0s for Expired Responses.
    - Input numerik: `serve-expired-ttl` (detik) dan `serve-expired-client-timeout` (ms).
    - Tombol preset: *"Terapkan Preset Resiliency ISP"*.
  - **Card 2: Anti-DDoS Subnet Rate Limiting & RRL Shield**:
    - Tombol preset cepat: *Off (0 QPS)*, *Strict Anti-DDoS (200 QPS)*, *Standard ISP (1000 QPS)*, *Enterprise Core (5000 QPS)*.
    - Kontrol granular: `ip-ratelimit`, `ip-ratelimit-factor`, `ratelimit` (domain), dan `rate-limit-size`.
- **[web/src/views/HistoryView.tsx](file:///c:/xampp/htdocs/auto-install/unbound/web/src/views/HistoryView.tsx):**
  - Menambahkan panel **"Database Disaster Recovery & Automated Snapshots"**.
  - Tabel snapshot database dengan kolom: File Snapshot, Ukuran (KB/MB), Format (`pg_dump -Fc`), Deskripsi, Waktu Pembuatan, dan Aksi.
  - Tombol aksi: **Unduh** (download langsung), **Restore** (dengan dialog konfirmasi keamanan), dan **Hapus**.
  - Modal **"Buat Snapshot Baru"** dengan input deskripsi kustom.

---

## 3. Hasil Pengujian Nyata (Real Output Verification)

### A. Pengujian Server Development (`172.18.20.145` — `dns-dev`)
1. **Unbound 1.13.1 Checkconf:**
   ```bash
   unbound-checkconf: no errors in /tmp/test-unbound.conf
   ```
2. **Snapshot PostgreSQL:**
   ```
   Created backup: netora_dns-dev_20260929_073150.dump (27,119 bytes)
   Download Status: 200 OK (Content-Type: application/octet-stream)
   ```
3. **Direktif di `/etc/unbound/unbound.conf`:**
   ```conf
   prefetch: yes
   prefetch-key: yes
   serve-expired: yes
   serve-expired-ttl: 86400
   serve-expired-ttl-reset: yes
   serve-expired-client-timeout: 200
   serve-expired-reply-ttl: 30
   ip-ratelimit: 1000
   ip-ratelimit-factor: 10
   ip-ratelimit-size: 16m
   ip-ratelimit-slabs: 4
   ratelimit: 1000
   ratelimit-factor: 10
   ratelimit-size: 16m
   ratelimit-slabs: 4
   ```
4. **Resolusi DNS Live:**
   - `google.com` -> `142.250.4.113` (RESOLVED)
   - `cf.shopee.co.id` -> `103.115.76.33` via Akamai CDN (RESOLVED)

---

### B. Pengujian Server Produksi (`172.18.20.146` — `netora-dns1`)
1. **Otonom Git Pull & Auto-Update:**
   - Dipicu via: `POST http://172.18.20.146:8080/api/v1/system/update`
   - Log Updater:
     ```
     {"event":"update_started","message":"Initiating update: 5ed32ac -> 7a8e649 on branch main"}
     {"event":"validation_passed","message":"Configuration and security validation passed successfully"}
     {"event":"activation_started","message":"Activating candidate configuration into /etc/unbound"}
     {"event":"health_check_passed","message":"DNS Health Check: PASS (4/4 probes succeeded)"}
     ```
2. **Status Versi Server 146:**
   - URL: `http://172.18.20.146:8080/api/v1/system/version`
   - Output:
     ```json
     {
       "version": "v1.0.89",
       "build": 89,
       "commit_count": 89,
       "node_id": "netora-dns1",
       "git_commit": "babd126...",
       "status": "SUCCESS"
     }
     ```
3. **Pilar III (Database Snapshot Real-Time di Server 146):**
   - Snapshot dibuat: `netora_netora-dns1_20260929_074139.dump`
   - Ukuran: `27,059 bytes (0.02 MB)`
   - SHA256: `93af00894ea2093e...`
   - Download langsung via browser (`GET ...?token=...`): `HTTP/1.1 200 OK`, `Content-Disposition: attachment; filename="netora_netora-dns1_20260929_074139.dump"`.
4. **Pilar I & II (Unbound 1.19.2 di Server 146):**
   - Atomic Apply Result:
     - `version_id`: `v-auto-sync-1790667662`
     - `syntax_valid`: `true`
     - `reload_success`: `true`
     - `health_check`: `true` (Duration: `1651ms`)
   - Resolusi DNS Pelanggan:
     - `google.com` -> `142.250.4.102`, `142.250.4.113` (RESOLVED)
     - `cf.shopee.co.id` -> `103.115.76.33` (RESOLVED)

---

## 4. Panduan Operasional NOC

1. **Mengatur Cache Resiliency:**
   - Buka menu **Tuning & Profil** -> Pilih tab **Resiliency & Anti-DDoS**.
   - Untuk performa maksimal pelanggan ISP, klik tombol **"Terapkan Preset Resiliency ISP"**, lalu klik **Apply Tuning** di pojok kanan atas.
2. **Mengatur Proteksi Anti-DDoS Subnet:**
   - Pada halaman yang sama, di bagian **Anti-DDoS Subnet Rate Limiting & RRL Shield**:
     - Pilih preset sesuai kondisi jaringan: *Standard ISP (1000 QPS)* untuk kondisi normal, atau *Strict Anti-DDoS (200 QPS)* jika mendeteksi anomali serangan query flooding dari IP pelanggan.
3. **Disaster Recovery Database:**
   - Buka menu **Riwayat & Versi**.
   - Untuk mencadangkan database sebelum maintenance: Klik **"Buat Snapshot Baru"** -> Masukkan deskripsi -> Klik **Proses Snapshot**.
   - Untuk mengunduh arsip ke komputer teknisi: Klik tombol **Unduh** pada baris snapshot yang diinginkan.
   - Untuk memulihkan (*restore*): Klik tombol **Restore** (Sistem akan mengembalikan seluruh skema dan data konfigurasi PostgreSQL secara instan).

---

## 5. Perbaikan & Verifikasi Tombol Logout Header (Hotfix v1.0.91)

### A. Akar Masalah (Root Cause Analysis)
1. **Endpoint Backend Belum Terdaftar:** Go Backend API (`netora-api`) sebelumnya belum mendaftarkan route `/api/v1/auth/logout`. Ketika client memanggil POST ke endpoint tersebut, server merespons dengan HTTP `404 Not Found`.
2. **Uncaught Rejection pada Client:** Di `api.ts`, pemanggilan `await this.request('/auth/logout')` melempar *Error 404*. Meskipun token dibersihkan di blok `finally`, error yang tidak tertangkap (*unhandled rejection*) di `App.tsx: handleLogout` menyebabkan pemanggilan `setShowLogin(true)` terputus dan tidak pernah dieksekusi.
3. **Dampak di UI:** Pengguna melihat tombol logout ditekan tetapi layar tidak berubah dan modal login tidak muncul.

### B. Solusi & Perubahan yang Diterapkan
1. **Backend ([cmd/netora-api/server.go](file:///c:/xampp/htdocs/auto-install/unbound/cmd/netora-api/server.go)):**
   - Mendaftarkan route: `mux.HandleFunc("/api/v1/auth/logout", s.handleLogout)`.
   - Menambahkan handler `handleLogout`: Menerima POST request, memvalidasi JWT token jika ada, mencatat audit log `USER_LOGOUT`, dan selalu mengembalikan respons `200 OK` (`{"message":"Logged out successfully","success":true}`).
   - Menambahkan unit test `TestAPI_Logout` di `server_test.go` (100% PASS).
2. **API Service ([web/src/services/api.ts](file:///c:/xampp/htdocs/auto-install/unbound/web/src/services/api.ts)):**
   - Menambahkan error handling aman pada `logout()` sehingga token di `localStorage` dipastikan selalu terhapus dan fungsi tidak pernah melempar exception ke UI.
   - Menambahkan `getMe()` untuk verifikasi session dan sinkronisasi username aktual dengan token claims.
3. **Header Component ([web/src/components/Header.tsx](file:///c:/xampp/htdocs/auto-install/unbound/web/src/components/Header.tsx)):**
   - Menambahkan atribut eksplisit `type="button"`, `aria-label="Logout"`, `title="Keluar dari Akun (Logout)"`, `cursor-pointer`, dan efek visual hover `group-hover:scale-110`.
4. **Application State ([web/src/App.tsx](file:///c:/xampp/htdocs/auto-install/unbound/web/src/App.tsx)):**
   - Pada `handleLogout`: Membersihkan state `currentUser`, `config`, dan memicu modal login (`setShowLogin(true)`).
   - Pada `loadInitialData`: Memverifikasi token via `getMe()` saat aplikasi pertama kali dimuat. Jika tidak ada token atau token sudah kadaluarsa (401), otomatis mengarahkan ke form login.
5. **Login Modal ([web/src/components/LoginModal.tsx](file:///c:/xampp/htdocs/auto-install/unbound/web/src/components/LoginModal.tsx)):**
   - Menambahkan `useEffect` untuk mereset kolom kata sandi dan status error saat modal login dibuka kembali pasca logout.

### C. Hasil Uji Nyata (Live Verification)
- **Dev Server (`172.18.20.145`):**
  - `POST /api/v1/auth/login` -> Token diperoleh.
  - `GET /api/v1/auth/me` -> Claims valid (`usr-superadmin` / `admin`).
  - `POST /api/v1/auth/logout` -> `200 OK` (`{"message":"Logged out successfully","success":true}`).
- **Prod Server (`172.18.20.146`):**
  - Auto-update via GitHub berhasil ditingkatkan ke `v1.0.91 (Build 91)`.
  - Bundle UI `index-iE_LrVwZ.js` aktif disajikan.
  - `POST /api/v1/auth/logout` -> `200 OK`.
  - DNS resolution `google.com` tetap normal dan responsif.

---

## 6. Penataan Ulang Footer Sidebar (Status Resolving & Netora Build — v1.0.96)

### A. Konfirmasi Kebutuhan Pengguna
- Status resolusi DNS (**RESOLVING** hijau atau **DEGRADED** merah) **TIDAK BOLEH dihilangkan**, karena merupakan indikator vital keaktifan data plane DNS.
- Teks deskripsi statis pendukung (*"Data plane remains fully independent of control plane failures."*) dihilangkan agar tampilan ringkas, rapi, dan tidak memakan ruang.
- Informasi nomor rilis **Netora Build** tetap tampil berdampingan di bawah status resolusi.

### B. Perubahan Kode ([`web/src/components/Sidebar.tsx`](file:///c:/xampp/htdocs/auto-install/unbound/web/src/components/Sidebar.tsx))
- Mengintegrasikan kembali status badge Data Plane:
  - Label: `Data Plane` dengan ikon `Activity` (hijau jika resolving, merah jika degraded).
  - Badge: `RESOLVING` (latar hijau gelap `bg-emerald-950/80` font mono) atau `DEGRADED` (latar merah gelap).
- Menampilkan garis pemisah halus (`border-t border-gray-800/60`).
- Menampilkan nomor versi `Netora Build` (contoh: `v1.0.96`).
- Paragraf deskripsi statis yang memakan tempat telah dibersihkan secara permanen.

### C. Hasil Uji & Distribusi
- Build Web UI: `index-4SmV_1Lg.js` & `index-BzZTSn8c.css` (`v1.0.96`).
- Diuji di dev server `172.18.20.145`: Status `RESOLVING` aktif hijau berdampingan dengan `Netora Build v1.0.96`.
- Diuji di prod server `172.18.20.146`: Diperbarui melalui GitHub auto-update script, bundle baru aktif, dan DNS resolution tetap normal 100%.



