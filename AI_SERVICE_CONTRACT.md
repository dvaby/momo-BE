# Kontrak AI Service — Momo-BE

Ini adalah kontrak **yang diharapkan BE dari AI Service kamu** (bukan endpoint publik BE). Diambil persis dari struct Go di `pkg/aiclient/types.go`, dan sudah teruji lewat Mock Server milik tim BE sendiri.

BE akan memanggil AI Service di alamat yang diset lewat env var `AI_SERVICE_URL`.

---

## 1. `POST {AI_SERVICE_URL}/process` — Ekstraksi PDF Materi/Soal

Dipanggil saat Guru upload PDF materi atau soal. BE sudah mengekstrak PDF jadi teks polos (pakai `pdftotext`) sebelum mengirim ke sini — AI Service **tidak perlu menangani parsing PDF**, cukup terima teks.

### Request dari BE:
```json
{
  "tipe": "materi",
  "teks_mentah": "isi teks hasil ekstraksi PDF di sini..."
}
```
`tipe` selalu salah satu dari `"materi"` atau `"soal"`.

### Response yang diharapkan — untuk `tipe: "materi"`:
```json
{
  "success": true,
  "data": {
    "materi": [
      { "urutan": 1, "judul": "Pengenalan", "konten": "..." },
      { "urutan": 2, "judul": "Pembahasan", "konten": "..." }
    ]
  }
}
```

### Response yang diharapkan — untuk `tipe: "soal"`:
```json
{
  "success": true,
  "data": {
    "soal": [
      {
        "pertanyaan": "Apa itu fotosintesis?",
        "pilihan_a": "...", "pilihan_b": "...", "pilihan_c": "...", "pilihan_d": "...",
        "kunci_jawaban": "B"
      }
    ]
  }
}
```
`kunci_jawaban` selalu 1 karakter: `"A"`, `"B"`, `"C"`, atau `"D"`.

### Kalau gagal / teks tidak cocok dengan `tipe` yang diminta:
```json
{
  "success": false,
  "message": "penjelasan kenapa gagal, akan diteruskan BE ke Guru",
  "data": { "materi": [], "soal": [] }
}
```
Field `message` **sangat direkomendasikan** diisi untuk kasus gagal — BE akan meneruskan pesan ini langsung ke Guru. Kalau kosong, BE pakai pesan generik.

**Penting:** BE mengandalkan AI Service untuk mendeteksi kalau teks yang dikirim (misal materi bacaan biasa) **tidak cocok** dengan `tipe` yang diminta (misal diminta `"soal"`). BE juga punya pengaman tambahan (menolak kalau array hasil kosong), tapi deteksi yang lebih akurat di sisi AI Service (bukan sekadar panjang teks) akan memberi pengalaman error yang jauh lebih baik untuk Guru.

---

## 2. `POST {AI_SERVICE_URL}/evaluate` — Evaluasi Jawaban Siswa

Dipanggil saat Siswa submit jawaban. Siswa menjawab dengan **kalimat bebas** (hasil Speech-to-Text) — bukan harus menyebut huruf pilihan secara eksplisit. Tugas AI Service: pahami maksud jawaban siswa, cocokkan ke salah satu pilihan, lalu evaluasi.

### Request dari BE:
```json
{
  "pertanyaan": "Apa fungsi klorofil dalam fotosintesis?",
  "pilihan_a": "...", "pilihan_b": "...", "pilihan_c": "...", "pilihan_d": "...",
  "kunci_jawaban": "B",
  "jawaban_siswa_mentah": "aku rasa jawabannya yang B"
}
```
`jawaban_siswa_mentah` adalah teks apa adanya dari siswa — bisa berupa kalimat natural, bukan cuma satu huruf.

### Response yang diharapkan:
```json
{
  "success": true,
  "data": {
    "jawaban_terdeteksi": "B",
    "benar": true,
    "feedback": "Betul! Jawaban kamu tepat."
  }
}
```
- `jawaban_terdeteksi` — pilihan (`A`/`B`/`C`/`D`) yang menurut AI paling sesuai dengan maksud `jawaban_siswa_mentah`.
- `benar` — hasil pencocokan `jawaban_terdeteksi` dengan `kunci_jawaban`.
- `feedback` — teks yang akan **dibacakan lewat Text-to-Speech** ke siswa (target pengguna: siswa tunanetra) — buat singkat, jelas, dan enak didengar, bukan teks panjang formal.

### Kalau gagal:
```json
{
  "success": false,
  "message": "penjelasan kenapa gagal",
  "data": { "jawaban_terdeteksi": "", "benar": false, "feedback": "" }
}
```

---

## Catatan Umum

- Semua request/response murni JSON, tidak ada file/multipart di kontrak ini.
- BE menunggu response maksimal **30 detik** sebelum timeout — pastikan AI Service merespons dalam batas waktu ini.
- Field bahasa Indonesia (`pertanyaan`, `kunci_jawaban`, dst.) dipertahankan konsisten dengan struktur data di sisi BE, supaya tidak perlu mapping tambahan.
- Kalau ada penyesuaian field yang diperlukan dari sisi AI Service kamu, diskusikan dengan tim BE — perubahan di titik ini relatif murah dilakukan di sisi BE (terisolasi di satu file `pkg/aiclient/types.go`).
