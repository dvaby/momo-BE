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
**Versi:** 1.2 · **Tanggal:** 17 September 2026 · **Ditujukan untuk:** Tim AI Service / Konfigurasi LLM
**Status backend:** Production (`https://momo-be-production.up.railway.app`) · Demo lomba: **H-3 (20 Sept)**
**Riwayat:** v1.0 (asumsi SD) → v1.1 (multi-jenjang, field jenjang eksplisit) → **v1.2 (tanpa field jenjang; inferensi dari string konteks — nol migrasi DB)**

---

## 1. Konteks Produk

- **Momo** adalah platform belajar online untuk **anak tuna netra jenjang SD, SMP, dan SMA** (usia ±6–18 tahun).
- Satu platform multi-jenjang: dari penjumlahan (SD) sampai fisika, kimia, dan analisis teks (SMA).
- Seluruh interaksi siswa berbasis **SUARA**: soal dibacakan Text-to-Speech (TTS), jawaban masuk sebagai teks bebas hasil Speech-to-Text (STT). Siswa tidak melihat layar.
- Semua output LLM pada akhirnya **DIDENGAR**. Format visual (markdown, tabel, simbol) = racun.
- **Tidak ada field jenjang terstruktur di backend.** Jenjang disimpulkan AI dari string `konteks` (§2.2).

---

## 2. Prinsip Audio-First & Adaptif-Jenjang

### 2.1 Aturan TTS-safe (semua jenjang)
1. **DILARANG:** markdown (`#`, `*`, `-`, `>`), tabel, emoji, kode, bullet simbol.
2. **DILARANG simbol matematika:** `× ÷ → √ ° ± ≠ ≤ ≥`. Ganti kata: "kali", "dibagi", "menjadi", "akar", "derajat", "kurang lebih".
3. **Angka & satuan:** angka dengan digit, satuan dengan kata setelahnya. ✅ "40.000 rupiah" ❌ "Rp40.000" · ✅ "0,5 sekon" ❌ "0.5 s" · ✅ "25 derajat celsius" ❌ "25°C".
4. **Pangkat & pecahan ucap-friendly:** "x²" → "x kuadrat" · "10³" → "sepuluh pangkat tiga" · "1/2" → "satu per dua".
5. **Tanpa instruksi visual:** dilarang "perhatikan gambar di atas", "lihat tabel berikut".

### 2.2 Inferensi jenjang dari string `konteks` (PENGGANTI field jenjang)
Backend mengirim field `konteks` berisi penamaan yang sudah ada di sistem, contoh:
- `/process`: `"Modul: Getaran dan Gelombang | Kelas tertaut: Kelas 11 IPA 2 (Fisika)"`
- `/evaluate`: `"Kelas: Kelas 5 Matematika (Matematika) | Modul: Ipa"`

AI WAJIB menyimpulkan jenjang dengan tabel berikut, lalu menyesuaikan gaya bahasa & kedalaman konsep:

| Sinyal dalam `konteks` | Jenjang |
|---|---|
| Kelas 1–6, romawi I–VI, kata "satu"–"enam", "SD", "MI" | **SD** |
| Kelas 7–9, romawi VII–IX, "SMP", "MTs" | **SMP** |
| Kelas 10–12, romawi X–XII, "SMA", "MA", "SMK" | **SMA** |
| Tidak ada sinyal sama sekali | **SMP** (default aman) |

Gaya per jenjang:
| | **SD** | **SMP** | **SMA** |
|---|---|---|---|
| Panjang kalimat | ≤ 15 kata | ≤ 20 kata | ≤ 25 kata |
| Kosakata | konkret, sehari-hari | istilah pelajaran + penjelasan singkat | istilah teknis kurikulum boleh langsung |
| Analogi | wajib konkret | dianjurkan | opsional |
| Nada | ramah menyemangati, "kamu" | ramah-menengah, "kamu" | muda-respektful, "kamu" |
| Kedalaman | fakta & contoh | konsep & sebab-akibat | konsep, rumus verbal, penalaran |

Aturan keras: **jangan pernah menebak SD untuk konten yang jelas SMA** (terdengar merendahkan). Jika sinyal konteks dan kompleksitas materi bertentangan, menangkan kompleksitas materi.

---

## 3. Peta Endpoint & Kontrak JSON (SUMBER KEBENARAN = BACKEND GO)

Response WAJIB JSON murni — tanpa markdown fence, tanpa teks di luar JSON. Field tak dikenal diabaikan backend; field wajib hilang = gagal proses.

### 3.1 `POST /process` — tipe = `"materi"`
**Request:**
```json
{
  "tipe": "materi",
  "teks_mentah": "<seluruh teks hasil ekstraksi PDF>",
  "konteks": "Modul: Ipa | Kelas tertaut: Kelas 5 Matematika (Matematika)"
}
```
**Response WAJIB:**
```json
{
  "success": true,
  "message": "",
  "data": { "materi": [ { "urutan": 1, "judul": "...", "konten": "<narasi sesuai jenjang terinferensi>" } ] }
}
```

### 3.2 `POST /process` — tipe = `"soal"`
**Request:**
```json
{ "tipe": "soal", "teks_mentah": "<teks ekstraksi PDF>", "konteks": "Modul: Fisika | Kelas tertaut: Kelas 11 IPA 2 (Fisika)" }
```
**Response WAJIB:**
```json
{
  "success": true,
  "message": "",
  "data": {
    "soal": [
      { "pertanyaan": "...", "pilihan_a": "...", "pilihan_b": "...", "pilihan_c": "...", "pilihan_d": "...", "kunci_jawaban": "A|B|C|D" }
    ]
  }
}
```

### 3.3 `POST /evaluate` — menilai jawaban suara siswa
**Request:**
```json
{
  "pertanyaan": "...",
  "pilihan_a": "...", "pilihan_b": "...", "pilihan_c": "...", "pilihan_d": "...",
  "kunci_jawaban": "A|B|C|D",
  "jawaban_mentah": "<teks bebas hasil STT>",
  "konteks": "Kelas: Kelas 5 Matematika (Matematika) | Modul: Ipa"
}
```
**Response WAJIB:**
```json
{ "jawaban_terdeteksi": "A|B|C|D|\"\"", "benar": true|false, "feedback": "<audio-first, gaya jenjang terinferensi>", "perlu_klarifikasi": false }
```
`perlu_klarifikasi` opsional: `true` + `jawaban_terdeteksi: ""` bila ucapan tak terpetakan ke opsi mana pun.

### 3.4 `POST /tutor` — USULAN FASE 2 (chat pemandu belajar)
Belum ada di backend; route proxy menyusul setelah AI siap. Request: `{ materi_aktif, riwayat_jawaban, pesan_siswa, konteks }`. Response: `{ "balasan": "<maks 4 kalimat, audio-first, gaya jenjang>" }`. Tidak pernah menyebut huruf kunci; pertanyaan di luar materi diarahkan kembali dengan ramah.

---

## 4. Spesifikasi Perilaku per Endpoint

### 4.1 `/process` tipe materi
- Pecah menjadi bab/sub-bab berurutan (`urutan` mulai 1).
- Panjang konten per item sesuai jenjang terinferensi: SD 80–150 kata · SMP 100–200 kata · SMA 150–250 kata. Item lebih panjang WAJIB dipecah (satu item = satu kali tekan "bacakan ulang").
- Narasi murni (§2). Tabel/diagram dikonversi ke kalimat; lewati bila tak bisa dinarasikan.
- Kedalaman sesuai jenjang: SD contoh konkret; SMA boleh rumus verbal ("kecepatan sama dengan jarak dibagi waktu").
- PDF berisi SOAL bukan materi → `success: false` + `message`.

### 4.2 `/process` tipe soal
- **Ekstrak SEMUA soal valid, minimal 10, maksimal 20 per request.** ⚠️ Perilaku saat ini ±2 soal per PDF — **blocker demo.**
- Hanya pilihan ganda lengkap (4 opsi + kunci pasti). Essay/isian → lewati.
- Soal bergantung gambar: narasikan maks 2 kalimat bila mungkin; lewati bila gambar teknis rumit.
- Normalisasi TTS-safe (§2) tanpa mengubah substansi angka/konsep.
- **Pertahankan tingkat kesulitan asli.** Yang disesuaikan kejelasan kalimat, bukan kedalaman konsep — soal SMA tidak boleh "dikecilkan" jadi bahasa SD.
- `kunci_jawaban` WAJIB A/B/C/D pasti; tidak pasti → buang soal.

### 4.3 `/evaluate`
**Deteksi jawaban (normalisasi STT §5):** prioritas 1 huruf/fonetik ("be", "ce", "de", "ha"); prioritas 2 substansi isi opsi (cocok semantik, termasuk istilah teknis); prioritas 3 tak ada sinyal → `perlu_klarifikasi: true`. Dua huruf disebut → ambil pilihan final.
**Feedback (maks 3 kalimat, gaya jenjang terinferensi):**
- Benar: konfirmasi + SATU kalimat penguatan konsep.
  - SD: "Betul! Saldo akhir Dika memang berkurang tujuh belas ribu rupiah karena pengeluaran lebih besar dari pemasukan."
  - SMA: "Tepat. Resultan gaya nol berarti benda setimbang, sesuai hukum pertama Newton."
- Salah: JANGAN sebut huruf kunci; jelaskan substansi jawaban benar + kalimat semangat.
- Perlu klarifikasi: "Maaf, aku belum menangkap jawabanmu. Sebutkan huruf a, be, ce, atau de, atau bacakan isi jawaban yang kamu pilih ya."

### 4.4 `/tutor` (fase 2): lihat §3.4; petunjuk bertahap, bukan huruf kunci.

---

## 5. Robustness Input STT

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

## 6. Performa & Batas

| Endpoint | Target | Batas keras |
|---|---|---|
| `/evaluate` | p50 < 5 dtk, p95 < 10 dtk | 30 dtk |
| `/process` | < 60 dtk | 120 dtk (timeout backend) |
| `/tutor` | p50 < 4 dtk | 15 dtk |

Teks panjang → chunking internal. `/process` > 100 dtk → hasil partial `success: true` daripada timeout. `/evaluate` pakai model cepat.

---

## 7. Keamanan & Privasi

1. Stateless per request: jangan persisten teks buku, jawaban siswa, riwayat chat.
2. Feedback salah tidak membocorkan huruf kunci.
3. `/process` soal boleh memuat `kunci_jawaban` (hanya backend yang melihat).
4. Data pribadi siswa tidak masuk prompt/log pihak ketiga.

---

## 8. Acceptance Criteria

1. ✅ JSON valid 100% pada 50 sample acak.
2. ✅ Nol karakter terlarang §2 pada field yang sampai ke siswa.
3. ✅ `/process` soal ≥ 10 soal dari PDF kumpulan 15–40 halaman (**blocker demo**).
4. ✅ Akurasi deteksi huruf ≥ 95% per jenjang pada set kasus §5.
5. ✅ Feedback salah tidak mengandung huruf kunci.
6. ✅ **Inferensi jenjang benar pada 20 sample nama kelas nyata** (format angka, romawi, kata: "Kelas 11 IPA 2", "Kelas VII B", "kelas lima", "XII IPA 3") — review manual gaya bahasa per jenjang, nol sample SMA bergaya SD dan sebaliknya.
7. ✅ Latensi sesuai §6 pada 20 request beruntun.
8. ✅ `perlu_klarifikasi` tidak pernah menghasilkan huruf acak.

---

## 9. Checklist Implementasi

- [ ] Terima field `konteks` di semua endpoint; inferensi jenjang sesuai tabel §2.2; default SMP bila tanpa sinyal
- [ ] System prompt `/process` materi: TTS-safe + panjang konten per jenjang + pemecahan sub-bab
- [ ] System prompt `/process` soal: limit 10–20, soal gambar, validasi kunci, pertahankan tingkat kesulitan asli
- [ ] System prompt `/evaluate`: normalisasi STT §5, prioritas deteksi §4.3, gaya feedback per jenjang
- [ ] Field opsional `perlu_klarifikasi`
- [ ] Validator JSON output (retry sekali)
- [ ] Set kasus uji per jenjang + 20 sample inferensi nama kelas + load test latensi
- [ ] Endpoint `/tutor` (fase 2)
- [ ] Deploy ke environment production backend

**Koordinasi lintas tim:**
- Backend: mulai mengirim `konteks` (rakitan nama kelas + mata pelajaran + nama modul dari data existing — tanpa kolom baru).
- FE guru: tambah placeholder hint di form buat kelas/modul: "Contoh: Kelas 11 IPA 2" (teks saja, bukan struktur) — memperbaiki kualitas inferensi jangka panjang.

---

## Lampiran A — Konvensi Pembacaan Huruf (AI + FE)
TTS membaca huruf opsi: A→"a" · B→"be" · C→"ce" · D→"de". AI memakai peta homofon SAMA (§5.3) saat memetakan ucapan siswa kembali ke huruf.

## Lampiran B — Kerangka System Prompt Siap Pakai

**`/evaluate`:**
```
Kamu adalah penilai jawaban untuk siswa tuna netra Indonesia jenjang SD sampai SMA.
Input: pertanyaan pilihan ganda, empat opsi, kunci jawaban, ucapan siswa hasil transkripsi suara yang berantakan, dan string konteks berisi nama kelas, mata pelajaran, dan nama modul.
Tugas: (1) simpulkan jenjang dari konteks (kelas 1-6/I-VI/SD/MI→sd; 7-9/VII-IX/SMP/MTs→smp; 10-12/X-XII/SMA/MA/SMK→sma; tanpa sinyal→smp; jika sinyal bertentangan dengan kompleksitas materi, menangkan kompleksitas materi); (2) normalisasi ucapan (buang filler; homofon a/ha→A, be/ve→B, ce/se→C, de/di→D; atau cocokkan substansi isi opsi secara semantik termasuk istilah teknis dan simbol ucap); (3) tentukan huruf jawaban atau tandai perlu_klarifikasi; (4) tulis feedback maks 3 kalimat bahasa Indonesia AMAN dibacakan text-to-speech: tanpa markdown dan simbol matematika, angka pakai digit dengan satuan kata, sapaan "kamu", gaya sesuai jenjang (sd konkret pendek; smp menengah; sma istilah teknis boleh).
Aturan keras: jawaban salah JANGAN disebut huruf kuncinya, jelaskan substansi jawaban benar; jawaban benar dikonfirmasi plus satu kalimat penguatan konsep; ucapan tak terpetakan → perlu_klarifikasi true dan minta ulang dengan ramah.
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
Simpulkan jenjang dari string konteks (kelas 1-6→sd, 7-9→smp, 10-12→sma, tanpa sinyal→smp).
Pecah teks menjadi bab/sub-bab bernomor urut mulai 1. Setiap item: judul maks 8 kata dan konten naratif dengan panjang sesuai jenjang (sd 80-150 kata, smp 100-200 kata, sma 150-250 kata), aman dibacakan text-to-speech: tanpa markdown, tabel, simbol; angka pakai digit dengan satuan kata; kalimat pendek; rumus ditulis verbal.
Kedalaman mengikuti jenjang: sd contoh konkret, smp hubungan sebab-akibat, sma konsep dan penalaran. Bab panjang wajib dipecah menjadi beberapa item berurutan.
Jika teks ternyata berisi soal bukan materi, kembalikan {"success":false,"message":"...","data":null}.
Output HANYA JSON: {"success":true,"message":"","data":{"materi":[{"urutan":...,"judul":"...","konten":"..."}]}}
```