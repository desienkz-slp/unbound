# Walkthrough — Perbaikan Masalah Menu Tuning & Profile Blank

## 1. Ringkasan Investigasi & Akar Masalah (Root Cause)

Berdasarkan investigasi nyata pada Server **172.18.20.146** dan pelacakan alur eksekusi aplikasi frontend/backend, ditemukan akar masalah yang menyebabkan halaman **Tuning & Profile** menjadi blank (kosong):

1. **Kendala Hak Akses Migrasi PostgreSQL (Table Ownership):**
   - Pada Server 146, tabel `nodes` dibuat dengan pemilik (*owner*) user `postgres`.
   - Backend `netora-api` terhubung menggunakan akun `netora`. Dalam PostgreSQL, perintah `ALTER TABLE` membutuhkan hak kepemilikan tabel (*table owner*) atau superuser.
   - Akibatnya, perintah `ALTER TABLE nodes ADD COLUMN IF NOT EXISTS disk_total_gb INT ...` pada inisialisasi backend gagal secara senyap (*silent failure*).
2. **Kegagalan Query Node List:**
   - Karena kolom `disk_total_gb` belum terbuat di tabel `nodes`, panggilan REST API `GET /api/v1/nodes` menghasilkan error database:
     ```
     pq: column "disk_total_gb" does not exist at position 2:13 (42703)
     ```
3. **Pemutusan Rantai Inisialisasi Frontend:**
   - Pada file [App.tsx](file:///c:/xampp/htdocs/auto-install/unbound/web/src/App.tsx), fungsi `loadInitialData()` memanggil `await api.getNodes()`.
   - Ketika `getNodes()` melempar exception, alur JavaScript langsung melompat ke blok `catch` sebelum sempat memanggil `api.getDesiredState(activeId)` dan `setConfig(cfg)`.
   - Akibatnya, state `config` bernilai `null`.
4. **Blank Render Guard pada PerformanceView:**
   - Pada [PerformanceView.tsx](file:///c:/xampp/htdocs/auto-install/unbound/web/src/views/PerformanceView.tsx) baris 238 terdapat guard:
     ```tsx
     if (!localConfig) return null;
     ```
   - Karena `config` bernilai `null`, komponen merender `null` (layar kosong/blank) tanpa pesan error atau loading spinner.

---

## 2. Perubahan & Perbaikan yang Diterapkan

### A. Backend Storage Resilience ([pkg/storage/postgres.go](file:///c:/xampp/htdocs/auto-install/unbound/pkg/storage/postgres.go))
- Menambahkan fallback query transparan pada metode `UpsertNode()`, `GetNode()`, dan `ListNodes()`.
- Jika PostgreSQL mengembalikan error `column "disk_total_gb" does not exist` (kode SQL 42703), backend secara otomatis mengeksekusi fallback query tanpa kolom disk (`disk_total_gb = 0`, `disk_free_gb = 0`) sehingga endpoint `/api/v1/nodes` **tidak akan pernah mengalami crash atau gagal kirim data**.

### B. Otomatisasi Migrasi Updater ([scripts/netora-dns-update.sh](file:///c:/xampp/htdocs/auto-install/unbound/scripts/netora-dns-update.sh))
- Menambahkan langkah migrasi skema database berbasis root sebelum merestart service `netora-api`:
  ```bash
  su - postgres -c "psql -d netora -c '
      ALTER TABLE IF EXISTS nodes OWNER TO netora;
      ALTER TABLE IF EXISTS nodes ADD COLUMN IF NOT EXISTS disk_total_gb INT NOT NULL DEFAULT 0;
      ALTER TABLE IF EXISTS nodes ADD COLUMN IF NOT EXISTS disk_free_gb INT NOT NULL DEFAULT 0;
      GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO netora;
  '"
  ```

### C. Standardisasi Fresh Installer ([install.sh](file:///c:/xampp/htdocs/auto-install/unbound/install.sh))
- Memperbarui alur migrasi database agar mengeksekusi seluruh berkas skema di direktori `migrations/*.sql` secara berurutan dan mengalihkan seluruh kepemilikan tabel skema publik ke user `netora` (`ALTER TABLE ... OWNER TO ${DB_USER}`).

### D. Isolasi Inisialisasi Frontend ([web/src/App.tsx](file:///c:/xampp/htdocs/auto-install/unbound/web/src/App.tsx))
- Mengisolasi pemanggilan `api.getNodes()`, `api.getDesiredState()`, dan `api.getNodeHealth()` dalam blok `try-catch` terpisah.
- Menambahkan fallback otomatis pendeteksian `node_id` melalui endpoint versi sistem (`/api/v1/system/version`) jika daftar node kosong/gagal, sehingga konfigurasi desired state tetap dapat dimuat dengan sempurna.

### E. Eliminasi Layar Blank ([web/src/views/PerformanceView.tsx](file:///c:/xampp/htdocs/auto-install/unbound/web/src/views/PerformanceView.tsx))
- Menghapus klausa `if (!localConfig) return null;` dan menggantikannya dengan tampilan loading skeleton modern:
  - Ikon spinner cyan beranimasi dengan efek glow.
  - Judul *"Memuat Konfigurasi Hardware & Profil..."*.
  - Informasi node aktif yang sedang dimuat.
- Memperbaiki penanganan `effectiveNodeId = nodeId || node?.id || ''` agar seluruh fitur rotasi log (laporan AI, uji dry-run, dan rotasi darurat) selalu menggunakan ID node yang valid.

---

## 3. Hasil Pengujian & Verifikasi Nyata di Server Produksi

### A. Rilis & Deployment Mandiri (GitHub & Server 146)
- Seluruh kode frontend telah di-*compile* (`npm run build`), aset Web UI (`dist/*`) dipindahkan ke `deploy/ui/`, dan binary Go backend (`netora-api-linux-amd64` & `netora-cli-linux-amd64`) dikompilasi untuk arsitektur Linux AMD64.
- Perubahan di-commit dan di-push ke GitHub:
  - **Commit:** `c869391`
  - **Version:** `v1.0.86 (Build 86)`
- Update dipicu secara otonom melalui HTTP API `POST /api/v1/system/update`. Log rilis `/var/log/netora-dns/update.log` mencatat:
  ```json
  {"time":"2026-09-29T06:52:41Z","level":"INFO","event":"health_check_started","message":"Running comprehensive DNS data-plane health checks"}
  {"time":"2026-09-29T06:52:47Z","level":"INFO","event":"health_check_passed","message":"DNS Health Check: PASS (4/4 probes succeeded)"}
  {"time":"2026-09-29T06:52:47Z","level":"INFO","event":"update_completed","message":"Update to revision c869391dec9883235ba96280cd4f5d5dbe3c0de0 successfully completed in 110s"}
  ```

### B. Verifikasi Skema Database PostgreSQL di Server 146
Query skema tabel `nodes` di database `netora`:
```
                                        Table "public.nodes"
     Column      |           Type           | Collation | Nullable |           Default            
-----------------+--------------------------+-----------+----------+------------------------------
 id              | character varying(64)    |           | not null | 
 hostname        | character varying(128)   |           | not null | 
 management_ip   | character varying(64)    |           | not null | 
 status          | character varying(32)    |           | not null | 'HEALTHY'::character varying
 unbound_version | character varying(64)    |           |          | 
 cpu_cores       | integer                  |           | not null | 1
 ram_mb          | integer                  |           | not null | 1024
 config_json     | jsonb                    |           |          | 
 last_seen_at    | timestamp with time zone |           |          | 
 created_at      | timestamp with time zone |           | not null | now()
 updated_at      | timestamp with time zone |           | not null | now()
 disk_total_gb   | integer                  |           | not null | 0
 disk_free_gb    | integer                  |           | not null | 0
```
- **Owner:** `netora`
- Kolom `disk_total_gb` dan `disk_free_gb` telah aktif dan sinkron.

### C. Verifikasi REST API Telemetri & Desired State Server 146
Pengujian endpoint menggunakan auth token:
1. **`GET /api/v1/nodes`**:
   ```json
   {
       "id": "netora-dns1",
       "hostname": "netora-dns1",
       "management_ip": "172.18.20.146",
       "status": "HEALTHY",
       "unbound_version": "1.19.2",
       "cpu_cores": 4,
       "ram_mb": 15615,
       "disk_total_gb": 14,
       "disk_free_gb": 4
   }
   ```
2. **`GET /api/v1/nodes/netora-dns1/config`**:
   ```
   Desired state loaded: Profile=ISP, Threads=4, MsgCache=1024m
   ```
3. **`GET /api/v1/nodes/netora-dns1/logrotate`**:
   ```
   Logrotate report: Frequency=daily, MaxSize=50M, Title=Keseimbangan Performa & Riwayat Diagnostik
   ```

### D. Verifikasi Aset Web UI di Server 146
- Bundle file baru telah terpasang di `/var/www/netora-ui`:
  - `index-D7kqQxl-.js` (492 KB)
  - `index-DyMxgM4t.css` (45.9 KB)
  - `version.json` menunjukkan versi `v1.0.86 / c869391`
- Request HTTP `GET http://172.18.20.146:8080/` mengembalikan `HTTP 200 OK`.

---

## 4. Status Penyelesaian

- [x] Akar masalah penyebab halaman blank berhasil diidentifikasi tanpa asumsi.
- [x] Hak kepemilikan tabel dan kolom `disk_total_gb`/`disk_free_gb` telah diperbaiki di PostgreSQL.
- [x] Backend Go dilengkapi fallback transparan anti-crash.
- [x] Frontend React bebas dari blank render (`return null`) dan dilengkapi skeleton loader responsif.
- [x] Kode baru berhasil di-*build*, di-push ke GitHub, dan di-*deploy* ke server 146 via auto-update.
- [x] Seluruh endpoint konfigurasi, hardware telemetri, dan rotasi log terverifikasi 100% normal dan operasional.
