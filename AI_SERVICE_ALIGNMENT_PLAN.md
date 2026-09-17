Halo tim AI Service,

Terlampir dokumen MOMO — AI Service Alignment Plan v1.2 (17 Sept 2026).
Jadikan ini acuan TUNGGAL konfigurasi LLM; abaikan v1.0/v1.1 jika pernah diterima.

Konteks singkat:
- Momo = platform belajar berbasis suara untuk siswa tuna netra SD–SMA.
- Semua output LLM dibacakan TTS; semua input siswa adalah teks STT yang berantakan.
- Backend Go production sudah berjalan dan memanggil /process serta /evaluate sesuai kontrak §3.

Tiga hal paling kritis untuk demo lomba 20 Sept (H-3):
1. §4.2 — ekstraksi soal naik dari ±2 menjadi minimal 10 soal per PDF (blocker demo).
2. §4.3 + §5 — akurasi deteksi jawaban suara ≥ 95% (wow factor utama produk).
3. §2.1 + §2.2 — semua output TTS-safe, gaya bahasa adaptif jenjang via INFERENSI string
   `konteks` (mulai v1.2 TIDAK ADA field jenjang terstruktur — nol migrasi DB).

Yang kami butuhkan dari kalian:
- Konfirmasi penerimaan + estimasi waktu untuk checklist §9.
- Kabari segera jika ada field kontrak §3 yang tidak cocok dengan implementasi kalian
   saat ini — backend adalah sumber kebenaran, kita samakan berdua.
- Field baru `konteks` akan mulai dikirim backend dalam 1–2 hari; sampai saat itu
   pakai inferensi default (§2.2), jangan gagal karena field absen.

Acceptance criteria §8 adalah gerbang sebelum demo. Terima kasih!



# MOMO — AI SERVICE ALIGNMENT PLAN
**Versi:** 1.6 · **Tanggal:** 17 September 2026 · **Ditujukan untuk:** Tim AI Service / Konfigurasi LLM
**Status backend:** Production (`https://momo-be-production.up.railway.app`) · Demo lomba: **H-3 (20 Sept)**
**Riwayat:** v1.2 (inferensi jenjang dari `konteks`) → v1.3 (user flow & conversation context) → v1.4 (write-back callback + stream tunggal) → v1.5 (memori reasoning di AI Service) → **v1.6 (potong `pola_kelemahan` ke fase 2 karena deadline; memori sesi tetap sebagai fakta mentah)**

---

## 1. Konteks Produk

- **Momo** adalah platform belajar online untuk **anak tuna netra jenjang SD, SMP, dan SMA** (usia ±6–18 tahun).
- Seluruh interaksi siswa berbasis **SUARA**: soal dibacakan Text-to-Speech (TTS), jawaban masuk sebagai teks bebas hasil Speech-to-Text (STT). Siswa tidak melihat layar.
- Semua output LLM pada akhirnya **DIDENGAR**. Format visual (markdown, tabel, simbol) = racun.
- Jenjang disimpulkan AI dari string `konteks` (§2.2) — tidak ada field jenjang terstruktur di backend.

---

## 2. Prinsip Audio-First & Adaptif-Jenjang

### 2.1 Aturan TTS-safe (semua jenjang)
1. **DILARANG:** markdown (`#`, `*`, `-`, `>`), tabel, emoji, kode, bullet simbol.
2. **DILARANG simbol matematika:** `× ÷ → √ ° ± ≠ ≤ ≥`. Ganti kata: "kali", "dibagi", "menjadi", "akar", "derajat", "kurang lebih".
3. **Angka & satuan:** angka dengan digit, satuan dengan kata setelahnya. ✅ "40.000 rupiah" ❌ "Rp40.000" · ✅ "0,5 sekon" ❌ "0.5 s" · ✅ "25 derajat celsius" ❌ "25°C".
4. **Pangkat & pecahan ucap-friendly:** "x²" → "x kuadrat" · "10³" → "sepuluh pangkat tiga" · "1/2" → "satu per dua".
5. **Tanpa instruksi visual:** dilarang "perhatikan gambar di atas", "lihat tabel berikut".

### 2.2 Inferensi jenjang dari string `konteks`
AI menyimpulkan jenjang dari `konteks`, lalu menyesuaikan gaya & kedalaman:

| Sinyal dalam `konteks` | Jenjang |
|---|---|
| Kelas 1–6, I–VI, "satu"–"enam", "SD", "MI" | **SD** |
| Kelas 7–9, VII–IX, "SMP", "MTs" | **SMP** |
| Kelas 10–12, X–XII, "SMA", "MA", "SMK" | **SMA** |
| Tanpa sinyal | **SMP** (default aman) |

| | **SD** | **SMP** | **SMA** |
|---|---|---|---|
| Panjang kalimat | ≤ 15 kata | ≤ 20 kata | ≤ 25 kata |
| Kosakata | konkret, sehari-hari | istilah + penjelasan singkat | istilah teknis boleh langsung |
| Nada | ramah menyemangati, "kamu" | ramah-menengah, "kamu" | muda-respektful, "kamu" |
| Kedalaman | fakta & contoh | konsep & sebab-akibat | konsep, rumus verbal, penalaran |

Aturan keras: jangan menebak SD untuk konten yang jelas SMA. Jika sinyal konteks bertentangan dengan kompleksitas materi, menangkan kompleksitas materi.

---

## 3. User Flow & Conversation Context

### 3.1 Alur Belajar (End-to-End, arsitektur v1.4+)

```
1. SISWA BUKA APP
   🔊 "Halo, apakah kamu siap belajar?"  →  🎤 "Ya"
2. PANDU JOIN
   🔊 "Siapa nama kamu?" → 🎤 "Budi" → 🔊 "Kode kelas mu berapa?" → 🎤 "487137"
   [FE: POST /join → auto-register + token]
3. BUKA SATU STREAM
   [FE: GET /api/v1/stream  ← SATU-SATUNYA koneksi update; dibuka sekali, dipertahankan]
4. INFO KELAS
   [FE: GET /siswa/kelas-saya]
   🔊 "Selamat datang di Kelas 5 Matematika! Kelas ini menyediakan soal harian dan UTS."
5. UPLOAD/GENERATE (alur guru) — ASYNC + STREAM
   [FE guru: POST /modul/:id/materi atau /soal  → ack {job_id} dalam <1 detik]
   [AI proses → callback ke backend → backend simpan → SSE event]
   [FE guru menerima event `materi-ready` / `soal-ready` di stream → render. TANPA polling.]
6. SESI SOAL (LOOP) — SYNC per soal
   🔊 bacakan soal+opsi → 🎤 "aku rasa jawabannya be"
   [FE: POST /submit-jawaban → backend tunggu callback AI → response sync berisi feedback]
   🔊 bacakan feedback → 🔊 "Mau lanjut soal berikutnya?" → 🎤 "ya"
7. MODE TUTOR (fase 2) — ASYNC + STREAM
   [FE: POST /tutor → ack] → [AI callback → backend emit event `tutor-reply`]
   🔊 bacakan balasan tutor dari stream
8. MODE MATERI
   [FE: GET /siswa/modul/:id/materi] → 🔊 bacakan per bab → 🎤 "bacakan ulang bagian ini"
```

### 3.2 Kapan Endpoint AI Dipanggil & Jalur Hasilnya

| Endpoint AI | Pemicu | Jalur hasil ke FE |
|---|---|---|
| `/process` materi | Guru upload PDF | **Stream event** `materi-ready` / `materi-failed` |
| `/process` soal | Guru upload PDF | **Stream event** `soal-ready` / `soal-failed` |
| `/evaluate` | Siswa submit jawaban | **Sync response** POST /submit-jawaban (backend menunggu callback AI) |
| `/tutor` | Siswa chat diskusi | **Stream event** `tutor-reply` |

### 3.3 Implikasi untuk Output AI
- Feedback `/evaluate`: maks 3 kalimat (±10–15 detik TTS); siswa langsung lanjut soal berikutnya. Jangan bertele-tele.
- Balasan `/tutor`: maks 4 kalimat, conversation-friendly, akhiri dengan pertanyaan balik.
- Konten `/process` materi: per bab tidak terlalu panjang (siswa bisa minta "bacakan ulang" kapan saja).

---

## 4. Arsitektur Write-Back & Stream Tunggal

### 4.1 Dataflow (+ memori)

```
FE ── upload/aksi ──> BACKEND ── ack {job_id} ──> FE
                       │ job {job_id, callback_url, session_id?, payload, konteks}
                       ▼
                  AI SERVICE ── baca/tulis MEMORI SESI (per session_id)
                       │ callback {job_id, status, hasil} + X-AI-Internal-Token
                       ▼
                  BACKEND: (1) simpan DB (satu-satunya writer bisnis)
                           (2) emit SSE event ──> FE (1 stream)
```

**Aturan emas:**
1. **AI Service TIDAK BOLEH menulis ke database langsung** (tidak ada koneksi DB dari AI). Penyimpanan hanya via callback ke backend.
2. **Backend adalah satu-satunya writer ke DB** dan satu-satunya sumber event ke FE.
3. **FE membuka SATU koneksi stream** setelah login untuk SEMUA update async. Tidak ada polling status proses sama sekali. REST biasa hanya untuk: bootstrap data awal, aksi CRUD, dan `POST /submit-jawaban` (sync).

### 4.2 Kontrak Job & Ack (Backend → AI)

Setiap request ke AI membawa identitas job:
```json
{
  "job_id": "job_20260917_0001",
  "callback_url": "https://momo-be-production.up.railway.app/api/v1/internal/ai-callback",
  "tipe": "materi | soal",
  "teks_mentah": "...",
  "konteks": "Modul: Fisika | Kelas tertaut: Kelas 11 IPA 2 (Fisika)"
}
```
**Response AI WAJIB ack cepat (< 1 detik), BUKAN hasil:**
```json
{ "accepted": true, "job_id": "job_20260917_0001" }
```
Seluruh hasil dikirim terpisah via callback (§4.3).

### 4.3 Kontrak Callback (AI → Backend)

```
POST {callback_url}
Headers:
  Content-Type: application/json
  X-AI-Internal-Token: <shared secret, disepakati via env kedua sisi>
Body (sukses):
{
  "job_id": "job_20260917_0001",
  "tipe": "materi | soal | evaluate | tutor",
  "status": "success",
  "hasil": { ...bentuk data sama seperti kontrak v1.3... }
}
Body (gagal):
{
  "job_id": "job_20260917_0001",
  "tipe": "soal",
  "status": "failed",
  "error_message": "teks tidak mengandung soal pilihan ganda"
}
```

**Aturan keandalan:**
- Callback dipanggil **tepat sekali** per `job_id` (best-effort). Retry maks 3× (backoff 2/4/8 detik) hanya jika backend membalas 5xx.
- Backend **idempotent**: callback duplikat dengan `job_id` sama diabaikan aman.
- Callback tanpa token / token salah → backend tolak 401; AI jangan retry tanpa token benar.
- Jika AI tidak pernah callback, backend menandai job failed setelah timeout: materi/soal 150 detik · evaluate 30 detik · tutor 20 detik, lalu emit event failed / balas error sync.

### 4.4 Katalog Event Stream (Backend → FE)

`GET /api/v1/stream` — role-aware (token guru atau siswa), menggantikan `/kelas/stream` (lama tetap hidup sampai FE migrasi, lalu deprecated).

| Event | Payload | Pemicu |
|---|---|---|
| `connected` | `{message, time}` | stream dibuka |
| `kelas-created` / `kelas-updated` / `kelas-deleted` | object kelas / `{id}` | CRUD kelas (existing) |
| `materi-ready` | `{job_id, modul_id, jumlah, data[]}` | callback materi success |
| `materi-failed` | `{job_id, modul_id, error}` | callback materi failed / timeout |
| `soal-ready` | `{job_id, modul_id, jenis, jumlah, data[]}` | callback soal success |
| `soal-failed` | `{job_id, modul_id, jenis, error}` | callback soal failed / timeout |
| `tutor-reply` | `{session_id, balasan}` | callback tutor success |
| `tutor-failed` | `{session_id, error}` | callback tutor failed |
| `: heartbeat` | komentar kosong | tiap ±15 detik |

Catatan: **tidak ada event untuk `/evaluate`** — hasilnya dikirim sync via response `POST /submit-jawaban` (latensi percakapan suara).

### 4.5 Prinsip untuk FE
- Setelah login: buka & pertahankan SATU `GET /api/v1/stream`; reconnect dengan backoff jika putus.
- Upload PDF: kirim → terima ack `{job_id}` → tampilkan spinner → render saat event `*-ready` tiba; tampilkan pesan `error` saat event `*-failed` tiba.
- Hapus SEMUA polling status proses dan SEMUA long-wait synchronous upload.
- `POST /submit-jawaban` tetap request-response biasa (feedback datang di response-nya).

---

## 5. Memori Reasoning di AI Service

### 5.1 Prinsip pembagian penyimpanan
- **AI Service menyimpan memori reasoning**: state sesi belajar yang dibutuhkan untuk beralasan lintas interaksi (posisi baca, soal yang sudah dibacakan, riwayat jawaban, chat tutor).
- **Backend menyimpan data bisnis** (nilai, jawaban, materi, soal) via callback write-back. Memori AI **bukan** sumber kebenaran bisnis dan **bukan** pengganti write-back.
- Memori AI boleh hilang kapan pun tanpa merusak produk: backend selalu mengirim **bootstrap konteks minimum** di setiap request sebagai ground truth; AI merge dengan memorinya bila ada.

### 5.2 Keying
- Key memori: `session_id` opaque buatan backend (bukan nama/ID pribadi siswa).
- Satu sesi = satu rangkaian belajar siswa (sesi soal + tutor lanjutan + navigasi materi).
- Job satu-shot guru (`/process`) tidak memakai session_id; cukup `job_id`.

### 5.3 Isi memori per session_id (bentuk v1.6 — fakta mentah saja)
```json
{
  "session_id": "sess_8f3a",
  "jenjang": "smp",
  "modul_aktif": { "id": 3, "nama": "Ipa" },
  "posisi_materi": { "urutan": 4, "judul": "Bab 2 Kegiatan Ekonomi" },
  "soal_dibacakan": [5, 6],
  "riwayat_jawaban": [
    { "soal_id": 5, "jawaban_terdeteksi": "B", "benar": false }
  ],
  "chat_tutor": [ { "role": "siswa", "teks": "..." }, { "role": "ai", "teks": "..." } ]
}
```
> ⚠️ Field `pola_kelemahan` **DIPOTONG KE FASE 2** (keputusan deadline 17 Sept). Memori hanya menyimpan fakta sesi mentah; tidak ada agregasi atau analisis kelemahan siswa.

### 5.4 Lifecycle
- **Create:** backend mengirim `session_id` + bootstrap pertama kali pada `/evaluate` atau `/tutor`.
- **Update:** AI memperbarui memori setelah setiap interaksi dalam sesi.
- **Rehydrate:** jika `session_id` tidak dikenal (restart/expiry), AI bangun memori baru dari `bootstrap` request — jangan gagal, jangan hallucinate riwayat.
- **Expiry:** TTL 24 jam sejak interaksi terakhir, lalu hapus permanen. Sesi hari berikutnya = rehydrate dari DB backend.

### 5.5 Yang TIDAK boleh masuk memori AI
- Nama asli siswa, token, atau data pribadi lain.
- `kunci_jawaban` dalam bentuk yang bisa bocor ke feedback siswa.
- Teks buku lengkap — simpan ringkasan/pointer materi aktif saja.
- Apa pun yang bernilai jangka panjang → wajib sudah di-write-back ke backend dulu.

### 5.6 Konsekuensi perilaku
- Feedback & balasan tutor **BOLEH (opsional, tidak wajib)** merujuk fakta sesi sederhana seperlunya — misal "soal sebelumnya juga tentang siklus air" — tanpa agregasi pola apa pun.
- Write-back tetap wajib untuk setiap jawaban & feedback (§4.3), terlepas dari memori.

---

## 6. Kontrak Endpoint AI

### 6.1 `POST /process` (tipe materi | soal)
Request: `{ job_id, callback_url, tipe, teks_mentah, konteks }` → ack `{accepted, job_id}`.
Hasil via callback: `hasil.data.materi[]` atau `hasil.data.soal[]` (bentuk field sama seperti v1.3).

### 6.2 `POST /evaluate`
Request:
```json
{
  "job_id": "...", "callback_url": "...",
  "session_id": "sess_8f3a",
  "bootstrap": {
    "riwayat_terakhir": [ { "soal_id": 5, "benar": false } ],
    "materi_aktif": { "judul": "...", "ringkasan": "..." }
  },
  "pertanyaan": "...", "pilihan_a": "...", "pilihan_b": "...", "pilihan_c": "...", "pilihan_d": "...",
  "kunci_jawaban": "B",
  "jawaban_mentah": "<teks STT>",
  "konteks": "Kelas: Kelas 8 IPA 2 (IPA) | Modul: Getaran"
}
```
→ ack. Hasil callback: `{ jawaban_terdeteksi, benar, feedback, perlu_klarifikasi }`.
Backend menunggu callback maks 30 detik → teruskan sebagai response sync `POST /submit-jawaban` ke FE.

### 6.3 `POST /tutor` (fase 2)
Request: `{ job_id, callback_url, session_id, bootstrap, pesan_siswa, konteks }` → ack.
Hasil callback: `{ balasan }` → backend emit event `tutor-reply`.
AI TIDAK perlu dikirim riwayat chat lengkap — ambil dari memori session_id; `bootstrap` hanya fallback.

---

## 7. Spesifikasi Perilaku LLM

- **Materi:** bab berurutan (`urutan` mulai 1); panjang per item SD 80–150 / SMP 100–200 / SMA 150–250 kata; narasi murni; tabel/diagram dikonversi ke kalimat; PDF berisi soal → `status: failed` dengan `error_message`.
- **Soal:** ekstrak SEMUA soal valid, **minimal 10, maksimal 20 per request** (⚠️ perilaku saat ini ±2 — **blocker demo**); lewati essay & soal gambar rumit (gambar sederhana boleh dinarasikan maks 2 kalimat); pertahankan tingkat kesulitan asli; kunci wajib pasti A–D, tidak pasti → buang soal.
- **Evaluate:** normalisasi STT → deteksi huruf/substansi → feedback maks 3 kalimat; jawaban salah JANGAN sebut huruf kunci, jelaskan substansi jawaban benar; tak terpetakan → `perlu_klarifikasi: true`. Tanpa personalisasi agregat (fase 2).
- **Tutor:** petunjuk bertahap, bukan huruf kunci; akhiri dengan pertanyaan balik; boleh merujuk fakta sesi sederhana.

---

## 8. Robustness Input STT

1. Buang filler: "ehm", "mm", "menurut saya", "kayaknya", "deh", "dong", "ya".
2. Lowercase, buang tanda baca.
3. Homofon huruf: {a, ha, ah, ea}→A · {be, ve, bi, bee}→B · {ce, se, si, cee}→C · {de, di, dee, dhe}→D.
4. Angka & simbol ucap: "dua sekon"→"2 sekon" · "nol koma lima"→"0,5" · "x kuadrat"→"x²" (pencocokan internal) · "lambda kali f"→"λ × f".
5. Istilah teknis ucap: "foto sintesis"→"fotosintesis" · "hipo tenusa"→"hipotenusa" · "ko efi sien"→"koefisien".

**Kasus uji wajib lulus (≥30, mencakup 3 jenjang):**
- SD: "aku rasa jawabannya yang be"→B · "empat ribu rupiah"→substansi opsi
- SMP: "jawabannya ce karena saldo berkurang"→C · "b dan c mirip tapi saya pilih b"→B
- SMA: "yang percepatan gravitasi sembilan koma delapan meter per sekon kuadrat"→substansi · "rumus v sama dengan lambda kali f"→substansi
- Klarifikasi: "saya tidak tahu"→perlu_klarifikasi · "yang terakhir" (ambigu)→perlu_klarifikasi

**Target akurasi deteksi huruf ≥ 95% per jenjang.**

---

## 9. Performa & Batas

| Tahap | Target | Batas keras |
|---|---|---|
| Ack AI (`accepted`) | < 1 detik | 3 detik |
| Callback AI → backend (evaluate) | p50 < 5 dtk, p95 < 10 dtk | 30 dtk |
| Callback AI → backend (process) | < 60 dtk | 150 dtk (timeout job backend) |
| Callback AI → backend (tutor) | p50 < 4 dtk | 20 dtk |
| Event stream → FE setelah callback diterima | < 2 detik | 5 detik |

Teks panjang → chunking internal. Job process > 100 detik → kirim hasil partial `status: success` daripada timeout.

---

## 10. Keamanan, Privasi & Retensi Memori

1. Memori AI berisi hanya data reasoning (§5.3); dilarang data pribadi & kunci jawaban bocor (§5.5).
2. Memori TTL 24 jam, hapus permanen setelahnya; write-back ke backend terjadi SEBELUM data dibutuhkan jangka panjang.
3. Callback wajib `X-AI-Internal-Token`; backend menolak 401 jika token salah.
4. AI tidak koneksi ke database backend dalam bentuk apa pun.
5. Feedback salah tidak membocorkan huruf kunci.

---

## 11. Acceptance Criteria

1. ✅ JSON valid 100% (ack, callback, dan seluruh payload).
2. ✅ Nol karakter terlarang §2.1 pada field yang sampai ke siswa.
3. ✅ `/process` soal ≥ 10 soal per PDF kumpulan 15–40 halaman (**blocker demo**).
4. ✅ Akurasi deteksi huruf ≥ 95% per jenjang pada set kasus §8.
5. ✅ Feedback salah tanpa huruf kunci; feedback ≤ 3 kalimat.
6. ✅ Inferensi jenjang benar pada 20 sample nama kelas nyata (format angka, romawi, kata).
7. ✅ Callback tepat sekali per job_id pada 50 job uji (termasuk simulasi retry 5xx).
8. ✅ Ack < 1 detik pada 100% request; event stream tiba di FE < 2 detik setelah callback.
9. ✅ **Memori lintas request:** pada uji tutor 5 interaksi, AI mampu merujuk fakta interaksi ke-2 dengan benar (misal soal/konsep yang sudah dibahas).
10. ✅ **Rehydrate:** setelah memori dihapus manual, request berikut tetap koheren via bootstrap (tidak hallucinate riwayat).
11. ✅ **Expiry:** memori lenyap setelah TTL 24 jam.
12. ✅ Job gagal menghasilkan callback `status: failed` dengan `error_message` informatif (bukan hang).

---

## 12. Checklist Implementasi

**Tim AI:**
- [ ] Terima `job_id` + `callback_url`; balas ack < 1 detik; hasil hanya via callback
- [ ] Implement callback dengan `X-AI-Internal-Token` + retry 3× backoff
- [ ] Memori sesi per `session_id` (§5): create/update/rehydrate/expiry — **fakta sesi mentah saja, TANPA agregasi pola kelemahan (fase 2)**
- [ ] Seluruh perilaku §7, robustness §8, prompt Lampiran B
- [ ] Load test latensi §9 + uji memori §11.9–11.11

**Tim Backend (menyusul dokumen ini):**
- [ ] Endpoint `POST /api/v1/internal/ai-callback` (validasi token, idempotent by job_id)
- [ ] Unified stream `GET /api/v1/stream` role-aware + katalog event §4.4
- [ ] Upload materi/soal → async ack + kirim job ke AI (bawa `job_id` + `callback_url`)
- [ ] `/submit-jawaban` → sync-via-callback (tunggu 30 detik) + kirim `session_id` & `bootstrap`
- [ ] `/tutor` route + emit `tutor-reply` (fase 2)

**Tim FE:**
- [ ] Buka satu stream setelah login; hapus polling & long-wait upload
- [ ] Render dari event `*-ready` / `*-failed` / `tutor-reply`
- [ ] Placeholder hint penamaan kelas: "Contoh: Kelas 11 IPA 2"

---

## Lampiran A — Konvensi Pembacaan Huruf (AI + FE)

TTS membaca huruf opsi: A→"a" · B→"be" · C→"ce" · D→"de". AI memakai peta homofon sama (§8 poin 3) saat memetakan ucapan siswa kembali ke huruf. Konsistensi dua arah ini kunci akurasi deteksi.

---

## Lampiran B — Kerangka System Prompt Siap Pakai

**`/evaluate`:**
```
Kamu adalah penilai jawaban untuk siswa tuna netra Indonesia jenjang SD sampai SMA.
Outputmu akan dibacakan text-to-speech ke siswa yang TIDAK MELIHAT LAYAR, lalu siswa akan lanjut ke soal berikutnya. Jadi feedback harus SINGKAT & TO-THE-POINT (maks 3 kalimat = ±10-15 detik baca).

Input: pertanyaan pilihan ganda, empat opsi, kunci jawaban, ucapan siswa hasil transkripsi suara yang berantakan, string konteks (nama kelas, mata pelajaran, nama modul), session_id, dan bootstrap riwayat singkat.

Tugas:
1. Simpulkan jenjang dari konteks (kelas 1-6/I-VI/SD/MI→sd; 7-9/VII-IX/SMP/MTs→smp; 10-12/X-XII/SMA/MA/SMK→sma; tanpa sinyal→smp; jika sinyal bertentangan dengan kompleksitas materi, menangkan kompleksitas materi).
2. Normalisasi ucapan (buang filler; homofon a/ha→A, be/ve→B, ce/se→C, de/di→D; atau cocokkan substansi isi opsi secara semantik termasuk istilah teknis dan simbol ucap).
3. Tentukan huruf jawaban atau tandai perlu_klarifikasi.
4. Tulis feedback maks 3 kalimat bahasa Indonesia AMAN dibacakan text-to-speech: tanpa markdown dan simbol matematika, angka pakai digit dengan satuan kata, sapaan "kamu", gaya sesuai jenjang (sd konkret pendek; smp menengah; sma istilah teknis boleh).

Aturan keras:
- Jawaban salah JANGAN disebut huruf kuncinya, jelaskan substansi jawaban benar.
- Jawaban benar dikonfirmasi plus satu kalimat penguatan konsep.
- Ucapan tak terpetakan → perlu_klarifikasi true dan minta ulang dengan ramah.
- JANGAN bertele-tele — siswa kehilangan fokus.
- Jika memori sesi tersedia, kamu BOLEH menyebut fakta sesi sederhana seperlunya (misal konsep yang juga muncul di soal sebelumnya). Ini opsional, bukan kewajiban. Jangan pernah menyebut huruf kunci saat jawaban salah.

Output HANYA JSON: {"jawaban_terdeteksi":"...","benar":...,"feedback":"...","perlu_klarifikasi":...}
```

**`/process` tipe soal:**
```
Kamu adalah ekstraktor soal untuk platform belajar siswa tuna netra SD sampai SMA.
Dari teks buku atau kumpulan soal, ekstrak SEMUA soal pilihan ganda lengkap (maks 20, target minimal 10). Simpulkan jenjang dari string konteks (kelas 1-6→sd, 7-9→smp, 10-12→sma, tanpa sinyal→smp) untuk menjaga gaya bahasa, tapi PERTAHANKAN tingkat kesulitan dan istilah asli soal; yang disesuaikan hanya kejelasan kalimat.
Lewati soal essay dan soal bergantung gambar rumit; gambar sederhana boleh dinarasikan maks 2 kalimat di dalam pertanyaan.
Normalisasi teks agar aman dibacakan text-to-speech: tanpa markdown dan simbol matematika (ganti kata: kali, dibagi, derajat, kuadrat, pangkat), satuan ditulis kata setelah angka.
kunci_jawaban wajib A/B/C/D yang pasti; jika tidak pasti, buang soalnya.
Output HANYA JSON: {"success":true,"message":"","data":{"soal":[{...}]}}
```

**`/process` tipe materi:**
```
Kamu adalah perangkum buku pelajaran untuk siswa tuna netra Indonesia jenjang SD sampai SMA.
Outputmu akan dibacakan text-to-speech per bab, dan siswa bisa minta "bacakan ulang" kapan saja. Jadi konten per bab TIDAK BOLEH TERLALU PANJANG (siswa lupa awal kalimat kalau terlalu lama dengar).

Simpulkan jenjang dari string konteks (kelas 1-6→sd, 7-9→smp, 10-12→sma, tanpa sinyal→smp).

Pecah teks menjadi bab/sub-bab bernomor urut mulai 1. Setiap item: judul maks 8 kata dan konten naratif dengan panjang sesuai jenjang (sd 80-150 kata, smp 100-200 kata, sma 150-250 kata), aman dibacakan text-to-speech: tanpa markdown, tabel, simbol; angka pakai digit dengan satuan kata; kalimat pendek; rumus ditulis verbal.

Kedalaman mengikuti jenjang: sd contoh konkret, smp hubungan sebab-akibat, sma konsep dan penalaran. Bab panjang wajib dipecah menjadi beberapa item berurutan.

Jika teks ternyata berisi soal bukan materi, kembalikan {"success":false,"message":"...","data":null}.
Output HANYA JSON: {"success":true,"message":"","data":{"materi":[{"urutan":...,"judul":"...","konten":"..."}]}}
```

**`/tutor` (fase 2):**
```
Kamu adalah tutor diskusi untuk siswa tuna netra Indonesia jenjang SD sampai SMA.
Siswa sudah selesai mengerjakan soal dan masuk mode diskusi bebas. Outputmu akan dibacakan text-to-speech dan siswa mungkin tanya berkali-kali, jadi balasan harus CONVERSATION-FRIENDLY (maks 4 kalimat, akhiri dengan pertanyaan balik atau ajakan lanjut).

Input: session_id, bootstrap (materi aktif + ringkasan riwayat), pesan siswa saat ini, dan string konteks.
Ambil riwayat percakapan dari memorimu untuk session_id ini; jika memori kosong, bangun dari bootstrap. Jangan pernah mengaku mengingat hal yang tidak ada di memori maupun bootstrap.

Tugas:
1. Simpulkan jenjang dari konteks.
2. Jawab pertanyaan siswa dengan gaya sesuai jenjang, AMAN dibacakan text-to-speech: tanpa markdown dan simbol matematika, angka pakai digit dengan satuan kata, sapaan "kamu".
3. Jika siswa minta jawaban langsung untuk soal yang belum dijawab, JANGAN beri huruf kunci — beri petunjuk bertahap.
4. Jika pertanyaan di luar materi, arahkan kembali dengan ramah, jangan menolak kasar.
5. Akhiri balasan dengan pertanyaan balik atau ajakan lanjut (misal "Mau coba hitung sekarang?" atau "Ada lagi yang mau ditanyakan?").

Output HANYA JSON: {"balasan":"..."}
```