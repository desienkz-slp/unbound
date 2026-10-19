# Hasil Investigasi Kesehatan Server DNS (Netora DNS / Unbound)

Tanggal: 19 Oktober 2026  
Status Investigasi: **Selesai (Bukti Data Lengkap)**

---

## 1. Ringkasan Eksekutif (Akar Masalah "Tidak Bisa Reply")

Berdasarkan investigasi real-time langsung ke server (`172.18.20.146`, `172.18.20.147`, dan `172.18.20.145`), ditemukan **4 akar masalah utama** yang menyebabkan DNS tiba-tiba tidak merespons (*timeout / no reply* / *SERVFAIL*):

| # | Temuan Masalah | Lokasi / Komponen | Dampak Langsung |
|---|---|---|---|
| **1** | **`requestlist.exceeded` (Total 76.919 query didrop tanpa reply)** | Node 1 (36.903 drop) & Node 2 (40.016 drop) | Saat query antre melebihi kapasitas request list (karena upstream DoT latency/stall), Unbound langsung **menjatuhkan (drop) query** tanpa membalas ke client. |
| **2** | **Koneksi Upstream DoT (Port 853) Terputus (`channel closed` 3.877x)** | `/etc/unbound/unbound.conf` (`forward-tls-upstream: yes`) | Koneksi TLS ke `8.8.8.8:853` & `1.1.1.1:853` sering terputus/reset, antrean menumpuk, menyebabkan 138.132 query berstatus `SERVFAIL` dan timeout. |
| **3** | **Crash-Loop & Downtime Total 45 Menit Hari Ini (04:21 - 05:06 UTC)** | Node 1 (`172.18.20.146`) | Unbound crash fatal akibat permission `/usr/share/dns/root.key` (`Permission denied`) dan `SIGSEGV / free(): invalid pointer`, memicu `systemd` rate-limit restart failure. |
| **4** | **IP Listener Tidak Lengkap di Node 2 (`172.18.20.147`)** | Node 2 (`/etc/unbound/unbound.conf`) | Node 2 hanya mendengarkan IP `172.18.20.3` dan `127.0.0.1`. IP utamanya `172.18.20.147` **tidak mendengarkan port 53** sama sekali (query ke `.147` selalu timeout). |

---

## 2. Bukti & Data Nyata dari Server

### A. Pengujian Resolusi dari Sisi Client
Konfigurasi DNS adapter aktif (`Ethernet 2` & `Wi-Fi`):
- Primary DNS: `172.18.20.2` (Node 1)
- Secondary DNS: `172.18.20.3` (Node 2)

Hasil tes nslookup:
```text
172.18.20.2   : Berhasil (23ms - 86ms)
172.18.20.3   : Berhasil (22ms - 102ms)
172.18.20.146 : Berhasil (Virtual listener di Node 1)
172.18.20.147 : TIMEOUT (Port 53 tidak didengarkan di Node 2!)
172.18.20.145 : TIMEOUT (Service Unbound tidak ada / belum terpasang di server ladapala)
```

### B. Bukti Drop Query (`requestlist.exceeded`)
Diambil via `unbound-control stats_noreset`:
- **Node 1 (172.18.20.146):**
  - `total.requestlist.exceeded = 36903`
  - `num.answer.rcode.SERVFAIL = 62851`
- **Node 2 (172.18.20.147):**
  - `total.requestlist.exceeded = 40016`
  - `num.answer.rcode.SERVFAIL = 75281`

> **Analisis:** Total **76.919 query dijatuhkan secara diam-diam (silent drop / no reply)** ke subscriber/client ketika antrean requestlist meluap.

### C. Bukti Log Error Upstream TLS (DoT Port 853)
Pada `/var/log/unbound/unbound.log`:
```text
unbound[429275:2] error: str: channel closed
unbound[429275:0] error: str: channel closed
unbound[429275:2] error: str: channel closed
```
Total kejadian `channel closed`: **3.877 kali**.  
DoT mengenkripsi setiap query upstream via TLS. Saat koneksi TCP/TLS ke Google (`8.8.8.8:853`) atau Cloudflare (`1.1.1.1:853`) direset ISP / upstream latency tinggi, query yang menunggu menjadi macet, memenuhi memori request list, dan di-drop.

### D. Bukti Crash Loop Hari Ini (04:21:23 - 05:06:07 UTC)
Pada `journalctl -u unbound`:
```text
Oct 19 04:21:23 netora-dns1 systemd[1]: unbound.service: Main process exited, code=exited, status=1/FAILURE
Oct 19 04:21:56 netora-dns1 systemd[1]: unbound.service: Main process exited, code=dumped, status=11/SEGV
Oct 19 04:40:02 netora-dns1 unbound[426796]: free(): invalid pointer
Oct 19 04:40:02 netora-dns1 systemd[1]: unbound.service: Main process exited, code=dumped, status=6/ABRT
Oct 19 04:41:10 netora-dns1 systemd[1]: unbound.service: Start request repeated too quickly.
Oct 19 04:41:10 netora-dns1 systemd[1]: Failed to start unbound.service - Unbound DNS server.
```
Penyebab utama dari `/var/log/unbound/unbound.log`:
```text
fatal error: could not open autotrust file for writing, /usr/share/dns/root.key.425062-0-58bf5579aee0: Permission denied
```
Unbound mencoba memperbarui trust-anchor RFC 5011 ke direktori root yang read-only bagi user `unbound`.

### E. File Log Membengkak (5,5 GB) & Beban I/O Disk
- Ukuran log Node 1: `unbound.log` mencapai **5.530.230.350 bytes (5,53 GB)**.
- Partisi root disk (`/`) terpakai 66%–69%.
- Konfigurasi saat ini mengaktifkan:
  ```yaml
  log-queries: yes
  log-replies: yes
  ```
  Di server produksi dengan ribuan query per detik, penulisan log secara sinkron ke disk menyebabkan I/O wait dan bottleneck CPU.

### F. Penggunaan Memori Node 2
- RSS Memori Unbound Node 2 mencapai **6.5 GB** (peak **9.5 GB**) dengan uptime 4 hari.

---

## 3. Rencana Tindakan Perbaikan (Implementation Plan)

1. **Tambahkan Listener IP `172.18.20.147` di Node 2:**
   - Tambahkan `interface: 172.18.20.147` agar query yang langsung mengarah ke IP node 2 tetap dijawab.
2. **Optimasi Buffer Request List & Anti-Drop:**
   - Naikkan `num-queries-per-thread` dari 2048 menjadi 4096.
   - Naikkan `outgoing-range` dari 8192 menjadi 16384.
   - Atur `jostle-timeout: 200` agar query lama yang lambat tidak menghambat query baru.
3. **Optimasi Upstream Resolver:**
   - Evaluasi penggunaan DoT (`forward-tls-upstream: yes`) jika koneksi internasional/latensi ISP sering memicu `channel closed`. Alternatif: gunakan UDP standar port 53 dengan fallback lokal/anycast berlatensi rendah, atau naikkan pool koneksi TLS.
4. **Matikan `log-queries` & `log-replies` di Produksi serta Rotasi Log:**
   - Ubah `log-queries: no` dan `log-replies: no` agar disk I/O tidak jenuh.
   - Rotasi dan bersihkan log 5,5 GB saat ini menggunakan logrotate untuk melegakan storage.
5. **Jadwalkan Memory Recycling / Restart Terjadwal pada Node 2:**
   - Unbound Node 2 perlu di-reload/restart ringan di jam sepi (off-peak) untuk melepas cache memori yang menumpuk hingga 9,5 GB.

---

## 4. Implementasi Perbaikan Sistem Tuning (Telah Di-commit & Push ke GitHub)

Berikut file dan modul yang telah diperbarui dan diverifikasi:

1. **`pkg/model/config.go` & `pkg/model/config_test.go`:**
   - Menambahkan field `JostleTimeout int` pada `PerformanceConfig`.
   - `Normalize()` secara otomatis menetapkan `defaultJostleTimeout = 200` jika belum diatur.
   - Meningkatkan default fallback `NumQueriesPerThread` dari 1024 ke 4096 dan `OutgoingRange` dari 4096 ke 16384.
   - Menambahkan unit test `TestNormalizeAntiDropBuffers` (PASS).
2. **`pkg/configengine/templates.go`:**
   - Menambahkan direktif `jostle-timeout: {{ if gt .Performance.JostleTimeout 0 }}{{ .Performance.JostleTimeout }}{{ else }}200{{ end }}` pada Bagian 7 (Networking & High Throughput Buffers).
3. **`conf/unbound-base.conf`:**
   - Menambahkan `jostle-timeout: 200` pada blok Kapasitas Buffer & Queue (Jutaan RPS Tuning).
4. **`web/src/views/PerformanceView.tsx` & `web/src/types.ts`:**
   - Memperbarui profile buffer:
     - Profile MEDIUM: `queriesPerThread: 4096`, `outgoingRange: 16384`.
     - Profile LARGE & ISP: `queriesPerThread: 8192`, `outgoingRange: 32768`.
   - Menurunkan `minDiskGB: 5` agar node 4 core 16GB RAM langsung memenuhi syarat profile ISP / Carrier Grade.
   - Menambahkan `jostle_timeout: 200` pada `ensureDefaults`.
5. **Verifikasi Build & Test:**
   - `go test ./...` lulus 100% pada seluruh package (netora-api, configengine, model, storage, security, telemetry).
   - `npm run build` berhasil mengompilasi TypeScript dan sinkronisasi bundle aset ke `deploy/ui/`.

