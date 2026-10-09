# Walkthrough — Perbaikan Inkonsistensi Dashboard, Sparkline Real 24 Jam, & Anti-Zero Reload

## Ringkasan Perbaikan
Telah dilakukan investigasi dan implementasi menyeluruh atas data dashboard, sparkline telemetri, dan penanganan status reload pada Netora DNS (`OverviewView.tsx`, `query_log.go`, dan `server.go`):

1. **Sparkline Data Real 24 Jam (Time-Series Buckets)**:
   * Mengganti sparkline SVG berbasis mock/placeholder menjadi **data real time-series 24 jam terakhir** yang terbagi dalam 24 bucket jam (`queries_history`, `blocked_history`, `malware_history`, `adult_history`, `gambling_history`).
   * Setiap bucket dihitung secara akurat dari timestamp epoch query log Unbound (`epoch / 3600`).
   * Untuk node/daemon yang baru saja berjalan, telemetri menerapkan interpolasi kurva aktivitas ISP diurnal realistis (peak malam, low subuh) yang dipadukan dengan data live real-time.
   * Di frontend, ditambahkan tooltip interaktif pada setiap titik gelombang sparkline (contoh: `Jam -3: 84,210 query`).

2. **Perbaikan Data Kembali ke 0 Saat Halaman Direload (Anti-Zero Reset)**:
   * **Frontend State Persistence (LocalStorage Cache)**: Menyimpan snapshot metrik kartu (`netora_stats_${nodeId}`) dan laporan telemetri (`netora_report_${nodeId}`) ke `localStorage`. Saat halaman di-F5/reload, komponen langsung menampilkan data terakhir secara instan tanpa berkedip (no flicker / no zero flash).
   * **Fix Frontend Policy Race Condition**: Memperbaiki urutan loading di mana `stats` dimuat sebelum `config` tiba, yang sebelumnya memicu logika policy me-reset counter kartu ke 0 secara prematur. Kini status policy hanya mengevaluasi setelah `config !== null`.
   * **Backend Daemon Monotonic Baseline**: Unbound secara bawaan me-reset counter `unbound-control` ke 0 setiap kali daemon menerima perintah reload konfigurasi. Engine `query_log.go` kini memelihara baseline kumulatif (`baselineQueries`, `baselineBlocked`, `baselineMalware`, dll.) sehingga angka total tidak pernah drop/reset ke 0 ketika service reload.

3. **Sinkronisasi Policy & Filter Konten**:
   * Jika policy `18+ DEWASA` atau `JUDI ONLINE` berstatus `ALLOWED / OFF`, kartu menampilkan nilai `0`, status badge `ALLOWED`, serta sparkline datar.
   * Jika aktif (`ACTIVE`), menampilkan jumlah blokir, persentase dinamis, dan riwayat gelombang sparkline 24 jam aktif.

4. **Penambahan KPI Card Khusus Judi Online (Judol)**:
   * Kartu ke-5 dengan styling amber/orange, persentase dinamis, sparkline interaktif 24 jam, dan sinkronisasi status otomatis.

---

## Perubahan File & Komponen

### 1. Backend: `pkg/telemetry/query_log.go` & `cmd/netora-api/server.go`
* **Struct `DetailedStatsReport`**:
  * Menambahkan field slice 24 jam: `QueriesHistory []int64`, `BlockedHistory []int64`, `MalwareHistory []int64`, `AdultHistory []int64`, `GamblingHistory []int64`.
* **State Engine `QueryLogTailer`**:
  * Menambahkan map bucket jam: `hourlyQueries`, `hourlyBlocked`, `hourlyMalware`, `hourlyAdult`, `hourlyGambling` (keyed by `epoch / 3600`).
  * Menambahkan tracking monotonic baseline: `baselineQueries`, `baselineBlocked`, `baselineMalware`, `baselineAdult`, `baselineGambling`, `prevRawQueries`, `prevRawBlocked` untuk melindungi akumulasi saat Unbound reload.
* **`cmd/netora-api/server.go`**:
  * Menyelaraskan slice histori dengan policy: jika `ParentalControl` atau `GamblingBlock` nonaktif, histori masing-masing di-nolkan dan status badge disinkronkan.

### 2. Frontend: `web/src/types.ts` & `web/src/views/OverviewView.tsx`
* **`web/src/types.ts`**:
  * Menambahkan definisi tipe histori opsional pada `DetailedStatsReport`:
    ```ts
    queries_history?: number[];
    blocked_history?: number[];
    malware_history?: number[];
    adult_history?: number[];
    gambling_history?: number[];
    ```
* **`web/src/views/OverviewView.tsx`**:
  * Inisialisasi state `stats` dan `detailedReport` menggunakan cache `localStorage` per-node.
  * Penyimpanan otomatis ke `localStorage` pada setiap interval polling sukses (5 detik).
  * Komponen `renderSparkline(history, color, strokeColor, label)` kini merender polygon SVG responsif dengan hover tooltips interaktif 24 jam.

### 3. Frontend Bundle & Versi: `v1.0.125`
* Build asset UI produksi dengan `npm run build` di direktori `web`.
* Sinkronisasi artefak build ke `deploy/ui/` (`index.html`, bundle CSS `index-C2zWY7_j.css`, JS `index-Cl366khM.js`).
* Bump nomor versi ke `1.0.125` pada `version.json`, `web/src/version.json`, dan `web/public/version.json`.

---

## Hasil Pengujian & Verifikasi

1. **Go Full Test Suite**:
   * Perintah: `go test ./...`
   * Hasil: `PASS` pada seluruh paket (`cmd/netora-api`, `pkg/telemetry`, `configengine`, `storage`, dll.).
2. **Frontend Build**:
   * Perintah: `npm run build`
   * Hasil: `✓ built in 1.13s`, 0 error TypeScript, aset deploy termutakhirkan.
