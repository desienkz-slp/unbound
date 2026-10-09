# Walkthrough — Verifikasi Server 146, Fungsi Tombol Logrotate, dan Auto-Update

## Ringkasan Eksekusi
Telah dilakukan investigasi mendalam, pengujian nyata via SSH, perbaikan UI feedback, build, dan push ke GitHub serta verifikasi langsung pada server produksi **172.18.20.146** (`netora-dns1`). Selain itu, dilakukan penyempurnaan menyeluruh pada script `install.sh`, `uninstall.sh`, binary `netora-api`, dan dokumentasi `README.md` agar instalasi pada fresh Ubuntu (termasuk Ubuntu 20.04 LTS) tetap berjalan mulus dalam **1 sistem instalasi tunggal (One-Line Installer)** tanpa kegagalan pada start Unbound.

---

## 1. Bukti Pengujian Nyata di Server 172.18.20.146

Semua endpoint backend dan aksi sistem pada server 146 telah diuji langsung menggunakan token otentikasi JWT dan terbukti **100% Berfungsi Normal**:

### A. Uji 3 Tombol Logrotate pada Server 146
1. **Tombol `Uji Validasi (Dry Run)`** (`POST /api/v1/nodes/netora-dns1/logrotate/test`):
   * **HTTP Status**: `200 OK`
   * **Hasil**: `Success: True`, menghasilkan 42 baris output simulasi `logrotate -d`.
   * **Perilaku**: Memverifikasi sintaks direktif konfigurasi tanpa memodifikasi file log asli.

2. **Tombol `Rotasi Sekarang (Paksa & Kosongkan)`** (`POST /api/v1/nodes/netora-dns1/logrotate/force-rotate`):
   * **HTTP Status**: `200 OK`
   * **Hasil**: `Success: True`, `Message: "Rotasi manual logrotate selesai dijalankan"`.
   * **Perilaku**: Mengeksekusi `logrotate -f /etc/logrotate.d/unbound`, memutar file log aktif, mengompresi ke `.gz`, dan membebaskan ruang disk.

3. **Tombol `Terapkan Konfigurasi Logrotate`** (`POST /api/v1/nodes/netora-dns1/logrotate/apply`):
   * **HTTP Status**: `200 OK`
   * **Hasil**: `Success: True`, `Message: "Konfigurasi logrotate berhasil diterapkan dan diverifikasi oleh sistem."`.
   * **Perilaku**: Menulis ulang `/etc/logrotate.d/unbound` lengkap dengan izin `su unbound unbound` dan memanggil `unbound-control log_reopen`.

### B. Uji Integrasi Tuning & Profil AI pada Server 146
* **`GET /api/v1/nodes/netora-dns1/config`**: `200 OK`, mengambil desired state aktif node.
* **`POST /api/v1/nodes/netora-dns1/apply`**: `200 OK`, `Success: True`, konfigurasi Unbound diperbarui dan dimuat ulang secara atomik.
* **Resolusi Unbound DNS**: Aktif dan merespons kueri pada port 53 (`127.0.0.1` dan `172.18.20.146`) dalam hitungan milidetik.

---

## 2. Akar Masalah Persepsi Pengguna & Solusi Perbaikan

### Akar Masalah:
1. **Posisi Pesan Notifikasi Tersembunyi**: Sebelumnya, banner `logrotateMsg` diletakkan di bagian paling atas tab Logrotate. Ketika pengguna berada di bagian bawah (posisi 3 tombol aksi), notifikasi tidak terlihat di layar (*out of viewport*).
2. **Output Kosong pada Force Rotate**: Perintah sistem `logrotate -f` tidak mengeluarkan teks apapun ke stdout saat berhasil. Kondisi `{consoleOutput && ...}` pada UI sebelumnya tidak membuka jendela terminal jika output berupa string kosong.
3. **Ketiadaan Indikator Inline**: Tidak ada indikator visual langsung di dekat tombol saat tombol ditekan.

### Solusi yang Diimplementasikan:
1. **Inline Feedback Banner Langsung di Atas Tombol**:
   * Menambahkan komponen alert visual tepat di atas baris 3 tombol aksi.
   * Dilengkapi warna status (hijau emerald untuk sukses, merah untuk eror), ikon status, dan tombol tutup.
2. **Jaminan Konten Terminal**:
   * Memberikan fallback teks penjelasan yang jelas pada `consoleOutput` saat stdout kosong, sehingga jendela konsol terminal selalu terbuka secara otomatis saat tombol ditekan.
3. **Kompilasi & Auto-Deploy**:
   * Frontend dikompilasi ulang ke versi **v1.0.128** (`npm run build`).
   * Perubahan di-commit (`7bf63a0`) dan di-push ke GitHub repository `origin/main`.
   * Layanan otomatis `netora-dns-updater.service` di server 146 secara otonom menarik versi terbaru dan memperbarui direktori `/var/www/netora-ui`.

---

## 3. Status Terkini Server 146

| Komponen | Status | Detail / Bukti |
| :--- | :--- | :--- |
| **netora-api** | **Active (Running)** | PID 81313, melayani port `:8080` |
| **Web UI** | **v1.0.128** | Aset terpasang: `index-BxaW6hmI.js` & `index-ZObDTXIv.css` |
| **Unbound DNS** | **Active (Running)** | Resolusi 100% normal pada `172.18.20.146:53` |
| **Logrotate Host** | **Valid & Active** | Konfigurasi `/etc/logrotate.d/unbound` aktif |
| **Auto-Updater** | **Active (Timer)** | `netora-dns-updater.timer` aktif memantau branch `main` |

---

## 4. Penyempurnaan Universal One-Line Installer (Zero-Friction Install)

Menjawab kendala saat instalasi pada mesin fresh Ubuntu (Unbound 1.9.4 / Ubuntu 20.04 LTS), dilakukan penguatan otomatis pada `install.sh`, `uninstall.sh`, dan dokumentasi `README.md` agar tetap menjadi **1 sistem instalasi tunggal tanpa intervensi manual**:

1. **Pelepasan Port 53 Otomatis & Kompatibilitas Versi systemd**:
   * Pada Ubuntu 20.04 (systemd 245), folder drop-in `/etc/systemd/resolved.conf.d/` belum dibaca oleh systemd-resolved.
   * `install.sh` kini memodifikasi berkas `/etc/systemd/resolved.conf` secara langsung sekaligus menyematkan drop-in untuk Ubuntu 22.04/24.04 (systemd 246+).
   * Menambahkan penghentian paksa jika `systemd-resolved` masih menahan socket port 53, sambil tetap menjaga DNS publik fallback (`1.1.1.1`, `8.8.8.8`) di `/etc/resolv.conf` agar proses unduh paket tidak terputus.

2. **Deteksi Otomatis Kemampuan IPv6**:
   * Memeriksa keberadaan antarmuka IPv6 aktif di `/proc/net/if_inet6`.
   * Mencegah eror fatal `cannot bind socket: Address family not supported by protocol` pada host/VM yang menonaktifkan IPv6.

3. **Remote-Control Tanpa Hambatan Izin Sertifikat**:
   * Mengatur `control-use-cert: no` pada loopback `127.0.0.1:8953` (konsisten dengan konfigurasi inti engine).
   * Memberikan izin penuh user `unbound` pada `/var/lib/unbound`, `/var/log/unbound`, dan berkas konfigurasi.
   * Menambahkan *local override* AppArmor secara otomatis agar daemon tidak diblokir saat menulis log.

4. **Diagnostik Otomatis & Dokumentasi**:
   * Jika Unbound gagal start karena anomali sistem luar, `install.sh` otomatis mencetak 25 baris log diagnostik `journalctl` ke layar sebelum keluar.
   * `README.md` diperbarui untuk menegaskan bahwa platform tetap menggunakan **1 sistem instalasi satu-baris** yang menangani semua dependensi secara otonom.

---

## 5. Hasil Audit Seluruh File `.sh` pada README.md

Seluruh referensi skrip `.sh` pada `README.md` telah diaudit dan dipastikan valid, teruji sintaks (`bash -n`), dan konsisten:

| Skrip di README.md | Posisi Baris | Status | Peran & Konsistensi |
| :--- | :--- | :--- | :--- |
| **`install.sh`** | Baris 31, 33, 40, 60, 70, 79, 89, 112 | **VALID & SINKRON** | Skrip installer tunggal universal untuk seluruh distro (Interactive, Silent, Private PAT). |
| **`uninstall.sh`** | Baris 513, 515 | **VALID & SINKRON** | Skrip pembersih sistem lengkap yang mengembalikan `systemd-resolved` dan menghapus service. |
| **`netora`** | Baris 234–270 | **VALID & SINKRON** | CLI manajemen terpasang di `/usr/local/bin/netora` (membungkus `scripts/netora-cli.sh`). |
| **`netora-reset-password`** | Baris 308–316 | **VALID & SINKRON** | Helper darurat terpasang di `/usr/local/bin/netora-reset-password` (`scripts/netora-reset-password.sh`). |
| **`netora update`** | Baris 476 | **VALID & SINKRON** | Menjalankan `/usr/local/sbin/netora-dns-update` (`scripts/netora-dns-update.sh`) untuk pull otomatis. |

---

## 6. Verifikasi & Sinkronisasi Password Admin (`ADMIN_PASS`)

* **Logika Prompt `install.sh`**:
  * Default: `ADMIN_PASS="admin123"` (jika pengguna langsung menekan Enter saat prompt).
  * Kustom: Jika pengguna memasukkan teks, `ADMIN_PASS` otomatis menggunakan nilai baru tersebut.
* **Perbaikan Backend `netora-api`**:
  * Sebelumnya fungsi seed akun di `cmd/netora-api/main.go` masih menggunakan string literal `"admin123"`.
  * Kini diperbarui membaca variabel lingkungan `NETORA_ADMIN_PASS` (dengan fallback `"admin123"` jika kosong).
* **Sinkronisasi Otomatis Database**:
  * Di `install.sh`, ditambahkan sinkronisasi langsung menggunakan binary `/usr/local/bin/netora-cli -mode reset-password` agar password yang aktif di PostgreSQL selalu 100% cocok dengan apa yang diinput pengguna saat instalasi.
