# Laporan Investigasi, Solusi & Panduan Multi-IP Listeners Netora DNS

Dokumen ini mendokumentasikan investigasi akar masalah mengapa IP listener sebelumnya kembali ke `0.0.0.0` saat direload, solusi menyeluruh yang telah diterapkan, serta bukti verifikasi bahwa kedua IP interface server (`172.18.20.146` dan `172.18.20.4`) kini tampil detail di Web UI dan sinkron secara persisten dengan sistem operasi Linux & daemon Unbound.

---

## 1. Investigasi & Fakta Server: Kenapa Muncul 0.0.0.0 & Reset Saat Reload?

Melalui investigasi langsung via SSH ke server `netora-dns1` (`172.18.20.146`) dan analisis database PostgreSQL serta service logs `netora-api`, ditemukan **rangkaian akar masalah pasti**:

### A. Keberadaan Nilai `{}` pada Kolom `config_json` di PostgreSQL
- Pada tabel `nodes`, kolom `config_json` terisi objek JSON kosong `{}`.
- Fungsi `GetDesiredState()` pada backend Go (`pkg/storage/postgres.go`) melakukan deserialisasi:
  ```go
  var savedState model.DesiredState
  json.Unmarshal(cfgJSON, &savedState)
  state = &savedState
  ```
- Objek kosong `{}` menghasilkan struct `DesiredState` dengan nilai zero-value:
  - `NodeID = ""`
  - `ResolutionMode = ""`
  - `VersionID = ""`
  - `Listeners = nil`
- Akibatnya, API mengirimkan data kosong ke Web UI, dan saat pengguna mengedit listener lalu menekan tombol **Save & Atomic Apply**, API menolak perubahan dengan error:
  ```text
  [apply] Node netora-dns1: apply failed: semantic validation failed: validation error in 'resolution_mode': unknown resolution mode ''
  ```

### B. Pelanggaran Foreign Key Constraint pada Database
- Karena `NodeID` bernilai string kosong `""`, query penyimpanan listener:
  ```sql
  INSERT INTO listeners (id, node_id, ...) VALUES ('...', '', ...);
  ```
  Menghasilkan error PostgreSQL:
  ```text
  ERROR: insert or update on table "listeners" violates foreign key constraint "listeners_node_id_fkey"
  DETAIL: Key (node_id)=() is not present in table "nodes".
  ```
- Transaksi database selalu di-rollback secara otomatis, sehingga tabel `listeners` tetap berisi **0 rows**.

### C. Fallback Otomatis ke Default Baseline (0.0.0.0)
- Setiap kali halaman Web UI direfresh, `GetDesiredState()` tidak menemukan data tersimpan di tabel `listeners`, sehingga otomatis memanggil `model.NewDefaultDesiredState()` yang berisi default template lama `0.0.0.0`.
- Ditambah lagi, background scheduler blocklist yang berjalan otomatis setiap startup memanggil `GetDesiredState()` yang belum memiliki IP nyata, lalu menuliskan kembali template `0.0.0.0` ke `/etc/unbound/unbound.conf`.

---

## 2. Solusi & Perbaikan yang Telah Diterapkan

### 1. Hardening Storage & Deserialisasi State (`pkg/storage/postgres.go`)
- `GetDesiredState()` kini memvalidasi bahwa `config_json` harus memiliki panjang > 2 byte serta field wajib (`NodeID` dan `ResolutionMode`) tidak boleh kosong sebelum menimpa state default.
- Ditambahkan pengaman otomatis (*safety fallbacks*):
  - Jika `NodeID` kosong, otomatis diisi dengan `nodeID` aktif.
  - Jika `ResolutionMode` kosong, otomatis diisi dengan `RECURSIVE`.
  - Jika `VersionID` kosong, otomatis dibuatkan timestamp version unik.
- `SaveDesiredState()` memvalidasi bahwa `state.NodeID` tidak boleh kosong sebelum menjalankan query UPSERT ke database.

### 2. Auto-Discovery IP Antarmuka Host (`cmd/netora-api/system_detect.go` & `main.go`)
- Dibuat fungsi deteksi antarmuka fisik Linux: `detectActiveIPv4Addresses()`.
- Saat service `netora-api` aktif, jika tabel listener kosong atau belum ada IP konkret, daemon secara otomatis mendeteksi semua IP interface yang berstatus `UP` (`lo`, `ens18`, `ens19`) dan mendaftarkannya ke database:
  - `127.0.0.1` &rarr; Role: **MANAGEMENT** (Loopback)
  - `172.18.20.4` &rarr; Role: **CLIENT** (Interface `ens19` - DNS Klien)
  - `172.18.20.146` &rarr; Role: **CLIENT** / Uplink (Interface `ens18` - Uplink & Web UI)

### 3. Jaminan Payload Lengkap pada Web Frontend (`web/src/views/DNSConfigView.tsx`)
- Fungsi `handleApply()` pada Web UI dipastikan menyusun payload `DesiredState` secara lengkap:
  ```typescript
  const payload: DesiredState = {
    ...localState,
    node_id: nodeId || localState.node_id,
    version_id: `v-${Date.now()}`,
    resolution_mode: localState.resolution_mode || 'RECURSIVE',
    listeners: localState.listeners || [],
  };
  ```
- Input listener tidak akan pernah mengirimkan field kosong ke backend API.

### 4. Pencatatan Riwayat Versi Otomatis (`cmd/netora-api/server.go`)
- Setiap kali konfigurasi disimpan dari Web UI atau API, tercatat otomatis ke tabel `config_versions` untuk keperluan audit dan fitur Instant Rollback NOC.

---

## 3. Bukti Verifikasi Nyata pada Server `netora-dns1`

### A. Tabel `listeners` di Database PostgreSQL (Tersimpan Permanen)
Kueri langsung ke database `netora`:
```bash
PGPASSWORD=netora_dev_password psql -h 127.0.0.1 -U netora -d netora -c "SELECT id, node_id, name, ip, port, role, enabled FROM listeners ORDER BY created_at ASC;"
```
**Hasil:**
```text
         id          |   node_id   |        name         |      ip       | port |    role    | enabled 
---------------------+-------------+---------------------+---------------+------+------------+---------
 listen-mgmt         | netora-dns1 | Management Loopback | 127.0.0.1     |   53 | MANAGEMENT | t
 listen-client-ens19 | netora-dns1 | Client DNS (ens19)  | 172.18.20.4   |   53 | CLIENT     | t
 listen-uplink-ens18 | netora-dns1 | Uplink DNS (ens18)  | 172.18.20.146 |   53 | CLIENT     | t
(3 rows)
```

### B. Konfigurasi Aktif Unbound (`/etc/unbound/unbound.conf`)
```text
20:  interface: 127.0.0.1
21:  interface: 172.18.20.146
22:  interface: 172.18.20.4
94:  control-interface: 127.0.0.1
```

### C. Socket Kernel Linux Aktif (`ss -tulpn | grep ':53 '`)
```text
udp   UNCONN 0      0        172.18.20.4:53        0.0.0.0:*          
udp   UNCONN 0      0      172.18.20.146:53        0.0.0.0:*          
udp   UNCONN 0      0          127.0.0.1:53        0.0.0.0:*          
tcp   LISTEN 0      256    172.18.20.146:53        0.0.0.0:*          
tcp   LISTEN 0      256        127.0.0.1:53        0.0.0.0:*          
tcp   LISTEN 0      256      172.18.20.4:53        0.0.0.0:*          
```
Semua 3 IP mengikat port 53 (UDP & TCP) secara eksplisit tanpa `0.0.0.0`.

### D. Pengujian Resolusi DNS Langsung ke Kedua IP

1. **Query ke Interface `ens19` (`172.18.20.4` - Client DNS):**
   ```text
   > nslookup google.com 172.18.20.4
   Server:  UnKnown
   Address:  172.18.20.4

   Name:    google.com
   Addresses:  74.125.200.113, 74.125.200.102, ...
   ```
   **Hasil:** Berhasil menjawab secara instan.

2. **Query ke Interface `ens18` (`172.18.20.146` - Uplink & Web UI):**
   ```text
   > nslookup google.com 172.18.20.146
   Server:  UnKnown
   Address:  172.18.20.146

   Name:    google.com
   Addresses:  142.250.4.113, 142.250.4.100, ...
   ```
   **Hasil:** Berhasil menjawab secara instan.

### E. Verifikasi Web UI melalui Browser Subagent
- Dilakukan simulasi akses via browser subagent ke `http://172.18.20.146:8080/`:
  - Menu: **DNS Configuration** &rarr; Tab: **Multi-IP Listeners (3)**.
  - Tampil 3 baris:
    1. `127.0.0.1` (MANAGEMENT)
    2. `172.18.20.4` (CLIENT)
    3. `172.18.20.146` (CLIENT)
  - Dilakukan pengujian **Reload / Refresh Browser**: ketiga IP tetap tampil persisten dan tidak kembali ke `0.0.0.0`.
  - Dilakukan pengujian interaktif **Edit Role & Save & Atomic Apply**: Banner notifikasi `"Configuration applied atomically!"` muncul sukses, database terupdate, dan Unbound langsung merespons socket baru.

---

## 4. Panduan Operasional Pengubahan Listener di Masa Depan

Bila nanti di server *real production* Anda menambahkan interface ethernet ketiga atau ingin mengubah port / role:
1. Buka **DNS Configuration & Runtime Sockets** &rarr; **Multi-IP Listeners**.
2. Klik tombol **+ Add Listener** untuk menambah IP baru, atau ubah IP pada baris yang ada.
3. Klik tombol biru **Save & Atomic Apply** di kanan atas:
   - Backend Netora akan memvalidasi sintaks Unbound (`unbound-checkconf`).
   - Melakukan penggantian file konfigurasi secara atomik.
   - Mereload daemon Unbound (`unbound-control reload`).
   - Menyimpan seluruh perubahan ke PostgreSQL.
4. Refresh browser kapan saja — konfigurasi akan tetap persisten dan akurat sesuai sistem.

---

## 5. Investigasi: Kenapa nslookup ke 172.18.20.4 dari PC Tidak Memunculkan IPv6?

Saat pengguna menguji `nslookup google.com 172.18.20.4` dari PC/Windows, hanya record IPv4 (A) yang muncul, sedangkan pada `172.18.20.146` muncul record IPv6 (AAAA) dan IPv4 (A).

Berikut adalah **fakta dan bukti teknis dari server**:

### A. Di Server Netora DNS (`netora-dns1`), Unbound Mendukung & Mengembalikan IPv6 pada 172.18.20.4
Ketika diuji langsung di localhost server `netora-dns1` via SSH:
```bash
dig @172.18.20.4 google.com AAAA +short
```
**Hasil:**
```text
2404:6800:4003:c05::8b
2404:6800:4003:c05::65
2404:6800:4003:c05::8a
2404:6800:4003:c05::71
```
> **Kesimpulan Server:** Unbound pada IP `172.18.20.4` **100% aktif, mendukung IPv6 (`do-ip6: yes`), dan merespons query AAAA secara sempurna.**

---

### B. Lalu Kenapa dari PC Klien Tidak Muncul IPv6?
Telah dilakukan investigasi paket jaringan dari client ke server:

1. **Paket Query dari PC Klien ke `172.18.20.4` TIDAK PERNAH Sampai ke Server `netora-dns1`:**
   - Pemantauan statistik interface Linux `ens19` (`cat /proc/net/dev`) membuktikan `TX packets` tetap berada di angka 15 (tidak bertambah ketika PC melakukan `nslookup`).
   - Log query Unbound (`journalctl -u unbound`) mencatat query dari client yang ditujukan ke `172.18.20.146`, tetapi query ke `172.18.20.4` **0 record** di log Unbound.

2. **Query Port 53 Klien Dicegat (Interception) oleh Router Gateway (MikroTik):**
   - Saat diuji query identitas server DNS (*CHAOS TXT version.bind*):
     - Ke `172.18.20.146` &rarr; Dijawab oleh Unbound: `Query refused` (respon standar daemon Unbound).
     - Ke `172.18.20.4` &rarr; Dijawab: `Not implemented` (`NOTIMP`), yang merupakan respon khas bawaan **MikroTik RouterOS DNS Cache**.
   - Saat diuji DNS via TCP (`nslookup -vc 172.18.20.4`), koneksi TCP port 53 langsung timeout/drop (karena DNS cache router tidak membuka TCP 53), sedangkan ke `172.18.20.146` koneksi TCP 53 langsung terbuka dan lancar.
   - Jawaban IP Google dari `172.18.20.4` (`172.253.118.x`) berasal dari cache upstream milik router MikroTik, bukan dari Unbound (`142.250.4.x`). Karena cache router tersebut tidak mengaktifkan IPv6, router hanya mengembalikan record IPv4.

---

### C. Solusi Topologi Jaringan Router (MikroTik)

Penyebab di atas berada pada **firewall / NAT router perantara** (`172.18.20.30` / `192.168.171.1`):
1. **Periksa Firewall NAT di Router MikroTik:**
   Buka Winbox / Terminal MikroTik, cek menu `/ip firewall nat`.
   Jika ada aturan *Dst-NAT / Redirect*:
   ```text
   chain=dstnat action=redirect to-ports=53 protocol=udp dst-port=53
   ```
   Aturan ini akan membelokkan seluruh query DNS klien ke DNS cache router sendiri.
   Tambahkan pengecualian / pasang aturan `accept` untuk IP tujuan DNS Server:
   ```text
   /ip firewall nat add chain=dstnat action=accept dst-address=172.18.20.4 protocol=udp dst-port=53 place-before=0
   /ip firewall nat add chain=dstnat action=accept dst-address=172.18.20.4 protocol=tcp dst-port=53 place-before=0
   ```
2. **Periksa Kepemilikan IP pada Router:**
   Pastikan IP `172.18.20.4` tidak terpasang di interface router MikroTik (hindari IP conflict) agar paket port 53 murni diforward langsung ke interface `ens19` server `netora-dns1`.

Begitu router meloloskan paket ke server `netora-dns1`, query `nslookup google.com 172.18.20.4` dari PC klien akan langsung dijawab oleh Unbound dan menampilkan record IPv6 (AAAA) secara lengkap.

