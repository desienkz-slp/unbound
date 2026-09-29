<div align="center">
  <img src="assets/logo.png" alt="Netora DNS Logo" width="150" />
  <h1>Netora DNS — Enterprise Carrier-Grade ISP Platform</h1>
  <p><strong>Autonomous High-Speed Unbound DNS Resolver with Real-Time NOC Telemetry, Suffix-Trie Filtering, and Zero-Downtime CI/CD Fleet Auto-Updates.</strong></p>

  <p>
    <img src="https://img.shields.io/badge/Release-v1.0.104-cyan?style=flat-square" alt="Release" />
    <img src="https://img.shields.io/badge/Ubuntu-20.04%20|%2022.04%20|%2024.04-orange?style=flat-square&logo=ubuntu" alt="Ubuntu" />
    <img src="https://img.shields.io/badge/Debian-11%20|%2012-red?style=flat-square&logo=debian" alt="Debian" />
    <img src="https://img.shields.io/badge/Unbound-1.13+-blue?style=flat-square&logo=dns" alt="Unbound" />
    <img src="https://img.shields.io/badge/Throughput-30K+%20QPS-brightgreen?style=flat-square" alt="QPS" />
    <img src="https://img.shields.io/badge/Rollback-Atomic%20%3C150ms-emerald?style=flat-square" alt="Atomic Rollback" />
    <img src="https://img.shields.io/badge/License-Proprietary-purple?style=flat-square" alt="License" />
  </p>
</div>

---

## ⚡ Quick Start — Auto-Install Script

Deploy a production-ready Netora DNS node on any fresh Ubuntu or Debian server in under 2 minutes:

### 1. One-Line Standard Installation (Interactive)

```bash
curl -sSL https://raw.githubusercontent.com/desienkz-slp/unbound/main/install.sh | sudo bash
```
*(Or using `wget`: `wget -qO- https://raw.githubusercontent.com/desienkz-slp/unbound/main/install.sh | sudo bash`)*

### 2. Non-Interactive / Automated Fleet Installation (Silent Mode)

For automated provisioning (Cloud-Init, Ansible, or PXE boot), pass `-y` and configuration flags:

```bash
curl -sSL https://raw.githubusercontent.com/desienkz-slp/unbound/main/install.sh | sudo bash -s -- \
  --silent \
  --node-id "dns-jakarta-01" \
  --mode recursive \
  --web-port 8080 \
  --admin-pass "SuperSecretNocPass123"
```

### 3. Private GitHub Repository Installation

If your repository is private, provide your GitHub Personal Access Token (PAT).

> [!IMPORTANT]
> Use a **Fine-Grained PAT** (not classic token) scoped to **only this repository** with **Contents: Read-only** and **Metadata: Read-only** permissions. This is the minimum required scope for installation and auto-updates.

#### Option A: One-Line Silent Fleet Installer (Recommended for Private Repo)

```bash
TOKEN="github_pat_yourFineGrainedTokenHere"

curl -H "Authorization: token $TOKEN" -sSL https://raw.githubusercontent.com/desienkz-slp/unbound/main/install.sh | sudo bash -s -- \
  --silent \
  --node-id "dns-jakarta-01" \
  --mode recursive \
  --web-port 8080 \
  --admin-pass "SuperSecretNocPass123" \
  --github-token "$TOKEN"
```

> [!NOTE]
> * **Header `curl`**: Wajib menggunakan format `-H "Authorization: token $TOKEN"` agar GitHub mengizinkan pengunduhan script `install.sh` dari repository private.
> * **Parameter `--github-token`**: Wajib dioper ke script agar proses `git clone` di server baru berjalan 100% otomatis tanpa meminta Username & Password interaktif.

#### Option B: Export Environment Variable
```bash
# Export your fine-grained PAT first
export GITHUB_TOKEN="github_pat_yourFineGrainedTokenHere"

# Run the installer (-E retains GITHUB_TOKEN for sudo)
curl -H "Authorization: token $GITHUB_TOKEN" -sSL https://raw.githubusercontent.com/desienkz-slp/unbound/main/install.sh | sudo -E bash -s -- \
  --silent \
  --node-id "dns-jakarta-01" \
  --mode recursive \
  --web-port 8080 \
  --admin-pass "SuperSecretNocPass123"
```

#### Option C: Local Script Execution
```bash
sudo bash install.sh --github-token "github_pat_yourFineGrainedTokenHere" --silent
```

The token is stored securely in `/etc/netora-dns/github.token` (mode `600`, root-only) and used by the autonomous auto-update engine for all subsequent pulls.

> [!TIP]
> Jika token GitHub Anda kadaluarsa (*expired*) atau ingin dirotasi di kemudian hari, lihat [Panduan Update / Rotasi GitHub Token](#-panduan-update--rotasi-github-token-di-server).


---

## 🖥️ NOC Web Dashboard & Default Credentials

Once the installation finishes, open your browser and navigate to:

* **URL:** `http://<YOUR_SERVER_IP>:8080/`
* **Default Username:** `admin`
* **Default Password:** `admin123` *(or the custom password provided during installation)*
* **Prometheus Telemetry:** `http://<YOUR_SERVER_IP>:8080/metrics`
* **API Health Check:** `http://<YOUR_SERVER_IP>:8080/api/v1/health`

### Key Web UI Capabilities:
* **Dashboard Overview:** Real-time query counts, cache hit ratio, top clients, top queried domains, top blocked domains, and live upstream latency.
* **DNS Configuration:** Listeners, upstream DNS over TLS (DoT) reordering/presets, cache tuning, and **DNS Rewrites** (with wildcard redirect support).
* **Policy & Filtering:** Suffix-trie domain blocking, blocklist auto-updates, adult/gambling scheduled filters, and custom allowlists.
* **Fleet Management & Updates:** Clickable version badge in the footer to inspect Git SHA, build numbers, and trigger immediate fleet updates.
* **Audit & Diagnostics:** Real-time domain resolution inspector, audit logs, and Hermes AI-NOC deep diagnostics.

---

## 📋 System Requirements & Port Matrix

| Requirement | Minimum | Recommended for High-Traffic ISP |
|:---|:---|:---|
| **Operating System** | Ubuntu 20.04 / 22.04 / 24.04 LTS, Debian 11 / 12 | Ubuntu 22.04 LTS (Kernel 5.15+) |
| **CPU Architecture** | 1 vCPU (x86_64 / amd64 or arm64) | 4+ vCPUs |
| **Memory (RAM)** | 1 GB RAM | 4 GB - 16 GB RAM (Auto-tuned cache) |
| **Disk Storage** | 5 GB SSD / NVMe | 20 GB+ NVMe SSD |
| **Network Interfaces** | 1 Gbps Ethernet | 2.5 Gbps - 10 Gbps Ethernet |

### Network Port Allocation
* **`53/UDP` & `53/TCP`:** Unbound DNS Data Plane (Client & Upstream Queries)
* **`8080/TCP`:** Netora Control Plane REST API & React NOC Web UI
* **`5432/TCP`:** PostgreSQL Database (`127.0.0.1` loopback only)
* **`8953/TCP`:** Unbound Remote Control (`127.0.0.1` loopback with TLS certificates)

---

## ⚙️ Installer Command-Line Flags

| Option | Default | Description |
|:---|:---|:---|
| `-y`, `--silent` | `false` | Run without interactive prompts, accepting all default values. |
| `--mode <mode>` | `recursive` | DNS resolution engine: `recursive` (IANA Root Hints) or `forwarder` (Cloudflare/Google). |
| `--node-id <id>` | *hostname* | Unique cluster node identifier displayed in NOC telemetry. |
| `--web-port <port>` | `8080` | TCP port for NOC Web UI Dashboard and REST API. |
| `--dns-port <port>` | `53` | UDP/TCP port for DNS query answering. |
| `--admin-pass <pass>`| `admin123` | Master password for operator authentication in the Web UI. |
| `--db-pass <pass>` | `netora_dev_password` | Dedicated PostgreSQL password for the `netora` role. |
| `--subnets "<subnets>"`| `10.0.0.0/8 172.16.0.0/12 192.168.0.0/16 127.0.0.0/8` | Space-separated list of allowed client CIDR ranges. |
| `--channel <chan>` | `development` | Release channel for updates: `development` or `production`. |
| `--github-token <tok>`| *empty* | GitHub Token for pulling private repositories securely. |
| `--no-auto-update` | `false` | Disable the autonomous systemd auto-update timer. |
| `-h`, `--help` | — | Display help information and exit. |

---

## 🚀 Architectural Invariants & Key Capabilities

```
                              NETORA DNS PLATFORM
                              
               ┌────────────────────────────────────────────────┐
               │                 CONTROL PLANE                  │
               │  • Web UI (React/TS/Tailwind - Dark NOC Theme) │
               │  • REST API (cmd/netora-api :8080)             │
               │  • PostgreSQL (Database on 127.0.0.1:5432)     │
               │  • Cluster Manager (Multi-node HA)             │
               │  • Telemetry & Prometheus Exporter (/metrics)  │
               │  • Model Context Protocol (cmd/netora-mcp)     │
               │  • Hermes AI-NOC Investigator                  │
               └───────────────────────┬────────────────────────┘
                                       │ 
                 Atomic Configuration  │ Staging, Validation,
                 & Health Verification │ & Reload (<150ms)
                                       ▼
               ┌────────────────────────────────────────────────┐
               │                  DATA PLANE                    │
               │             NLnet Labs UNBOUND                 │
               │  • High-Speed DNS Cache (Auto-tuned to RAM)    │
               │  • Cryptographic DNSSEC Validation             │
               │  • Suffix-Trie Policy Filter (358ns / query)   │
               │  • Carrier CDN Optimization (Anti-Bengong)     │
               │  • Zero Open Resolver Semantic Guardrails       │
               │  • Recursive / Forwarding / Hybrid Modes       │
               └────────────────────────────────────────────────┘
```

1. **Total Data Plane Independence (Anti-Bengong):**  
   Unbound resolves DNS queries directly from flat, atomically validated configurations. If PostgreSQL, the REST API, or the Web UI crashes, **Unbound continues answering 100% of customer DNS queries without missing a single packet**.
2. **Verified High-Throughput Performance:**  
   Benchmarked at **30,445+ QPS** on a modest 2 vCPU server with **1.22 ms** median latency (P50).
3. **Carrier CDN Compatibility (Critical Invariant #16):**  
   Strict `qname-minimisation: no` is enforced to prevent non-RFC-compliant CDN authoritative nameservers (such as Baishan Cloud / Shopee CDN / TikTok / Akamai) from returning incorrect `NXDOMAIN` on intermediate empty non-terminals.
4. **Versions & Atomic 1-Click Rollback:**  
   Every configuration change generates a versioned snapshot with SHA256 verification. If an operator misconfigures a rule, 1-click rollback restores the previous state in **< 150 ms** with zero dropped packets.
5. **Zero Open Resolver Guarantee:**  
   The configuration engine rejects arbitrary internet client access (`0.0.0.0/0 allow`) at the semantic validation level.
6. **DNS Rewrites & Wildcard Redirects:**  
   Configure custom DNS responses for A, AAAA, CNAME, PTR, and TXT records with wildcard subdomain support (`*.domain.com`) generated via Unbound's native `local-zone redirect` directives with zero downtime.
7. **Resilient Dual-Source NOC Telemetry:**  
   Telemetry combines Unbound control sockets with background query log and `journalctl` streaming. Provides zero-loss live metrics for Top Clients, Top Queried Domains, Top Blocked Domains, and Upstream Latencies even across log rotations and process restarts.
8. **1-Click Fleet Updates & Priority Presets:**  
   Administrators can trigger node updates fleet-wide directly from the Web UI footer modal with guaranteed zero IP listener/topology loss, plus apply 1-click optimized CDN presets (Cloudflare + Google DoT).

---

## 🛠️ NOC Operator CLI Management (`netora`)

The installer places the convenient `netora` CLI tool into `/usr/local/bin/netora`:

```bash
# 1. Check overall system health, Unbound status, and updater state
netora status

# 2. Run instant DNS resolution and API health checks
netora health

# 3. Pull and atomically apply the latest revision from GitHub
netora update

# 4. Rollback to the previous stable configuration snapshot
netora rollback

# 5. Gracefully restart Unbound and Netora API
netora restart

# 6. Stream live service logs
netora logs unbound
netora logs api
netora logs updater

# 7. Print access endpoints, IP topology, and operator credentials
netora info

# 8. Reset operator password interactively or via parameters
netora reset-password
netora reset-password admin "MyNewPassword123"

# 9. List registered NOC operator accounts and database status
netora users

# 10. Update GitHub PAT token for auto-updates & verify repository access
netora set-token "github_pat_yourNewTokenHere"
```

---

## 🔑 Panduan Reset Password Operator via CLI Langsung di Server

Untuk keperluan pemulihan darurat (*disaster recovery*), situasi saat administrator lupa password Web UI, atau akun NOC terkunci, Netora DNS menyediakan utilitas CLI native dan script otomatis yang dapat dijalankan langsung di server via SSH dengan **Zero-Downtime** (tanpa perlu restart Unbound maupun Netora API):

### 1. Metode Cepat: Menggunakan Perintah `netora reset-password` (Direkomendasikan)
Perintah ini terintegrasi langsung dalam utility `/usr/local/bin/netora`:

#### A. Mode Interaktif (Paling Aman — Password Tidak Tersimpan di History Shell)
Cukup ketik perintah berikut tanpa argumen password:
```bash
netora reset-password
```
Sistem akan menampilkan prompt rahasia:
```text
========================================================
  Netora DNS - CLI Password Reset Utility
========================================================
Enter username to reset [admin]: admin
Enter new password for 'admin' (min 6 characters): [Ketik Password]
Confirm new password: [Ketik Ulang Password]
```

#### B. Mode Non-Interaktif / 1-Baris (Cocok untuk Script & Otomatisasi)
```bash
netora reset-password <username> "<password_baru>"
```
*Contoh:*
```bash
netora reset-password admin "SuperSecretNocPass123"
```

---

### 2. Metode Skrip Dedicated: `netora-reset-password`
Tersedia juga script eksekusi langsung di `/usr/local/bin/netora-reset-password`:
```bash
# Mode interaktif
netora-reset-password

# Mode langsung dengan parameter
netora-reset-password admin "SuperSecretNocPass123"
```

---

### 3. Metode Native Binary: `netora-cli`
Binary engine `/usr/local/bin/netora-cli` berkomunikasi langsung dengan database PostgreSQL dan menyediakan kontrol akun yang lengkap:

#### A. Reset Password Akun Tertentu:
```bash
netora-cli -mode reset-password -username admin -new-password "SuperSecretNocPass123"
```
*Output:*
```text
========================================================
  Netora DNS - Password Reset Successful
========================================================
  User ID   : usr-superadmin
  Username  : admin
  Role      : SUPER_ADMIN
  Status    : Password successfully updated in PostgreSQL
  Timestamp : 2026-09-26T06:26:17Z
========================================================
You can now sign in to the Netora NOC Web UI with your new password.
```

#### B. Menampilkan Daftar Operator Terdaftar:
```bash
netora-cli -mode list-users
```
*Output:*
```text
=========================================================================================
  Netora DNS - Registered Operators
=========================================================================================
  USER ID          USERNAME         ROLE           LAST UPDATED          
-----------------------------------------------------------------------------------------
  usr-superadmin   admin            SUPER_ADMIN    2026-09-26 13:26:17   
=========================================================================================
```

#### C. Membuat Akun Operator / Admin Baru Secara Darurat:
```bash
netora-cli -mode create-user -username noc_operator -new-password "PassOperator123" -role OPERATOR
```

---

### 4. Metode Emergency Rescue Fallback (Direct PostgreSQL)
Jika dalam skenario bencana (*disaster*) binary belum tersedia, Anda dapat langsung meng-update hash password melalui Python / PostgreSQL CLI bawaan server:
```bash
python3 -c "
import bcrypt, subprocess
pw = b'SuperSecretNocPass123'
hashed = bcrypt.hashpw(pw, bcrypt.gensalt(10)).decode('utf-8')
subprocess.run(['sudo', '-u', 'postgres', 'psql', '-d', 'netora', '-c', f\"UPDATE users SET password_hash = '{hashed}', updated_at = NOW() WHERE username = 'admin';\"])
print('Password successfully updated!')
"
```

> [!NOTE]
> **Keamanan & Konsistensi Database:**  
> Seluruh password di-hash secara aman menggunakan algoritma **bcrypt** (cost 10) standar industri. Setiap perubahan kredensial via CLI otomatis dicatat ke tabel Audit Log (`audit_logs`) untuk keperluan kepatuhan (*compliance*) keamanan jaringan ISP.

---

## 🔐 Panduan Update / Rotasi GitHub Token di Server

Token autentikasi GitHub disimpan secara aman di file konfigurasi `/etc/netora-dns/github.token` dengan permission ketat `600` (hanya dapat dibaca/ditulis oleh user `root`). Token ini digunakan oleh engine auto-updater (`netora-dns-update`), systemd background timer (`netora-dns-updater.timer`), dan fitur pembaruan 1-klik di Web UI.

### ❓ Kapan Token Perlu Diperbarui?
1. **Token Kadaluarsa (*Expired*):** GitHub **Fine-Grained Personal Access Token (PAT)** memiliki masa berlaku terbatas (misal 30, 60, atau 90 hari) demi standar keamanan.
2. **Rotasi Token Keamanan:** Dilakukan secara berkala sesuai kebijakan dan SOP keamanan jaringan ISP.
3. **Gejala Token Expired:**
   - Perintah `netora update` menghasilkan error:
     ```text
     [ERROR] update_failed: Cannot reach GitHub or invalid credentials. Preserving current DNS configuration.
     ```
   - Log auto-update di `/var/log/netora-dns/update.log` mencatat `fatal: Authentication failed`.
   - Tombol / status update di Web UI gagal mendeteksi revisi commit terbaru dari repository GitHub.

---

### 📋 Spesifikasi Token Baru yang Dibutuhkan
Buat token baru di GitHub (**Settings** ➔ **Developer Settings** ➔ **Personal access tokens** ➔ **Fine-grained tokens**):
* **Repository access:** *Only select repositories* ➔ Pilih repository Netora DNS (`desienkz-slp/unbound` atau repository privat Anda).
* **Permissions (Repository permissions):**
  * **Contents:** `Read-only`
  * **Metadata:** `Read-only`

---

### 🚀 Cara Update Token di Server

Pilih salah satu metode berikut (dijalankan langsung di server via SSH):

#### 1. Metode Cepat 1-Baris via SSH (Paling Praktis & Universal)
Jalankan perintah berikut di terminal server (ganti `github_pat_yourNewTokenHere` dengan token baru Anda):

```bash
echo "github_pat_yourNewTokenHere" | sudo tee /etc/netora-dns/github.token > /dev/null && sudo chmod 600 /etc/netora-dns/github.token
```

*Atau jika Anda sudah login langsung sebagai user `root`:*
```bash
echo "github_pat_yourNewTokenHere" > /etc/netora-dns/github.token && chmod 600 /etc/netora-dns/github.token
```

---

#### 2. Metode Menggunakan CLI Utility `netora set-token` (Interaktif & Auto-Verifikasi)
Utility `/usr/local/bin/netora` menyediakan perintah bawaan untuk memperbarui token dan langsung memverifikasi akses repository:

**A. Mode Interaktif (Aman — Token tidak tersimpan di riwayat shell history):**
```bash
sudo netora set-token
```
Sistem akan meminta input token secara tersembunyi (*hidden input*):
```text
Enter new GitHub Personal Access Token (PAT): [Tempel Token Baru]
GitHub Token successfully updated in /etc/netora-dns/github.token (mode 600).
Verifying repository access with new token...
Verification PASS: Token is valid and has read access!
```

**B. Mode Parameter 1-Baris:**
```bash
sudo netora set-token "github_pat_yourNewTokenHere"
```

---

#### 3. Metode Edit Manual Menggunakan Text Editor (`nano`)
```bash
sudo nano /etc/netora-dns/github.token
```
1. Hapus token lama, lalu tempel (*paste*) token baru (pastikan hanya berisi 1 baris string token tanpa spasi atau baris kosong tambahan).
2. Simpan perubahan dengan menekan `Ctrl + O`, tekan `Enter`, lalu keluar dengan `Ctrl + X`.
3. Pastikan izin file tetap aman (root-only):
```bash
sudo chmod 600 /etc/netora-dns/github.token
```

---

### 🔍 Cara Verifikasi Bahwa Token Baru Sudah Berfungsi

> [!NOTE]
> **Zero-Downtime:** Anda **TIDAK PERLU** merestart service Unbound maupun Netora API setelah memperbarui token. Token dibaca secara dinamis dari file `/etc/netora-dns/github.token` setiap kali proses auto-update atau pengecekan remote berjalan.

Uji keberhasilan token baru dengan salah satu langkah berikut:

#### A. Verifikasi Koneksi Git Remote Langsung:
```bash
TOKEN=$(sudo cat /etc/netora-dns/github.token)
git ls-remote "https://${TOKEN}@github.com/desienkz-slp/unbound.git" refs/heads/main
```
*Jika token valid, output akan menampilkan hash commit terbaru (misalnya: `6c354b2... refs/heads/main`).*

#### B. Uji Jalankan Auto-Update Manual:
```bash
sudo netora update
```

#### C. Uji Endpoint Status API:
```bash
curl -s http://127.0.0.1:8080/api/v1/system/status?check_remote=true | jq .
```
Output jika berhasil:
```json
{
  "channel": "development",
  "git_commit": "6c354b2",
  "remote_commit": "6c354b2",
  "update_available": false
}
```

---

## 🔄 Autonomous CI/CD Auto-Update Engine

Netora DNS uses a decentralized pull-based deployment model:
* **Background Timer:** `/etc/systemd/system/netora-dns-updater.timer` runs every 5 minutes with randomized jitter to prevent server thundering herds.
* **Web UI 1-Click Fleet Update:** Operators can trigger updates on demand via the "Perbarui Platform Sekarang" button in the footer popover modal.
* **Sandboxed Staging:** Candidates from GitHub are assembled in `/opt/netora-dns/staging/` and pre-validated using `unbound-checkconf`.
* **Atomic Swap:** Symlink `/opt/netora-dns/current` is atomically updated and Unbound is reloaded via `unbound-control reload`.
* **Zero Configuration Loss:** Each node's unique IP topology (listeners `interface:`), local database data, and specific network bindings remain immutable across updates.
* **Auto-Rollback on Failure:** If reload fails or post-activation health checks fail, the previous verified configuration is restored within seconds.
* **Secure Token Storage:** GitHub token is stored at `/etc/netora-dns/github.token` (mode `600`). A **Fine-Grained PAT** with **Selected Repository + Contents: Read-only** scope is sufficient and recommended over classic all-repo tokens.

---

## 🗑️ Uninstallation Runbook

To completely remove Netora DNS and restore the default system resolver (`systemd-resolved`):

```bash
curl -sSL https://raw.githubusercontent.com/desienkz-slp/unbound/main/uninstall.sh | sudo bash
```
*(Or run `sudo ./uninstall.sh` from the repository root)*

---

## 📚 Technical Documentation & References

* [Final Engineering Walkthrough & Audit Report](walkthrough.md)
* [Autonomous CI/CD Engine SOP](docs/auto-update.md)
* [Carrier Rollback & Recovery Guide](docs/rollback.md)
* [Multi-Node Cluster Deployment](docs/deployment.md)
* [DNS Validation & CDN Compatibility Skill](.agents/skills/dns-validation/SKILL.md)

---

<div align="center">
  <small>© Netora DNS Platform • Engineered for High-Reliability Internet Service Providers.</small>
</div>
