# Walkthrough — Default DNS Rewrites Otomatis Sesuai IP Server & Nama Node

## Ringkasan Pekerjaan
Telah diimplementasikan mekanisme otomatis yang menjamin **setiap server memiliki default DNS Rewrite (A record) sesuai dengan IP aktif dari server tersebut, dengan nama domain persis sesuai nama node**:
* **Jika server memiliki 1 IP**: Memiliki **1 DNS rewrite** secara default (`<nama_node> -> <IP1>`).
* **Jika server memiliki 2 IP**: Memiliki **2 DNS rewrite** secara default (`<nama_node> -> <IP1>` dan `<nama_node> -> <IP2>`).
* **Preservasi Custom Records**: Rewrite custom milik user untuk domain lain tetap terjaga dan tidak tertimpa/terhapus.

---

## Perubahan Komponen & File

### 1. Model Domain: `pkg/model/config.go` & `pkg/model/config_test.go`
* Menambahkan method `EnsureNodeDnsRewrites(nodeName string, ips []string) bool` pada struct `*DesiredState`:
  * Jika daftar IP tidak diberikan secara eksplisit, method otomatis mengekstrak seluruh IPv4 konkret dari listener aktif node (`state.Listeners`).
  * Memfilter dan mendeduplikasi IP (mengabaikan `127.0.0.1`, `0.0.0.0`, dan IPv6).
  * Menyelaraskan A record untuk domain `nodeName`:
    * 1 IP menghasilkan tepat 1 record A dengan TTL 300.
    * 2 IP menghasilkan tepat 2 record A dengan TTL 300.
  * Bersifat idempoten (tidak menduplikasi record bila sudah sesuai).
* Menambahkan unit test komprehensif di `pkg/model/config_test.go`:
  * `TestEnsureNodeDnsRewrites_SingleIP`: Verifikasi 1 IP -> 1 rewrite.
  * `TestEnsureNodeDnsRewrites_DualIP`: Verifikasi 2 IP -> 2 rewrites.
  * `TestEnsureNodeDnsRewrites_PreservesOtherDomains`: Verifikasi custom record domain lain tidak hilang.
  * `TestEnsureNodeDnsRewrites_ExtractFromListeners`: Verifikasi fallback ekstraksi IP dari konfigurasi listener.

### 2. Config Engine Generator: `pkg/configengine/generator.go` & `pkg/configengine/generator_test.go`
* Pada `generator.go`, memperbaiki auto-injection rDNS PTR dan record A agar mendeteksi keberadaan A record yang sudah ada (`existingAs`), mencegah duplikasi baris `local-data` pada `conf.d/local-records.conf`.
* Menambahkan pengujian `TestGenerator_NodeDnsRewrites` pada `generator_test.go` untuk memastikan file konfigurasi Unbound dirender dengan tepat (1 baris untuk 1 IP, 2 baris untuk 2 IP tanpa duplikasi).

### 3. Storage Layer: `pkg/storage/postgres.go`
* Pada fungsi `GetDesiredState()`, menambahkan pemanggilan `EnsureNodeDnsRewrites()` sebelum mengembalikan konfigurasi node. Hal ini memastikan database dan API query selalu menyertakan default rewrite untuk nama node tersebut meskipun konfigurasi lama di DB belum memilikinya.

### 4. API Control Plane: `cmd/netora-api/main.go` & `cmd/netora-api/server.go`
* **`main.go`**:
  * Pada inisialisasi node baru (`!hasConcreteListeners`): menyertakan `EnsureNodeDnsRewrites(*nodeID, activeIPs)` saat membuat `baseState`.
  * Pada node yang sudah ada (`existingState != nil`): menyinkronkan rewrites node jika terdeteksi perubahan atau record belum ada.
* **`server.go`**:
  * Menambahkan field `mgmtIP` pada struct `Server`.
  * Pada endpoint `GET /api/v1/nodes/{id}/config`, memastikan konfigurasi yang dikembalikan ke UI NOC selalu memiliki default DNS rewrite sesuai IP node lokal.
  * Pada endpoint `POST /api/v1/nodes/{id}/apply`, memastikan default rewrite untuk nama node tetap dipertahankan saat apply dilakukan via web UI.

### 5. Installer Shell Script: `install.sh`
* Menambahkan modul auto-detection IPv4 pada tahap konfigurasi Unbound (antara pembuatan `unbound.conf` dan validasi `unbound-checkconf`):
  * Mendeteksi seluruh alamat IP host publik/LAN non-loopback melalui `ip -4 -o addr` atau `hostname -I`.
  * Menulis `/etc/unbound/conf.d/local-records.conf` yang berisi `local-data: "<NODE_ID> 300 A <IP>"` dan `local-data-ptr: "<IP> <NODE_ID>"` untuk setiap IP yang terdeteksi.

### 6. Frontend Setup Wizard: `web/src/views/WizardView.tsx`
* Mengubah inisialisasi `local_records` pada Wizard agar menggunakan `nodeName` dinamis (`nodeId || node?.id || 'dns-node'`) dan menyertakan `clientIp` aktual, bukan nilai dummy statis.
* Build asset UI termutakhirkan ke versi `v1.0.126`.

---

## Hasil Pengujian & Verifikasi

1. **Unit Test Go (`pkg/model`)**:
   ```
   === RUN   TestEnsureNodeDnsRewrites_SingleIP
   --- PASS: TestEnsureNodeDnsRewrites_SingleIP (0.00s)
   === RUN   TestEnsureNodeDnsRewrites_DualIP
   --- PASS: TestEnsureNodeDnsRewrites_DualIP (0.00s)
   === RUN   TestEnsureNodeDnsRewrites_PreservesOtherDomains
   --- PASS: TestEnsureNodeDnsRewrites_PreservesOtherDomains (0.00s)
   === RUN   TestEnsureNodeDnsRewrites_ExtractFromListeners
   --- PASS: TestEnsureNodeDnsRewrites_ExtractFromListeners (0.00s)
   ok  	netora-dns/pkg/model	0.351s
   ```

2. **Unit Test Go (`pkg/configengine`)**:
   ```
   === RUN   TestGenerator_NodeDnsRewrites
   --- PASS: TestGenerator_NodeDnsRewrites (0.01s)
   ok  	netora-dns/pkg/configengine	0.563s
   ```

3. **Full Project Go Test Suite**:
   * Perintah: `go test ./...`
   * Hasil: `PASS` pada seluruh paket (`cmd/netora-api`, `pkg/model`, `pkg/storage`, `pkg/configengine`, dll.).

4. **Production Frontend Build**:
   * Perintah: `npm run build` di direktori `web`
   * Hasil: `✓ built in 1.20s`, tersinkronisasi ke `deploy/ui/` (`v1.0.126`).
