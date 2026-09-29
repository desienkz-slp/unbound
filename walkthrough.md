# Walkthrough — Disk-Aware Performance Tuning

## Fitur Baru: Disk sebagai Faktor Rekomendasi

### Perubahan yang Dilakukan

#### Backend (Go)

| File | Perubahan |
|------|-----------|
| `cmd/netora-api/disk_detect_linux.go` | NEW — `detectDiskSpecs()` via `syscall.Statfs` (Linux build tag) |
| `cmd/netora-api/disk_detect_other.go` | NEW — stub return `0,0` untuk Windows/dev |
| `cmd/netora-api/main.go` | Wire `detectDiskSpecs()` → `NodeMetadata` + log startup |
| `pkg/model/config.go` | `NodeMetadata` + `DiskTotalGB`, `DiskFreeGB` |
| `pkg/storage/store.go` | `NodeRecord` + `DiskTotalGB`, `DiskFreeGB` |
| `pkg/storage/postgres.go` | `UpsertNode`, `GetNode`, `ListNodes` dengan kolom disk + `COALESCE` backward compat |
| `pkg/storage/memory.go` | `UpsertNode` propagate disk fields |
| `pkg/storage/migrations/001_initial_schema.sql` | Kolom `disk_total_gb`, `disk_free_gb` di `CREATE TABLE nodes` |
| `pkg/storage/migrations/002_add_disk_columns_to_nodes.sql` | Migration idempotent untuk existing DB |

#### Frontend (React/TypeScript)

| File | Perubahan |
|------|-----------|
| `web/src/types.ts` | `NodeRecord` + `disk_total_gb`, `disk_free_gb` |
| `web/src/views/PerformanceView.tsx` | Full rewrite — disk-aware UI |

---

### Cara Kerja Baru

#### 1. Deteksi Disk (Backend)
- Saat startup, `detectDiskSpecs()` membaca root filesystem (`/`) menggunakan `syscall.Statfs`
- Mengembalikan `disk_total_gb` dan `disk_free_gb`
- Disimpan ke PostgreSQL via `UpsertNode`, diperbarui setiap restart API

#### 2. Rekomendasi Profile (Frontend)
Sebelumnya: hanya `cpu_cores`
Sekarang: `cpu_cores + ram_mb + disk_total_gb` (semuanya harus memenuhi minimum)

```
Priority: malware > ads > adult > gambling > custom
Profile ISP     = CPU>=9, RAM>=28GB, Disk>=40GB
Profile LARGE   = CPU>=5, RAM>=12GB, Disk>=20GB
Profile MEDIUM  = CPU>=3, RAM>=4GB,  Disk>=10GB
Profile SMALL   = CPU>=1, RAM>=0GB,  Disk>=4GB
```

#### 3. Komponen UI Baru

**Hardware Topology Cards (3 panel):**
- CPU — jumlah core
- RAM — total physical memory
- Disk — total/free GB dengan progress bar berwarna:
  - 🟢 Hijau: <75% used (Healthy)
  - 🟡 Amber: 75-89% used (Low Space)
  - 🔴 Merah: ≥90% used (CRITICAL)

**Alert Banners:**
- Critical (≥90%): tampilkan perintah remediation langsung di UI
- Warn (≥75%): reminder untuk prune

**Disk Setup Recommendations Panel:**
| Tier | Root Disk | RAM | vCPU | QPS Target |
|------|-----------|-----|------|------------|
| Minimum | 8 GB | 1 GB | 1-2 | <5k |
| Recommended | 25+ GB | 4-8 GB | 4-8 | 30k-80k |
| Carrier Grade | 50+ GB | 32+ GB | 16+ | 100k+ |

**Profile Cards:** sekarang menampilkan "Min Disk" per profile, berwarna merah jika disk aktual < minimum.

---

### Database Migration

Untuk server yang sudah ada, jalankan:
```sql
-- File: pkg/storage/migrations/002_add_disk_columns_to_nodes.sql
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
        WHERE table_name = 'nodes' AND column_name = 'disk_total_gb')
    THEN ALTER TABLE nodes ADD COLUMN disk_total_gb INT NOT NULL DEFAULT 0; END IF;

    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
        WHERE table_name = 'nodes' AND column_name = 'disk_free_gb')
    THEN ALTER TABLE nodes ADD COLUMN disk_free_gb INT NOT NULL DEFAULT 0; END IF;
END $$;
```

Atau via psql:
```bash
PGPASSWORD=netora_dev_password psql -h 127.0.0.1 -U netora -d netora \
  -f /opt/netora-dns/current/pkg/storage/migrations/002_add_disk_columns_to_nodes.sql
```

---

### Deployment

| Server | Revision | Status |
|--------|----------|--------|
| 145 (dns-dev) | `519b683` | In progress → auto-update |
| 146 (netora-dns1) | `519b683` | In progress → via API |

Kolom `disk_total_gb` dan `disk_free_gb` langsung terisi otomatis saat `netora-api` restart setelah update.
