# API Reference — Momo-BE

Dokumen ini disusun berdasarkan **testing langsung terhadap kode yang berjalan** (bukan asumsi), pada tanggal 1 September 2026. Semua contoh request/response di bawah adalah hasil `curl` nyata.

---

## Info Umum

| | |
|---|---|
| **Base URL (dev)** | `http://localhost:8080` |
| **Base URL (production)** | *belum ditentukan — update dokumen ini begitu sudah deploy* |
| **Format data** | JSON (`application/json`) untuk sebagian besar endpoint; `multipart/form-data` untuk upload file PDF |
| **Autentikasi** | JWT via header `Authorization: Bearer <token>` — ada 2 jenis token berbeda, lihat di bawah |

### Dua Jenis Token JWT

| | Token Guru | Token Siswa |
|---|---|---|
| Didapat dari | `POST /api/v1/guru/login` | `POST /api/v1/join` |
| Isi claim | `guru_id`, `role: "guru"` | `siswa_id`, `kelas_id` |
| Masa berlaku | 24 jam | 12 jam |
| Dipakai untuk endpoint | Semua endpoint kelola Guru (Modul, Kelas, dst.) | `GET /modul/:id/soal`, `POST /submit-jawaban` |

Kedua jenis token **tidak bisa dipertukarkan** — token Siswa tidak akan diterima di endpoint khusus Guru, dan sebaliknya.

### CORS

Origin berikut sudah diizinkan mengakses API ini dari browser:
- `http://localhost:3000`, `http://localhost:5173` (dev, bisa ditambah lewat env `CORS_ALLOWED_ORIGINS` di sisi BE kalau perlu port lain)
- Semua subdomain `*.vercel.app` (production maupun preview deployment)

Origin lain akan mendapat `403 Forbidden`. Request tanpa header `Origin` (misal panggilan server-to-server dari AI Service) **tidak terpengaruh** aturan ini sama sekali.

### Rate Limiting

`POST /api/v1/join` dibatasi — melebihi batas akan mendapat **`429 Too Many Requests`**. Tangani ini di FE dengan pesan "terlalu banyak percobaan, coba lagi nanti". (Detail angka limit persisnya belum diverifikasi ulang di dokumen ini — cek dengan tim BE kalau perlu presisi untuk UX retry.)

### Format Error Umum

Kebanyakan error dikembalikan sebagai:
```json
{ "error": "pesan error dalam bahasa Indonesia" }
```
Error validasi field (400) kadang menampilkan pesan mentah dari library validasi Go, contoh:
```json
{ "error": "Key: 'createModulRequest.Nama' Error:Field validation for 'Nama' failed on the 'required' tag" }
```
FE sebaiknya menampilkan pesan generik ("mohon lengkapi form") untuk kasus ini, bukan menampilkan pesan mentah ini ke pengguna akhir.

---

## A. Auth & Registrasi

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

**Error — field kosong (400):** lihat format error validasi di atas (`nama`, `email`, `password` semua `required`).

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

## B. Modul (Butuh Token Guru)

Semua endpoint di bagian ini **terisolasi per Guru** — Guru A tidak bisa melihat/mengakses Modul milik Guru B (akan dapat `404`, bukan error khusus, seolah datanya tidak ada).

### `POST /api/v1/modul`
**Request:**
```json
{ "nama": "Modul Retest", "deskripsi": "Deskripsi retest" }
```
**Response (201):**
```json
{
  "id": 2, "guru_id": 3, "nama": "Modul Retest", "deskripsi": "Deskripsi retest",
  "created_at": "...", "updated_at": "...", "materi": null, "soal": null
}
```

### `GET /api/v1/modul`
List semua Modul **milik guru yang sedang login saja**.
**Response (200):** array objek Modul seperti di atas.

### `GET /api/v1/modul/:id`
**Response sukses (200):**
```json
{
  "id": 2, "judul": "Modul Retest", "deskripsi": "Deskripsi retest",
  "soal": [ { "id": 1, "modul_id": 2, "jenis": "uts", "pertanyaan": "...", "pilihan_a": "...", "pilihan_b": "...", "pilihan_c": "...", "pilihan_d": "..." } ]
}
```
⚠️ Perhatikan field **`judul`** di response detail ini (beda dari `nama` yang dipakai di request/list). `kunci_jawaban` **sengaja tidak pernah muncul** di endpoint ini, termasuk untuk Guru pemilik soal — kalau kamu (Guru) butuh lihat kunci jawaban, tanyakan ke tim BE, ini keputusan yang mungkin masih akan direvisi.

**Error — tidak ditemukan / bukan milik guru ini (404):**
```json
{ "error": "Modul tidak ditemukan" }
```

### `POST /api/v1/modul/:id/materi`
Upload PDF materi. `Content-Type: multipart/form-data`, field file bernama **`file`**.

### `POST /api/v1/modul/:id/soal?jenis=uts`
Upload PDF soal. `Content-Type: multipart/form-data`, field file **`file`**. Query param `jenis` wajib, salah satu: `harian`, `uts`, `uas`.

**Response sukses (201):**
```json
{
  "data": [ { "id": 1, "modul_id": 2, "jenis": "uts", "pertanyaan": "...", "pilihan_a": "...", "pilihan_b": "...", "pilihan_c": "...", "pilihan_d": "..." } ],
  "jenis": "uts", "jumlah": 1,
  "message": "Soal berhasil diproses dan disimpan"
}
```
*(`kunci_jawaban` juga tidak muncul di sini — lihat catatan di atas.)*

### `GET /api/v1/modul/:id/soal?jenis=uts`
⚠️ Endpoint ini dipakai **Guru maupun Siswa** (Siswa butuh Token Siswa, dan Modul-nya harus sudah di-assign ke Kelas siswa itu — lihat bagian D). Response sama seperti di atas.

---

## C. Kelas (Butuh Token Guru)

### `POST /api/v1/kelas`
**Request:**
```json
{ "nama": "Kelas Retest" }
```
⚠️ **BREAKING CHANGE PENTING:** field yang benar adalah **`nama`**, BUKAN `nama_kelas`. Mengirim `nama_kelas` akan gagal validasi.

**Response (201):**
```json
{ "id": 3, "guru_id": 3, "nama_kelas": "Kelas Retest", "kode_kelas": "487137", "created_at": "..." }
```
⚠️ Perhatikan asimetri ini: **request** pakai field `nama`, tapi **response** mengembalikannya sebagai `nama_kelas`.

### `GET /api/v1/kelas/:id`
**Response (200):**
```json
{
  "id": 3, "guru_id": 3, "nama_kelas": "Kelas Retest", "kode_kelas": "487137",
  "siswa": [ { "id": 1, "kelas_id": 3, "nama": "Siswa Retest", "created_at": "..." } ],
  "modul": [ { "id": 2, "guru_id": 3, "nama": "Modul Retest", ... } ],
  "created_at": "..."
}
```

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

### `GET /api/v1/kelas/:id/nilai?modul_id=X&jenis=Y`
Dashboard rekap nilai. `modul_id` wajib, `jenis` opsional.

**Response (200):**
```json
[ { "siswa_id": 1, "nama": "Siswa Retest", "jumlah_soal_dijawab": 1, "jumlah_benar": 1, "skor_persen": 100 } ]
```
*(Array kosong `[]` kalau belum ada siswa yang mengerjakan apapun.)*

---

## D. Alur Siswa (Butuh Token Siswa)

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

## Known Issues / Hal yang Perlu Diperhatikan Tim FE

1. **Inkonsistensi nama field `nama` vs `nama_kelas`** di endpoint Kelas (lihat bagian C) — request pakai `nama`, response balikin `nama_kelas`. Ini sudah dikonfirmasi sebagai perilaku aktual, tapi mungkin akan dirapikan di masa depan.
2. **Lokasi token beda antara Login Guru (`data.token`) dan Join Siswa (`.token` langsung)** — pastikan FE menangani dua struktur berbeda ini.
3. `GET /modul/:id` mengembalikan field `judul` untuk nama Modul, sementara endpoint lain (create/list) pakai `nama` — perhatikan baik-baik saat parsing response.
4. Guru tidak bisa melihat `kunci_jawaban` soal miliknya sendiri lewat API manapun saat ini — kalau dibutuhkan fitur ini di masa depan, perlu endpoint terpisah.
