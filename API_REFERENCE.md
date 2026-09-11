# API Reference — Momo-BE

Dokumen ini disusun berdasarkan **testing langsung terhadap kode yang berjalan** (bukan asumsi).
Versi awal: 1 September 2026 · **Update terakhir: 11 September 2026**.
Semua contoh request/response di bawah adalah hasil `curl` nyata.

---

## Daftar Isi

- [Info Umum](#info-umum)
- [A. Health, Auth & Registrasi](#a-health-auth--registrasi)
- [B. Modul (Token Guru)](#b-modul-token-guru)
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
| Dipakai untuk endpoint | Semua endpoint kelola Guru (Modul, Kelas, dst.) | `GET /modul/:id/soal`, `POST /submit-jawaban` |

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
Error validasi field (400) kadang menampilkan pesan mentah dari library validasi Go, contoh:
```json
{ "error": "Key: 'createModulRequest.Nama' Error:Field validation for 'Nama' failed on the 'required' tag" }
```
FE sebaiknya menampilkan pesan generik ("mohon lengkapi form") untuk kasus ini, bukan menampilkan pesan mentah ke pengguna akhir.

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

## B. Modul (Token Guru)

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
⚠️ Perhatikan field **`judul`** di response detail ini (beda dari `nama` yang dipakai di request/list). `kunci_jawaban` **sengaja tidak pernah muncul** di endpoint ini, termasuk untuk Guru pemilik soal.

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
*(`kunci_jawaban` juga tidak muncul di sini.)*

### `GET /api/v1/modul/:id/soal?jenis=uts`
⚠️ Endpoint ini dipakai **Guru maupun Siswa** (Siswa butuh Token Siswa, dan Modul-nya harus sudah di-assign ke Kelas siswa itu — lihat bagian C & D). Response sama seperti di atas.

---

## C. Kelas (Token Guru)

### `POST /api/v1/kelas`
🆕 **UPDATE 11 Sept:** field `mata_pelajaran` sekarang **WAJIB** diisi.

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

### 🆕 `GET /api/v1/kelas`
List semua kelas **milik guru yang sedang login saja**.

**Response (200):**
```json
[
  {
    "id": 3, "guru_id": 3, "nama_kelas": "Kelas 5A", "mata_pelajaran": "Matematika",
    "kode_kelas": "487137", "created_at": "...", "updated_at": "..."
  }
]
```

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

### 🆕 `PUT /api/v1/kelas/:id`
Update nama dan/atau mata pelajaran kelas. Hanya pemilik kelas yang bisa update (guru lain dapat error).

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

### 🆕 `DELETE /api/v1/kelas/:id`
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

### 🆕 `DELETE /api/v1/kelas/:id/modul/:modul_id` (Lepas Modul dari Kelas)
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

🆕 **UPDATE 11 Sept.** Backend menyediakan **Server-Sent Events (SSE)** agar FE tidak perlu polling untuk update data kelas.

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
| 200 | OK | GET, PUT, DELETE berhasil, join siswa |
| 201 | Created | register guru, buat modul/kelas/siswa, submit jawaban |
| 400 | Bad Request | validasi gagal, nama duplikat, bukan pemilik resource |
| 401 | Unauthorized | token hilang/salah/kedaluwarsa, login gagal, kode kelas salah |
| 403 | Forbidden | origin CORS tidak diizinkan |
| 404 | Not Found | resource tidak ada / bukan milik user yang login |
| 429 | Too Many Requests | rate limit terlampaui |
| 500 | Internal Server Error | kesalahan tak terduga di server |

---

## Known Issues / Catatan untuk FE

1. **Inkonsistensi nama field `nama` vs `nama_kelas`** di endpoint Kelas — request pakai `nama`, response balikin `nama_kelas`. Ini perilaku aktual yang sudah dikonfirmasi.
2. **Lokasi token beda** antara Login Guru (`data.token`) dan Join Siswa (`.token` langsung) — pastikan FE menangani dua struktur berbeda ini.
3. `GET /modul/:id` mengembalikan field **`judul`** untuk nama Modul, sementara endpoint lain (create/list) pakai `nama` — perhatikan saat parsing response.
4. **`kunci_jawaban` tidak pernah dikirim** ke endpoint manapun, termasuk ke guru pemilik soal — ini keputusan keamanan yang disengaja.
5. 🆕 **`mata_pelajaran` wajib** diisi saat membuat kelas sejak 11 Sept 2026 — form FE harus menambahkan field ini.
6. 🆕 **Gunakan SSE** (`GET /kelas/stream`) untuk sinkronisasi list kelas, bukan polling `GET /kelas` berulang.
7. 🆕 Baris `: heartbeat` di stream SSE adalah komentar — parser FE harus mengabaikannya.

---

## Changelog

### 11 September 2026
- 🆕 `GET /api/v1/kelas` — list semua kelas milik guru
- 🆕 `PUT /api/v1/kelas/:id` — update kelas
- 🆕 `DELETE /api/v1/kelas/:id` — hapus kelas
- 🆕 `DELETE /api/v1/kelas/:id/modul/:modul_id` — lepas modul dari kelas
- 🆕 `GET /api/v1/kelas/stream` — real-time updates via SSE
- ⚠️ BREAKING: `POST /api/v1/kelas` sekarang wajib menyertakan `mata_pelajaran`
- ✅ Base URL production ditetapkan: `https://momo-be-production.up.railway.app`

### 1 September 2026
- Dokumentasi awal berdasarkan testing langsung seluruh endpoint existing.