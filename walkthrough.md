# Walkthrough: Penambahan Sistem Logrotate & AI Storage Guard pada Netora DNS Tuning

## Ringkasan Perubahan

Fitur **Host Log Rotation (Logrotate) & Storage AI Guard** telah berhasil ditambahkan ke dalam ekosistem **Tuning & Profiles** Netora DNS. Fitur ini memungkinkan sistem DNS Unbound melakukan auto-tuning siklus rotasi log secara otomatis, kustomisasi penuh sesuai kondisi kapasitas disk & CPU/RAM server, serta memberikan rekomendasi cerdas berbasis analisis perangkat keras (AI Reasoning Advisor) untuk mencegah server crash akibat disk penuh (*disk exhaustion / ENOSPC*).

---

## 1. Temuan Investigasi Nyata di Server

Sebelum implementasi, dilakukan audit diagnostik mendalam pada server produksi dan pengembangan:
1. **Silent Failure pada Logrotate Bawaan:**
   - File `/etc/logrotate.d/unbound` tidak memiliki directive `su unbound unbound`.
   - Pada Linux Ubuntu 20.04/22.04+, `logrotate` secara otomatis menolak dan membatalkan rotasi file jika direktori induknya (`/var/log/unbound`) memiliki permission grup non-root tanpa directive `su`.
   - Hasil pengujian debug `logrotate -d /etc/logrotate.d/unbound`:
     ```text
     error: skipping "/var/log/unbound/unbound.log" because parent directory has insecure permissions
     Set "su" directive in config file to tell logrotate which user/group should be used for rotation.
     ```
2. **Penumpukan Log DNS Query:**
   - File `/var/log/unbound/unbound.log` dan arsipnya membengkak hingga **152 MB** pada server VPS/Edge dengan kapasitas disk hanya 7.8 GB (tersisa hanya 1.2 GB / 84% terpakai).
   - Menghasilkan error sistem `ENOSPC: No space left on device` saat proses backup otomatis berlangsung.
3. **Keterbatasan Sistem Tuning Lama:**
   - Halaman *Performance & Tuning* sebelumnya hanya membaca metrik CPU dan RAM, belum memonitor ruang penyimpanan disk, volume log aktif, maupun status penjadwalan rotasi log host.

---

## 2. Arsitektur & Implementasi Solusi

### A. Backend Go Engine (`pkg/logrotate/`)
Dibuat paket modular baru `pkg/logrotate` dengan komponen:
- **`model.go`**: Mendefinisikan struct `LogrotatePolicy`, `LogTelemetry`, `AIRecommendation`, dan `LogrotateReport`.
- **`engine.go`**:
  - `CalculateAIRecommendation()`: Mesin analisis cerdas yang mengevaluasi CPU cores, total RAM, total disk, sisa disk kosong, persentase penggunaan disk, dan volume log saat ini.
  - `GenerateUnboundConfig()`: Generator konfigurasi `/etc/logrotate.d/unbound` standar dengan proteksi `su unbound unbound`, ambang batas `maxsize`, kompresi gzip, dan perintah atomik `/usr/sbin/unbound-control log_reopen`.
  - `GenerateNetoraConfig()`: Generator konfigurasi `/etc/logrotate.d/netora-dns` dengan mekanisme `copytruncate`.
- **`manager_linux.go`**:
  - `inspectHostLogrotate()`: Membaca statistik log aktif, arsip terotasi, status directive `su`, dan tanggal rotasi terakhir dari `/var/lib/logrotate/status`.
  - `applyHostLogrotate()`: Menulis konfigurasi ke `/etc/logrotate.d/` dan menjalankan verifikasi sintaks via `logrotate -d`.
  - `forceRotateHost()`: Menjalankan `logrotate -f /etc/logrotate.d/unbound` untuk reklamasi ruang disk seketika.
- **`manager_other.go`**: Mock/stub aman untuk lingkungan non-Linux (pengembangan lokal Windows).
- **`engine_test.go`**: Unit test komprehensif untuk algoritma AI, generator konfigurasi, dan format byte.

### B. Auto-Migration Idempotent Database (`pkg/storage/postgres.go`)
Menambahkan perintah auto-migration pada `NewPostgresStore`:
```go
_, _ = db.Exec(`ALTER TABLE nodes ADD COLUMN IF NOT EXISTS disk_total_gb INT NOT NULL DEFAULT 0;`)
_, _ = db.Exec(`ALTER TABLE nodes ADD COLUMN IF NOT EXISTS disk_free_gb INT NOT NULL DEFAULT 0;`)
```
Menjamin seluruh node cluster yang diperbarui otomatis memiliki skema database yang sinkron tanpa perlu eksekusi skrip manual.

### C. REST API Endpoints (`cmd/netora-api/server.go`)
Menambahkan sub-rute baru pada node API:
| Method | Endpoint | Fungsi |
|---|---|---|
| `GET` | `/api/v1/nodes/{id}/logrotate` | Mengambil telemetri log, status keamanan `su`, dan rekomendasi AI |
| `POST` | `/api/v1/nodes/{id}/logrotate/apply` | Menerapkan konfigurasi logrotate ke sistem operasi host |
| `POST` | `/api/v1/nodes/{id}/logrotate/test` | Menjalankan uji validasi dry-run (`logrotate -d`) |
| `POST` | `/api/v1/nodes/{id}/logrotate/force-rotate` | Melakukan rotasi paksa darurat untuk mengosongkan disk |

### D. Frontend Antarmuka NOC (`web/src/views/PerformanceView.tsx`)
1. **Tab Switcher Navigasi:**
   - `Semua Sistem Tuning`
   - `CPU & Cache Engine`
   - `Logrotate & Storage AI Guard` (dilengkapi badge peringatan jika status log tidak sehat).
2. **4 Kartu Telemetri Real-Time:**
   - Log Aktif Unbound (`/var/log/unbound/unbound.log`).
   - Arsip Terotasi (jumlah file & total ukuran terkompresi).
   - Total Jejak Log di Disk (termasuk updater log).
   - Status Kesehatan Rotasi (indikator hijau/merah untuk directive `su`).
3. **Hero Card Rekomendasi AI:**
   - Menjelaskan rasional teknis mengapa konfigurasi tertentu direkomendasikan untuk hardware spesifik server.
   - Tombol **"⚡ Auto-Tune dengan AI"** untuk menerapkan parameter ideal dalam satu klik.
4. **Preset Profil:**
   - *Ultra Storage Saver* (Disk &le; 10GB / Free &le; 2GB): Daily, 2–3 rotasi, maxsize 15M–25M, gzip + delaycompress.
   - *Balanced Standard* (Disk 10–40GB): Daily, 7 rotasi, maxsize 50M, gzip + delaycompress.
   - *Carrier Audit Retention* (Disk &gt; 40GB): Weekly, 30 rotasi, maxsize 100M, gzip.
5. **Form Kustomisasi & Live Syntax Preview:**
   - Kontrol frekuensi, jumlah retensi, ambang batas `maxsize`, kompresi gzip, dan penundaan kompresi (`delaycompress`).
   - Pratinjau teks `/etc/logrotate.d/unbound` secara langsung.
6. **Action Toolbar & Interactive Console:**
   - Tombol Uji Validasi (Dry Run), Rotasi Sekarang (Paksa & Kosongkan), dan Terapkan Konfigurasi.
   - Jendela konsol terminal interaktif untuk melihat log eksekusi secara transparan.

---

## 3. Hasil Pengujian & Verifikasi

### Pengujian di Server Dev (172.18.20.145)
1. **Pengambilan Telemetri & Rekomendasi AI:**
   - Terdeteksi: Disk 7.8GB, sisa 1GB (85% terpakai), log Unbound 151.7 MB.
   - Deteksi masalah: Directive `su unbound unbound` hilang (status: `CRITICAL`).
   - Rekomendasi AI: Profil `STORAGE_SAVER` (Daily, retensi 2x, maxsize 15M, kompresi gzip aktif).
2. **Penerapan Konfigurasi Otomatis:**
   - File `/etc/logrotate.d/unbound` berhasil diperbarui dengan menyertakan `su unbound unbound`.
   - Validasi `logrotate -d` berjalan mulus tanpa peringatan permission.
3. **Eksekusi Rotasi Paksa (Reclaim Disk):**
   - File log Unbound terpangkas dari **151.7 MB** menjadi **31.4 MB**.
   - Sisa ruang disk meningkat dari 1.2 GB menjadi **1.4 GB**.
   - Directive `su unbound unbound` terkonfirmasi aktif dan aman.

---

## 4. Panduan Operasional NOC

1. Buka menu **Tuning & Profiles** di Web UI Netora DNS (`http://<ip-node>:8080/`).
2. Pilih tab **Logrotate & Storage AI Guard**.
3. Periksa kartu status telemetri dan kartu rekomendasi AI.
4. Untuk menerapkan rekomendasi ideal, klik tombol **"Auto-Tune dengan AI"**, lalu klik **"Terapkan Konfigurasi Logrotate"**.
5. Jika server mengalami lonjakan log mendadak yang menghabiskan disk, klik tombol **"Rotasi Sekarang (Paksa & Kosongkan)"** untuk membebaskan ruang disk seketika tanpa menghentikan layanan DNS Unbound.
