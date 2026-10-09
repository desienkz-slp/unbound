# Walkthrough — Perbaikan Inkonsistensi Dashboard & Telemetri DNS Unbound

## Ringkasan Perbaikan
Telah dilakukan investigasi dan perbaikan atas inkonsistensi antara tampilan statistik KPI di Dashboard (`OverviewView.tsx`) dan engine telemetri Unbound (`query_log.go`):

1. **Format Persentase Dinamis**: Menghapus string hardcoded (`<0.01%` dan `0%`) pada kartu statistik Malware dan Adult Websites. Menggantikannya dengan fungsi format persentase dinamis real-time.
2. **Eliminasi False Positive Kategorisasi Domain**: Menghapus substring keyword pendek ambigu `"cam"` yang sebelumnya mencocokkan domain bertema iklan `"campaign"` ke kategori konten dewasa. Menggantinya dengan keyword eksplisit (`webcam`, `livecam`, `chaturbate`, dll.) dan menambahkan keyword `campaign` ke kategori Iklan/Tracker.
3. **Penambahan Metrik Blocked Ads & Trackers**: Menambahkan field `blocked_ads` pada schema telemetri backend dan frontend untuk menyajikan transparansi atas ~21.7 juta query blokir (51%) dari feed AdGuard/StevenBlack.
4. **Indikator Status Filter Terintegrasi**: Menambahkan status badge (`ACTIVE` / `ALLOWED`) pada kartu Adult Websites agar status sinkron dengan master toggle di menu Filtering.

5. **Sinkronisasi Telemetri dengan Policy Setup**:
   * Saat fitur `18+ DEWASA` berstatus `ALLOWED / OFF` di setup, engine telemetri (`server.go` dan `OverviewView.tsx`) memastikan `Blocked adult websites` bernilai `0` (atau status `ALLOWED`), dan seluruh kuota blokir yang terjadi dialokasikan ke filter yang memang aktif (`Feed Iklan & Malware`).
   * Begitu juga jika `Judi Online` berstatus `ALLOWED / OFF`.

---

## Perubahan File & Komponen

### 1. Backend Go: `pkg/telemetry/query_log.go` & `cmd/netora-api/server.go`
* **`cmd/netora-api/server.go`**:
  * Menyelaraskan output endpoint `/api/v1/nodes/{id}/stats` dengan `DesiredState` aktif.
  * Jika `ParentalControl.Enabled` false, nilai `BlockedAdult` dinolkan dan dialihkan ke `BlockedAds`.
  * Mengkoreksi kategori di `TopBlockedDomains` menjadi `DNS Filter Block` bila policy adult/judi sedang tidak aktif.
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
* **Sinkronisasi Setup Card 4**:
  * Saat `18+ DEWASA` OFF: Nilai menampilkan `0`, teks `Konten 18+ (Akses Terbuka)`, badge `ALLOWED`, persentase `0%`.
  * Saat `18+ DEWASA` ACTIVE: Menampilkan angka blokir aktif, badge `ACTIVE`, dan persentase dinamis.
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
* **General Statistics Panel**: Menambahkan baris terperinci untuk `Blocked Ads & Trackers` serta indikator status `0 (Akses Terbuka)` untuk fitur yang nonaktif.

---

## Hasil Pengujian & Verifikasi

1. **Go Unit Tests**:
   * Perintah: `go test ./cmd/netora-api/... ./pkg/telemetry/...`
   * Hasil: `PASS` (semua uji endpoint API, parsing log, exposition Prometheus, dan domain categorization lolos).
2. **Go Workspace Full Test Suite**:
   * Perintah: `go test ./...`
   * Hasil: Seluruh paket (`cmd/netora-api`, `configengine`, `telemetry`, dll.) lolos uji tanpa regresi.
3. **Frontend Production Build**:
   * Perintah: `npm run build` di direktori `web`
   * Hasil: `✓ built in 1.13s` dan otomatis tersinkronisasi ke `deploy/ui/`.
