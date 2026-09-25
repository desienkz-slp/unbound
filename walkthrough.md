# Walkthrough: Solusi SafeSearch, Toggle Parental Control (18+) & Toggle Judi Online Netora DNS

Dokumen ini mendokumentasikan investigasi teknis, perbaikan arsitektur DNS Unbound untuk mengatasi sensor ISP, implementasi **Katalog Upstream DoH / DoT dengan Switch Toggle ON/OFF**, implementasi **Sistem Toggle ON/OFF pada DNS Policy Filtering & Hosts Files**, penambahan **Toggle ON/OFF Parental Control (18+ Dewasa)**, serta penambahan **Dedicated Toggle ON/OFF Judi Online (Gambling Block)** yang masing-masing bekerja secara independen dan modular.

---

## 1. Ringkasan Masalah & Bukti Investigasi Nyata

Sebelumnya, pengguna melaporkan error saat mengakses domain dewasa secara langsung melalui DNS server `172.18.20.145`:

```cmd
C:\Users\desie>nslookup xhamster.com
Server:  UnKnown
Address:  172.18.20.145

*** UnKnown can't find xhamster.com: Server failed
```

### Fakta Hasil Investigasi Server (`172.18.20.145`):
1. **Unbound TIDAK Memblokir Domain Dewasa:**
   - Di `/etc/unbound/conf.d/`, **TIDAK ADA** file filter domain dewasa (`adult-redirect.conf` dinonaktifkan).
   - Hanya rule SafeSearch (`safesearch.conf`) yang aktif untuk membelokkan mesin pencari (Google, Bing) ke VIP SafeSearch.
2. **Akar Masalah Nyata: DPI / Pembajakan DNS Port 53 oleh ISP (Kominfo/Internet Positif):**
   - Sebelumnya, Unbound dikonfigurasikan meneruskan query ke upstream via port biasa **UDP 53** (`1.1.1.2`, `9.9.9.9`, dll.).
   - Jalur internet keluar server `172.18.20.145` mengalami **Transparent DNS Interception/Hijacking** oleh ISP.
   - Uji `dig @1.1.1.1 xhamster.com` dijawab dalam waktu **0 ms** dengan IP `182.23.79.195` (Landing Page Kominfo/Trust Positif).
3. **Mengapa Terjadi `Server failed` (DNS SERVFAIL)?**
   - Unbound menjalankan validasi **DNSSEC aktif** (`module-config: "validator iterator"`).
   - Jawaban palsu IP Kominfo tidak memiliki tanda tangan digital kriptografi yang sah (*DNSSEC spoofing*).
   - Unbound secara cerdas mendeteksi paket tersebut sebagai **BOGUS / DNS Tampering** dan menolak menyerahkan IP palsu tersebut ke pengguna, lalu mengembalikan status **SERVFAIL (`Server failed`)**.

---

## 2. Solusi Teknis: DNS-over-TLS (DoT) & Enkripsi Upstream

Untuk memotong manipulasi ISP tanpa mematikan DNSSEC atau SafeSearch, Unbound dikonfigurasi menggunakan **DNS-over-TLS (DoT pada Port TCP 853)** dengan bundel sertifikat CA TLS terverifikasi:

```conf
server:
  tls-cert-bundle: "/etc/ssl/certs/ca-certificates.crt"

forward-zone:
  name: "."
  forward-first: yes
  forward-tls-upstream: yes
  forward-addr: 1.1.1.1@853#cloudflare-dns.com
  forward-addr: 1.0.0.1@853#cloudflare-dns.com
  forward-addr: 8.8.8.8@853#dns.google
  forward-addr: 8.8.4.4@853#dns.google
```

Jalur TCP 853 terenkripsi TLS tidak dapat diintip maupun dibajak oleh ISP. Jawaban asli dari Cloudflare/Google diterima dengan aman dan validasi DNSSEC berstatus `NOERROR`.

---

## 3. Matriks Fitur Independen: SafeSearch vs 18+ Dewasa vs Judi Online

Tiga pilar proteksi konten kini dapat dikendalikan secara terpisah sesuai kebutuhan:

| Fitur | Tujuan Proteksi | Target Intersepsi | Status Default | Saklar Mandiri |
| :--- | :--- | :--- | :---: | :---: |
| **SafeSearch** | Menyaring hasil pencarian mesin pencari | Google, Bing, YouTube VIP (`216.239.38.120`) | **ON** | Ya (ToggleSwitch) |
| **18+ Dewasa** | Memblokir akses domain dewasa/pornografi | `xhamster.com`, `pornhub.com`, dll. | **Bebas Dipilih** | Ya (ToggleSwitch) |
| **Judi Online** | Memblokir akses domain judi, slot & taruhan | `sbobet.com`, `slot88.com`, `pragmaticplay.com`, dll. | **Bebas Dipilih** | Ya (ToggleSwitch) |

---

## 4. Arsitektur Filter Judi Online di Unbound

1. **File Konfigurasi Dedicated:** `/etc/unbound/conf.d/gambling-filter.conf`
   - **Saat Judi Online ON:**
     Unbound me-load file konfigurasi berisi *local-zone static sinkholes* (atau redirect IP) untuk domain judi & slot populer:
     ```conf
     server:
       local-zone: "sbobet.com." static
       local-zone: "maxbet.com." static
       local-zone: "slot88.com." static
       local-zone: "slot777.com." static
       local-zone: "pragmaticplay.com." static
       ...
     ```
     Setiap query DNS dari klien langsung dibalas dengan status `NXDOMAIN` (Domain tidak ditemukan/tidak dapat dibuka).
   - **Saat Judi Online OFF:**
     Unbound me-replace file secara atomik menjadi komentar nonaktif:
     ```conf
     # Judi Online / Gambling Filter is DISABLED (Gambling websites allowed)
     ```
     Query DNS diteruskan secara terenkripsi DoT ke upstream global tanpa blokir.

2. **Dukungan Feed Blocklist Eksternal:**
   Tersedia preset blocklist hosts judi di modal *Add Blocklist*:
   - URL: `https://raw.githubusercontent.com/StevenBlack/hosts/master/alternates/gambling/hosts`

3. **Persistensi Database PostgreSQL (`nodes.config_json`):**
   Seluruh status switch (SafeSearch, Parental Control, Judi Online) disimpan pada kolom `config_json JSONB` di tabel `nodes`. Data tidak akan hilang saat server di-reboot atau halaman di-refresh.

---

## 5. Bukti Verifikasi Pengujian Nyata

Semua pengujian dijalankan langsung dari terminal Windows pengguna dan diverifikasi pada server Unbound:

### Uji 1: Saat Judi Online ON (Aktif)
```cmd
C:\Users\desie>nslookup sbobet.com 172.18.20.145
Server:  UnKnown
Address:  172.18.20.145

*** UnKnown can't find sbobet.com: Non-existent domain

C:\Users\desie>nslookup slot88.com 172.18.20.145
Server:  UnKnown
Address:  172.18.20.145

*** UnKnown can't find slot88.com: Non-existent domain
```
> **Hasil:** Query langsung di-sinkhole / diblokir dengan status **Non-existent domain (`NXDOMAIN`)**.

---

### Uji 2: Saat Judi Online OFF (Nonaktif)
```cmd
C:\Users\desie>nslookup sbobet.com 172.18.20.145
Server:  UnKnown
Address:  172.18.20.145

Non-authoritative answer:
Name:    sbobet.com
Address:  185.115.167.129
```
> **Hasil:** Domain judi kembali dapat diakses dan mengembalikan IP asli tanpa error.

---

### Uji 3: Independensi Penuh Antar Fitur
Ketika **Judi Online = ON**, tetapi **Parental Control (18+) = OFF**:

1. Query domain judi `sbobet.com` -> **DIBLOKIR (`NXDOMAIN`)**
2. Query domain dewasa `xhamster.com` -> **TERBUKA BEBAS**:
   ```cmd
   C:\Users\desie>nslookup xhamster.com 172.18.20.145
   Name:    xhamster.com
   Addresses:  104.16.4.81, 104.16.3.81
   ```
3. Query pencarian `www.google.com` -> **SAFESEARCH AMAN**:
   ```cmd
   C:\Users\desie>nslookup www.google.com 172.18.20.145
   Name:    www.google.com
   Address:  216.239.38.120
   ```
> **Kesimpulan:** Setiap fitur bekerja secara modular tanpa saling mengganggu.

---

## 6. Tampilan Baru pada Web UI Netora DNS

Pada menu **Policy Filtering** (`http://172.18.20.145:8080`), baris KPI card di bagian atas kini menampilkan 5 modul kartu responsif:

1. **THREAT FEEDS**: Jumlah feed aktif & total rule Trie.
2. **SAFESEARCH**: Switch ON/OFF pemaksaan mode aman Google, Bing, YouTube.
3. **18+ DEWASA**: Switch ON/OFF pemblokiran situs pornografi & konten dewasa (Warna Ungu).
4. **JUDI ONLINE**: Switch ON/OFF pemblokiran situs judi online, taruhan, & slot gacor (Warna Amber / Oranye dengan ikon Dadu).
5. **ALLOWLIST**: Pengecualian domain bypass.

---

## 7. Cara Menggunakan via Web UI

1. Buka browser ke: **`http://172.18.20.145:8080`**  
   *(Login dengan user: `admin` / pass: `admin123` jika diminta)*
2. Buka menu **Policy Filtering** di bilah kiri.
3. **Mengatur Judi Online:**
   - Lihat kartu ke-4 bertuliskan **JUDI ONLINE**.
   - Klik switch toggle:
     - **ON (Amber / `BLOCKED`):** Situs judi online diblokir total di tingkat DNS.
     - **OFF (Abu-abu / `ALLOWED`):** Situs judi online tidak diblokir.
4. **Mengatur 18+ Dewasa:**
   - Lihat kartu ke-3 bertuliskan **18+ DEWASA**.
   - Klik switch toggle sesuai preferensi (ON untuk blokir, OFF untuk buka).
5. Konfigurasi langsung diterapkan ke Unbound secara atomik (*zero-downtime*) dan tersimpan permanen di database PostgreSQL.
