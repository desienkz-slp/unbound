# .agents/AGENTS.md â€” Project SSOT Â- unbound

> **GLOBAL SSOT:** `C:\Users\desie\.gemini\config\AGENTS.md` (aturan universal â€” WAJIB dipatuhi lebih dulu).
> File ini HANYA berisi hal spesifik proyek **unbound**. Jangan menyalin aturan global ke sini.

---

## 0. Identitas Proyek

| Item | Nilai |
| :--- | :--- |
| **Nama** | unbound |
| **Repository** | _(isi URL git)_ |
| **Default Branch** | _(main / master)_ |
| **Stack** | _(mis. Laravel / Vite+React / Flutter / Node)_ |
| **Database** | _(PostgreSQL / MySQL / SQLite)_ |

## 1. Target Server & Deploy

| Node | Host | SSH Key | App Path | Restart |
| :--- | :--- | :--- | :--- | :--- |
| Dev | _(ip)_ | _(key)_ | _(path)_ | _(cmd)_ |
| Prod | _(ip)_ | _(key)_ | _(path)_ | _(cmd)_ |

_Alur deploy spesifik proyek (git pull / scp / build) ditulis di sini._

## 2. Rules & Skills Spesifik Proyek

- Rules: [`.agents/rules/`](./rules) â€” HANYA aturan khusus proyek ini.
- Skills: [`.agents/skills/`](./skills) â€” workflow khusus proyek ini.

## 3. Catatan Penting

_(quirk, gotcha, kontrak API yang tak boleh berubah, dsb.)_
