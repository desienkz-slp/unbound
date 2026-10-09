# Walkthrough — Verifikasi Server 146, Fungsi Tombol Logrotate, dan Auto-Update

## Ringkasan Eksekusi
Telah dilakukan investigasi mendalam, pengujian nyata via SSH, perbaikan UI feedback, build, dan push ke GitHub serta verifikasi langsung pada server produksi **172.18.20.146** (`netora-dns1`).

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
