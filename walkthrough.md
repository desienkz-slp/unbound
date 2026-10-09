# Walkthrough — Perbaikan Inkonsistensi Dashboard & Telemetri DNS Unbound

## Ringkasan Perbaikan
Telah dilakukan investigasi dan perbaikan atas inkonsistensi antara tampilan statistik KPI di Dashboard (`OverviewView.tsx`) dan engine telemetri Unbound (`query_log.go`):

1. **Format Persentase Dinamis**: Menghapus string hardcoded (`<0.01%` dan `0%`) pada kartu statistik Malware dan Adult Websites. Menggantikannya dengan fungsi format persentase dinamis real-time.
2. **Eliminasi False Positive Kategorisasi Domain**: Menghapus substring keyword pendek ambigu `"cam"` yang sebelumnya mencocokkan domain bertema iklan `"campaign"` ke kategori konten dewasa. Menggantinya dengan keyword eksplisit (`webcam`, `livecam`, `chaturbate`, dll.) dan menambahkan keyword `campaign` ke kategori Iklan/Tracker.
3. **Penambahan Metrik Blocked Ads & Trackers**: Menambahkan field `blocked_ads` pada schema telemetri backend dan frontend untuk menyajikan transparansi atas ~21.7 juta query blokir (51%) dari feed AdGuard/StevenBlack.
4. **Indikator Status Filter Terintegrasi**: Menambahkan status badge (`ACTIVE` / `ALLOWED`) pada kartu Adult Websites agar status sinkron dengan master toggle di menu Filtering.

---

## Perubahan File & Komponen

### 1. Backend Go: `pkg/telemetry/query_log.go`
* **Struct `DetailedStatsReport`**:
  * Menambahkan field `BlockedAds int64` (`json:"blocked_ads"`).
* **Fungsi `CategorizeDomain()`**:
  * Memprioritaskan pengecekan Malware & Security Threats terlebih dahulu.
  * Memperbaiki `adultKeywords`: menghapus keyword ambigu `"cam"` dan menambahkan whitelist/boundary check agar tidak salah mencocokkan kata seperti `campaign` atau nama daerah `essex`/`sussex`.
  * Menambahkan keyword `"campaign"` ke dalam `adKeywords`.
* **Fungsi `GetDetailedReport()`**:
  * Mengakumulasikan hitungan `adsBlocked`.
  * Mengalokasikan residu blokir NXDOMAIN ke kategori Ads & Trackers.

### 2. Unit Test: `pkg/telemetry/query_log_test.go`
* Menambahkan pengujian `CategorizeDomain("adcampaign.google.com")` yang terverifikasi menghasilkan `"Advertising / Tracker"`.
* Menambahkan pengujian `CategorizeDomain("livecam.adult.com")` yang terverifikasi menghasilkan `"18+ Adult"`.
* Memverifikasi nilai `BlockedAds >= 0`.

### 3. Frontend Types: `web/src/types.ts`
* Menambahkan `blocked_ads?: number;` pada interface `DetailedStatsReport`.

### 4. Frontend View: `web/src/views/OverviewView.tsx`
* **State `stats`**: Ditambahkan properti `blockedAds: 0` dan mapping dari response API `s.blocked_ads`.
* **Fungsi Helper `formatPercentage()`**:
  ```ts
  const formatPercentage = (count: number, total: number): string => {
    if (!total || count <= 0) return '0%';
    const pct = (count / total) * 100;
    if (pct < 0.01) return '<0.01%';
    if (pct < 1) return `${pct.toFixed(2)}%`;
    return `${pct.toFixed(1)}%`;
  };
  ```
* **Card 3 (Blocked Malware/Phishing)**: Menggunakan `formatPercentage(stats.blockedMalware, stats.totalQueries)`.
* **Card 4 (Blocked Adult Websites)**: Menggunakan `formatPercentage(stats.blockedAdult, stats.totalQueries)` serta menyertakan badge status `ACTIVE` / `ALLOWED` sesuai konfigurasi `config?.parental_control?.enabled`.
* **General Statistics Panel**: Menambahkan baris terperinci untuk `Blocked Ads & Trackers`, `Blocked malware/phishing`, dan `Blocked adult websites` dengan ketinggian panel yang disesuaikan.

---

## Hasil Pengujian & Verifikasi

1. **Go Unit Tests**:
   * Perintah: `go test -v ./pkg/telemetry/...`
   * Hasil: `PASS` (semua uji parsing log, dump_infra, exposition Prometheus, dan kategorisasi domain lulus 100%).
2. **Go Workspace Full Test Suite**:
   * Perintah: `go test ./...`
   * Hasil: Seluruh paket (`cmd/netora-api`, `configengine`, `telemetry`, dll.) lolos uji.
3. **Frontend Production Build**:
   * Perintah: `npm run build` di direktori `web`
   * Hasil: `✓ built in 7.65s` dan otomatis tersinkronisasi ke `deploy/ui/`.
