# API Reference — Momo-BE

Dokumen ini disusun berdasarkan **testing langsung terhadap kode yang berjalan** (bukan asumsi).
Versi awal: 1 September 2026 · **Update terakhir: 12 September 2026**.
Semua contoh request/response di bawah adalah hasil `curl` nyata.

---

## Daftar Isi

- [Info Umum](#info-umum)
- [A. Health, Auth & Registrasi](#a-health-auth--registrasi)
- [B. Modul & Materi (Token Guru)](#b-modul--materi-token-guru)
- [C. Kelas (Token Guru)](#c-kelas-token-guru)
- [D. Alur Siswa (Token Siswa)](#d-alur-siswa-token-siswa)
- [E. Real-time Updates / SSE (Token Guru)](#e-real-time-updates--sse-token-guru)
- [Error Handling](#error-handling)
- [Known Issues / Catatan untuk FE](#known-issues--catatan-untuk-fe)
- [Changelog](#changelog)

---

## Info Umum

| | |
|---|---|
| **Base URL (dev)** | `http://localhost:8080` |
| **Base URL (production)** | `https://momo-be-production.up.railway.app` |
| **Format data** | JSON (`application/json`) untuk sebagian besar endpoint; `multipart/form-data` untuk upload file PDF |
| **Autentikasi** | JWT via header `Authorization: Bearer <token>` — ada 2 jenis token berbeda, lihat di bawah |

Semua endpoint berada di bawah prefix `/api/v1`, kecuali `/health`.

### Dua Jenis Token JWT

| | Token Guru | Token Siswa |
|---|---|---|
| Didapat dari | `POST /api/v1/guru/login` | `POST /api/v1/join` |
| Isi claim | `guru_id`, `role: "guru"` | `siswa_id`, `kelas_id` |
| Masa berlaku | 24 jam | 12 jam |
| Dipakai untuk endpoint | Semua endpoint kelola Guru (Modul, Materi, Kelas, dst.) | `GET /modul/:id/soal`, `POST /submit-jawaban` |

Kedua jenis token **tidak bisa dipertukarkan** — token Siswa tidak akan diterima di endpoint khusus Guru, dan sebaliknya.

### CORS

Origin berikut sudah diizinkan mengakses API ini dari browser:
- `http://localhost:3000`, `http://localhost:5173` (dev; bisa ditambah lewat env `CORS_ALLOWED_ORIGINS` di sisi BE)
- Semua subdomain `*.vercel.app` (production maupun preview deployment)

Origin lain akan mendapat `403 Forbidden`. Request tanpa header `Origin` (misal panggilan server-to-server dari AI Service) **tidak terpengaruh** aturan CORS sama sekali.

### Rate Limiting

| Limiter | Batas | Diterapkan di |
|---|---|---|
| Auth limiter | 5 request / 12 detik per IP | `POST /guru/register`, `POST /guru/login`, `POST /join` |
| AI limiter | 3 request / 6 detik per IP | `POST /test-extract-pdf`, `POST /submit-jawaban` |

Melebihi batas akan mendapat **`429 Too Many Requests`**:
```json
{ "error": "Terlalu banyak permintaan. Silakan coba lagi dalam beberapa saat." }
```
Tangani di FE dengan pesan "terlalu banyak percobaan, coba lagi nanti".

### Format Error Umum

Kebanyakan error dikembalikan sebagai:
```json
{ "error": "pesan error dalam bahasa Indonesia" }
```
Beberapa error menyertakan field `code` untuk memudahkan FE menangani kasus spesifik (lihat bagian [Error Handling](#error-handling)).

---

## A. Health, Auth & Registrasi

### `GET /health`
Publik. Cek server hidup.

**Response 200:**
```json
{ "status": "ok", "message": "server is running!" }
```

### `POST /api/v1/guru/register`
Publik.

**Request:**
```json
{ "nama": "Bu Sari", "email": "sari@sekolah.com", "password": "rahasia123" }
```

**Response sukses (201 Created):**
```json
{
  "data": {
    "id": 3, "nama": "Bu Sari", "email": "sari@sekolah.com",
    "created_at": "2026-09-01T01:15:58Z", "updated_at": "2026-09-01T01:15:58Z"
  },
  "message": "pendaftaran guru berhasil"
}
```

**Error — email sudah terdaftar (400):**
```json
{ "error": "email sudah terdaftar" }
```

**Error — validasi field (400):** pesan sudah ramah, contoh:
```json
{ "error": "Nama wajib diisi (minimal 2 karakter)" }
{ "error": "Format email tidak valid" }
{ "error": "Password minimal 6 karakter" }
```

### `POST /api/v1/guru/login`
Publik.

**Request:**
```json
{ "email": "sari@sekolah.com", "password": "rahasia123" }
```

**Response sukses (200):**
```json
{
  "data": {
    "token": "eyJhbGci...",
    "guru": { "id": 3, "nama": "Bu Sari", "email": "sari@sekolah.com", "created_at": "...", "updated_at": "..." }
  },
  "message": "login berhasil"
}
```
⚠️ **Token ada di `data.token`, BUKAN di root object.**

**Error — email/password salah (401):**
```json
{ "error": "email atau password salah" }
```

### `POST /api/v1/join` (Siswa masuk Kelas)
Publik. Kode kelas selalu **6 digit angka** (contoh: `"487137"`) — sengaja tanpa huruf untuk kemudahan pengucapan lewat voice/STT.

**Request:**
```json
{ "kode_kelas": "487137", "nama": "Siswa Retest" }
```

**Response sukses (200):**
```json
{ "siswa_id": 1, "kelas_id": 3, "nama": "Siswa Retest", "token": "eyJhbGci..." }
```
⚠️ **Beda dengan login Guru — token di sini langsung di root object (`.token`), bukan `.data.token`.**

**Error — kode tidak ditemukan (401):**
```json
{ "error": "kelas dengan kode '000000' tidak ditemukan" }
```

**Error — nama tidak terdaftar di kelas itu (401):**
```json
{ "error": "nama 'Nama Asing' tidak terdaftar di kelas ini" }
```
*(Catatan: siswa harus DIDAFTARKAN GURU dulu lewat `POST /kelas/:id/siswa` sebelum bisa join — siswa tidak bisa daftar sendiri.)*

---

## B. Modul & Materi (Token Guru)

Semua endpoint di bagian ini **terisolasi per Guru** — Guru A tidak bisa melihat/mengakses Modul milik Guru B (akan dapat `404`/`403`, bukan error khusus, seolah datanya tidak ada).

### 📦 Struktur Data: Modul vs Materi vs Soal

```
GURU (pemilik)
 └── MODUL  ← "wadah/unit belajar" (contoh: "Ipa")
      ├── MATERI  ← ISI belajar (bab/rangkuman)
      ├── SOAL    ← BANK SOAL (harian/uts/uas)
      └── tertaut ke KELAS (many2many)
```

| | **MODUL** | **MATERI** |
|---|---|---|
| Level | Wadah/unit | Isi/bab |
| Field utama | `nama`, `deskripsi` | `judul`, `konten`, `urutan` |
| Kepunyaan | Langsung milik Guru | Milik Modul (ikut guru pemilik modul) |
| Cascade | Hapus modul = hapus semua materi & soal di dalamnya | Hapus materi = modul tidak terpengaruh |

---

### `POST /api/v1/modul` — Buat modul baru
**Request:**
```json
{ "nama": "Modul IPA", "deskripsi": "Ilmu Pengetahuan Alam untuk Kelas 5" }
```
**Response (201):**
```json
{
  "id": 2, "guru_id": 3, "nama": "Modul IPA", "deskripsi": "Ilmu Pengetahuan Alam untuk Kelas 5",
  "created_at": "...", "updated_at": "..."
}
```

### `GET /api/v1/modul`
List semua Modul **milik guru yang sedang login saja**.
**Response (200):** array objek Modul seperti di atas.

### `GET /api/v1/modul/:id`
**Response sukses (200):**
```json
{
  "id": 2, "judul": "Modul IPA", "deskripsi": "Ilmu Pengetahuan Alam untuk Kelas 5",
  "materi": [ ... ],
  "soal": [ { "id": 1, "modul_id": 2, "jenis": "uts", "pertanyaan": "...", "pilihan_a": "...", "pilihan_b": "...", "pilihan_c": "...", "pilihan_d": "..." } ]
}
```
⚠️ Perhatikan field **`judul`** di response detail ini (beda dari `nama` yang dipakai di request/list). `kunci_jawaban` **sengaja tidak pernah muncul** di endpoint ini, termasuk untuk Guru pemilik soal.

**Error — tidak ditemukan / bukan milik guru ini (404):**
```json
{ "error": "Modul tidak ditemukan" }
```

### 🆕 `PUT /api/v1/modul/:id` — Update modul (12 Sept 2026)

Update nama dan/atau deskripsi modul. **Minimal salah satu field harus diisi.**

**Request:** (kirim hanya field yang ingin diubah, atau keduanya)
```json
{
  "nama": "Ipa (Updated)",
  "deskripsi": "Ilmu Pengetahuan Alam untuk Kelas 5"
}
```

**Response (200):**
```json
{
  "message": "Modul berhasil diperbarui",
  "data": {
    "id": 3, "guru_id": 7, "nama": "Ipa (Updated)",
    "deskripsi": "Ilmu Pengetahuan Alam untuk Kelas 5",
    "created_at": "...", "updated_at": "..."
  }
}
```
⚠️ Response **TIDAK** menyertakan `materi` atau `soal` secara default. Gunakan `GET /modul/:id` untuk melihat konten lengkapnya.

**Error — bukan pemilik (400):**
```json
{ "error": "modul tidak ditemukan atau bukan milik Anda" }
```

**Error — field kosong (400):**
```json
{ "error": "Minimal salah satu field (nama atau deskripsi) harus diisi" }
```

### 🆕 `DELETE /api/v1/modul/:id` — Hapus modul (12 Sept 2026)

Hapus modul beserta **semua materi dan soal** di dalamnya (cascade delete).

⚠️ **PERINGATAN:** Operasi ini tidak bisa dibatalkan. Semua materi (hasil AI maupun manual) dan soal (harian/uts/uas) akan ikut terhapus. Tautan modul dengan kelas juga akan terlepas.

**Response (200):**
```json
{ "message": "Modul berhasil dihapus beserta semua materi dan soal di dalamnya" }
```

**Error — bukan pemilik (400):**
```json
{ "error": "modul tidak ditemukan atau bukan milik Anda" }
```

---

### CRUD Materi Manual (12 Sept 2026)

Selain upload PDF (yang dirangkum AI), guru juga bisa menulis materi **manual** langsung di form. Materi manual dan hasil AI **bercampur dalam satu list** dan terurut berdasarkan field `urutan`.

#### `GET /api/v1/modul/:id/materi` — List semua materi di modul
Mengembalikan semua materi milik modul tersebut, terurut berdasarkan `urutan` ASC.

**Response (200):**
```json
{
  "jumlah": 2,
  "data": [
    {
      "id": 11, "modul_id": 2, "urutan": 1,
      "judul": "Pengenalan IPA",
      "konten": "IPA adalah ilmu yang mempelajari...",
      "created_at": "...", "updated_at": "..."
    },
    {
      "id": 12, "modul_id": 2, "urutan": 2,
      "judul": "Makhluk Hidup",
      "konten": "...",
      "created_at": "...", "updated_at": "..."
    }
  ]
}
```

**Error — bukan milik guru (403):**
```json
{ "error": "akses ditolak: modul tidak ditemukan atau bukan milik Anda" }
```

#### `POST /api/v1/modul/:id/materi/manual` — Buat materi manual
**Request:**
```json
{
  "judul": "Pengenalan IPA",
  "konten": "IPA adalah ilmu yang mempelajari alam sekitar.",
  "urutan": 5
}
```
`urutan` **opsional** — jika tidak diisi atau ≤ 0, otomatis ditempatkan di posisi terakhir (max urutan + 1).

**Response (201):**
```json
{
  "message": "Materi berhasil ditambahkan",
  "data": {
    "id": 149, "modul_id": 3, "urutan": 5,
    "judul": "Pengenalan IPA",
    "konten": "IPA adalah ilmu yang mempelajari alam sekitar.",
    "created_at": "..."
  }
}
```

**Error (400):**
```json
{ "error": "Judul dan konten materi wajib diisi" }
```

#### `PUT /api/v1/materi/:id` — Update materi
⚠️ Path parameter-nya `id` materi (bukan id modul). Validasi kepemilikan otomatis dilakukan (materi harus milik modul milik guru ini).

**Request:**
```json
{
  "judul": "Pengenalan IPA (Revisi)",
  "konten": "IPA adalah ilmu tentang alam dan isinya.",
  "urutan": 5
}
```

**Response (200):**
```json
{
  "message": "Materi berhasil diperbarui",
  "data": {
    "id": 149, "modul_id": 3, "urutan": 5,
    "judul": "Pengenalan IPA (Revisi)",
    "konten": "IPA adalah ilmu tentang alam dan isinya.",
    "created_at": "...", "updated_at": "..."
  }
}
```

**Error — bukan milik guru (400):**
```json
{ "error": "akses ditolak: materi ini bukan milik Anda" }
```

#### `DELETE /api/v1/materi/:id` — Hapus materi
Hapus satu materi (manual maupun hasil AI).

**Response (200):**
```json
{ "message": "Materi berhasil dihapus" }
```

**Error — bukan milik guru (400):**
```json
{ "error": "akses ditolak: materi ini bukan milik Anda" }
```

---

### `POST /api/v1/modul/:id/materi` — Upload PDF materi (SYNCHRONOUS)

Upload PDF materi untuk dirangkum AI. **Request DITAHAN oleh backend sampai AI selesai merangkum** (biasanya 10–60 detik tergantung ukuran PDF). FE **wajib** menampilkan spinner/loading selama menunggu.

**Headers:**
```http
Authorization: Bearer <token_guru>
Content-Type: multipart/form-data
```
**Body:** field file bernama **`file`** (wajib `.pdf`, maksimal **25MB**).

**Response sukses (200 OK) — hasil rangkuman ASLI:**
```json
{
  "message": "Materi berhasil diproses",
  "jumlah": 2,
  "data": [
    {
      "id": 11, "modul_id": 2, "urutan": 1,
      "judul": "Pengenalan IPA",
      "konten": "IPA adalah ilmu yang mempelajari..."
    }
  ]
}
```
⚠️ **BREAKING CHANGE dari kontrak lama:** endpoint ini TIDAK LAGI mengembalikan pesan "sedang diproses di background". Panel hasil di FE harus merender array **`data`**, bukan `message`.

**Error (422):**
```json
{
  "error": "Gagal memproses materi dari PDF. Layanan AI terlalu lama memproses atau tidak tersedia. Coba lagi atau gunakan PDF yang lebih kecil.",
  "code": "MATERI_PROCESSING_FAILED"
}
```
Varian pesan `error` lain yang mungkin:
- `...File PDF tidak bisa dibaca. Pastikan PDF berisi teks, bukan hasil scan gambar.`
- `...AI tidak menemukan konten materi yang valid. Pastikan PDF berisi materi pembelajaran, bukan soal.`

**Error validasi file (400):**
```json
{ "error": "Ukuran file terlalu besar. Maksimal 25MB.", "code": "FILE_TOO_LARGE" }
{ "error": "Hanya file PDF yang diperbolehkan", "code": "INVALID_FILE_TYPE" }
```

**Error kepemilikan (403):**
```json
{ "error": "akses ditolak: modul tidak ditemukan atau bukan milik Anda" }
```

### `POST /api/v1/modul/:id/soal?jenis=uts` — Upload PDF soal (ASYNC)

Upload PDF soal. Field file **`file`** (wajib `.pdf`, maksimal **5MB**). Query param `jenis` wajib: `harian`, `uts`, `uas`.

Endpoint ini **tetap async**: response langsung kembali, ekstraksi AI berjalan di background.

**Response sukses (202 Accepted):**
```json
{
  "message": "PDF sedang diproses di background, cek daftar soal beberapa saat lagi",
  "jenis": "uts"
}
```
FE harus **polling** `GET /modul/:id/soal?jenis=...` (misal tiap 5 detik) sampai data soal muncul.

**Error kepemilikan (403):** dicek synchronous sebelum proses dimulai:
```json
{ "error": "akses ditolak: modul tidak ditemukan atau bukan milik Anda" }
```

**Error validasi file (400):**
```json
{ "error": "Ukuran file terlalu besar. Maksimal 5MB.", "code": "FILE_TOO_LARGE" }
{ "error": "Hanya file PDF yang diperbolehkan", "code": "INVALID_FILE_TYPE" }
```

### `GET /api/v1/modul/:id/soal?jenis=uts`
Dipakai **Guru maupun Siswa** (Siswa butuh Token Siswa, dan Modul-nya harus sudah di-assign ke Kelas siswa itu — lihat bagian C & D).

**Response (200):**
```json
{
  "jenis": "uts",
  "jumlah": 1,
  "data": [ { "id": 1, "modul_id": 2, "jenis": "uts", "pertanyaan": "...", "pilihan_a": "...", "pilihan_b": "...", "pilihan_c": "...", "pilihan_d": "..." } ]
}
```

**Response (404) — soal belum selesai diproses / belum ada:**
```json
{ "error": "Belum ada soal untuk modul dan jenis ini" }
```

---

## C. Kelas (Token Guru)

### `POST /api/v1/kelas`
Field `mata_pelajaran` **WAJIB** diisi.

**Request:**
```json
{ "nama": "Kelas 5A", "mata_pelajaran": "Matematika" }
```
⚠️ Field yang benar adalah **`nama`**, BUKAN `nama_kelas`. Mengirim `nama_kelas` akan gagal validasi.

**Response (201):**
```json
{
  "id": 3, "guru_id": 3, "nama_kelas": "Kelas 5A", "mata_pelajaran": "Matematika",
  "kode_kelas": "487137", "created_at": "...", "updated_at": "..."
}
```
⚠️ Perhatikan asimetri ini: **request** pakai field `nama`, tapi **response** mengembalikannya sebagai `nama_kelas`.

📡 Memicu event SSE `kelas-created`.

### `GET /api/v1/kelas` — DENGAN PAGINATION
List semua kelas milik guru yang sedang login.

**Query parameters (opsional):**
| Param | Default | Keterangan |
|---|---|---|
| `page` | `1` | Nomor halaman |
| `limit` | `10` | Jumlah data per halaman (maks 50) |

**Contoh:** `GET /api/v1/kelas?page=1&limit=10`

**Response (200):**
```json
{
  "data": [
    {
      "id": 3, "guru_id": 3, "nama_kelas": "Kelas 5A", "mata_pelajaran": "Matematika",
      "kode_kelas": "487137", "created_at": "...", "updated_at": "..."
    }
  ],
  "meta": {
    "page": 1,
    "limit": 10,
    "total": 3,
    "total_page": 1
  }
}
```
⚠️ **BREAKING CHANGE ringan:** response sekarang berbentuk object `{ data, meta }`, bukan array langsung. FE wajib membaca `response.data` untuk list kelas.

### `GET /api/v1/kelas/:id`
**Response (200):**
```json
{
  "id": 3, "guru_id": 3, "nama_kelas": "Kelas 5A", "mata_pelajaran": "Matematika",
  "kode_kelas": "487137",
  "siswa": [ { "id": 1, "kelas_id": 3, "nama": "Siswa Retest", "created_at": "..." } ],
  "modul": [ { "id": 2, "guru_id": 3, "nama": "Modul Retest", "deskripsi": "...", "created_at": "...", "updated_at": "..." } ],
  "created_at": "...", "updated_at": "..."
}
```

### `PUT /api/v1/kelas/:id`
Update nama dan/atau mata pelajaran kelas. Hanya pemilik kelas yang bisa update.

**Request:** (kirim hanya field yang ingin diubah)
```json
{ "nama": "Kelas 5A Updated", "mata_pelajaran": "Matematika Lanjut" }
```

**Response (200):**
```json
{
  "message": "Kelas berhasil diperbarui",
  "data": {
    "id": 3, "guru_id": 3, "nama_kelas": "Kelas 5A Updated",
    "mata_pelajaran": "Matematika Lanjut", "kode_kelas": "487137",
    "created_at": "...", "updated_at": "..."
  }
}
```

**Error — bukan pemilik / tidak ditemukan (400):**
```json
{ "error": "kelas tidak ditemukan atau Anda tidak memiliki akses" }
```

📡 Memicu event SSE `kelas-updated`.

### `DELETE /api/v1/kelas/:id`
Hapus kelas. Hanya pemilik kelas yang bisa menghapus.

**Response (200):**
```json
{ "message": "Kelas berhasil dihapus" }
```

**Error — bukan pemilik / tidak ditemukan (400):**
```json
{ "error": "kelas tidak ditemukan atau Anda tidak memiliki akses" }
```

📡 Memicu event SSE `kelas-deleted`.

### `POST /api/v1/kelas/:id/siswa` (Daftarkan Siswa)
**Request:**
```json
{ "nama": "Siswa Retest" }
```
**Response (201):**
```json
{ "id": 1, "kelas_id": 3, "nama": "Siswa Retest", "created_at": "..." }
```
**Error — nama duplikat dalam kelas yang sama (400):**
```json
{ "error": "siswa dengan nama 'Siswa Retest' sudah terdaftar di kelas ini" }
```

### `POST /api/v1/kelas/:id/modul` (Assign Modul ke Kelas)
**Request:**
```json
{ "modul_id": 2 }
```
**Response (200):**
```json
{ "message": "Modul berhasil ditautkan ke kelas" }
```
*(Modul yang di-assign harus milik Guru yang sama dengan pemilik Kelas — kalau tidak, ditolak.)*

### `DELETE /api/v1/kelas/:id/modul/:modul_id` (Lepas Modul dari Kelas)
Melepas tautan modul dari kelas **tanpa** menghapus modul itu sendiri dari akun guru.

**Response (200):**
```json
{ "message": "Modul berhasil dihapus dari kelas" }
```

### `GET /api/v1/kelas/:id/nilai?modul_id=X&jenis=Y`
Dashboard rekap nilai. `modul_id` wajib, `jenis` opsional (`harian`/`uts`/`uas`).

**Response (200):**
```json
[ { "siswa_id": 1, "nama": "Siswa Retest", "jumlah_soal_dijawab": 1, "jumlah_benar": 1, "skor_persen": 100 } ]
```
*(Array kosong `[]` kalau belum ada siswa yang mengerjakan apapun.)*

---

## D. Alur Siswa (Token Siswa)

### `GET /api/v1/modul/:id/soal?jenis=uts`
Sama seperti bagian B, tapi dengan Token Siswa. Modul harus sudah di-assign ke Kelas siswa tersebut, kalau tidak akan ditolak.

### `POST /api/v1/submit-jawaban`
**Request:**
```json
{ "soal_id": 1, "jawaban_mentah": "aku rasa jawabannya yang B" }
```
⚠️ **`siswa_id` TIDAK dikirim di body** — sengaja diambil dari token JWT yang sedang login, untuk mencegah siswa "menjawab atas nama siswa lain". `jawaban_mentah` sengaja berupa teks bebas (hasil Speech-to-Text), bukan harus huruf `A`/`B`/`C`/`D`.

**Response sukses (201):**
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
*(Untuk soal jenis `uts`/`uas`, endpoint ini hanya bisa dipanggil SEKALI per siswa per soal — percobaan kedua akan ditolak.)*

---

## E. Real-time Updates / SSE (Token Guru)

Backend menyediakan **Server-Sent Events (SSE)** agar FE tidak perlu polling untuk update data kelas.

### `GET /api/v1/kelas/stream`
Membuka koneksi streaming yang tetap terbuka. Server mengirim event setiap ada perubahan data kelas.

**Headers wajib:**
```http
Authorization: Bearer <token_guru>
```

**Contoh output mentah (hasil curl nyata):**
```text
event: connected
data: {"message":"Terhubung ke stream kelas","time":"2026-09-11T04:16:26Z"}

: heartbeat

event: kelas-created
data: {"id":10,"guru_id":7,"nama_kelas":"Kelas SSE Test","mata_pelajaran":"Fisika","kode_kelas":"372475","created_at":"...","updated_at":"..."}
```

### Daftar Event

| Event | Kapan dikirim | Isi `data` |
|---|---|---|
| `connected` | Saat koneksi pertama dibuka | `{ "message", "time" }` |
| `kelas-created` | Ada kelas baru dibuat | Object kelas lengkap |
| `kelas-updated` | Ada kelas di-update | Object kelas lengkap |
| `kelas-deleted` | Ada kelas dihapus | `{ "id": <id_kelas> }` |
| `: heartbeat` | Setiap ±15 detik | komentar kosong — **abaikan saja**, fungsinya menjaga koneksi tetap hidup |

### Cara Pakai di Frontend

⚠️ `EventSource` bawaan browser **tidak bisa** mengirim header `Authorization`. Gunakan `fetch` + `ReadableStream` seperti contoh berikut:

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
      buffer = chunks.pop(); // simpan sisa pesan yang belum lengkap

      for (const chunk of chunks) {
        if (!chunk.trim() || chunk.startsWith(':')) continue; // abaikan heartbeat

        const eventMatch = chunk.match(/^event: (.+)$/m);
        const dataMatch  = chunk.match(/^data: (.+)$/m);
        if (!eventMatch || !dataMatch) continue;

        const eventType = eventMatch[1];
        const data = JSON.parse(dataMatch[1]);

        // Sebarkan sebagai custom event agar mudah didengar komponen mana pun
        window.dispatchEvent(new CustomEvent('sse-' + eventType, { detail: data }));
      }
    }
  };

  stream();
};

// Panggil sekali setelah login guru
connectSSE();

// Dengarkan di komponen (React/Vue/vanilla sama saja)
window.addEventListener('sse-kelas-created', (e) => {
  console.log('Kelas baru:', e.detail);   // -> tambah ke list / refetch
});
window.addEventListener('sse-kelas-updated', (e) => {
  console.log('Kelas diubah:', e.detail); // -> update item di list
});
window.addEventListener('sse-kelas-deleted', (e) => {
  console.log('Kelas dihapus:', e.detail.id); // -> buang item dari list
});
```

**Tips:**
- Jika koneksi putus (network error), panggil ulang `connectSSE()` setelah jeda beberapa detik (retry dengan backoff).
- Satu koneksi per tab browser sudah cukup — semua event kelas masuk lewat satu stream ini.
- ⚠️ SSE **hanya** untuk event kelas. Untuk status proses SOAL (yang async), tetap gunakan polling `GET /modul/:id/soal`.

### Test cepat via curl
```bash
# Terminal 1: buka stream
curl -N -H "Authorization: Bearer $TOKEN_GURU" \
  https://momo-be-production.up.railway.app/api/v1/kelas/stream

# Terminal 2: picu event
curl -X POST https://momo-be-production.up.railway.app/api/v1/kelas \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN_GURU" \
  -d '{"nama": "Kelas SSE Test", "mata_pelajaran": "Fisika"}'
```

---

## Error Handling

### HTTP Status Codes yang Dipakai

| Code | Arti | Contoh kasus |
|---|---|---|
| 200 | OK | GET, PUT, DELETE berhasil, join siswa, upload materi (sync sukses), update/delete modul |
| 201 | Created | register guru, buat modul/kelas/siswa/materi, submit jawaban |
| 202 | Accepted | upload soal (diproses di background) |
| 400 | Bad Request | validasi gagal, nama duplikat, file terlalu besar/bukan PDF, bukan pemilik resource |
| 401 | Unauthorized | token hilang/salah/kedaluwarsa, login gagal, kode kelas salah |
| 403 | Forbidden | modul/kelas bukan milik user, origin CORS tidak diizinkan |
| 404 | Not Found | resource tidak ada / bukan milik user yang login |
| 422 | Unprocessable Entity | proses AI gagal (materi) |
| 429 | Too Many Requests | rate limit terlampaui |
| 500 | Internal Server Error | kesalahan tak terduga di server |

### Error Codes (field `code`)

| Code | Kapan muncul | Saran penanganan di FE |
|---|---|---|
| `TOKEN_MISSING` | Header Authorization tidak ada | Redirect ke halaman login |
| `TOKEN_INVALID_FORMAT` | Format header bukan `Bearer <token>` | Perbaiki cara kirim header |
| `TOKEN_INVALID` | Token tidak valid / kedaluwarsa | Hapus token lama, redirect login. Pesan sudah ramah: "Sesi Anda telah berakhir. Silakan login kembali." |
| `FILE_TOO_LARGE` | Upload melebihi batas (materi 25MB / soal 5MB) | Tampilkan pesan ukuran maksimal |
| `INVALID_FILE_TYPE` | File bukan `.pdf` | Tampilkan "hanya PDF yang didukung" |
| `MATERI_PROCESSING_FAILED` | AI gagal merangkum materi | Tampilkan field `error` apa adanya (sudah ramah) |

---

## Known Issues / Catatan untuk FE

1. **Inkonsistensi nama field `nama` vs `nama_kelas`** di endpoint Kelas — request pakai `nama`, response balikin `nama_kelas`. Ini perilaku aktual yang sudah dikonfirmasi.
2. **Lokasi token beda** antara Login Guru (`data.token`) dan Join Siswa (`.token` langsung) — pastikan FE menangani dua struktur berbeda ini.
3. `GET /modul/:id` mengembalikan field **`judul`** untuk nama Modul, sementara endpoint lain (create/list) pakai `nama` — perhatikan saat parsing response.
4. **`kunci_jawaban` tidak pernah dikirim** ke endpoint manapun, termasuk ke guru pemilik soal — ini keputusan keamanan yang disengaja.
5. 🔄 **Materi sekarang SYNCHRONOUS** (12 Sept): `POST /modul/:id/materi` menahan request 10–60 detik lalu mengembalikan array `data`. FE wajib: (a) tampilkan spinner + disable tombol selama menunggu, (b) render `data`, JANGAN render `message` sebagai isi rangkuman, (c) hapus template statis "Status Pemrosesan Materi" peninggalan kontrak async lama.
6. 🔄 **`GET /kelas` sekarang berbentuk `{ data, meta }`** karena pagination — FE wajib membaca `response.data`, bukan array langsung.
7. **Soal masih ASYNC**: setelah `POST /modul/:id/soal` (202), FE polling `GET /modul/:id/soal?jenis=...` tiap ±5 detik sampai muncul data.
8. **Gunakan SSE** (`GET /kelas/stream`) untuk sinkronisasi list kelas, bukan polling `GET /kelas` berulang.
9. Baris `: heartbeat` di stream SSE adalah komentar — parser FE harus mengabaikannya.
10. **Materi punya 2 sumber**: hasil AI (dari upload PDF) dan tulis manual. Keduanya bercampur rapi di `GET /modul/:id/materi`, terurut berdasarkan field `urutan`. FE perlu tombol "Generate dari PDF" dan "Tulis Manual" yang terpisah.
11. **Endpoint CRUD materi pakai path berbeda**: list pakai `/modul/:id/materi` (scoped modul), tapi update/delete pakai `/materi/:id` langsung (karena ID materi sudah unik).
12. 🆕 **CRUD Modul sekarang LENGKAP**: selain Create & Read, sudah tersedia `PUT /modul/:id` dan `DELETE /modul/:id`. **Hapus modul = hapus semua materi & soal di dalamnya** (cascade). FE sebaiknya menampilkan konfirmasi peringatan sebelum delete.
13. 🆕 **Response `PUT /modul/:id` tidak menyertakan materi/soal** — hanya info modul itu sendiri. Kalau FE butuh data lengkap, panggil ulang `GET /modul/:id`.

---

## Changelog

### 12 September 2026 (update 3)
- 🆕 **CRUD Modul Lengkap:** tambahkan Update & Delete
  - `PUT /api/v1/modul/:id` — update nama/deskripsi modul
  - `DELETE /api/v1/modul/:id` — hapus modul + cascade delete materi & soal
- ✅ Validasi kepemilikan modul (hanya guru pemilik yang bisa update/delete)
- ✅ Pesan error ramah untuk kasus bukan pemilik

### 12 September 2026 (update 2)
- 🆕 **CRUD Materi Manual:** 4 endpoint baru untuk menulis materi tanpa PDF
  - `GET /api/v1/modul/:id/materi` — list semua materi
  - `POST /api/v1/modul/:id/materi/manual` — buat materi manual
  - `PUT /api/v1/materi/:id` — update materi
  - `DELETE /api/v1/materi/:id` — hapus materi
- ✅ Materi dari AI dan materi manual bercampur rapi, terurut berdasarkan `urutan`
- ✅ Validasi kepemilikan materi (hanya guru pemilik modul yang bisa CRUD)

### 12 September 2026
- 🔄 **BREAKING:** `POST /api/v1/modul/:id/materi` sekarang **SYNCHRONOUS** — response 200 berisi array `data` rangkuman asli (sebelumnya 202 async dengan pesan status)
- 🔄 **BREAKING ringan:** `GET /api/v1/kelas` mendukung pagination (`?page=&limit=`) dan response berbentuk `{ data, meta }`
- ✅ Validasi upload file: materi maks **25MB**, soal maks **5MB**, wajib ekstensi `.pdf` (code `FILE_TOO_LARGE`, `INVALID_FILE_TYPE`)
- ✅ Pesan error autentikasi lebih ramah + field `code` (`TOKEN_MISSING`, `TOKEN_INVALID_FORMAT`, `TOKEN_INVALID`)
- ✅ Pesan error validasi register/login lebih manusiawi
- ✅ Error proses AI materi mengembalikan 422 dengan `code: MATERI_PROCESSING_FAILED` dan pesan siap tampil
- ✅ Timeout AI Service dinaikkan ke 120 detik untuk PDF besar

### 11 September 2026
- 🆕 `GET /api/v1/kelas` — list semua kelas milik guru
- 🆕 `PUT /api/v1/kelas/:id` — update kelas
- 🆕 `DELETE /api/v1/kelas/:id` — hapus kelas
- 🆕 `DELETE /api/v1/kelas/:id/modul/:modul_id` — lepas modul dari kelas
- 🆕 `GET /api/v1/kelas/stream` — real-time updates via SSE
- ⚠️ BREAKING: `POST /api/v1/kelas` sekarang wajib menyertakan `mata_pelajaran`
- ✅ Base URL production ditetapkan: `https://momo-be-production.up.railway.app`
- ✅ Optimasi performa: index database, connection pooling, keep-warm Neon

### 1 September 2026
- Dokumentasi awal berdasarkan testing langsung seluruh endpoint existing.