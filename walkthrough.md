# Panduan Arsitektur & Konfigurasi Dual-Ethernet (Multi-NIC) Netora DNS

Dokumen ini menjelaskan rancangan arsitektur, konfigurasi Web UI, dan routing operasional untuk server Netora DNS dengan 2 kartu jaringan (Dual-Ethernet):
- **Interface 1 (`ens18`):** IP `172.18.20.146/25` &rarr; Uplink / Internet / Web Admin UI (NOC).
- **Interface 2 (`ens19`):** IP `172.18.20.4/27` &rarr; Client DNS Server (Khusus melayani query DNS pelanggan/LAN).

---

## 1. Keunggulan Arsitektur Dual-Ethernet (Separation of Concerns)

Pemisahan interface ini adalah **ISP & Enterprise Best Practice**:
1. **Keamanan (Security Isolation):**
   - Port 53 di interface `ens19` (`172.18.20.4`) khusus melayani query DNS dari klien LAN.
   - Interface `ens18` (`172.18.20.146`) terhubung ke upstream/internet dan hanya membuka port Web UI manajemen (port 8080) serta egress Unbound ke Root Server dunia. Server Anda tidak akan menjadi *Open Resolver* yang rentan disalahgunakan pihak luar.
2. **Kinerja & Bandwidth Isolation:**
   - Trafik query lokal klien di `ens19` tidak bercampur dengan trafik resolver rekursif ke internet di `ens18`.

---

## 2. Pengaturan di Netora Web UI (`http://172.18.20.146:8080`)

Buka menu **DNS Configuration & Runtime Sockets**:

### A. Tab "Multi-IP Listeners"
Atur tabel listener sebagai berikut:

| Role | IP Address | Port | Protocol | Family | Enabled | Deskripsi Fungsi |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **MANAGEMENT** | `127.0.0.1` | `53` | UDP & TCP | IPv4 | Checked | Loopback internal untuk health-check lokal daemon |
| **CLIENT** | `172.18.20.4` | `53` | UDP & TCP | IPv4 | Checked | **Socket utama DNS klien di interface `ens19`** |
| *(Opsional)* | `172.18.20.146` | `53` | UDP & TCP | IPv4 | Unchecked / Checked | Jika di-uncheck, port 53 di ens18 mati (ens18 murni untuk Web UI) |

> **PENTING:**
> - Hapus baris `0.0.0.0` jika Anda ingin mengunci listener hanya pada IP interface tertentu.
> - Pastikan tidak ada *typo* (misalnya `172.16...` yang sebelumnya menyebabkan Unbound gagal bind socket).

### B. Tab "Client ACL Networks"
Agar klien di segmen `ens19` diizinkan melakukan query rekursif:
- Tambahkan subnet klien Anda, misalnya:
  - **CIDR:** `172.18.20.0/27` (Subnet `ens19`) &rarr; Action: `allow`
  - Tambahkan juga subnet LAN pelanggan lainnya jika ada (misal `192.168.0.0/16`, `10.0.0.0/8`).

### C. Tab "Upstream Resolvers"
- Pilih mode resolusi yang Anda kehendaki:
  - **RECURSIVE (Direkomendasikan):** Unbound di server ini akan langsung menanyakan ke Root DNS dunia dan Authoritative DNS melalui interface internet `ens18`.
  - **FORWARDING:** Query diteruskan ke DNS upstream lain (DoT / UDP).
  - **HYBRID:** Forward ke upstream utama, otomatis fallback ke root jika upstream lambat/down.

---

## 3. Verifikasi Routing Sistem Operasi Linux

Agar Unbound dapat me-resolve domain internet dengan lancar:
1. Pastikan **Default Gateway** Linux server berada di `ens18`:
   ```bash
   ip route show | grep default
   # Harusnya: default via <gateway_ens18> dev ens18
   ```
2. Unbound secara otomatis mengirimkan paket query rekursif keluar melalui `ens18` mengikuti tabel rute Linux tersebut.
3. Klien yang mengarahkan DNS ke `172.18.20.4` akan langsung dilayani oleh Unbound di `ens19`, dan Unbound mengambil jawaban dari internet via `ens18`.

---

## 4. Hasil Pengujian Sukses dari Backend API

Konfigurasi dual IP ini telah diuji dan divalidasi langsung pada node `172.18.20.146`:
```json
{
  "version_id": "v-1790440854",
  "semantic_valid": true,
  "syntax_valid": true,
  "reload_success": true,
  "health_check_success": true,
  "rolled_back": false
}
```
Hasil pengujian DNS:
- `dig @172.18.20.4 google.com` &rarr; **Berhasil dijawab oleh Netora Unbound**
- `dig @172.18.20.146 google.com` &rarr; **Berhasil dijawab oleh Netora Unbound**
- Keduanya kini memberikan respon yang konsisten sesuai mode resolusi aktif.

---

## 5. Akses SSH Passwordless (`desienkz@172.18.20.146`)

Kunci SSH Ed25519 baru telah dibuat di lokal:
- **Private Key:** `C:\Users\desie\.ssh\dns-146`
- **Public Key:** `C:\Users\desie\.ssh\dns-146.pub`
- **Host Config:** Ditambahkan ke `C:\Users\desie\.ssh\config` sebagai `172.18.20.146` dan `netora-dns1`.

### Public Key untuk Server:
```
ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMDzIHpx26PKZmi0JZBJnO5Qe3xaR5C9HAMPssBloaRz desienkz@172.18.20.146
```

### Cara Pemasangan Cepat di Server 146:
Jalankan di server `172.18.20.146` (sebagai user `desienkz`):
```bash
mkdir -p ~/.ssh && chmod 700 ~/.ssh
echo "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMDzIHpx26PKZmi0JZBJnO5Qe3xaR5C9HAMPssBloaRz desienkz@172.18.20.146" >> ~/.ssh/authorized_keys
chmod 600 ~/.ssh/authorized_keys
```

Setelah itu, koneksi SSH passwordless dapat langsung diuji:
```powershell
ssh desienkz@172.18.20.146
# atau
ssh netora-dns1
```
