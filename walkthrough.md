# Walkthrough — Penambahan Domain Detector & Captive Portal ke Whitelist

## Ringkasan Perubahan
Domain detector internet dan *captive portal probe* dari berbagai ekosistem perangkat (Android/Google, Apple, Microsoft Windows, Xiaomi, Huawei, Linux) telah resmi didaftarkan ke dalam **whitelist pengecualian** (`local-zone: ... transparent`).

Langkah ini menjamin bahwa perangkat pengguna tidak akan mengalami salah deteksi jaringan seperti tanda seru **(!)** pada ikon Wi-Fi atau pesan palsu *"Tersambung tanpa internet"* (*Connected, no internet*) akibat filter iklan/malware/custom yang terlalu ketat.

---

## 1. File yang Diperbarui

### a. [`blocklist/whitelist.txt`](file:///c:/xampp/htdocs/auto-install/unbound/blocklist/whitelist.txt)
Menambahkan aturan ABP-format (`@@||domain^`) untuk domain detector dan bank nasional:
* **Google / Android / ChromeOS:**
  * `connectivitycheck.gstatic.com`
  * `connectivitycheck.android.com`
  * `clients3.google.com`
  * `play.googleapis.com`
* **Apple (iOS, iPadOS, macOS, watchOS):**
  * `captive.apple.com`
  * `appleiphonecell.com`
  * `airport.us`
* **Microsoft Windows (NCSI):**
  * `msftconnecttest.com`
  * `msftncsi.com`
  * `ipv6.msftconnecttest.com`
* **Xiaomi (MIUI / HyperOS):**
  * `connect.rom.miui.com`
* **Huawei (HarmonyOS / EMUI):**
  * `connectivitycheck.platform.hicloud.com`
* **Linux / Ubuntu (NetworkManager):**
  * `connectivity-check.ubuntu.com`
* **Core & Banking (Preserved & Aligned):**
  * `wa.me`, `doubleclick.net`, `googleadservices.com`, `bankmandiri.co.id`, `bca.co.id`

### b. [`pkg/configengine/generator.go`](file:///c:/xampp/htdocs/auto-install/unbound/pkg/configengine/generator.go)
Memperluas array `DefaultWhitelistedDomains` sehingga jika sistem dijalankan dengan fallback atau file `whitelist.txt` belum dimuat di node baru, Unbound tetap meng-generate zona `transparent` untuk seluruh domain probe ini secara otomatis.

---

## 2. Hasil Verifikasi & Pengujian
Pengujian unit test backend dijalankan untuk memastikan tidak ada regresi:
* Perintah: `go test ./...`
* Status: **PASS (Exit code: 0)**
  * `netora-dns/pkg/configengine` — PASS
  * `netora-dns/pkg/blocklist` — PASS
  * `netora-dns/pkg/policy` — PASS
  * `netora-dns/cmd/netora-api` — PASS
