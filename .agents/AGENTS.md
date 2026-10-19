# .agents/AGENTS.md — Project SSOT - unbound

> **GLOBAL SSOT:** `C:\Users\desie\.gemini\config\AGENTS.md` (aturan universal — WAJIB dipatuhi lebih dulu).
> File ini HANYA berisi hal spesifik proyek **unbound**. Jangan menyalin aturan global ke sini.

---

## 0. Identitas Proyek

| Item | Nilai |
| :--- | :--- |
| **Nama** | unbound |
| **Repository** | https://github.com/desienkz-slp/unbound.git |
| **Default Branch** | main |
| **Stack** | Go (Backend Control Plane) / Vite + React + TS (Frontend) / Unbound DNS |
| **Database** | PostgreSQL |

## 1. Target Server & Deploy

| Node | Host | SSH Key | App Path | Restart |
| :--- | :--- | :--- | :--- | :--- |
| Node 1 | 172.18.20.146 | ~/.ssh/dns-146 (user: desienkz) | /usr/local/bin/netora-api | systemctl restart netora-api |
| Node 2 | 172.18.20.147 | ~/.ssh/unbound-1 (user: desienkz) | /usr/local/bin/netora-api | systemctl restart netora-api |

## 2. Rules & Skills Spesifik Proyek

- Rules: [`.agents/rules/`](./rules)
  - [`01-architecture.md`](./rules/01-architecture.md)
  - [`02-security.md`](./rules/02-security.md)
  - [`03-performance.md`](./rules/03-performance.md)
  - [`04-config-apply.md`](./rules/04-config-apply.md)
  - [`05-testing.md`](./rules/05-testing.md)
  - [`06-ui.md`](./rules/06-ui.md)
  - [`07-logging.md`](./rules/07-logging.md)
  - [`08-cdn-compatibility.md`](./rules/08-cdn-compatibility.md)
  - [`09-preserve-local-config.md`](./rules/09-preserve-local-config.md) (Anti-Overwrite Policy)
- Skills: [`.agents/skills/`](./skills)

## 3. Aturan Mutlak & Larangan Keras (Non-Negotiable)

### ⛔ LARANGAN MERUBAH SETTINGAN LOKAL YANG SUDAH BERJALAN SAAT UPDATE
1. **Dilarang Overwrite Settingan Eksisting:**
   Ketika melakukan update aplikasi, build baru, sync deploy, atau perbaikan kode, **DILARANG KERAS** merubah atau menimpa settingan yang sudah berjalan di server lokal produksi (`netora-dns1` / `netora-dns2`).
2. **Entity yang Dilindungi:**
   - Password / kredensial user admin.
   - Daftar upstream resolver yang sudah aktif.
   - Daftar listener IP yang sudah aktif.
   - Subnet klien / access-control lists (`client_networks`).
   - Filter, whitelist, blacklist, policy profile yang sudah disetel operator.
   - Konfigurasi Unbound aktif di PostgreSQL (`nodes.config_json`) dan `/etc/unbound/unbound.conf`.
3. **Pola Pembaruan yang Diizinkan:**
   - Pembaruan hanya boleh meng-update binary (`netora-api`, `netora-cli`), static web assets (`deploy/ui/`), bug fixes logika, atau menambahkan field konfigurasi baru dengan nilai default yang aman (non-breaking, non-destructive).
   - Jangan pernah me-reset settingan ke default atau menjalankan `INSERT` overwrite destruktif pada konfigurasi lokal yang sedang aktif.
   - Jika ada parameter baru, gunakan pola non-destructive: hanya isi parameter baru jika belum ada (*fill missing*), jangan sentuh parameter lama yang sudah berjalan.
