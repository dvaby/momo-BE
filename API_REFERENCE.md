# API Reference — Momo-BE

Dokumen ini disusun berdasarkan **testing langsung terhadap kode yang berjalan** (bukan asumsi).
Versi awal: 1 September 2026 · **Update terakhir: 14 September 2026 (update 4)**.
Semua contoh request/response di bawah adalah hasil `curl` nyata terhadap environment production.

---

## Daftar Isi

- [Info Umum](#info-umum)
- [A. Health, Auth & Verifikasi](#a-health-auth--verifikasi)
- [B. Modul, Materi & Soal](#b-modul-materi--soal)
- [C. Kelas & Nilai (Token Guru)](#c-kelas--nilai-token-guru)
- [D. Alur Siswa (Token Siswa)](#d-alur-siswa-token-siswa)
- [E. Alur Belajar Siswa (14 Sept 2026)](#e-alur-belajar-siswa-14-sept-2026)
- [F. Real-time Updates / SSE (Token Guru)](#f-real-time-updates--sse-token-guru)
- [G. Lampiran: Endpoint Debug](#g-lampiran-endpoint-debug)
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
| Berlaku di | Semua endpoint Guru | Hanya endpoint Siswa (`GET /modul/:id/soal`, `POST /submit-jawaban`, `GET /siswa/*`) |

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
| AI limiter | 3 request / 6 detik per IP | `POST /test-extract-pdf`, `POST /submit-jawaban` |

Melebihi batas → `429 Too Many Requests`:
```json
{ "error": "Terlalu banyak permintaan. Silakan coba lagi dalam beberapa saat." }
```

### Format Error Umum

```json
{ "error": "pesan error dalam bahasa Indonesia" }
```
Sebagian error menyertakan field `code` untuk penanganan spesifik di FE — lihat [Error Handling](#error-handling).

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
✅ **Endpoint inilah yang dipakai halaman GURU** untuk preview/polling soal & materi — bukan `GET /modul/:id/soal`.

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
| `deskripsi` | string | ❌* | ⚠️ Nilai yang dikirim **selalu menimpa** deskripsi lama |

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

🚫 **TIDAK BOLEK dipanggil dari halaman Guru.** Halaman guru memakai `GET /modul/:id` lalu memfilter array `.soal`.

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

**Error (400):**
```json
{ "error": "Anda sudah menjawab soal ini sebelumnya" }
```
*(Berlaku untuk jenis `uts`/`uas`: satu siswa satu kali per soal. Jenis `harian` boleh berulang.)*

---

## E. Alur Belajar Siswa (14 Sept 2026)

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
{ "error": "akses ditolak: siswa ini bukan anggota kelas" }
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
{ "error": "ID modul tidak valid" }
{ "error": "modul ini tidak ditugaskan untuk kelas Anda" }
{ "error": "akses ditolak: siswa ini bukan anggota kelas" }
```

---

## F. Real-time Updates / SSE (Token Guru)

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

**Implementasi FE** (`EventSource` tidak mendukung header Authorization, pakai `fetch` + `ReadableStream`):
```javascript
const connectSSE = () => {
  const token = localStorage.getItem('token_guru');

  const stream = async () => {
    const response = await fetch(
      'https://momo-be-production.up.railway.app/api/v1/kelas/stream',
      { headers: { Authorization: `Bearer ${token}` } }
    );

    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    let buffer = '';

    while (true) {
      const { done, value } = await reader.read();
      if (done) break;

      buffer += decoder.decode(value, { stream: true });
      const chunks = buffer.split('\n\n');
      buffer = chunks.pop();

      for (const chunk of chunks) {
        if (!chunk.trim() || chunk.startsWith(':')) continue; // abaikan heartbeat

        const eventMatch = chunk.match(/^event: (.+)$/m);
        const dataMatch  = chunk.match(/^data: (.+)$/m);
        if (!eventMatch || !dataMatch) continue;

        window.dispatchEvent(new CustomEvent('sse-' + eventMatch[1], {
          detail: JSON.parse(dataMatch[1])
        }));
      }
    }
  };

  stream();
};

connectSSE(); // panggil sekali setelah login guru

window.addEventListener('sse-kelas-created', (e) => { /* tambah ke list */ });
window.addEventListener('sse-kelas-updated', (e) => { /* update item */ });
window.addEventListener('sse-kelas-deleted', (e) => { /* buang item, id = e.detail.id */ });
```

**Tips:** retry dengan backoff saat koneksi putus; satu koneksi per tab sudah cukup; SSE hanya untuk event kelas (status soal tetap polling `GET /modul/:id`).

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

## G. Lampiran: Endpoint Debug

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
| 302 | Found | redirect verify-email |
| 400 | Bad Request | validasi gagal, nama duplikat, file terlalu besar/bukan PDF, bukan pemilik resource |
| 401 | Unauthorized | token hilang/salah/kedaluwarsa/wrong role, login gagal, kode kelas salah |
| 403 | Forbidden | resource bukan milik user, modul belum ditugaskan ke kelas siswa |
| 404 | Not Found | resource tidak ada / bukan milik user |
| 422 | Unprocessable Entity | proses AI gagal (materi/soal) |
| 429 | Too Many Requests | rate limit terlampaui |
| 500 | Internal Server Error | kesalahan tak terduga di server |

### Error Codes (field `code`)

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
8. **Role isolation aktif**: token guru ≠ endpoint siswa, dan sebaliknya (`TOKEN_WRONG_ROLE`). Halaman guru pakai `GET /modul/:id` untuk preview soal; halaman siswa pakai `GET /modul/:id/soal`.
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
23. 🆕 **Auto-register siswa** (14 Sept update 4): `POST /join` sekarang otomatis membuat siswa baru jika nama belum terdaftar. Guru tetap bisa lihat daftar siswa real-time via SSE.
24. 🆕 **Endpoint siswa baru** (14 Sept update 4):
    - `GET /siswa/kelas-saya` — info kelas + modul + flag ketersediaan materi/soal
    - `GET /siswa/modul/:id/materi` — list materi untuk dibacakan TTS (validasi modul tertaut ke kelas)
25. 🆕 **AI bisa tahu modul mana yang punya materi/soal** dari field `punya_materi` dan `jenis_soal_tersedia` di response `GET /siswa/kelas-saya`.

---

## Changelog

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