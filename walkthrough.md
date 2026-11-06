# Walkthrough — Filter & Kontrol Sanitasi Invalid Domain (Pencegahan Query `null`)

## Ringkasan Perubahan
Menambahkan filter sanitasi domain internet FQDN (*Fully Qualified Domain Name*) pada backend telemetri Unbound ([`pkg/telemetry/query_log.go`](file:///c:/xampp/htdocs/auto-install/unbound/pkg/telemetry/query_log.go)) dan proteksi *defense-in-depth* pada UI Dashboard ([`web/src/views/OverviewView.tsx`](file:///c:/xampp/htdocs/auto-install/unbound/web/src/views/OverviewView.tsx)).

Pembaruan ini secara otomatis menyaring dan membuang query *junk / malformed* yang berasal dari bug coding aplikasi klien (seperti `null`, `undefined`, `localhost`, `wpad`, IP raw) sehingga tidak lagi mengotori peringkat **Top queried domains** maupun **Top blocked domains**.

---

## 1. File yang Diperbarui

### a. Backend Telemetri: [`pkg/telemetry/query_log.go`](file:///c:/xampp/htdocs/auto-install/unbound/pkg/telemetry/query_log.go)
* Menambahkan fungsi `IsCleanFQDN(d string) bool` yang memvalidasi domain dengan aturan:
  1. **Minimal 2 label terpisah titik:** Menghilangkan kata *single-label* seperti `null`, `undefined`, `localhost`, `wpad`.
  2. **Validasi TLD:** TLD minimal 2 karakter alfabet murni (misal `.com`, `.id`, `.net`) atau punycode IDN (`xn--`).
  3. **Penyaringan IP & Reverse DNS:** Menolak raw IP (`192.168.1.1`) dan reverse lookup (`.arpa`).
  4. **Karakter Legal RFC 1035:** Hanya mengizinkan huruf, angka, tanda hubung (`-`), titik (`.`), dan *underscore*. Menolak karakter ilegal atau *double dots* (`..`).
* Mengintegrasikan `validFQDN := IsCleanFQDN(domain)` ke dalam `parseLine`:
  * Query total dan IP klien tetap dihitung sebagai aktivitas jaringan.
  * Hanya domain yang lolos `IsCleanFQDN` yang dicatat ke `t.domainCounts` dan `t.blockedCounts`.

### b. Frontend Dashboard: [`web/src/views/OverviewView.tsx`](file:///c:/xampp/htdocs/auto-install/unbound/web/src/views/OverviewView.tsx)
* Menambahkan filter *defense-in-depth* pada `topQueriedDomains` dan `topBlockedDomains`:
  ```tsx
  const topQueriedDomains = (statsReport?.top_queried_domains || []).filter(
    (d) => d.domain && d.domain !== 'null' && d.domain !== 'undefined' && d.domain.includes('.')
  );
  ```
* Melindungi antarmuka dashboard dari data sampah lama yang mungkin masih tersimpan di cache lokal peramban (`localStorage`).

### c. Unit Tests: [`pkg/telemetry/query_log_test.go`](file:///c:/xampp/htdocs/auto-install/unbound/pkg/telemetry/query_log_test.go)
* Menambahkan fungsi uji `TestIsCleanFQDN` dengan berbagai sampel domain valid dan invalid.
* Menambahkan simulasi log dengan baris `null.` dan `undefined.` untuk membuktikan bahwa domain tersebut tidak masuk ke `TopQueriedDomains`.

---

## 2. Hasil Verifikasi & Uji Regresi
* **Go Tests:** `go test ./...` → **PASS (Exit code: 0)**
* **Frontend Build:** `npm run build` di folder `web/` → **PASS (Exit code: 0)**
  * TypeScript typecheck & bundling Vite berhasil tanpa kendala.
  * Sinkronisasi ke `deploy/ui/` selesai.
