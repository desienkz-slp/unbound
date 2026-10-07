# QWEN.md — Netora DNS (Pointer)

> **Qwen / Qwen Code:** Seluruh aturan engineering, arsitektur, invariant, dan
> instruksi operasional untuk repo ini berada di [`AGENTS.md`](./AGENTS.md) sebagai
> **Single Source of Truth (SSOT)**. Baca dan patuhi file itu beserta folder `.agents/`
> **sebelum** melakukan aksi apa pun.

## Mandatory bootstrap
1. Baca [`AGENTS.md`](./AGENTS.md).
2. Baca `.agents/00-bootstrap.md`.
3. Baca semua `.agents/rules/*.md`.
4. Baca semua `.agents/skills/*/SKILL.md`.
5. Baca semua `.agents/architecture/*.md`.
6. Baca `.agents/mcp/*`.
7. Inspeksi repository sebelum mengubah apa pun.

## Non-negotiable (ringkas)
- **Investigate First** — jangan menebak bug; kumpulkan bukti nyata dulu.
- **Zero Data Loss** — dilarang `DELETE`/`DROP` destruktif; wajib UPSERT non-destruktif.
- **DNS never depends on UI/API/PostgreSQL/AI** — data plane harus tetap jalan.
- **Never apply unvalidated Unbound config** — validasi `unbound-checkconf`, atomic apply + rollback.
- **Never create an open resolver / expose `unbound-control` publicly.**
- Detail lengkap & Definition of Done: [`AGENTS.md`](./AGENTS.md).
