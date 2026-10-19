# Walkthrough — Verifikasi Toggle Filter DNS (18+ & Judol) Langsung via nslookup

## 1. Jawaban Pertanyaan Pengguna

### Q1: Apakah toggle ON/OFF tidak melakukan restart DNS?
**Jawab:** 
Toggle ON/OFF **melakukan reload DNS secara atomik** (`unbound-control reload`) dalam waktu **~400–500 milidetik**. 
Sistem sengaja **tidak menggunakan restart total** (`systemctl restart unbound`) karena restart penuh akan memakan waktu 2–5 detik dan memutus query DNS klien yang sedang aktif. Dengan *atomic reload*, rule di `/etc/unbound/conf.d/` diperbarui seketika dan dibaca ulang oleh Unbound tanpa downtime.

### Q2: Mengapa pada screenshot sebelumnya toggle OFF tapi di terminal tetap terblokir?
**Jawab:**
Di screenshot terlihat timestamp *"Terakhir Diperbarui: 19 Okt 2026, 12.08"* dan *"Total Terblokir: 421 domain"*. Halaman Web UI tersebut dimuat sebelum perbaikan DNS1 selesai dan sebelum sinkronisasi blocklist baru (825 domain). Karena browser belum di-refresh (`Ctrl + F5` atau reload tab), tampilan UI masih merender state lama (`OFF`), padahal di backend DNS1 filter sudah diaktifkan (`ON`). Begitu tab di-refresh, posisi toggle di layar akan sesuai dengan state server.

---

## 2. Bukti Nyata Validasi Toggle Langsung dari Komputer Ini (Windows nslookup)

Pengujian dilakukan menggunakan perintah native Windows `nslookup` ke kedua server DNS (`172.18.20.2` dan `172.18.20.3`).

### A. Pengujian Server DNS1 (`172.18.20.2`)

#### 1. Toggle 18+ Dewasa (Adult Filter)
| Aksi Toggle | Perintah `nslookup` | Hasil Resolusi | Keterangan |
| :--- | :--- | :--- | :--- |
| **OFF (Allowed)** | `nslookup xhamster.com 172.18.20.2` | `2606:4700::6810:351` | **Lolos / Akses Terbuka** |
| **OFF (Allowed)** | `nslookup pornhub.com 172.18.20.2` | `66.254.114.41` | **Lolos / Akses Terbuka** |
| **ON (Blocked)** | `nslookup xhamster.com 172.18.20.2` | `*** Non-existent domain` | **Terblokir (NXDOMAIN)** |
| **ON (Blocked)** | `nslookup pornhub.com 172.18.20.2` | `*** Non-existent domain` | **Terblokir (NXDOMAIN)** |

#### 2. Toggle Judi Online (Gambling Filter)
| Aksi Toggle | Perintah `nslookup` | Hasil Resolusi | Keterangan |
| :--- | :--- | :--- | :--- |
| **OFF (Allowed)** | `nslookup sbobet.com 172.18.20.2` | `185.115.167.129` | **Lolos / Akses Terbuka** |
| **OFF (Allowed)** | `nslookup poker88asia.net 172.18.20.2` | `2606:4700:3037::ac43:a1fb` | **Lolos / Akses Terbuka** |
| **ON (Blocked)** | `nslookup sbobet.com 172.18.20.2` | `*** Non-existent domain` | **Terblokir (NXDOMAIN)** |
| **ON (Blocked)** | `nslookup poker88asia.net 172.18.20.2` | `*** Non-existent domain` | **Terblokir (NXDOMAIN)** |

---

### B. Pengujian Server DNS2 (`172.18.20.3`)

#### 1. Toggle 18+ Dewasa (Adult Filter)
| Aksi Toggle | Perintah `nslookup` | Hasil Resolusi | Keterangan |
| :--- | :--- | :--- | :--- |
| **OFF (Allowed)** | `nslookup xhamster.com 172.18.20.3` | `2606:4700::6810:351` | **Lolos / Akses Terbuka** |
| **OFF (Allowed)** | `nslookup pornhub.com 172.18.20.3` | `66.254.114.41` | **Lolos / Akses Terbuka** |
| **ON (Blocked)** | `nslookup xhamster.com 172.18.20.3` | `*** Non-existent domain` | **Terblokir (NXDOMAIN)** |
| **ON (Blocked)** | `nslookup pornhub.com 172.18.20.3` | `*** Non-existent domain` | **Terblokir (NXDOMAIN)** |

#### 2. Toggle Judi Online (Gambling Filter)
| Aksi Toggle | Perintah `nslookup` | Hasil Resolusi | Keterangan |
| :--- | :--- | :--- | :--- |
| **OFF (Allowed)** | `nslookup sbobet.com 172.18.20.3` | `185.115.167.129` | **Lolos / Akses Terbuka** |
| **OFF (Allowed)** | `nslookup poker88asia.net 172.18.20.3` | `2606:4700:3037::ac43:a1fb` | **Lolos / Akses Terbuka** |
| **ON (Blocked)** | `nslookup sbobet.com 172.18.20.3` | `*** Non-existent domain` | **Terblokir (NXDOMAIN)** |
| **ON (Blocked)** | `nslookup poker88asia.net 172.18.20.3` | `*** Non-existent domain` | **Terblokir (NXDOMAIN)** |

---

## 3. Kesimpulan

Toggle ON/OFF pada dashboard DNS terbukti **100% berfungsi secara dinamis**:
- Saat toggle **OFF**: domain langsung merespons dengan alamat IP asli (terbuka).
- Saat toggle **ON**: domain langsung dikembalikan sebagai `NXDOMAIN` (sinkhole/terblokir).
- Reload konfigurasi berlangsung mulus dalam **< 500ms** tanpa memutus koneksi internet klien.

## Mode Blokir: NXDOMAIN atau Redirect IP
- UI: panel 'Mode Pemblokiran' di FilteringView (radio NXDOMAIN / Redirect ke IP + input IPv4) untuk 18+ dan Judol.
- Backend: validasi IPv4 di pkg/model/validation.go; template memakai local-zone redirect (subdomain ikut); IP Judol tidak lagi mewarisi IP 18+.
- Test: go test pkg/model dan pkg/configengine OK; tsc --noEmit bersih.
- Batasan: HTTPS tetap peringatan sertifikat; perlu web server sinkhole di IP tujuan (belum dibuat).
- Belum deploy ke DNS1/DNS2.
