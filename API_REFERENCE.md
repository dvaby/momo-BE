# API Reference — Momo-BE

Dokumen ini disusun berdasarkan **testing langsung terhadap kode yang berjalan** (bukan asumsi).
Versi awal: 1 September 2026 · **Update terakhir: 19 September 2026 (update 10 — FINAL pre-demo)**.
Semua contoh request/response di bawah adalah hasil `curl` nyata terhadap environment production.

---

## Daftar Isi

- [Info Umum](#info-umum)
- [A. Health, Auth & Verifikasi](#a-health-auth--verifikasi)
- [B. Modul, Materi & Soal](#b-modul-materi--soal)
- [C. Kelas & Nilai (Token Guru)](#c-kelas--nilai-token-guru)
- [D. Alur Siswa (Token Siswa)](#d-alur-siswa-token-siswa)
- [E. Alur Belajar Siswa](#e-alur-belajar-siswa)
- [E2. Mode Tutor / Chat (18–19 Sept 2026)](#e2-mode-tutor--chat-1819-sept-2026)
- [F. SSE Legacy: Stream Kelas (Token Guru)](#f-sse-legacy-stream-kelas-token-guru)
- [G. Unified Stream (Guru + Siswa + Public)](#g-unified-stream-guru--siswa--public)
- [H. Lampiran: Endpoint Debug](#h-lampiran-endpoint-debug)
- [I. Panduan Integrasi FE — Halaman Study](#i-panduan-integrasi-fe--halaman-study)
- [Error Handling](#error-handling)
- [Known Issues / Catatan untuk FE](#known-issues--catatan-untuk-fe)
- [Changelog](#changelog)

---

## Info Umum

| | |
|---|---|
| **Base URL (dev)** | `http://localhost:8080` |
| **Base URL (production)** | `https://momo-be-production.up.railway.app` |
| **Prefix endpoint** | `/api/v1` (kecuali `/health`) |
| **Format data** | `application/json` untuk sebagian besar endpoint; `multipart/form-data` untuk upload PDF |
| **Autentikasi** | JWT via header `Authorization: Bearer <token>` — **kecuali `/chat` yang bisa anonim** |

### 🔒 Dua Jenis Token JWT & Role Isolation

| | Token Guru | Token Siswa |
|---|---|---|
| Didapat dari | `POST /api/v1/guru/login` | `POST /api/v1/join` |
| Isi claim | `guru_id`, `role: "guru"` | `siswa_id`, `kelas_id` |
| Masa berlaku | 24 jam | 12 jam |
| Lokasi di response | `data.token` | `token` (root object) |
| Berlaku di | Semua endpoint Guru | Endpoint Siswa (`GET /modul/:id/soal`, `POST /submit-jawaban`, `GET /siswa/*`) |

**Role isolation aktif (14 Sept 2026):**
- Token Guru dipakai di endpoint siswa → `401` dengan `code: TOKEN_WRONG_ROLE`
- Token Siswa dipakai di endpoint guru → `401` dengan `code: TOKEN_WRONG_ROLE`

**Pengecualian (19 Sept 2026):** `POST /chat` dan `GET /stream` bersifat **auth-opsional** (bisa anonim).

### CORS

Origin yang diizinkan:
- `http://localhost:3000`, `http://localhost:5173` (dev; dapat ditambah via env `CORS_ALLOWED_ORIGINS`)
- Semua subdomain `*.vercel.app` (production & preview)

Origin lain → `403 Forbidden`. Request tanpa header `Origin` (server-to-server, misal dari AI Service) tidak terpengaruh CORS.

### Rate Limiting

| Limiter | Batas | Diterapkan di |
|---|---|---|
| Auth limiter | 5 request / 12 detik per IP | `POST /guru/register`, `POST /guru/login`, `POST /join` |
| AI limiter | 3 request / 6 detik per IP | `POST /test-extract-pdf`, `POST /submit-jawaban`, `POST /chat`, `POST /tutor` |

Melebihi batas → `429 Too Many Requests`:
```json
{ "error": "Terlalu banyak permintaan. Silakan coba lagi dalam beberapa saat." }
```

### Format Error Umum

**Format standar (18 Sept 2026)** untuk error bisnis:

```json
{
  "code": "NOT_FOUND",
  "message": "soal dengan ID 999999 tidak ditemukan",
  "source": "client"
}
```

| Field | Keterangan |
|---|---|
| `code` | Kode error stabil untuk logika FE |
| `message` | Pesan bahasa Indonesia, aman ditampilkan langsung ke user |
| `source` | `client` (FE salah) \| `ai_service` (AI salah) \| `server` (backend salah) |

⚠️ Middleware stream masih memakai format lama `{"error":"...","code":"TOKEN_MISSING"}` — FE wajib menangani **kedua bentuk** (cek keberadaan field `source`).

---

## A. Health, Auth & Verifikasi

### `GET /health` — Cek server hidup

- **Auth:** Publik

**Response (200):**
```json
{ "status": "ok", "message": "server is running!" }
```

---

### `POST /api/v1/guru/register` — Registrasi guru

- **Auth:** Publik
- **Content-Type:** `application/json`
- **Rate limit:** Auth limiter

**Body:**
| Field | Tipe | Wajib | Keterangan |
|---|---|---|---|
| `nama` | string | ✅ | Minimal 2 karakter |
| `email` | string | ✅ | Format email valid |
| `password` | string | ✅ | Minimal 6 karakter |

```json
{ "nama": "Bu Sari", "email": "sari@sekolah.com", "password": "rahasia123" }
```

**curl:**
```bash
curl -X POST https://momo-be-production.up.railway.app/api/v1/guru/register \
  -H "Content-Type: application/json" \
  -d '{"nama":"Bu Sari","email":"sari@sekolah.com","password":"rahasia123"}'
```

**Response (201):**
```json
{
  "message": "pendaftaran berhasil, silakan cek email kamu untuk verifikasi akun",
  "data": {
    "id": 3, "nama": "Bu Sari", "email": "sari@sekolah.com",
    "created_at": "...", "updated_at": "..."
  }
}
```
📧 Registrasi memicu email verifikasi berisi link ke `GET /api/v1/guru/verify-email`.

**Error (400):**
```json
{ "error": "email sudah terdaftar" }
{ "error": "Nama wajib diisi (minimal 2 karakter)" }
{ "error": "Format email tidak valid" }
{ "error": "Password minimal 6 karakter" }
```

---

### `GET /api/v1/guru/verify-email` — Verifikasi email guru

- **Auth:** Publik (dipanggil dari link di email)

**Query parameter:**
| Param | Wajib | Keterangan |
|---|---|---|
| `token` | ✅ | Token verifikasi sekali pakai dari email |

**Perilaku:** **redirect (302)** ke URL frontend:
- Sukses → `{FE_VERIFY_REDIRECT_URL}?status=success`
- Gagal → `{FE_VERIFY_REDIRECT_URL}?status=error&message=<pesan>`

---

### `POST /api/v1/guru/login` — Login guru

- **Auth:** Publik
- **Content-Type:** `application/json`
- **Rate limit:** Auth limiter

**Body:**
| Field | Tipe | Wajib |
|---|---|---|
| `email` | string | ✅ |
| `password` | string | ✅ |

**curl:**
```bash
curl -X POST https://momo-be-production.up.railway.app/api/v1/guru/login \
  -H "Content-Type: application/json" \
  -d '{"email":"sari@sekolah.com","password":"rahasia123"}'
```

**Response (200):**
```json
{
  "message": "login berhasil",
  "data": {
    "token": "eyJhbGci...",
    "guru": { "id": 3, "nama": "Bu Sari", "email": "sari@sekolah.com", "created_at": "...", "updated_at": "..." }
  }
}
```
⚠️ Token berada di **`data.token`**, bukan di root object.

**Error (401):**
```json
{ "error": "email atau password salah" }
```

---

### 🔄 `POST /api/v1/join` — Siswa masuk kelas (AUTO-REGISTER)

- **Auth:** Publik
- **Content-Type:** `application/json`
- **Rate limit:** Auth limiter

🔄 **BREAKING CHANGE (14 Sept 2026):** Endpoint ini sekarang **auto-register siswa baru**.

**Body:**
| Field | Tipe | Wajib | Keterangan |
|---|---|---|---|
| `kode_kelas` | string | ✅ | 6 digit angka, diberikan guru |
| `nama` | string | ✅ | Nama siswa |

**curl:**
```bash
curl -X POST https://momo-be-production.up.railway.app/api/v1/join \
  -H "Content-Type: application/json" \
  -d '{"kode_kelas":"487137","nama":"Budi"}'
```

**Response (200):**
```json
{ "siswa_id": 1, "kelas_id": 3, "nama": "Budi", "token": "eyJhbGci..." }
```
⚠️ Token berada **langsung di root object (`.token`)** — berbeda dari login guru.

**Perilaku:**
- Nama sudah terdaftar di kelas → return siswa existing + token
- Nama belum terdaftar → **auto-create** siswa baru + return token
- Kode kelas tidak valid → error `401`

⚠️ **Catatan 19 Sept:** token ini TIDAK dibutuhkan untuk `/chat` (chat bebas). Token hanya diperlukan untuk mode soal/materi kelas (`/submit-jawaban`, `/siswa/*`).

**Error (401):**
```json
{ "error": "kelas dengan kode '000000' tidak ditemukan" }
```

---

## B. Modul, Materi & Soal

> Semua endpoint di bagian ini bertoken **Guru**, kecuali `GET /modul/:id/soal` yang bertoken **Siswa** (ditandai jelas).
> Data terisolasi per guru: guru lain mengakses modul Anda → `404`/`403`, seolah tidak ada.

### Struktur data

```
GURU (pemilik)
 └── MODUL   ← wadah/unit belajar (nama, deskripsi)
      ├── MATERI ← isi belajar (judul, konten, urutan)
      ├── SOAL   ← bank soal (pertanyaan, pilihan a-d, jenis)
      └── tertaut ke KELAS (many2many) ← pintu akses siswa
```

Object **Materi**:
```json
{ "id": 11, "modul_id": 2, "urutan": 1, "judul": "...", "konten": "...", "created_at": "..." }
```
⚠️ Object materi **tidak memiliki field `updated_at`**.
Object **Soal**:
```json
{ "id": 5, "modul_id": 3, "jenis": "uts", "pertanyaan": "...", "pilihan_a": "...", "pilihan_b": "...", "pilihan_c": "...", "pilihan_d": "...", "created_at": "..." }
```
⚠️ `kunci_jawaban` **tidak pernah** muncul di response manapun.

---

### `POST /api/v1/modul` — Buat modul baru

- **Auth:** Token Guru
- **Content-Type:** `application/json`

**Body:**
| Field | Tipe | Wajib |
|---|---|---|
| `nama` | string | ✅ |
| `deskripsi` | string | ❌ |

**Response (201):** object modul mentah
```json
{ "id": 2, "guru_id": 3, "nama": "Modul IPA", "deskripsi": "...", "created_at": "...", "updated_at": "...", "materi": null, "soal": null }
```

---

### `GET /api/v1/modul` — List modul milik guru

- **Auth:** Token Guru

**Response (200):** array object modul (hanya milik guru yang login)

---

### `GET /api/v1/modul/:id` — Detail modul (termasuk materi & soal)

- **Auth:** Token Guru

**Response (200):**
```json
{
  "id": 3,
  "judul": "Ipa (Updated)",
  "deskripsi": "Ilmu Pengetahuan Alam untuk Kelas 5",
  "materi": [ { "id": 73, "modul_id": 3, "urutan": 1, "judul": "...", "konten": "...", "created_at": "..." } ],
  "soal": [ { "id": 5, "modul_id": 3, "jenis": "harian", "pertanyaan": "...", "pilihan_a": "...", "pilihan_b": "...", "pilihan_c": "...", "pilihan_d": "...", "created_at": "..." } ]
}
```
⚠️ Field nama modul di sini bernama **`judul`** (bukan `nama`).
✅ **Endpoint inilah yang dipakai halaman GURU** untuk preview soal & materi.

**Error (404):**
```json
{ "error": "Modul tidak ditemukan" }
```

---

### `PUT /api/v1/modul/:id` — Update modul

- **Auth:** Token Guru
- **Content-Type:** `application/json`

**Body:**
| Field | Tipe | Wajib | Keterangan |
|---|---|---|---|
| `nama` | string | ❌* | *Minimal `nama` atau `deskripsi` harus diisi |
| `deskripsi` | string | ❌* | ⚠️ Nilai yang dikirim **selalu menimpa** deskripsi lama |

**Response (200):** `data` berisi object modul lengkap **termasuk preload materi & soal** (payload besar)

**Error (400):**
```json
{ "error": "Minimal salah satu field (nama atau deskripsi) harus diisi" }
{ "error": "modul tidak ditemukan atau bukan milik Anda" }
```

---

### `DELETE /api/v1/modul/:id` — Hapus modul (cascade)

- **Auth:** Token Guru

⚠️ Menghapus **seluruh materi & soal** di dalam modul + melepas tautan ke kelas.

**Response (200):**
```json
{ "message": "Modul berhasil dihapus beserta semua materi dan soal di dalamnya" }
```

---

### `GET /api/v1/modul/:id/materi` — List materi dalam modul

- **Auth:** Token Guru

**Response (200):** terurut `urutan` ASC
```json
{
  "jumlah": 2,
  "data": [
    { "id": 11, "modul_id": 3, "urutan": 1, "judul": "...", "konten": "...", "created_at": "..." }
  ]
}
```

---

### `POST /api/v1/modul/:id/materi/manual` — Buat materi manual

- **Auth:** Token Guru
- **Content-Type:** `application/json`

**Body:**
| Field | Tipe | Wajib | Keterangan |
|---|---|---|---|
| `judul` | string | ✅ | |
| `konten` | string | ✅ | |
| `urutan` | number | ❌ | Jika kosong/≤0 → otomatis posisi terakhir |

**Response (201):**
```json
{
  "message": "Materi berhasil ditambahkan",
  "data": { "id": 149, "modul_id": 3, "urutan": 5, "judul": "Pengenalan IPA", "konten": "...", "created_at": "..." }
}
```

---

### `PUT /api/v1/materi/:id` — Update materi

- **Auth:** Token Guru
- ⚠️ Path parameter adalah **ID materi**, bukan ID modul.

**Response (200):**
```json
{
  "message": "Materi berhasil diperbarui",
  "data": { "id": 149, "modul_id": 3, "urutan": 5, "judul": "Pengenalan IPA (Revisi)", "konten": "...", "created_at": "..." }
}
```

---

### `DELETE /api/v1/materi/:id` — Hapus materi

- **Auth:** Token Guru
- ⚠️ Path parameter adalah **ID materi**.

**Response (200):**
```json
{ "message": "Materi berhasil dihapus" }
```

---

### `POST /api/v1/modul/:id/materi` — Upload PDF materi (SYNCHRONOUS)

- **Auth:** Token Guru
- **Content-Type:** `multipart/form-data`

⚠️ Request **ditahan** sampai AI selesai merangkum (10–60 detik). FE wajib spinner + disable tombol.

**Body (form-data):**
| Field | Tipe | Batas | Keterangan |
|---|---|---|---|
| `file` | file | `.pdf`, maks **25MB** | PDF berisi materi pembelajaran |

**curl:**
```bash
curl -X POST https://momo-be-production.up.railway.app/api/v1/modul/3/materi \
  -H "Authorization: Bearer $TOKEN_GURU" \
  -F "file=@/path/materi.pdf"
```

**Response (200):**
```json
{
  "message": "Materi berhasil diproses",
  "jumlah": 2,
  "data": [ { "id": 11, "modul_id": 3, "urutan": 1, "judul": "Pengenalan IPA", "konten": "...", "created_at": "..." } ]
}
```
⚠️ FE harus merender array **`data`**, bukan `message`.

**Error (400):**
```json
{ "error": "File PDF wajib dilampirkan dengan field 'file'" }
{ "error": "Ukuran file terlalu besar. Maksimal 25MB.", "code": "FILE_TOO_LARGE" }
{ "error": "Hanya file PDF yang diperbolehkan", "code": "INVALID_FILE_TYPE" }
```

**Error (422):**
```json
{ "error": "Gagal memproses materi dari PDF. Layanan AI terlalu lama memproses atau tidak tersedia. Coba lagi atau gunakan PDF yang lebih kecil.", "code": "MATERI_PROCESSING_FAILED" }
```

---

### `POST /api/v1/modul/:id/soal?jenis=uts` — Upload PDF soal (SYNCHRONOUS)

- **Auth:** Token Guru
- **Content-Type:** `multipart/form-data`

🔄 **BREAKING CHANGE (14 Sept 2026):** SYNCHRONOUS — request ditahan sampai AI selesai (10–30 detik). **HAPUS semua logic polling di FE.**
⚡ **18 Sept 2026:** Parallel chunk processing — tipikal 47s → ~23s; hingga 20–21 soal per PDF.

**Query parameter:**
| Param | Wajib | Nilai valid |
|---|---|---|
| `jenis` | ✅ | `harian` / `uts` / `uas` |

**Body (form-data):**
| Field | Tipe | Batas | Keterangan |
|---|---|---|---|
| `file` | file | `.pdf`, maks **5MB** | PDF berisi soal pilihan ganda |

**curl:**
```bash
curl -X POST "https://momo-be-production.up.railway.app/api/v1/modul/3/soal?jenis=uts" \
  -H "Authorization: Bearer $TOKEN_GURU" \
  -F "file=@/home/user/Documents/soal-ipa.pdf"
```

**Response (200 OK):**
```json
{
  "message": "Soal berhasil diproses",
  "jenis": "uts",
  "jumlah": 6,
  "data": [
    {
      "id": 5, "modul_id": 3, "jenis": "uts",
      "pertanyaan": "Perhatikan gambar ayunan bandul dibawah ini...",
      "pilihan_a": "2 sekon dan 0,5 Hz",
      "pilihan_b": "3 sekon dan 1 Hz",
      "pilihan_c": "4 sekon dan 2 Hz",
      "pilihan_d": "5 sekon dan 3 Hz",
      "created_at": "..."
    }
  ]
}
```
⚠️ Timeout fetch di FE harus **> 120 detik**.

**Error (400 / 403 / 422):**
```json
{ "error": "Query param 'jenis' wajib salah satu dari: harian, uts, uas" }
{ "error": "File PDF wajib dilampirkan dengan field 'file'" }
{ "error": "Ukuran file terlalu besar. Maksimal 5MB.", "code": "FILE_TOO_LARGE" }
{ "error": "Hanya file PDF yang diperbolehkan", "code": "INVALID_FILE_TYPE" }
{ "error": "akses ditolak: modul tidak ditemukan atau bukan milik Anda" }
{ "error": "Gagal memproses soal dari PDF. ...", "code": "SOAL_PROCESSING_FAILED" }
```

---

### CRUD Soal Manual (14 Sept 2026)

⚠️ **Keamanan:** field `kunci_jawaban` **TIDAK PERNAH muncul** di response manapun.

#### `GET /api/v1/modul/:id/soal/list?jenis=uts` — List soal (KHUSUS GURU)

**Query parameter:** `jenis` opsional (`harian`/`uts`/`uas`; kosong = semua).

**Response (200):**
```json
{
  "jenis": "harian",
  "jumlah": 3,
  "data": [
    {
      "id": 5, "modul_id": 3, "jenis": "harian",
      "pertanyaan": "Perhatikan gambar ayunan bandul...",
      "pilihan_a": "2 sekon dan 0,5 Hz",
      "pilihan_b": "3 sekon dan 1 Hz",
      "pilihan_c": "4 sekon dan 2 Hz",
      "pilihan_d": "5 sekon dan 3 Hz",
      "created_at": "..."
    }
  ]
}
```

#### `POST /api/v1/modul/:id/soal/manual` — Buat soal manual

**Body:**
| Field | Tipe | Wajib | Keterangan |
|---|---|---|---|
| `jenis` | string | ✅ | `harian` / `uts` / `uas` |
| `pertanyaan` | string | ✅ | |
| `pilihan_a` … `pilihan_d` | string | ✅ | |
| `kunci_jawaban` | string | ✅ | `A` / `B` / `C` / `D` (case-insensitive) |

**Response (201):**
```json
{
  "message": "Soal berhasil ditambahkan",
  "data": { "id": 46, "modul_id": 3, "jenis": "harian", "pertanyaan": "...", "pilihan_a": "...", "pilihan_b": "...", "pilihan_c": "...", "pilihan_d": "...", "created_at": "..." }
}
```

#### `PUT /api/v1/soal/:id` — Update soal

⚠️ Path parameter = **ID soal**. `jenis` opsional (kosong = tidak berubah).

**Response (200):**
```json
{ "message": "Soal berhasil diperbarui", "data": { "id": 46, "modul_id": 3, "jenis": "uts", "pertanyaan": "...", "...": "..." } }
```

#### `DELETE /api/v1/soal/:id` — Hapus soal

**Response (200):**
```json
{ "message": "Soal berhasil dihapus" }
```

---

### ⚠️ `GET /api/v1/modul/:id/soal?jenis=uts` — Ambil soal (KHUSUS TOKEN SISWA)

🚫 **TIDAK BOLEK dipanggil dari halaman Guru.**

- **Auth:** **Token Siswa**

**Response (200):** pilihan jawaban **sudah diacak backend**
```json
{
  "jenis": "uts",
  "jumlah": 6,
  "data": [
    {
      "id": 5, "modul_id": 3, "jenis": "uts",
      "pertanyaan": "Perhatikan gambar ayunan bandul dibawah ini...",
      "pilihan_a": "2 sekon dan 0,5 Hz",
      "pilihan_b": "3 sekon dan 1 Hz",
      "pilihan_c": "4 sekon dan 2 Hz",
      "pilihan_d": "5 sekon dan 3 Hz",
      "created_at": "..."
    }
  ]
}
```

**Error (400 / 401 / 403 / 404):**
```json
{ "error": "Query param 'jenis' wajib salah satu dari: harian, uts, uas" }
{ "error": "Endpoint ini khusus siswa. Token Anda bukan token siswa.", "code": "TOKEN_WRONG_ROLE" }
{ "error": "modul ini tidak ditugaskan untuk kelas Anda" }
{ "error": "Belum ada soal untuk modul dan jenis ini" }
```

---

## C. Kelas & Nilai (Token Guru)

### `POST /api/v1/kelas` — Buat kelas baru

- **Auth:** Token Guru
- **Content-Type:** `application/json`

**Body:**
| Field | Tipe | Wajib | Keterangan |
|---|---|---|---|
| `nama` | string | ✅ | ⚠️ field bernama `nama`, bukan `nama_kelas` |
| `mata_pelajaran` | string | ✅ | |

**Response (201):**
```json
{ "id": 3, "guru_id": 3, "nama_kelas": "Kelas 5A", "mata_pelajaran": "Matematika", "kode_kelas": "487137", "created_at": "...", "updated_at": "..." }
```
⚠️ Asimetri: request `nama` → response `nama_kelas`.
📡 Memicu event SSE `kelas-created`.

---

### `GET /api/v1/kelas` — List kelas (dengan pagination)

- **Auth:** Token Guru

**Query parameter:** `page` (default 1), `limit` (default 10, maks 50).

**Response (200):**
```json
{
  "data": [ { "id": 3, "guru_id": 3, "nama_kelas": "Kelas 5A", "mata_pelajaran": "Matematika", "kode_kelas": "487137", "siswa": [ ... ], "modul": [ ... ], "created_at": "...", "updated_at": "..." } ],
  "meta": { "page": 1, "limit": 10, "total": 3, "total_page": 1 }
}
```
⚠️ FE wajib membaca `response.data`.

---

### `GET /api/v1/kelas/:id` — Detail kelas

- **Auth:** Token Guru

**Response (200):** object kelas + preload `siswa` dan `modul`.

**Error (404):**
```json
{ "error": "kelas tidak ditemukan atau Anda tidak memiliki akses" }
```

---

### `PUT /api/v1/kelas/:id` — Update kelas

- **Auth:** Token Guru

**Body:** `nama` dan/atau `mata_pelajaran` (opsional keduanya).

**Response (200):**
```json
{ "message": "Kelas berhasil diperbarui", "data": { "id": 3, "nama_kelas": "Kelas 5A Updated", "...": "..." } }
```
📡 Memicu event SSE `kelas-updated`.

---

### `DELETE /api/v1/kelas/:id` — Hapus kelas

- **Auth:** Token Guru

**Response (200):**
```json
{ "message": "Kelas berhasil dihapus" }
```
📡 Memicu event SSE `kelas-deleted`.

---

### `POST /api/v1/kelas/:id/siswa` — Daftarkan siswa ke kelas

- **Auth:** Token Guru
- **Body:** `{ "nama": "Siswa Retest" }`

**Response (201):**
```json
{ "id": 1, "kelas_id": 3, "nama": "Siswa Retest", "created_at": "..." }
```

**Error (400):**
```json
{ "error": "siswa dengan nama 'Siswa Retest' sudah terdaftar di kelas ini" }
```

---

### `POST /api/v1/kelas/:id/modul` — Tautkan modul ke kelas

- **Auth:** Token Guru
- **Body:** `{ "modul_id": 3 }`

**Response (200):**
```json
{ "message": "Modul berhasil ditautkan ke kelas" }
```

---

### `DELETE /api/v1/kelas/:id/modul/:modul_id` — Lepas modul dari kelas

- **Auth:** Token Guru

**Response (200):**
```json
{ "message": "Modul berhasil dihapus dari kelas" }
```

---

### `GET /api/v1/kelas/:id/nilai` — Rekap nilai

- **Auth:** Token Guru

**Query parameter:** `modul_id` (wajib), `jenis` (opsional).

**Response (200):**
```json
[ { "siswa_id": 1, "nama": "Siswa Retest", "jumlah_soal_dijawab": 1, "jumlah_benar": 1, "skor_persen": 100 } ]
```

---

## D. Alur Siswa (Token Siswa)

### `POST /api/v1/submit-jawaban` — Submit jawaban (dinilai AI)

- **Auth:** Token Siswa
- **Content-Type:** `application/json`
- **Rate limit:** AI limiter

**Body:**
| Field | Tipe | Wajib | Keterangan |
|---|---|---|---|
| `soal_id` | number | ✅ | |
| `jawaban_mentah` | string | ✅ | Teks bebas hasil Speech-to-Text, tidak harus huruf A-D |

⚠️ `siswa_id` **tidak dikirim** — diambil dari token.

**curl:**
```bash
curl -X POST https://momo-be-production.up.railway.app/api/v1/submit-jawaban \
  -H "Content-Type: application/json" -H "Authorization: Bearer $TOKEN_SISWA" \
  -d '{"soal_id":1,"jawaban_mentah":"aku rasa jawabannya yang B"}'
```

**Response (201):**
```json
{
  "id": 1, "siswa_id": 1, "soal_id": 1,
  "jawaban_mentah": "aku rasa jawabannya yang B",
  "jawaban_terdeteksi": "B",
  "benar": true,
  "feedback": "Betul! Jawaban kamu tepat.",
  "created_at": "..."
}
```

**Error (format standar):**
```json
{ "code": "NOT_FOUND", "message": "soal dengan ID 999999 tidak ditemukan", "source": "client" }
{ "code": "ALREADY_ANSWERED", "message": "Soal UTS/UAS hanya boleh dijawab sekali", "source": "client" }
{ "code": "AI_UNAVAILABLE", "message": "Gagal menghubungi AI Service untuk mengevaluasi jawaban. Silakan coba lagi.", "source": "ai_service" }
```
*(Jenis `uts`/`uas`: satu siswa satu kali per soal. Jenis `harian` boleh berulang.)*

---

## E. Alur Belajar Siswa

Endpoint khusus **Token Siswa** untuk alur belajar voice-first.

### `GET /api/v1/siswa/kelas-saya` — Info kelas & modul yang ditugaskan

**Response (200):**
```json
{
  "kelas": { "id": 3, "nama_kelas": "Kelas 5A", "mata_pelajaran": "Matematika", "kode_kelas": "487137" },
  "modul": [
    { "id": 3, "nama": "Ipa", "deskripsi": "...", "punya_materi": true, "jenis_soal_tersedia": ["harian", "uts"] },
    { "id": 5, "nama": "Matematika", "deskripsi": "...", "punya_materi": false, "jenis_soal_tersedia": ["uas"] }
  ]
}
```

**Kegunaan untuk FE:**
- `punya_materi` → TTS: *"Kelas ini menyediakan materi"* / *"belum ada materi"*
- `jenis_soal_tersedia` → TTS: *"Kelas ini menyediakan soal harian dan UTS"*

---

### `GET /api/v1/siswa/modul/:id` — Detail modul untuk siswa 🆕 (18 Sept)

**Response (200):**
```json
{
  "id": 3,
  "nama": "Ipa",
  "deskripsi": "Ilmu Pengetahuan Alam untuk Kelas 5",
  "jumlah_materi": 5,
  "jumlah_soal": 30,
  "jenis_soal_tersedia": ["harian", "uts"]
}
```

---

### `GET /api/v1/siswa/modul/:id/materi` — Ambil materi untuk dibacakan TTS

**Response (200):**
```json
{
  "jumlah": 53,
  "data": [
    { "id": 73, "modul_id": 3, "urutan": 1, "judul": "Pengantar Buku IPAS Kelas V", "konten": "Buku Ilmu Pengetahuan Alam dan Sosial (IPAS)...", "created_at": "..." }
  ]
}
```
⚠️ Field `konten` berisi teks lengkap siap TTS.

---

### `GET /api/v1/siswa/modul/:id/soal?jenis=harian` — Alias soal untuk siswa 🆕 (18 Sept)

Identik dengan `GET /modul/:id/soal` (pilihan teracak, tanpa kunci) — disediakan agar alur siswa konsisten prefix `/siswa/`.

---

## E2. Mode Tutor / Chat (18–19 Sept 2026)

### `POST /api/v1/chat` — SATU API percakapan realtime (AUTH OPSIONAL) 🆕

- **Auth:** **TIDAK WAJIB.** Tanpa token = siswa anonim; dengan token siswa = session menempel ke akun.
- **Content-Type:** `application/json`
- **Rate limit:** AI limiter (3 req / 6 detik per IP)
- **Alias:** `POST /api/v1/tutor` (handler sama)

**Body:**
| Field | Tipe | Wajib | Keterangan |
|---|---|---|---|
| `pesan` | string | ✅ | Teks hasil Speech-to-Text dari suara siswa |
| `session_id` | string | ❌ | Kunci memory percakapan; kirim balik nilai dari response sebelumnya |
| `kelas_nama` | string | ❌ | Nama kelas untuk personalisasi konteks |

```json
{ "pesan": "aku mau belajar getaran", "session_id": "anon_1789790529232415233_1" }
```

**curl:**
```bash
# Giliran 1 (tanpa token, tanpa session)
curl -s -X POST https://momo-be-production.up.railway.app/api/v1/chat \
  -H "Content-Type: application/json" \
  -d '{"pesan":"aku mau belajar"}'

# Giliran 2+ (pakai session_id dari response sebelumnya)
curl -s -X POST https://momo-be-production.up.railway.app/api/v1/chat \
  -H "Content-Type: application/json" \
  -d '{"pesan":"kasih contoh lainnya dong","session_id":"anon_1789790529232415233_1"}'
```

**Response (200, 5–30 detik):**
```json
{
  "message": "Balasan tutor diterima",
  "job_id": "tutor_1789790529232420063_2",
  "session_id": "anon_1789790529232415233_1",
  "balasan": "Wah, topik yang sangat menarik! Getaran adalah gerakan bolak-balik suatu benda secara teratur melalui titik tengahnya. Kamu bisa merasakannya saat meletakkan jari di tenggorokan sambil bersuara..."
}
```

⚠️ **FE WAJIB menyimpan `session_id`** (localStorage) dan mengirimnya balik tiap giliran — ini kunci memory percakapan.
⚠️ **Timeout fetch FE: 120 detik.** AI butuh 5–30 detik untuk reasoning.
✅ Balasan dijamin **maks 80 kata, TTS-safe** (tanpa markdown/emoji), bahasa Indonesia natural dengan **analogi auditori/taktil** untuk siswa tuna netra.

**Perilaku session:**
| Kondisi | Session yang dipakai |
|---|---|
| Tanpa token, tanpa `session_id` | Backend buat `anon_...` baru → return di response |
| Tanpa token, dengan `session_id` | Memory lanjut dari session itu |
| Dengan token siswa | Otomatis `siswa-<id>` (mengabaikan `session_id` body) |

**Error:**
```json
{ "code": "MISSING_FIELD", "message": "Field 'pesan' wajib diisi", "source": "client" }
{ "code": "AI_TIMEOUT", "message": "Momo terlalu lama merespons. Silakan coba lagi.", "source": "ai_service" }
{ "code": "AI_UNAVAILABLE", "message": "AI tutor sedang tidak tersedia. Silakan coba lagi dalam beberapa saat.", "source": "ai_service" }
```

**Contoh percakapan nyata (production, 19 Sept):**
```
🎤 "aku mau belajar"          → "Halo! Senang sekali bisa mendampingi kamu belajar hari ini. Kamu ingin belajar materi apa kali ini?"
🎤 "aku mau belajar getaran"  → "Getaran adalah gerakan bolak-balik suatu benda... rasakan saat meletakkan jari di tenggorokan sambil bersuara..."
🎤 "kasih contoh lainnya dong"→ "Contoh lainnya adalah ponsel yang bergetar saat ada pesan masuk... permukaan pengeras suara yang sedang memutar lagu..."
```

---

## F. SSE Legacy: Stream Kelas (Token Guru)

### `GET /api/v1/kelas/stream` — Stream event kelas

- **Auth:** Token Guru (header)

**Daftar event:**
| Event | Kapan dikirim | Isi `data` |
|---|---|---|
| `connected` | Koneksi pertama dibuka | `{ "message", "time" }` |
| `kelas-created` | Kelas baru dibuat | Object kelas lengkap |
| `kelas-updated` | Kelas di-update | Object kelas lengkap |
| `kelas-deleted` | Kelas dihapus | `{ "id": <id_kelas> }` |
| `: heartbeat` | Tiap ±15 detik | Komentar kosong — **abaikan** |

⚠️ Untuk kebutuhan real-time baru (materi/soal/jawaban/tutor), **gunakan Unified Stream di bagian G**.

---

## G. Unified Stream (Guru + Siswa + Public)

### Overview

| | |
|---|---|
| **Endpoint** | `GET /api/v1/stream` |
| **Method** | GET (Server-Sent Events) |
| **Auth** | **OPSIONAL** (19 Sept): token guru / token siswa / tanpa token (role `public`) |
| **Token via** | Header `Authorization: Bearer ...` ATAU query param `?token=...` |

### Event Catalog

| Event | Trigger | Scope |
|---|---|---|
| `connected` | Client connect | semua role |
| `materi-ready` | Guru upload PDF materi selesai | guru |
| `soal-ready` | Guru upload PDF soal selesai | guru |
| `jawaban-submitted` | Siswa submit jawaban selesai | guru |
| `tutor-reply` | AI tutor selesai membalas | siswa + public |
| `: heartbeat` | Tiap 15 detik | semua role |

Contoh:
```text
event: connected
data: {"message":"Terhubung ke stream Momo","role":"public","time":"2026-09-19T03:09:21Z"}

event: tutor-reply
data: {"balasan":"Halo! Senang sekali bisa belajar bersama kamu hari ini...","job_id":"tutor_..."}
```

### Ringkasan Event per Role

| Event | Guru | Siswa | Public (tanpa token) |
|---|:---:|:---:|:---:|
| `connected` / `heartbeat` | ✅ | ✅ | ✅ |
| `materi-ready` / `soal-ready` / `jawaban-submitted` | ✅ | ❌ | ❌ |
| `tutor-reply` | ❌ | ✅ | ✅ |

### ⚠️ Catatan penting untuk FE siswa

**Halaman `/study` TIDAK BOLEH memakai stream untuk percakapan.** Percakapan pakai `POST /chat` (sync). Stream hanya untuk dashboard guru dan listener demo publik. Jangan pernah `speak()` event `connected` — itu sumber bug suara "Terhubung ke stream Momo".

### Client Implementation (EventSource + query param)

```javascript
const es = new EventSource(
  `https://momo-be-production.up.railway.app/api/v1/stream?token=${token}` // token opsional
);
es.addEventListener('soal-ready', e => refreshSoalList(JSON.parse(e.data)));
es.onerror = () => console.warn('stream putus, auto-reconnect');
```

### Perilaku Koneksi

| Aspek | Nilai |
|---|---|
| Heartbeat interval | 15 detik |
| Reconnect strategy | Exponential backoff (1s → 2s → 4s → 8s → 16s) |
| Max reconnect attempts | 5 |
| Buffer event per client | 50 event |

---

## H. Lampiran: Endpoint Debug

### `POST /api/v1/test-extract-pdf` — Uji ekstraksi PDF (debug only)

- **Auth:** Publik
- **Rate limit:** AI limiter

Endpoint utilitas untuk menguji ekstraksi teks PDF **tanpa menyimpan ke database**. **Tidak dipakai oleh FE produksi.**

---

## I. Panduan Integrasi FE — Halaman Study

> Bagian ini ringkasan wajib untuk frontend. Bacanya dari sini dulu, baru ke section lain bila perlu detail.

### I.1 Yang HARUS DIHAPUS di halaman `/study`

```javascript
// ❌ HAPUS: stream di halaman study
const es = new EventSource(BASE + '/stream');
es.addEventListener('connected', ...);   // ❌ sumber suara "Terhubung ke stream Momo"
es.addEventListener('tutor-reply', ...); // ❌ tutor sekarang sync via /chat
speak(eventConnected.message);           // ❌ JANGAN speak event stream
```

**Aturan:** halaman `/study` = **nol stream, nol polling**. Stream hanya untuk dashboard guru.

### I.2 Modul chat siap pakai (`momoChat.js`)

```javascript
const BASE = 'https://momo-be-production.up.railway.app/api/v1';
const SESSION_KEY = 'momo_session_id';

// ---------- TTS ----------
export function speak(teks) {
  speechSynthesis.cancel(); // stop suara lama biar tidak menumpuk
  const u = new SpeechSynthesisUtterance(teks);
  u.lang = 'id-ID';
  u.rate = 0.95;
  speechSynthesis.speak(u);
}

// ---------- STT ----------
export function listenOnce(onResult, onError) {
  const SR = window.SpeechRecognition || window.webkitSpeechRecognition;
  if (!SR) return onError('unsupported');
  const rec = new SR();
  rec.lang = 'id-ID';
  rec.interimResults = false;
  rec.maxAlternatives = 1;
  rec.onresult = (e) => onResult(e.results[0][0].transcript);
  rec.onerror = (e) => onError(e.error);
  rec.start();
}

// ---------- CHAT (SATU-SATUNYA API percakapan) ----------
export async function kirimPercakapan(teks, { onLoading, onBalasan, onError }) {
  onLoading && onLoading(true);
  const controller = new AbortController();
  const watchdog = setTimeout(() => controller.abort(), 110000); // timeout 110 detik

  try {
    const res = await fetch(BASE + '/chat', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      signal: controller.signal,
      body: JSON.stringify({
        pesan: teks,
        session_id: localStorage.getItem(SESSION_KEY) || undefined,
      }),
    });

    const data = await res.json();

    if (res.ok) {
      if (data.session_id) localStorage.setItem(SESSION_KEY, data.session_id); // WAJIB
      onBalasan && onBalasan(data.balasan);
      speak(data.balasan);
    } else if (data.source === 'ai_service') {
      speak('Momo sedang sibuk, coba lagi sebentar lagi ya.');
      onError && onError(data);
    } else {
      speak(data.message || 'Maaf, terjadi kesalahan.');
      onError && onError(data);
    }
  } catch (err) {
    speak('Koneksi terputus, periksa internet kamu ya.');
    onError && onError({ code: 'NETWORK', source: 'client' });
  } finally {
    clearTimeout(watchdog);
    onLoading && onLoading(false); // WAJIB: loading tidak boleh nyangkut
  }
}

// ---------- Alur: klik mic → bicara → Momo menjawab ----------
export function mulaiPercakapan(handlers) {
  listenOnce(
    (teks) => kirimPercakapan(teks, handlers),
    (err) => {
      if (err === 'not-allowed') speak('Izinkan akses mikrofon dulu ya.');
      handlers.onError && handlers.onError({ code: 'MIC_' + err, source: 'client' });
    }
  );
}
```

**Pemakaian:**

```javascript
import { mulaiPercakapan, speak } from './momoChat.js';

tombolMic.onclick = () => mulaiPercakapan({
  onLoading: (v) => setAnimasiMomoBerpikir(v),
  onBalasan: (teks) => tampilkanBubbleMomo(teks),
  onError: (e) => console.warn('chat error:', e),
});

// Sapaan lokal saat masuk study (tanpa API):
speak('Halo! Klik kanan untuk bicara, klik kiri untuk mendengarkan jawabanku.');
```

### I.3 Aturan wajib FE

1. **Satu fetch per pesan** (`POST /chat`). Tidak ada stream/polling untuk tutor.
2. **Simpan `session_id`** di localStorage key `momo_session_id`; kirim balik tiap giliran.
3. **Speak HANYA dari `response.balasan`.** Jangan speak event stream / field `message` event `connected`.
4. **Loading:** `true` sebelum fetch, `false` di `finally`. Tidak boleh spinner abadi.
5. **Timeout fetch 110–120 detik.**
6. **Rate limit 3 req/6 detik** — jika `429`, tunggu 6 detik, retry sekali.
7. Token siswa (`/join`) hanya dibutuhkan untuk mode **soal/materi kelas**, bukan untuk chat bebas.

### I.4 Error cheatsheet

| Response | Aksi FE |
|---|---|
| `200` + `balasan` | `speak(balasan)` |
| `400 MISSING_FIELD` | Jangan kirim pesan kosong |
| `429` | Tunggu 6 detik, retry 1x |
| `503 AI_TIMEOUT` | Speak "Momo sibuk, coba lagi ya" |
| `503 AI_UNAVAILABLE` | Speak "Momo sedang istirahat, coba lagi nanti" |
| Network error / abort | Speak "Koneksi terputus" |

### I.5 Checklist test FE

```
[ ] Buka /study → TIDAK ada suara "Terhubung ke stream Momo"
[ ] Klik mic → "Halo aku mau belajar" → 5-30 detik → Momo menjawab via TTS
[ ] Mic lagi → "aku mau belajar getaran" → jawaban nyambung topik
[ ] Mic lagi → "kasih contoh lainnya" → masih nyambung (memory OK)
[ ] Refresh halaman → bicara lagi → memory masih nyambung
[ ] Matikan internet → klik mic → TTS "Koneksi terputus" (bukan spinner abadi)
[ ] Animasi "berpikir" muncul saat menunggu, HILANG setelah jawaban masuk
[ ] Klik kiri → membacakan bubble jawaban Momo (TTS ulang, tanpa API)
```

---

## Error Handling

### HTTP Status Codes

| Code | Arti | Contoh kasus |
|---|---|---|
| 200 | OK | GET/PUT/DELETE berhasil; join; upload sync; **chat sukses** |
| 201 | Created | register, buat modul/kelas/siswa/materi/soal manual, submit jawaban |
| 302 | Found | redirect verify-email |
| 400 | Bad Request | validasi gagal, file salah, bukan pemilik resource |
| 401 | Unauthorized | token hilang/salah/expired/wrong role (endpoint ber-auth) |
| 403 | Forbidden | resource bukan milik user, modul tidak ditugaskan |
| 404 | Not Found | resource tidak ada |
| 422 | Unprocessable Entity | proses AI gagal (materi/soal upload) |
| 429 | Too Many Requests | rate limit terlampaui |
| 500 | Internal Server Error | kesalahan tak terduga |
| 503 | Service Unavailable | AI error (chat/evaluate) |

### Error Codes Standar (`code` + `source`)

**Client errors (4xx) — `source: "client"`:**
| Code | HTTP | Saran FE |
|---|---|---|
| `INVALID_REQUEST` | 400 | Tampilkan message |
| `MISSING_FIELD` | 400 | Highlight field |
| `INVALID_FORMAT` | 400 | Tampilkan message |
| `INVALID_FILE` | 400 | Tampilkan batas file |
| `UNAUTHORIZED` | 401 | Redirect join/login |
| `FORBIDDEN` | 403 | Tampilkan message |
| `NOT_FOUND` | 404 | Tampilkan message |
| `DUPLICATE` | 409 | Tampilkan message |
| `ALREADY_ANSWERED` | 409 | Lanjut soal berikutnya |

**AI Service errors (5xx) — `source: "ai_service"`:**
| Code | HTTP | Saran FE |
|---|---|---|
| `AI_PROCESSING_FAILED` | 503 | "Coba lagi atau ganti PDF" |
| `AI_TIMEOUT` | 503 | Retry 1x, lalu message |
| `AI_UNAVAILABLE` | 503 | "AI sibuk, coba lagi nanti" |
| `AI_INVALID_RESPONSE` | 503 | Message generic |

**Server errors (5xx) — `source: "server"`:**
| Code | HTTP | Saran FE |
|---|---|---|
| `INTERNAL_ERROR` | 500 | Message generic + log |
| `DATABASE_ERROR` | 500 | Message generic + log |

**Pattern universal:**
```javascript
if (!res.ok) {
  const err = await res.json();
  setLoading(false);
  if (err.source === 'ai_service') showToast('AI sedang sibuk, coba lagi nanti', 'warning');
  else if (err.source === 'client') showToast(err.message, 'error');
  else showToast('Terjadi kesalahan sistem, coba lagi', 'error');
  if (res.status === 401) restartOnboarding();
  return;
}
```

### Error Codes Lama (middleware stream & endpoint guru lama)

| Code | Kapan muncul |
|---|---|
| `TOKEN_MISSING` | Header Authorization tidak ada (endpoint ber-auth) |
| `TOKEN_INVALID_FORMAT` | Format header bukan `Bearer <token>` |
| `TOKEN_INVALID` | Token tidak valid / kedaluwarsa |
| `TOKEN_WRONG_ROLE` | Token guru di endpoint siswa atau sebaliknya |
| `FILE_TOO_LARGE` | Upload melebihi batas |
| `INVALID_FILE_TYPE` | File bukan `.pdf` |
| `MATERI_PROCESSING_FAILED` | AI gagal merangkum materi |
| `SOAL_PROCESSING_FAILED` | AI gagal mengekstrak soal |

---

## Known Issues / Catatan untuk FE

1. **`nama` vs `nama_kelas`** di endpoint Kelas.
2. **Lokasi token berbeda**: login guru `data.token`, join siswa `.token` (root).
3. **`judul` vs `nama`** untuk modul: `GET /modul/:id` memakai `judul`.
4. **`kunci_jawaban` tidak pernah dikirim** ke endpoint manapun.
5. **Object materi tidak punya `updated_at`**.
6. **Materi & Soal SYNCHRONOUS**: spinner + render array `data`; hapus polling.
7. **`GET /kelas` berbentuk `{ data, meta }`**.
8. **Role isolation aktif** (`TOKEN_WRONG_ROLE`).
9. **Pilihan jawaban diacak backend** per request.
10. **`PUT /modul/:id` menimpa deskripsi selalu**.
11. **Response `PUT /modul/:id` payload besar** (preload).
12. **Materi dua sumber bercampur**, terurut `urutan` ASC.
13. **Path CRUD materi berbeda** (list scoped modul, update/delete scoped materi).
14. **Delete modul = cascade**.
15. **Submit `uts`/`uas` sekali per siswa per soal**.
16. **Registrasi guru memicu email verifikasi**.
17. **SSE heartbeat adalah komentar** — abaikan.
18. **`/test-extract-pdf` hanya debug**.
19. **Timeout fetch upload > 120 detik**.
20. **CRUD Soal Manual tersedia**.
21. **Kunci jawaban tidak bocor**; saat edit minta input ulang.
22. **Soal manual & AI bercampur** di list.
23. **Auto-register siswa** di `POST /join`.
24. **Endpoint siswa**: `/siswa/kelas-saya`, `/siswa/modul/:id/materi`.
25. **Flag `punya_materi` & `jenis_soal_tersedia`** untuk TTS info kelas.
26. **Stream support `?token=`** untuk EventSource native.
27. **`tutor-reply` targeted per siswa** (+ fallback public listener).
28. **Dua format error hidup bersamaan** — cek field `source`.
29. **WAJIB reset loading di semua error path**.
30. **Token hilang → restart onboarding**, jangan cuma toast.
31. **Upload parallel**: tipikal 20–30 detik.
32. **`GET /siswa/modul/:id`** untuk header halaman belajar.
33. **Mode tutor = 1 fetch sync** (`POST /chat`), bukan stream.
34. 🆕 **`/chat` publik**: `session_id` di localStorage adalah kunci memory; hilang = percakapan reset.
35. 🆕 **Jangan speak event `connected`** — penyebab bug "Terhubung ke stream Momo".
36. 🆕 **`/tutor` = alias `/chat`** — pakai `/chat` di kode baru.
37. 🆕 **Chat tidak butuh token/join**; token hanya untuk mode soal/materi kelas.

---

## Changelog

### 19 September 2026 (update 6–10, FINAL pre-demo)
- 🆕 **Stream tanpa token**: `GET /stream` tanpa auth → role `public` (menerima event scope siswa + all; event guru tetap aman)
- 🔄 **`POST /tutor` → SYNCHRONOUS**: response berisi `balasan` langsung (sync-via-callback, timeout internal 90s)
- 🆕 **`POST /api/v1/chat`**: satu API percakapan realtime, **AUTH OPSIONAL** (anonim pakai `session_id`, login pakai session `siswa-<id>`)
- 🧠 **Memory percakapan**: `session_id` backend-generated, dikembalikan di response, wajib disimpan FE
- ✅ Validasi production: 3 giliran percakapan anonim (sambutan → getaran → follow-up) dengan analogi taktil
- 📄 **Section I: Panduan Integrasi FE** (modul `momoChat.js`, aturan wajib, error cheatsheet, checklist test)
- ⚠️ Halaman `/study` dilarang pakai stream untuk percakapan; jangan speak event `connected`

### 18 September 2026 (update 5)
- 🆕 Mode Tutor Fase 2: `POST /tutor` async + event `tutor-reply` (targeted per siswa)
- 🆕 Endpoint siswa: `GET /siswa/modul/:id`, `GET /siswa/modul/:id/soal`
- 🆕 Standard error handling `{code, message, source}`
- 🆕 Stream menerima token via query param `?token=`
- ⚡ Parallel chunk processing: upload 47s → ~23s; ekstraksi hingga 20–21 soal/PDF

### 14 September 2026 (update 4)
- 🔄 BREAKING: `POST /join` auto-register siswa baru
- 🆕 `GET /siswa/kelas-saya`, `GET /siswa/modul/:id/materi`

### 14 September 2026 (update 3)
- 🆕 CRUD Soal Manual: `GET /modul/:id/soal/list`, `POST /modul/:id/soal/manual`, `PUT /soal/:id`, `DELETE /soal/:id`

### 14 September 2026 (update 2)
- 🔄 BREAKING: `POST /modul/:id/soal` SYNCHRONOUS + `SOAL_PROCESSING_FAILED`

### 14 September 2026
- 🔒 Role isolation aktif (`TOKEN_WRONG_ROLE`) + perapian dokumentasi

### 12 September 2026 (update 2 & 3)
- 🆕 CRUD Materi manual & CRUD Modul lengkap

### 12 September 2026
- 🔄 BREAKING: `POST /modul/:id/materi` SYNCHRONOUS; `GET /kelas` pagination `{data, meta}`

### 11 September 2026
- 🆕 CRUD Kelas + `GET /kelas/stream`; optimasi DB

### 1 September 2026
- Dokumentasi awal berdasarkan testing langsung seluruh endpoint existing.