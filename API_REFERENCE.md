# API Reference — Momo-BE

Dokumen ini disusun berdasarkan **testing langsung terhadap kode yang berjalan** (bukan asumsi).
Versi awal: 1 September 2026 · **Update terakhir: 18 September 2026 (update 5)**.
Semua contoh request/response di bawah adalah hasil `curl` nyata terhadap environment production.

---

## Daftar Isi

- [Info Umum](#info-umum)
- [A. Health, Auth & Verifikasi](#a-health-auth--verifikasi)
- [B. Modul, Materi & Soal](#b-modul-materi--soal)
- [C. Kelas & Nilai (Token Guru)](#c-kelas--nilai-token-guru)
- [D. Alur Siswa (Token Siswa)](#d-alur-siswa-token-siswa)
- [E. Alur Belajar Siswa](#e-alur-belajar-siswa)
- [E2. Mode Tutor (18 Sept 2026)](#e2-mode-tutor-18-sept-2026)
- [F. SSE Legacy: Stream Kelas (Token Guru)](#f-sse-legacy-stream-kelas-token-guru)
- [G. Unified Stream (Guru + Siswa)](#g-unified-stream-guru--siswa)
- [H. Lampiran: Endpoint Debug](#h-lampiran-endpoint-debug)
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
| **Autentikasi** | JWT via header `Authorization: Bearer <token>` |

### 🔒 Dua Jenis Token JWT & Role Isolation

| | Token Guru | Token Siswa |
|---|---|---|
| Didapat dari | `POST /api/v1/guru/login` | `POST /api/v1/join` |
| Isi claim | `guru_id`, `role: "guru"` | `siswa_id`, `kelas_id` |
| Masa berlaku | 24 jam | 12 jam |
| Lokasi di response | `data.token` | `token` (root object) |
| Berlaku di | Semua endpoint Guru | Hanya endpoint Siswa (`GET /modul/:id/soal`, `POST /submit-jawaban`, `GET /siswa/*`, `POST /tutor`) |

**Role isolation aktif (14 Sept 2026):**
- Token Guru dipakai di endpoint siswa → `401` dengan `code: TOKEN_WRONG_ROLE`
- Token Siswa dipakai di endpoint guru → `401` dengan `code: TOKEN_WRONG_ROLE`
- FE wajib memastikan jenis token sesuai halaman yang sedang dibuka.

### CORS

Origin yang diizinkan:
- `http://localhost:3000`, `http://localhost:5173` (dev; dapat ditambah via env `CORS_ALLOWED_ORIGINS`)
- Semua subdomain `*.vercel.app` (production & preview)

Origin lain → `403 Forbidden`. Request tanpa header `Origin` (server-to-server, misal dari AI Service) tidak terpengaruh CORS.

### Rate Limiting

| Limiter | Batas | Diterapkan di |
|---|---|---|
| Auth limiter | 5 request / 12 detik per IP | `POST /guru/register`, `POST /guru/login`, `POST /join` |
| AI limiter | 3 request / 6 detik per IP | `POST /test-extract-pdf`, `POST /submit-jawaban`, `POST /tutor` |

Melebihi batas → `429 Too Many Requests`:
```json
{ "error": "Terlalu banyak permintaan. Silakan coba lagi dalam beberapa saat." }
```

### Format Error Umum

**Format standar baru (18 Sept 2026)** untuk error bisnis di endpoint siswa, tutor, submit-jawaban, dan upload:

```json
{
  "code": "NOT_FOUND",
  "message": "soal dengan ID 999999 tidak ditemukan",
  "source": "client"
}
```

| Field | Keterangan |
|---|---|
| `code` | Kode error stabil untuk logika FE (lihat section Error Handling) |
| `message` | Pesan bahasa Indonesia, aman ditampilkan langsung ke user |
| `source` | Penyebab error: `client` (FE salah) \| `ai_service` (AI salah) \| `server` (backend salah) |

⚠️ Middleware stream (`/stream`) masih memakai format lama `{"error":"...","code":"TOKEN_MISSING"}` — FE wajib menangani **kedua bentuk** (cek keberadaan field `source`).

### Cara Membaca Blok Endpoint

Setiap endpoint didokumentasikan dengan urutan konsisten:
**Auth → Content-Type → Rate limit → Path/Query parameter → Body → contoh curl → Response → Error.**

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

- **Auth:** Publik (dipanggil dari link di email, biasanya oleh browser)

**Query parameter:**
| Param | Wajib | Keterangan |
|---|---|---|
| `token` | ✅ | Token verifikasi sekali pakai dari email |

**Perilaku:** endpoint ini **tidak mengembalikan JSON**, melainkan **redirect (302)** ke URL frontend:
- Sukses → `{FE_VERIFY_REDIRECT_URL}?status=success`
- Gagal → `{FE_VERIFY_REDIRECT_URL}?status=error&message=<pesan>`

FE cukup menampilkan halaman tujuan berdasarkan query `status`.

**curl:**
```bash
curl -i "https://momo-be-production.up.railway.app/api/v1/guru/verify-email?token=xxx"
# -> HTTP/1.1 302 Found, Location: http://localhost:3000/email-verified?status=success
```

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

```json
{ "email": "sari@sekolah.com", "password": "rahasia123" }
```

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

🔄 **BREAKING CHANGE (14 Sept 2026):** Endpoint ini sekarang **auto-register siswa baru**. Jika nama belum terdaftar di kelas, backend otomatis membuat record siswa baru (tidak lagi error).

**Body:**
| Field | Tipe | Wajib | Keterangan |
|---|---|---|---|
| `kode_kelas` | string | ✅ | 6 digit angka, diberikan guru |
| `nama` | string | ✅ | Nama siswa |

```json
{ "kode_kelas": "487137", "nama": "Budi" }
```

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

**Perilaku baru:**
- Nama sudah terdaftar di kelas → return siswa existing + token
- Nama belum terdaftar → **auto-create** siswa baru + return token
- Kode kelas tidak valid → error `401`

**Error (401):**
```json
{ "error": "kelas dengan kode '000000' tidak ditemukan" }
```

⚠️ Guru tetap bisa melihat daftar siswa real-time di dashboard (via SSE `kelas-updated`).

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

Object **Modul** (list/create):
```json
{ "id": 2, "guru_id": 3, "nama": "Modul IPA", "deskripsi": "...", "created_at": "...", "updated_at": "..." }
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

```json
{ "nama": "Modul IPA", "deskripsi": "Ilmu Pengetahuan Alam untuk Kelas 5" }
```

**curl:**
```bash
curl -X POST https://momo-be-production.up.railway.app/api/v1/modul \
  -H "Content-Type: application/json" -H "Authorization: Bearer $TOKEN_GURU" \
  -d '{"nama":"Modul IPA","deskripsi":"Ilmu Pengetahuan Alam untuk Kelas 5"}'
```

**Response (201):** object modul mentah
```json
{ "id": 2, "guru_id": 3, "nama": "Modul IPA", "deskripsi": "...", "created_at": "...", "updated_at": "...", "materi": null, "soal": null }
```

**Error (400):** pesan validator mentah jika `nama` kosong
```json
{ "error": "Key: 'createModulRequest.Nama' Error:Field validation for 'Nama' failed on the 'required' tag" }
```

---

### `GET /api/v1/modul` — List modul milik guru

- **Auth:** Token Guru

**curl:**
```bash
curl -H "Authorization: Bearer $TOKEN_GURU" \
  https://momo-be-production.up.railway.app/api/v1/modul
```

**Response (200):** array object modul (hanya milik guru yang login)
```json
[ { "id": 3, "guru_id": 7, "nama": "Ipa", "deskripsi": "...", "created_at": "...", "updated_at": "...", "materi": null, "soal": null } ]
```

---

### `GET /api/v1/modul/:id` — Detail modul (termasuk materi & soal)

- **Auth:** Token Guru

**Path parameter:**
| Param | Tipe | Keterangan |
|---|---|---|
| `id` | number | ID modul |

**curl:**
```bash
curl -H "Authorization: Bearer $TOKEN_GURU" \
  https://momo-be-production.up.railway.app/api/v1/modul/3
```

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
✅ **Endpoint inilah yang dipakai halaman GURU** untuk preview soal & materi — bukan `GET /modul/:id/soal`.

**Error (404):**
```json
{ "error": "Modul tidak ditemukan" }
```

---

### `PUT /api/v1/modul/:id` — Update modul

- **Auth:** Token Guru
- **Content-Type:** `application/json`

**Path parameter:** `id` = ID modul.

**Body:**
| Field | Tipe | Wajib | Keterangan |
|---|---|---|---|
| `nama` | string | ❌* | *Minimal `nama` atau `deskripsi` harus diisi |
| `deskripsi` | string | ❌* | ️ Nilai yang dikirim **selalu menimpa** deskripsi lama |

```json
{ "nama": "Ipa (Updated)", "deskripsi": "Ilmu Pengetahuan Alam untuk Kelas 5" }
```

**curl:**
```bash
curl -X PUT https://momo-be-production.up.railway.app/api/v1/modul/3 \
  -H "Content-Type: application/json" -H "Authorization: Bearer $TOKEN_GURU" \
  -d '{"nama":"Ipa (Updated)","deskripsi":"Ilmu Pengetahuan Alam untuk Kelas 5"}'
```

**Response (200):** `data` berisi object modul lengkap **termasuk preload materi & soal** (payload besar)
```json
{
  "message": "Modul berhasil diperbarui",
  "data": { "id": 3, "guru_id": 7, "nama": "Ipa (Updated)", "deskripsi": "...", "created_at": "...", "updated_at": "...", "materi": [ ... ], "soal": [ ... ] }
}
```

**Error (400):**
```json
{ "error": "Minimal salah satu field (nama atau deskripsi) harus diisi" }
{ "error": "modul tidak ditemukan atau bukan milik Anda" }
```

---

### `DELETE /api/v1/modul/:id` — Hapus modul (cascade)

- **Auth:** Token Guru

⚠️ Menghapus **seluruh materi & soal** di dalam modul + melepas tautan ke kelas. Tidak bisa dibatalkan.

**curl:**
```bash
curl -X DELETE https://momo-be-production.up.railway.app/api/v1/modul/3 \
  -H "Authorization: Bearer $TOKEN_GURU"
```

**Response (200):**
```json
{ "message": "Modul berhasil dihapus beserta semua materi dan soal di dalamnya" }
```

**Error (400):**
```json
{ "error": "modul tidak ditemukan atau bukan milik Anda" }
```

---

### `GET /api/v1/modul/:id/materi` — List materi dalam modul

- **Auth:** Token Guru

**Path parameter:** `id` = ID modul.

**curl:**
```bash
curl -H "Authorization: Bearer $TOKEN_GURU" \
  https://momo-be-production.up.railway.app/api/v1/modul/3/materi
```

**Response (200):** terurut `urutan` ASC
```json
{
  "jumlah": 2,
  "data": [
    { "id": 11, "modul_id": 3, "urutan": 1, "judul": "...", "konten": "...", "created_at": "..." },
    { "id": 12, "modul_id": 3, "urutan": 2, "judul": "...", "konten": "...", "created_at": "..." }
  ]
}
```

**Error (403):**
```json
{ "error": "akses ditolak: modul tidak ditemukan atau bukan milik Anda" }
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

```json
{ "judul": "Pengenalan IPA", "konten": "IPA adalah ilmu yang mempelajari alam sekitar.", "urutan": 5 }
```

**curl:**
```bash
curl -X POST https://momo-be-production.up.railway.app/api/v1/modul/3/materi/manual \
  -H "Content-Type: application/json" -H "Authorization: Bearer $TOKEN_GURU" \
  -d '{"judul":"Pengenalan IPA","konten":"IPA adalah ilmu yang mempelajari alam sekitar."}'
```

**Response (201):**
```json
{
  "message": "Materi berhasil ditambahkan",
  "data": { "id": 149, "modul_id": 3, "urutan": 5, "judul": "Pengenalan IPA", "konten": "...", "created_at": "..." }
}
```

**Error (400 / 403):**
```json
{ "error": "Judul dan konten materi wajib diisi" }
{ "error": "judul dan konten materi wajib diisi" }
{ "error": "akses ditolak: modul tidak ditemukan atau bukan milik Anda" }
```

---

### `PUT /api/v1/materi/:id` — Update materi

- **Auth:** Token Guru
- **Content-Type:** `application/json`

⚠️ Path parameter adalah **ID materi**, bukan ID modul.

**Body:**
| Field | Tipe | Wajib | Keterangan |
|---|---|---|---|
| `judul` | string | ✅ | |
| `konten` | string | ✅ | |
| `urutan` | number | ❌ | Jika >0 akan diubah |

```json
{ "judul": "Pengenalan IPA (Revisi)", "konten": "IPA adalah ilmu tentang alam dan isinya.", "urutan": 5 }
```

**curl:**
```bash
curl -X PUT https://momo-be-production.up.railway.app/api/v1/materi/149 \
  -H "Content-Type: application/json" -H "Authorization: Bearer $TOKEN_GURU" \
  -d '{"judul":"Pengenalan IPA (Revisi)","konten":"IPA adalah ilmu tentang alam dan isinya."}'
```

**Response (200):**
```json
{
  "message": "Materi berhasil diperbarui",
  "data": { "id": 149, "modul_id": 3, "urutan": 5, "judul": "Pengenalan IPA (Revisi)", "konten": "...", "created_at": "..." }
}
```

**Error (400):**
```json
{ "error": "Judul dan konten materi wajib diisi" }
{ "error": "akses ditolak: materi ini bukan milik Anda" }
{ "error": "materi tidak ditemukan" }
```

---

### `DELETE /api/v1/materi/:id` — Hapus materi

- **Auth:** Token Guru

⚠️ Path parameter adalah **ID materi**.

**curl:**
```bash
curl -X DELETE https://momo-be-production.up.railway.app/api/v1/materi/149 \
  -H "Authorization: Bearer $TOKEN_GURU"
```

**Response (200):**
```json
{ "message": "Materi berhasil dihapus" }
```

**Error (400):**
```json
{ "error": "akses ditolak: materi ini bukan milik Anda" }
{ "error": "materi tidak ditemukan" }
```

---

### `POST /api/v1/modul/:id/materi` — Upload PDF materi (SYNCHRONOUS)

- **Auth:** Token Guru
- **Content-Type:** `multipart/form-data`

⚠️ Request **ditahan** sampai AI selesai merangkum (10–60 detik). FE wajib menampilkan spinner + disable tombol.

**Path parameter:** `id` = ID modul.

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

**Response (200):** hasil rangkuman asli
```json
{
  "message": "Materi berhasil diproses",
  "jumlah": 2,
  "data": [
    { "id": 11, "modul_id": 3, "urutan": 1, "judul": "Pengenalan IPA", "konten": "...", "created_at": "..." }
  ]
}
```
⚠️ FE harus merender array **`data`**, bukan `message`.

**Error (400):**
```json
{ "error": "File PDF wajib dilampirkan dengan field 'file'" }
{ "error": "Ukuran file terlalu besar. Maksimal 25MB.", "code": "FILE_TOO_LARGE" }
{ "error": "Hanya file PDF yang diperbolehkan", "code": "INVALID_FILE_TYPE" }
```

**Error (403):**
```json
{ "error": "akses ditolak: modul tidak ditemukan atau bukan milik Anda" }
```

**Error (422):**
```json
{ "error": "Gagal memproses materi dari PDF. Layanan AI terlalu lama memproses atau tidak tersedia. Coba lagi atau gunakan PDF yang lebih kecil.", "code": "MATERI_PROCESSING_FAILED" }
```
Varian pesan lain: `...File PDF tidak bisa dibaca. Pastikan PDF berisi teks, bukan hasil scan gambar.` / `...AI tidak menemukan konten materi yang valid. Pastikan PDF berisi materi pembelajaran, bukan soal.`

---

### `POST /api/v1/modul/:id/soal?jenis=uts` — Upload PDF soal (SYNCHRONOUS)

- **Auth:** Token Guru
- **Content-Type:** `multipart/form-data`

🔄 **BREAKING CHANGE (14 Sept 2026):** Endpoint ini sekarang **SYNCHRONOUS** — request ditahan sampai AI selesai mengekstrak soal (biasanya 10–30 detik). FE wajib menampilkan spinner penuh dan **HAPUS semua logic polling**.

⚡ **UPDATE 18 Sept 2026:** Parallel chunk processing aktif — tipikal waktu turun dari 47s → ~23s untuk PDF besar; hingga 20–21 soal terekstrak per PDF kumpulan soal.

**Path parameter:**
| Param | Tipe | Keterangan |
|---|---|---|
| `id` | number | ID modul tujuan (harus milik guru yang login) |

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

**Response (200 OK):** hasil ekstraksi asli
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
⚠️ FE langsung render array **`data`** ke panel hasil — tidak perlu fetch ulang atau polling.
⚠️ Timeout fetch di FE harus **> 120 detik** (atau matikan timeout untuk request ini).

**Error (400):**
```json
{ "error": "Query param 'jenis' wajib salah satu dari: harian, uts, uas" }
{ "error": "File PDF wajib dilampirkan dengan field 'file'" }
{ "error": "Ukuran file terlalu besar. Maksimal 5MB.", "code": "FILE_TOO_LARGE" }
{ "error": "Hanya file PDF yang diperbolehkan", "code": "INVALID_FILE_TYPE" }
```

**Error (403):** dicek synchronous sebelum proses dimulai
```json
{ "error": "akses ditolak: modul tidak ditemukan atau bukan milik Anda" }
```

**Error (422):**
```json
{ "error": "Gagal memproses soal dari PDF. Layanan AI terlalu lama memproses atau tidak tersedia. Coba lagi atau gunakan PDF yang lebih kecil.", "code": "SOAL_PROCESSING_FAILED" }
```
Varian pesan lain: `...File PDF tidak bisa dibaca. Pastikan PDF berisi teks, bukan hasil scan gambar.` / `...AI tidak menemukan soal pilihan ganda yang valid di PDF ini.`

---

### CRUD Soal Manual (14 Sept 2026)

Selain upload PDF (yang diekstrak AI), guru juga bisa menulis soal **manual** langsung di form. Soal manual dan hasil AI **bercampur dalam satu list**.

⚠️ **Keamanan:** field `kunci_jawaban` **TIDAK PERNAH muncul** di response manapun (termasuk response `POST /soal/manual` dan `PUT /soal/:id`), konsisten dengan endpoint siswa.

#### `GET /api/v1/modul/:id/soal/list?jenis=uts` — List soal (KHUSUS GURU)

Endpoint khusus dashboard guru untuk melihat soal dengan filter jenis.

**Path parameter:**
| Param | Tipe | Keterangan |
|---|---|---|
| `id` | number | ID modul (harus milik guru yang login) |

**Query parameter:**
| Param | Wajib | Keterangan |
|---|---|---|
| `jenis` | ❌ | `harian` / `uts` / `uas` — jika kosong, kembalikan semua jenis |

**curl:**
```bash
# Filter hanya UTS
curl -H "Authorization: Bearer $TOKEN_GURU" \
  "https://momo-be-production.up.railway.app/api/v1/modul/3/soal/list?jenis=uts"

# Semua jenis
curl -H "Authorization: Bearer $TOKEN_GURU" \
  "https://momo-be-production.up.railway.app/api/v1/modul/3/soal/list"
```

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
⚠️ Tidak ada field `kunci_jawaban` di response.

**Error (400 / 403):**
```json
{ "error": "Query param 'jenis' wajib salah satu dari: harian, uts, uas" }
{ "error": "akses ditolak: modul tidak ditemukan atau bukan milik Anda" }
```

---

#### `POST /api/v1/modul/:id/soal/manual` — Buat soal manual

**Path parameter:** `id` = ID modul.

**Body:**
| Field | Tipe | Wajib | Keterangan |
|---|---|---|---|
| `jenis` | string | ✅ | `harian` / `uts` / `uas` |
| `pertanyaan` | string | ✅ | |
| `pilihan_a` | string | ✅ | |
| `pilihan_b` | string | ✅ | |
| `pilihan_c` | string | ✅ | |
| `pilihan_d` | string | ✅ | |
| `kunci_jawaban` | string | ✅ | `A` / `B` / `C` / `D` (case-insensitive) |

```json
{
  "jenis": "harian",
  "pertanyaan": "Siapakah presiden pertama Indonesia?",
  "pilihan_a": "Soekarno",
  "pilihan_b": "Soeharto",
  "pilihan_c": "Habibie",
  "pilihan_d": "Jokowi",
  "kunci_jawaban": "A"
}
```

**curl:**
```bash
curl -X POST https://momo-be-production.up.railway.app/api/v1/modul/3/soal/manual \
  -H "Content-Type: application/json" -H "Authorization: Bearer $TOKEN_GURU" \
  -d '{"jenis":"harian","pertanyaan":"Siapakah presiden pertama Indonesia?","pilihan_a":"Soekarno","pilihan_b":"Soeharto","pilihan_c":"Habibie","pilihan_d":"Jokowi","kunci_jawaban":"A"}'
```

**Response (201):**
```json
{
  "message": "Soal berhasil ditambahkan",
  "data": {
    "id": 46, "modul_id": 3, "jenis": "harian",
    "pertanyaan": "Siapakah presiden pertama Indonesia?",
    "pilihan_a": "Soekarno", "pilihan_b": "Soeharto",
    "pilihan_c": "Habibie", "pilihan_d": "Jokowi",
    "created_at": "..."
  }
}
```

**Error (400 / 403):**
```json
{ "error": "Semua field (jenis, pertanyaan, pilihan A-D, kunci jawaban) wajib diisi" }
{ "error": "jenis soal wajib salah satu dari: harian, uts, uas" }
{ "error": "kunci jawaban wajib salah satu dari: A, B, C, atau D" }
{ "error": "akses ditolak: modul tidak ditemukan atau bukan milik Anda" }
```

---

#### `PUT /api/v1/soal/:id` — Update soal

⚠️ Path parameter-nya `id` soal (bukan id modul). Validasi kepemilikan otomatis dilakukan (soal harus milik modul milik guru ini).

**Body:**
| Field | Tipe | Wajib | Keterangan |
|---|---|---|---|
| `pertanyaan` | string | ✅ | |
| `pilihan_a` | string | ✅ | |
| `pilihan_b` | string | ✅ | |
| `pilihan_c` | string | ✅ | |
| `pilihan_d` | string | ✅ | |
| `kunci_jawaban` | string | ✅ | `A` / `B` / `C` / `D` |
| `jenis` | string | ❌ | `harian` / `uts` / `uas` (jika kosong → jenis tidak berubah) |

```json
{
  "jenis": "uts",
  "pertanyaan": "Siapakah presiden pertama Indonesia? (direvisi)",
  "pilihan_a": "Soekarno",
  "pilihan_b": "Mohammad Hatta",
  "pilihan_c": "Sjahrir",
  "pilihan_d": "Tan Malaka",
  "kunci_jawaban": "A"
}
```

**curl:**
```bash
curl -X PUT https://momo-be-production.up.railway.app/api/v1/soal/46 \
  -H "Content-Type: application/json" -H "Authorization: Bearer $TOKEN_GURU" \
  -d '{"jenis":"uts","pertanyaan":"...(direvisi)","pilihan_a":"Soekarno","pilihan_b":"Mohammad Hatta","pilihan_c":"Sjahrir","pilihan_d":"Tan Malaka","kunci_jawaban":"A"}'
```

**Response (200):**
```json
{
  "message": "Soal berhasil diperbarui",
  "data": { "id": 46, "modul_id": 3, "jenis": "uts", "pertanyaan": "...(direvisi)", "pilihan_a": "...", ... }
}
```

**Error (400):**
```json
{ "error": "Semua field (pertanyaan, pilihan A-D, kunci jawaban) wajib diisi" }
{ "error": "akses ditolak: soal ini bukan milik Anda" }
{ "error": "soal tidak ditemukan" }
```

---

#### `DELETE /api/v1/soal/:id` — Hapus soal

**curl:**
```bash
curl -X DELETE https://momo-be-production.up.railway.app/api/v1/soal/46 \
  -H "Authorization: Bearer $TOKEN_GURU"
```

**Response (200):**
```json
{ "message": "Soal berhasil dihapus" }
```

**Error (400):**
```json
{ "error": "akses ditolak: soal ini bukan milik Anda" }
{ "error": "soal tidak ditemukan" }
```

---

### ⚠️ `GET /api/v1/modul/:id/soal?jenis=uts` — Ambil soal (KHUSUS TOKEN SISWA)

🚫 **TIDAK BOLEH dipanggil dari halaman Guru.** Halaman guru memakai `GET /modul/:id` lalu memfilter array `.soal`.

- **Auth:** **Token Siswa** (siswa harus sudah join kelas yang tertaut modul ini)

**Path parameter:**
| Param | Tipe | Keterangan |
|---|---|---|
| `id` | number | ID modul yang sudah di-assign ke kelas siswa |

**Query parameter:**
| Param | Wajib | Nilai valid |
|---|---|---|
| `jenis` | ✅ | `harian` / `uts` / `uas` |

**curl:**
```bash
curl "https://momo-be-production.up.railway.app/api/v1/modul/3/soal?jenis=uts" \
  -H "Authorization: Bearer $TOKEN_SISWA"
```

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

**Error (400):**
```json
{ "error": "Query param 'jenis' wajib salah satu dari: harian, uts, uas" }
{ "error": "ID modul tidak valid" }
```

**Error (401):** token guru dipakai di endpoint ini
```json
{ "error": "Endpoint ini khusus siswa. Token Anda bukan token siswa.", "code": "TOKEN_WRONG_ROLE" }
```

**Error (403):** modul belum ditugaskan ke kelas siswa
```json
{ "error": "modul ini tidak ditugaskan untuk kelas Anda" }
```

**Error (404):** soal belum ada / belum selesai diproses
```json
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

```json
{ "nama": "Kelas 5A", "mata_pelajaran": "Matematika" }
```

**curl:**
```bash
curl -X POST https://momo-be-production.up.railway.app/api/v1/kelas \
  -H "Content-Type: application/json" -H "Authorization: Bearer $TOKEN_GURU" \
  -d '{"nama":"Kelas 5A","mata_pelajaran":"Matematika"}'
```

**Response (201):**
```json
{ "id": 3, "guru_id": 3, "nama_kelas": "Kelas 5A", "mata_pelajaran": "Matematika", "kode_kelas": "487137", "created_at": "...", "updated_at": "..." }
```
⚠️ Asimetri: request `nama` → response `nama_kelas`.
📡 Memicu event SSE `kelas-created`.

**Error (400):**
```json
{ "error": "Nama dan Mata Pelajaran wajib diisi" }
```

---

### `GET /api/v1/kelas` — List kelas (dengan pagination)

- **Auth:** Token Guru

**Query parameter:**
| Param | Default | Keterangan |
|---|---|---|
| `page` | `1` | Nomor halaman |
| `limit` | `10` | Data per halaman (maks 50) |

**curl:**
```bash
curl -H "Authorization: Bearer $TOKEN_GURU" \
  "https://momo-be-production.up.railway.app/api/v1/kelas?page=1&limit=10"
```

**Response (200):** setiap item kelas menyertakan preload `siswa` dan `modul`
```json
{
  "data": [
    {
      "id": 3, "guru_id": 3, "nama_kelas": "Kelas 5A", "mata_pelajaran": "Matematika",
      "kode_kelas": "487137", "siswa": [ ... ], "modul": [ ... ],
      "created_at": "...", "updated_at": "..."
    }
  ],
  "meta": { "page": 1, "limit": 10, "total": 3, "total_page": 1 }
}
```
⚠️ Response berbentuk object `{ data, meta }` — FE wajib membaca `response.data`.

---

### `GET /api/v1/kelas/:id` — Detail kelas

- **Auth:** Token Guru

**Path parameter:** `id` = ID kelas.

**curl:**
```bash
curl -H "Authorization: Bearer $TOKEN_GURU" \
  https://momo-be-production.up.railway.app/api/v1/kelas/3
```

**Response (200):**
```json
{
  "id": 3, "guru_id": 3, "nama_kelas": "Kelas 5A", "mata_pelajaran": "Matematika", "kode_kelas": "487137",
  "siswa": [ { "id": 1, "kelas_id": 3, "nama": "Siswa Retest", "created_at": "..." } ],
  "modul": [ { "id": 2, "guru_id": 3, "nama": "Modul Retest", "deskripsi": "...", "created_at": "...", "updated_at": "..." } ],
  "created_at": "...", "updated_at": "..."
}
```

**Error (404):**
```json
{ "error": "kelas tidak ditemukan atau Anda tidak memiliki akses" }
```

---

### `PUT /api/v1/kelas/:id` — Update kelas

- **Auth:** Token Guru
- **Content-Type:** `application/json`

**Body:** (kirim hanya field yang diubah)
| Field | Tipe | Wajib |
|---|---|---|
| `nama` | string | ❌ |
| `mata_pelajaran` | string | ❌ |

```json
{ "nama": "Kelas 5A Updated", "mata_pelajaran": "Matematika Lanjut" }
```

**curl:**
```bash
curl -X PUT https://momo-be-production.up.railway.app/api/v1/kelas/3 \
  -H "Content-Type: application/json" -H "Authorization: Bearer $TOKEN_GURU" \
  -d '{"nama":"Kelas 5A Updated","mata_pelajaran":"Matematika Lanjut"}'
```

**Response (200):**
```json
{
  "message": "Kelas berhasil diperbarui",
  "data": { "id": 3, "guru_id": 3, "nama_kelas": "Kelas 5A Updated", "mata_pelajaran": "Matematika Lanjut", "kode_kelas": "487137", "created_at": "...", "updated_at": "..." }
}
```
📡 Memicu event SSE `kelas-updated`.

**Error (400):**
```json
{ "error": "kelas tidak ditemukan atau Anda tidak memiliki akses" }
```

---

### `DELETE /api/v1/kelas/:id` — Hapus kelas

- **Auth:** Token Guru

**curl:**
```bash
curl -X DELETE https://momo-be-production.up.railway.app/api/v1/kelas/3 \
  -H "Authorization: Bearer $TOKEN_GURU"
```

**Response (200):**
```json
{ "message": "Kelas berhasil dihapus" }
```
📡 Memicu event SSE `kelas-deleted`.

**Error (400):**
```json
{ "error": "kelas tidak ditemukan atau Anda tidak memiliki akses" }
```

---

### `POST /api/v1/kelas/:id/siswa` — Daftarkan siswa ke kelas

- **Auth:** Token Guru
- **Content-Type:** `application/json`

**Body:**
| Field | Tipe | Wajib |
|---|---|---|
| `nama` | string | ✅ |

```json
{ "nama": "Siswa Retest" }
```

**curl:**
```bash
curl -X POST https://momo-be-production.up.railway.app/api/v1/kelas/3/siswa \
  -H "Content-Type: application/json" -H "Authorization: Bearer $TOKEN_GURU" \
  -d '{"nama":"Siswa Retest"}'
```

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
- **Content-Type:** `application/json`

**Body:**
| Field | Tipe | Wajib |
|---|---|---|
| `modul_id` | number | ✅ |

**Response (200):**
```json
{ "message": "Modul berhasil ditautkan ke kelas" }
```

**Error (400):** modul bukan milik guru / sudah tertaut / tidak ditemukan.

---

### `DELETE /api/v1/kelas/:id/modul/:modul_id` — Lepas modul dari kelas

- **Auth:** Token Guru

Melepas tautan **tanpa** menghapus modul dari akun guru.

**Response (200):**
```json
{ "message": "Modul berhasil dihapus dari kelas" }
```

---

### `GET /api/v1/kelas/:id/nilai` — Rekap nilai

- **Auth:** Token Guru

**Query parameter:**
| Param | Wajib | Keterangan |
|---|---|---|
| `modul_id` | ✅ | ID modul yang direkap |
| `jenis` | ❌ | `harian` / `uts` / `uas` (jika kosong = semua jenis) |

**curl:**
```bash
curl -H "Authorization: Bearer $TOKEN_GURU" \
  "https://momo-be-production.up.railway.app/api/v1/kelas/3/nilai?modul_id=3&jenis=uts"
```

**Response (200):**
```json
[ { "siswa_id": 1, "nama": "Siswa Retest", "jumlah_soal_dijawab": 1, "jumlah_benar": 1, "skor_persen": 100 } ]
```
*(Array kosong `[]` jika belum ada siswa mengerjakan.)*

---

## D. Alur Siswa (Token Siswa)

### `GET /api/v1/modul/:id/soal?jenis=uts`
Lihat spesifikasi lengkap di bagian B (endpoint khusus siswa). Ringkasan: mengembalikan soal dengan pilihan teracak, tanpa kunci jawaban.

### `POST /api/v1/submit-jawaban` — Submit jawaban (dinilai AI)

- **Auth:** Token Siswa
- **Content-Type:** `application/json`
- **Rate limit:** AI limiter

**Body:**
| Field | Tipe | Wajib | Keterangan |
|---|---|---|---|
| `soal_id` | number | ✅ | |
| `jawaban_mentah` | string | ✅ | Teks bebas hasil Speech-to-Text, tidak harus huruf A-D |

⚠️ `siswa_id` **tidak dikirim** — diambil dari token (mencegah menjawab atas nama siswa lain).

```json
{ "soal_id": 1, "jawaban_mentah": "aku rasa jawabannya yang B" }
```

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

**Error (format standar 18 Sept):**
```json
{ "code": "NOT_FOUND", "message": "soal dengan ID 999999 tidak ditemukan", "source": "client" }
{ "code": "ALREADY_ANSWERED", "message": "Soal UTS/UAS hanya boleh dijawab sekali", "source": "client" }
{ "code": "AI_UNAVAILABLE", "message": "Gagal menghubungi AI Service untuk mengevaluasi jawaban. Silakan coba lagi.", "source": "ai_service" }
```
*(Berlaku untuk jenis `uts`/`uas`: satu siswa satu kali per soal. Jenis `harian` boleh berulang.)*

---

## E. Alur Belajar Siswa

Endpoint ini khusus **Token Siswa** dan dirancang untuk mendukung alur belajar interaktif berbasis suara (voice-first) untuk anak tuna netra.

### `GET /api/v1/siswa/kelas-saya` — Info kelas & modul yang ditugaskan

Endpoint ini dipanggil setelah siswa berhasil join kelas, untuk mengetahui modul apa saja yang tersedia dan apakah menyediakan materi/soal.

**curl:**
```bash
curl -H "Authorization: Bearer $TOKEN_SISWA" \
  https://momo-be-production.up.railway.app/api/v1/siswa/kelas-saya
```

**Response (200):**
```json
{
  "kelas": {
    "id": 3,
    "nama_kelas": "Kelas 5A",
    "mata_pelajaran": "Matematika",
    "kode_kelas": "487137"
  },
  "modul": [
    {
      "id": 3,
      "nama": "Ipa",
      "deskripsi": "Ilmu Pengetahuan Alam untuk Kelas 5",
      "punya_materi": true,
      "jenis_soal_tersedia": ["harian", "uts"]
    },
    {
      "id": 5,
      "nama": "Matematika",
      "deskripsi": "...",
      "punya_materi": false,
      "jenis_soal_tersedia": ["uas"]
    }
  ]
}
```

**Kegunaan untuk FE:**
- Field `punya_materi` → AI bisa bilang *"Kelas ini menyediakan materi"* atau *"Kelas ini belum ada materi"*
- Field `jenis_soal_tersedia` → AI bisa bilang *"Kelas ini menyediakan soal harian dan UTS"* atau *"Kelas ini hanya menyediakan soal UAS"*

**Error (403):**
```json
{ "code": "FORBIDDEN", "message": "akses ditolak: siswa ini bukan anggota kelas", "source": "client" }
```

---

### `GET /api/v1/siswa/modul/:id` — Detail modul untuk siswa 🆕 (18 Sept)

Endpoint ini mengambil detail modul yang **sudah ditugaskan ke kelas siswa**. Berguna untuk header halaman belajar.

**Path parameter:**
| Param | Tipe | Keterangan |
|---|---|---|
| `id` | number | ID modul (harus tertaut ke kelas siswa) |

**curl:**
```bash
curl -H "Authorization: Bearer $TOKEN_SISWA" \
  https://momo-be-production.up.railway.app/api/v1/siswa/modul/3
```

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

**Error (400 / 403):**
```json
{ "code": "INVALID_FORMAT", "message": "ID modul tidak valid", "source": "client" }
{ "code": "FORBIDDEN", "message": "modul ini tidak ditugaskan untuk kelas Anda", "source": "client" }
```

---

### `GET /api/v1/siswa/modul/:id/materi` — Ambil materi untuk dibacakan TTS

Endpoint ini mengambil list materi dalam modul yang **sudah ditugaskan ke kelas siswa**. Validasi ketat: modul harus tertaut ke kelas, kalau tidak ditolak.

**Path parameter:**
| Param | Tipe | Keterangan |
|---|---|---|
| `id` | number | ID modul (harus tertaut ke kelas siswa) |

**curl:**
```bash
curl -H "Authorization: Bearer $TOKEN_SISWA" \
  https://momo-be-production.up.railway.app/api/v1/siswa/modul/3/materi
```

**Response (200):**
```json
{
  "jumlah": 53,
  "data": [
    {
      "id": 73,
      "modul_id": 3,
      "urutan": 1,
      "judul": "Pengantar Buku Ilmu Pengetahuan Alam dan Sosial Kelas V",
      "konten": "Buku Ilmu Pengetahuan Alam dan Sosial (IPAS) untuk SD/MI Kelas V...",
      "created_at": "..."
    }
  ]
}
```
⚠️ Field `konten` berisi teks lengkap yang siap dibacakan oleh Text-to-Speech (TTS) di FE.

**Error (400 / 403):**
```json
{ "code": "INVALID_FORMAT", "message": "ID modul tidak valid", "source": "client" }
{ "code": "FORBIDDEN", "message": "modul ini tidak ditugaskan untuk kelas Anda", "source": "client" }
```

---

### `GET /api/v1/siswa/modul/:id/soal?jenis=harian` — Alias soal untuk siswa 🆕 (18 Sept)

Sama persis dengan `GET /api/v1/modul/:id/soal` (bagian B) — disediakan agar seluruh alur belajar siswa konsisten memakai prefix `/siswa/`.

**curl:**
```bash
curl -H "Authorization: Bearer $TOKEN_SISWA" \
  "https://momo-be-production.up.railway.app/api/v1/siswa/modul/3/soal?jenis=harian"
```

**Response (200):** identik dengan `GET /modul/:id/soal` (pilihan teracak, tanpa kunci).

---

## E2. Mode Tutor (18 Sept 2026) 🆕

### `POST /api/v1/tutor` — Kirim pesan ke tutor (ASYNC fire-and-forget)

- **Auth:** Token Siswa
- **Content-Type:** `application/json`
- **Rate limit:** AI limiter (3 req / 6 detik per IP)

**Body:**
| Field | Tipe | Wajib | Keterangan |
|---|---|---|---|
| `pesan` | string | ✅ | Teks hasil Speech-to-Text dari suara siswa |
| `kelas_nama` | string | ❌ | Nama kelas untuk personalisasi konteks |

```json
{ "pesan": "Halo Momo, apa itu getaran dalam fisika?", "kelas_nama": "Kelas 3 Matematika" }
```

**curl:**
```bash
curl -X POST https://momo-be-production.up.railway.app/api/v1/tutor \
  -H "Authorization: Bearer $TOKEN_SISWA" \
  -H "Content-Type: application/json" \
  -d '{"pesan":"Halo Momo, apa itu getaran dalam fisika?","kelas_nama":"Kelas 3 Matematika"}'
```

**Response (202 Accepted, < 1 detik):**
```json
{
  "message": "Permintaan tutor diterima, balasan akan dikirim via stream",
  "job_id": "tutor_1789721312376790988_2"
}
```

⚠️ **Pola async:** response 202 hanya ACK. Balasan AI TIDAK ada di response ini — ia datang sebagai event `tutor-reply` di stream (5–30 detik kemudian). FE wajib sudah terhubung ke stream sebelum/sesudah mengirim request.

**Error (format standar `{code, message, source}`):**
```json
{ "code": "MISSING_FIELD", "message": "Field 'pesan' wajib diisi", "source": "client" }
{ "code": "UNAUTHORIZED", "message": "Token siswa tidak valid. Silakan login kembali.", "source": "client" }
{ "code": "AI_UNAVAILABLE", "message": "AI tutor sedang tidak tersedia. Silakan coba lagi dalam beberapa saat.", "source": "ai_service" }
```

### Karakter balasan AI tutor (jaminan backend + AI Service)

- Maksimal **80 kata** (~400 karakter) — aman untuk TTS
- **TTS-safe:** tanpa markdown, bullet, emoji, atau simbol kompleks
- Bahasa Indonesia natural, ramah, memakai analogi auditori/taktil (cocok untuk siswa tuna netra)
- Contoh balasan nyata dari production:
  > "Halo! Senang sekali bisa berkenalan dan belajar bareng kamu. Getaran adalah gerakan bolak-balik suatu benda secara teratur melalui titik setimbangnya. Kamu bisa merasakannya saat menempelkan jemari di lehermu sewaktu berbicara, ada gerakan bolak-balik cepat yang terasa di sana."

### Alur FE lengkap (Mode Tutor)

```javascript
// 1. User bicara → STT → teks
const teks = await speechToText();

// 2. Kirim ke tutor → ACK < 1 detik
const res = await fetch(`${BASE}/tutor`, {
  method: 'POST',
  headers: {
    'Authorization': `Bearer ${token}`,
    'Content-Type': 'application/json',
  },
  body: JSON.stringify({ pesan: teks, kelas_nama: namaKelas }),
});

if (res.status === 202) {
  setLoading(true);              // "Sedang Memahami..."
  setFallbackTimer(35000);       // watchdog: kalau 35s tidak ada event → error message
  return;                        // balasan ditunggu dari stream, bukan dari sini
}

// 3. Handle error (401/400/503) → WAJIB setLoading(false) di sini
handleError(await res.json());

// 4. Di stream listener (dipasang sekali saat halaman study dibuka):
es.addEventListener('tutor-reply', (e) => {
  const d = JSON.parse(e.data);
  clearFallbackTimer();
  setLoading(false);             // ← stop "Sedang Memahami..."
  speak(d.balasan);              // TTS bacakan balasan
});
```

**Watchdog wajib:** kalau event `tutor-reply` tidak datang dalam 35 detik, stop loading dan ucapkan "Momo terlalu lama merespons, coba lagi ya." Jangan biarkan UI stuck selamanya.

---

## F. SSE Legacy: Stream Kelas (Token Guru)

### `GET /api/v1/kelas/stream` — Stream event kelas

- **Auth:** Token Guru

Koneksi streaming yang tetap terbuka; server mengirim event setiap ada perubahan data kelas.

**Headers wajib:**
```http
Authorization: Bearer <token_guru>
```

**Contoh output mentah (curl nyata):**
```text
event: connected
data: {"message":"Terhubung ke stream kelas","time":"2026-09-11T04:16:26Z"}

: heartbeat

event: kelas-created
data: {"id":10,"guru_id":7,"nama_kelas":"Kelas SSE Test","mata_pelajaran":"Fisika","kode_kelas":"372475","created_at":"...","updated_at":"..."}
```

**Daftar event:**
| Event | Kapan dikirim | Isi `data` |
|---|---|---|
| `connected` | Koneksi pertama dibuka | `{ "message", "time" }` |
| `kelas-created` | Kelas baru dibuat | Object kelas lengkap |
| `kelas-updated` | Kelas di-update | Object kelas lengkap |
| `kelas-deleted` | Kelas dihapus | `{ "id": <id_kelas> }` |
| `: heartbeat` | Tiap ±15 detik | Komentar kosong — **abaikan** |

⚠️ Untuk kebutuhan real-time baru (materi/soal/jawaban/tutor), **gunakan Unified Stream di bagian G** — endpoint legacy ini hanya memuat event kelas.

**curl test:**
```bash
# Terminal 1
curl -N -H "Authorization: Bearer $TOKEN_GURU" \
  https://momo-be-production.up.railway.app/api/v1/kelas/stream
# Terminal 2
curl -X POST https://momo-be-production.up.railway.app/api/v1/kelas \
  -H "Content-Type: application/json" -H "Authorization: Bearer $TOKEN_GURU" \
  -d '{"nama":"Kelas SSE Test","mata_pelajaran":"Fisika"}'
```

---

## G. Unified Stream (Guru + Siswa)

### Overview

| | |
|---|---|
| **Endpoint** | `GET /api/v1/stream` |
| **Method** | GET (Server-Sent Events) |
| **Auth** | Token Guru atau Siswa via **header** ATAU **query param** (baru 18 Sept) |
| **Scope** | Role-aware: event difilter berdasarkan role token |

Satu koneksi stream untuk semua update async. Menggantikan kebutuhan polling.

### Authentication (2 cara)

**Cara 1 — Header** (untuk fetch-based client):
```http
Authorization: Bearer <token_guru_atau_siswa>
```

**Cara 2 — Query param** 🆕 (18 Sept 2026, untuk `EventSource` native yang tidak support header):
```
GET /api/v1/stream?token=<token_guru_atau_siswa>
```

Sumber token:
- **Guru:** `POST /api/v1/guru/login` → `.data.token`
- **Siswa:** `POST /api/v1/join` → `.token`

Response tanpa token / token invalid: `401` dengan body `{"error":"...","code":"TOKEN_MISSING"|"TOKEN_INVALID"}`

### Event Catalog

#### 1. `connected` — Welcome Event
- **Trigger:** saat client pertama kali connect
- **Scope:** semua role

```text
event: connected
data: {"message":"Terhubung ke stream Momo","role":"siswa","time":"2026-09-18T08:48:25Z"}
```

#### 2. `materi-ready`
- **Trigger:** setelah guru upload PDF materi dan AI selesai ekstraksi
- **Scope:** guru only

```text
event: materi-ready
data: {"modul_id":3,"jumlah":5}
```
**FE action:** refresh list materi untuk `modul_id` tersebut.

#### 3. `soal-ready`
- **Trigger:** setelah guru upload PDF soal dan AI selesai ekstraksi
- **Scope:** guru only

```text
event: soal-ready
data: {"modul_id":3,"jenis":"uts","jumlah":10}
```
**FE action:** refresh list soal untuk `modul_id` + `jenis` tersebut.

#### 4. `jawaban-submitted`
- **Trigger:** setelah siswa submit jawaban dan AI selesai evaluasi
- **Scope:** guru only

```text
event: jawaban-submitted
data: {"siswa_id":42,"soal_id":88,"benar":true}
```
**FE action:** update statistik/rekap nilai real-time (opsional).

#### 5. `tutor-reply` 🆕 (18 Sept 2026)
- **Trigger:** setelah AI tutor selesai membalas pesan siswa (lihat `POST /api/v1/tutor`)
- **Scope:** **siswa yang meminta saja** (targeted per siswa, BUKAN broadcast ke semua siswa)

```text
event: tutor-reply
data: {"job_id":"tutor_1789721312376790988_2","balasan":"Halo! Getaran adalah gerakan bolak-balik suatu benda..."}
```

Varian gagal (AI error):
```text
event: tutor-reply
data: {"job_id":"tutor_xxx","balasan":"Maaf, aku sedang mengalami kesulitan. Silakan coba lagi.","error":true}
```
**FE action:** stop loading "Sedang Memahami...", bacakan `balasan` dengan TTS.

#### 6. `heartbeat` — Keep-Alive
- **Trigger:** setiap 15 detik
- **Scope:** semua role

```text
: heartbeat
```
**FE action:** tidak perlu apa-apa; ini comment SSE.

### Ringkasan Event per Role

| Event | Guru | Siswa |
|---|:---:|:---:|
| `connected` | ✅ | ✅ |
| `heartbeat` | ✅ | ✅ |
| `materi-ready` | ✅ | ❌ |
| `soal-ready` | ✅ | ❌ |
| `jawaban-submitted` | ✅ | ❌ |
| `tutor-reply` | ❌ | ✅ (hanya siswa yang meminta) |

### Client Implementation

#### Opsi 1 — EventSource native + query param (PALING MUDAH) 🆕

```javascript
const token = localStorage.getItem('token');
const es = new EventSource(
  `https://momo-be-production.up.railway.app/api/v1/stream?token=${token}`
);

es.addEventListener('connected', e => console.log(JSON.parse(e.data)));
es.addEventListener('tutor-reply', e => {
  const d = JSON.parse(e.data);
  stopLoading();
  speak(d.balasan); // TTS
});
es.addEventListener('soal-ready', e => refreshSoalList(JSON.parse(e.data)));
es.onerror = () => console.warn('stream putus, EventSource auto-reconnect');
```

#### Opsi 2 — Fetch-based client + header (class MomoStream)

```javascript
class MomoStream {
  constructor(token, baseURL = 'https://momo-be-production.up.railway.app/api/v1') {
    this.token = token;
    this.baseURL = baseURL;
    this.reconnectAttempts = 0;
    this.maxReconnectAttempts = 5;
    this.reconnectDelay = 1000;
    this.listeners = {};
  }

  async connect() {
    try {
      const response = await fetch(`${this.baseURL}/stream`, {
        headers: {
          'Authorization': `Bearer ${this.token}`,
          'Accept': 'text/event-stream',
        },
      });
      if (!response.ok) throw new Error(`HTTP ${response.status}`);

      const reader = response.body.getReader();
      const decoder = new TextDecoder();
      let buffer = '';

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        buffer += decoder.decode(value, { stream: true });
        const chunks = buffer.split('\n\n');
        buffer = chunks.pop();
        for (const msg of chunks) {
          if (msg.trim()) this.parseMessage(msg);
        }
      }
      this.handleDisconnect();
    } catch (error) {
      this.emit('error', { error: error.message });
      this.handleDisconnect();
    }
  }

  parseMessage(message) {
    let eventType = 'message';
    let data = '';
    for (const line of message.split('\n')) {
      if (line.startsWith('event: ')) eventType = line.slice(7);
      else if (line.startsWith('data: ')) data = line.slice(6);
      else if (line.startsWith(': ')) return; // heartbeat comment
    }
    if (data) {
      try { this.emit(eventType, JSON.parse(data)); }
      catch (e) { console.warn('Unparseable event data:', data); }
    }
  }

  emit(type, data) {
    (this.listeners[type] || []).forEach(cb => cb(data));
  }

  on(type, cb) {
    (this.listeners[type] = this.listeners[type] || []).push(cb);
  }

  handleDisconnect() {
    if (this.reconnectAttempts < this.maxReconnectAttempts) {
      this.reconnectAttempts++;
      const delay = this.reconnectDelay * 2 ** (this.reconnectAttempts - 1);
      setTimeout(() => this.connect(), delay);
    }
  }

  disconnect() {
    this.reconnectAttempts = this.maxReconnectAttempts;
  }
}

// --- Usage ---
const stream = new MomoStream(localStorage.getItem('token'));

stream.on('connected', d => console.log('Connected:', d));
stream.on('soal-ready', d => refreshSoalList(d.modul_id, d.jenis));
stream.on('materi-ready', d => refreshMateriList(d.modul_id));
stream.on('jawaban-submitted', d => updateRekapNilai(d));
stream.on('tutor-reply', d => {
  stopLoading();
  speak(d.balasan);
});
stream.on('error', d => console.error('Stream error:', d.error));

stream.connect();
```

### Perilaku Koneksi

| Aspek | Nilai |
|---|---|
| Heartbeat interval | 15 detik |
| Reconnect strategy | Exponential backoff (1s → 2s → 4s → 8s → 16s) |
| Max reconnect attempts | 5 |
| Buffer event per client | 50 event |
| Latency tutor-reply | 5–30 detik setelah POST /tutor |

### Notes

- Stream tidak mengirim riwayat event lama — hanya event setelah client connect.
- Untuk data awal (initial load), tetap pakai endpoint REST; stream hanya untuk update real-time.
- `tutor-reply` di-route per `siswa_id`: dua siswa berbeda tidak saling menerima balasan tutor masing-masing.

---

## H. Lampiran: Endpoint Debug

### `POST /api/v1/test-extract-pdf` — Uji ekstraksi PDF (debug only)

- **Auth:** Publik
- **Content-Type:** `multipart/form-data`
- **Rate limit:** AI limiter

Endpoint utilitas untuk menguji ekstraksi teks PDF **tanpa menyimpan ke database**. **Tidak dipakai oleh FE produksi** — bentuk response mengikuti output AI Service dan dapat berubah sewaktu-waktu.

---

## Error Handling

### HTTP Status Codes

| Code | Arti | Contoh kasus |
|---|---|---|
| 200 | OK | GET, PUT, DELETE berhasil; join siswa; upload materi/soal sync sukses |
| 201 | Created | register, buat modul/kelas/siswa/materi, submit jawaban |
| 202 | Accepted | POST /tutor (ACK fire-and-forget) |
| 302 | Found | redirect verify-email |
| 400 | Bad Request | validasi gagal, nama duplikat, file terlalu besar/bukan PDF, bukan pemilik resource |
| 401 | Unauthorized | token hilang/salah/kedaluwarsa/wrong role, login gagal, kode kelas salah |
| 403 | Forbidden | resource bukan milik user, modul belum ditugaskan ke kelas siswa |
| 404 | Not Found | resource tidak ada / bukan milik user |
| 422 | Unprocessable Entity | proses AI gagal (materi/soal) |
| 429 | Too Many Requests | rate limit terlampaui |
| 500 | Internal Server Error | kesalahan tak terduga di server |
| 503 | Service Unavailable | AI Service error (tutor, evaluate) |

### Error Codes Standar (field `code` + `source`) 🆕

**Client errors (4xx) — `source: "client"`:**
| Code | HTTP | Kapan muncul | Saran FE |
|---|---|---|---|
| `INVALID_REQUEST` | 400 | Body tidak valid | Tampilkan message |
| `MISSING_FIELD` | 400 | Field wajib kosong | Highlight field |
| `INVALID_FORMAT` | 400 | Format salah (ID, jenis) | Tampilkan message |
| `INVALID_FILE` | 400 | File bukan PDF / > batas | Tampilkan batas |
| `UNAUTHORIZED` | 401 | Token hilang/salah/expired | Redirect join/login |
| `FORBIDDEN` | 403 | Bukan pemilik / modul tidak ditugaskan | Tampilkan message |
| `NOT_FOUND` | 404 | Resource tidak ada | Tampilkan message |
| `DUPLICATE` | 409 | Nama siswa sudah ada di kelas | Tampilkan message |
| `ALREADY_ANSWERED` | 409 | Soal UTS/UAS sudah dijawab | Lanjut soal berikutnya |

**AI Service errors (5xx) — `source: "ai_service"`:**
| Code | HTTP | Kapan muncul | Saran FE |
|---|---|---|---|
| `AI_PROCESSING_FAILED` | 503 | AI gagal ekstrak/evaluasi | "Coba lagi atau ganti PDF" |
| `AI_TIMEOUT` | 503 | AI terlalu lama | Retry 1x, lalu message |
| `AI_UNAVAILABLE` | 503 | AI Service down / reject | "AI sibuk, coba lagi nanti" + retry backoff |
| `AI_INVALID_RESPONSE` | 503 | Response AI tidak dikenali | Message generic |

**Server errors (5xx) — `source: "server"`:**
| Code | HTTP | Saran FE |
|---|---|---|
| `INTERNAL_ERROR` | 500 | Message generic + log |
| `DATABASE_ERROR` | 500 | Message generic + log |

**Pattern penanganan universal:**
```javascript
if (!res.ok) {
  const err = await res.json();
  setLoading(false);                       // WAJIB di semua path
  if (err.source === 'ai_service') showToast('AI sedang sibuk, coba lagi nanti', 'warning');
  else if (err.source === 'client') showToast(err.message, 'error');
  else showToast('Terjadi kesalahan sistem, coba lagi', 'error');
  if (res.status === 401) restartOnboarding(); // token mati → alur join ulang
  return;
}
```

### Error Codes Lama (masih dipakai di middleware stream & sebagian endpoint guru)

| Code | Kapan muncul | Saran penanganan FE |
|---|---|---|
| `TOKEN_MISSING` | Header Authorization tidak ada | Redirect login |
| `TOKEN_INVALID_FORMAT` | Format header bukan `Bearer <token>` | Perbaiki pengiriman header |
| `TOKEN_INVALID` | Token tidak valid / kedaluwarsa | Hapus token, redirect login ("Sesi Anda telah berakhir...") |
| `TOKEN_WRONG_ROLE` | Token guru di endpoint siswa atau sebaliknya | Bug FE: samakan jenis token dengan halaman; redirect ke halaman yang benar |
| `FILE_TOO_LARGE` | Upload melebihi batas (materi 25MB / soal 5MB) | Tampilkan batas ukuran |
| `INVALID_FILE_TYPE` | File bukan `.pdf` | Tampilkan "hanya PDF yang didukung" |
| `MATERI_PROCESSING_FAILED` | AI gagal merangkum materi | Tampilkan field `error` apa adanya |
| `SOAL_PROCESSING_FAILED` | AI gagal mengekstrak soal | Tampilkan field `error` apa adanya |

---

## Known Issues / Catatan untuk FE

1. **`nama` vs `nama_kelas`** di endpoint Kelas: request pakai `nama`, response mengembalikan `nama_kelas`.
2. **Lokasi token berbeda**: login guru `data.token`, join siswa `.token` (root).
3. **`judul` vs `nama`** untuk modul: `GET /modul/:id` memakai `judul`, endpoint lain memakai `nama`.
4. **`kunci_jawaban` tidak pernah dikirim** ke endpoint manapun — keputusan keamanan.
5. **Object materi tidak punya `updated_at`** — hanya `created_at`.
6. **Materi & Soal keduanya SYNCHRONOUS** (14 Sept): request ditahan 10–60 detik sampai AI selesai; FE wajib spinner + render array `data` (bukan `message`). **HAPUS semua logic polling untuk upload soal.**
7. **`GET /kelas` berbentuk `{ data, meta }`** karena pagination — baca `response.data`.
8. **Role isolation aktif**: token guru ≠ endpoint siswa, dan sebaliknya (`TOKEN_WRONG_ROLE`). Halaman guru pakai `GET /modul/:id` untuk preview soal; halaman siswa pakai `GET /modul/:id/soal` atau `/siswa/modul/:id/soal`.
9. **Pilihan jawaban diacak backend** per request pada `GET /modul/:id/soal` — FE tidak perlu mengacak lagi.
10. **`PUT /modul/:id` menimpa deskripsi selalu**: mengirim hanya `nama` akan mengosongkan deskripsi. Kirim keduanya untuk aman.
11. **Response `PUT /modul/:id` menyertakan preload materi & soal** (payload besar) — jangan render langsung sebagai list, ambil field yang diperlukan saja.
12. **Materi dua sumber** (AI & manual) bercampur di `GET /modul/:id/materi`, terurut `urutan` ASC.
13. **Path CRUD materi berbeda**: list scoped modul (`/modul/:id/materi`), update/delete scoped materi (`/materi/:id`).
14. **Delete modul = cascade**: semua materi & soal di dalamnya ikut terhapus — FE wajib konfirmasi ganda.
15. **Submit jawaban `uts`/`uas` sekali per siswa per soal**; `harian` boleh berulang.
16. **Registrasi guru memicu email verifikasi**; link memanggil `GET /guru/verify-email` yang me-redirect ke FE dengan `?status=success|error`.
17. **SSE heartbeat** (`: heartbeat`) adalah komentar — parser FE wajib mengabaikannya.
18. **`/test-extract-pdf` hanya untuk debug** — jangan dipakai di alur produksi.
19. **Timeout fetch untuk upload soal/materi**: set > 120 detik atau matikan timeout, karena proses AI bisa memakan waktu 30–60 detik untuk PDF besar.
20. **CRUD Soal Manual tersedia** (14 Sept): `POST /modul/:id/soal/manual`, `GET /modul/:id/soal/list`, `PUT /soal/:id`, `DELETE /soal/:id`.
21. **`kunci_jawaban` tidak pernah bocor** di response manapun, termasuk saat create/update soal manual. Saat edit, FE perlu minta user memasukkan ulang kunci jawaban (tidak bisa ditampilkan nilai saat ini).
22. **Soal manual & hasil AI bercampur** di `GET /modul/:id/soal/list` — FE bisa menandai soal AI vs manual berdasarkan field `created_at` atau menambahkan flag UI jika perlu.
23. **Auto-register siswa** (14 Sept update 4): `POST /join` sekarang otomatis membuat siswa baru jika nama belum terdaftar. Guru tetap bisa lihat daftar siswa real-time via SSE.
24. **Endpoint siswa baru** (14 Sept update 4): `GET /siswa/kelas-saya`, `GET /siswa/modul/:id/materi`.
25. **AI bisa tahu modul mana yang punya materi/soal** dari field `punya_materi` dan `jenis_soal_tersedia` di response `GET /siswa/kelas-saya`.
26. 🆕 **Stream support token via query param** `?token=` (18 Sept) — solusi untuk `EventSource` native yang tidak bisa kirim header Authorization.
27. 🆕 **`tutor-reply` targeted per siswa** — hanya siswa yang mengirim pesan yang menerima balasannya; jangan broadcast ke semua siswa di FE.
28. 🆕 **Dua format error hidup bersamaan** (18 Sept): format standar `{code, message, source}` di endpoint bisnis, format lama `{error, code}` di middleware stream. Cek field `source` untuk membedakan.
29. 🆕 **WAJIB reset loading di semua error path** — insiden 18 Sept: UI stuck "Sedang Memahami..." karena error path (token kosong / request gagal) tidak memanggil `setLoading(false)`.
30. 🆕 **Token siswa hilang/expired jangan cuma di-toast** — restart alur onboarding suara (langkah 1–2 plan) atau redirect ke halaman join, karena seluruh alur siswa bergantung pada token dari `POST /join`.
31. 🆕 **Upload soal/materi sekarang parallel** (18 Sept): tipikal 20–30 detik (sebelumnya hingga 60 detik). Timeout fetch tetap > 120 detik untuk PDF besar.
32. 🆕 **Endpoint detail modul siswa** `GET /siswa/modul/:id` tersedia (18 Sept) — pakai ini untuk header halaman belajar, bukan `GET /modul/:id` (yang khusus guru).
33. 🆕 **Mode tutor = 2 langkah**: `POST /tutor` (ACK 202) lalu tunggu event `tutor-reply` di stream; pasang watchdog 35 detik di FE.

---

## Changelog

### 19 September 2026 (update 6)
- 🆕 **Stream tanpa token (public listening mode):** `GET /api/v1/stream` tanpa auth → role `public`, menerima event scope siswa + all (connected, heartbeat, materi-ready, soal-ready, tutor-reply)
- 🔒 Event scope guru (`jawaban-submitted`) tetap guru-only — tidak bocor ke listener publik
- ⚠️ Catatan keamanan: percakapan tutor-reply terlihat oleh listener anonim; wajib-token akan diaktifkan kembali setelah demo

### 18 September 2026 (update 5)
- 🆕 **Mode Tutor (Fase 2 plan alignment):** `POST /api/v1/tutor` — async fire-and-forget, ACK `202` < 1 detik, balasan AI dikirim via stream event `tutor-reply` (targeted per siswa)
- 🆕 **Event `tutor-reply`** di unified stream (scope siswa yang meminta); balasan AI max 80 kata, TTS-safe, analogi auditori/taktil
- 🆕 **Endpoint siswa:** `GET /api/v1/siswa/modul/:id` (detail modul: jumlah materi/soal + jenis soal tersedia) dan `GET /api/v1/siswa/modul/:id/soal` (alias soal siswa)
- 🆕 **Standard error handling:** format `{code, message, source}` dengan 3 source (`client` / `ai_service` / `server`) — FE bisa membedakan "FE salah", "AI salah", "backend salah"
- 🆕 **Stream `/api/v1/stream` menerima token via query param** `?token=` — mendukung `EventSource` native browser
- ⚡ **Parallel chunk processing:** upload soal/materi 47s → ~23s (goroutine per chunk)
- ✅ **AI extraction meningkat:** hingga 20–21 soal per PDF kumpulan soal (sebelumnya 2)
- ✅ Validasi end-to-end tutor: POST /tutor 202 (1.08s) → event tutor-reply diterima stream siswa

### 14 September 2026 (update 4)
- 🔄 **BREAKING:** `POST /join` sekarang **auto-register** siswa baru (nama belum ada = dibuat otomatis, bukan error)
- 🆕 **Endpoint siswa baru:**
  - `GET /api/v1/siswa/kelas-saya` — info kelas + modul + flag ketersediaan materi/soal
  - `GET /api/v1/siswa/modul/:id/materi` — list materi untuk dibacakan TTS (validasi modul tertaut ke kelas)
- ✅ Mendukung alur belajar voice-first untuk anak tuna netra
- ✅ AI bisa tahu modul mana yang punya materi/soal dari field `punya_materi` dan `jenis_soal_tersedia`

### 14 September 2026 (update 3)
- 🆕 **CRUD Soal Manual:** 4 endpoint baru untuk menulis soal tanpa PDF
  - `GET /api/v1/modul/:id/soal/list?jenis=...` — list soal (khusus guru, bisa filter jenis)
  - `POST /api/v1/modul/:id/soal/manual` — buat soal manual
  - `PUT /api/v1/soal/:id` — update soal
  - `DELETE /api/v1/soal/:id` — hapus soal
- ✅ Soal dari AI dan soal manual bercampur rapi dalam satu list
- ✅ Validasi kepemilikan soal (hanya guru pemilik modul yang bisa CRUD)
- ✅ Validasi ketat: pertanyaan, 4 pilihan, dan kunci jawaban (A/B/C/D) wajib
- ✅ `kunci_jawaban` tetap tidak bocor di response (tag `json:"-"`)

### 14 September 2026 (update 2)
- 🔄 **BREAKING:** `POST /modul/:id/soal` sekarang **SYNCHRONOUS** — request ditahan sampai AI selesai (10–30 detik), response `200` berisi array `data` soal asli
- ✅ FE tidak perlu lagi polling setelah upload soal — langsung render hasil dari response
- ✅ Error code baru: `SOAL_PROCESSING_FAILED` untuk kegagalan ekstraksi soal
- ✅ Validasi pipeline soal sync: 2 soal UAS berhasil diekstrak dalam 16.9 detik

### 14 September 2026
- 🔒 **Role isolation aktif** di middleware: token guru di endpoint siswa (dan sebaliknya) → `401 TOKEN_WRONG_ROLE`
- 📄 Dokumentasi: semua blok endpoint dirapikan dengan format konsisten (Auth → Content-Type → Rate limit → Params → Body → curl → Response → Error)
- 📄 Dokumentasi: request spec lengkap untuk endpoint soal (upload & ambil)
- 📄 Dokumentasi: koreksi field materi (tanpa `updated_at`), perilaku `PUT /modul/:id` (deskripsi selalu ditimpa; response menyertakan preload), endpoint `verify-email`, dan lampiran `test-extract-pdf`
- ✅ Validasi pipeline soal: 6 soal UTS berhasil diekstrak dari PDF kumpulan soal

### 12 September 2026 (update 3)
- 🆕 CRUD Modul lengkap: `PUT /modul/:id`, `DELETE /modul/:id` (cascade materi & soal)

### 12 September 2026 (update 2)
- 🆕 CRUD Materi manual: `GET /modul/:id/materi`, `POST /modul/:id/materi/manual`, `PUT /materi/:id`, `DELETE /materi/:id`

### 12 September 2026
- 🔄 BREAKING: `POST /modul/:id/materi` menjadi SYNCHRONOUS (200 + array `data`)
- 🔄 BREAKING ringan: `GET /kelas` berbentuk `{ data, meta }` (pagination)
- ✅ Validasi upload file (materi 25MB / soal 5MB, wajib `.pdf`)
- ✅ Error auth ramah + field `code`; timeout AI 120 detik

### 11 September 2026
- 🆕 CRUD Kelas lengkap + `GET /kelas/stream` (SSE)
- ⚠️ BREAKING: `POST /kelas` wajib `mata_pelajaran`
- ✅ Optimasi performa: index DB, connection pooling, keep-warm Neon

### 1 September 2026
- Dokumentasi awal berdasarkan testing langsung seluruh endpoint existing.