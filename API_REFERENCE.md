📚 API Reference — Momo-BE (Updated)
Dokumen ini disusun berdasarkan testing langsung terhadap kode yang berjalan.
Info Umum
Base URL (Dev)
http://localhost:8080
Base URL (Production)
🆕 https://momo-be-production.up.railway.app
Format data
JSON (application/json) atau multipart/form-data (upload PDF)
Autentikasi
JWT via header Authorization: Bearer <token>
Dua Jenis Token JWT
Token Guru
Token Siswa
Didapat dari
POST /api/v1/guru/login
POST /api/v1/join
Isi claim
guru_id, role: "guru"
siswa_id, kelas_id
Lokasi di Response
⚠️ data.token
⚠️ token (di root)
Dipakai untuk
Kelola Modul, Kelas, Nilai
Ambil Soal, Submit Jawaban
CORS
Origin yang diizinkan: http://localhost:3000, http://localhost:5173, dan semua *.vercel.app.
A. Auth & Registrasi
POST /api/v1/guru/login
Request:
json

1
Response sukses (200):
json

123456
POST /api/v1/join (Siswa masuk Kelas)
Request:
json

1
Response sukses (200):
json

123456
B. Modul (Butuh Token Guru)
POST /api/v1/modul/:id/soal?jenis=harian
Upload PDF soal. Content-Type: multipart/form-data, field file file.
Response sukses (202 Accepted):
json

1234
💡 TIPS UNTUK FE: Jangan melakukan polling (fetch berulang) ke endpoint GET /modul/:id. Gunakan endpoint SSE (Server-Sent Events) di bawah ini untuk mendapatkan notifikasi real-time saat AI selesai memproses.
🆕 GET /api/v1/modul/:id/stream (Real-time Status via SSE)
Endpoint ini menggunakan Server-Sent Events. FE cukup membuka koneksi ini setelah upload, dan server akan "mendorong" status secara otomatis.
Cara pakai di JavaScript (Frontend):
javascript

123456789101112131415161718
C. Kelas (Butuh Token Guru)
⚠️ PENTING: Field request tetap menggunakan nama, tetapi response akan mengembalikannya sebagai nama_kelas. Ini adalah perilaku yang sudah fix.
🆕 POST /api/v1/kelas (Buat Kelas)
Request: (Field mata_pelajaran sekarang wajib)
json

1234
Response (201 Created):
json

12345678
🆕 GET /api/v1/kelas (List Semua Kelas Guru)
Response (200 OK):
json

123456
GET /api/v1/kelas/:id (Detail Kelas)
Response (200 OK): Mengembalikan detail kelas termasuk array siswa dan array modul yang sudah di-assign.
🆕 PUT /api/v1/kelas/:id (Update Kelas)
Request: (Kirim hanya field yang ingin diubah)
json

1234
Response (200 OK):
json

1234
🆕 DELETE /api/v1/kelas/:id (Hapus Kelas)
Response (200 OK):
json

1
(Catatan: Menghapus kelas juga akan menghapus relasi siswa di dalamnya secara cascade tergantung setup DB).
POST /api/v1/kelas/:id/siswa (Daftarkan Siswa Manual)
Request: { "nama": "Budi Santoso" }
Response (201): { "id": 1, "kelas_id": 3, "nama": "Budi Santoso" }
POST /api/v1/kelas/:id/modul (Assign Modul ke Kelas)
Request: { "modul_id": 2 }
Response (200): { "message": "Modul berhasil ditautkan ke kelas" }
🆕 DELETE /api/v1/kelas/:id/modul/:modul_id (Hapus Modul dari Kelas)
Melepaskan tautan modul dari kelas tertentu tanpa menghapus modul itu sendiri dari database.
Response (200 OK):
json

1
GET /api/v1/kelas/:id/nilai?modul_id=X&jenis=harian
Response (200 OK):
json

123456789
D. Alur Siswa (Butuh Token Siswa)
GET /api/v1/modul/:id/soal?jenis=harian
Mengambil soal untuk dikerjakan.
⚠️ Keamanan: Response TIDAK PERNAH mengandung field kunci_jawaban. Opsi jawaban (pilihan_a s/d d) juga sudah diacak (shuffled) oleh backend sebelum dikirim.
POST /api/v1/submit-jawaban
Request:
json

1234
(Field siswa_id tidak perlu dikirim, backend mengambilnya otomatis dari Token JWT).
Response sukses (200/201):
json

12345678910
📝 Ringkasan Perubahan untuk Tim Frontend
URL Production Sudah Fix: Gunakan https://momo-be-production.up.railway.app.
Form Buat Kelas: Tambahkan input field mata_pelajaran. Payload sekarang wajib mengirim { "nama": "...", "mata_pelajaran": "..." }.
Halaman Daftar Kelas: Sekarang FE bisa menampilkan tombol Edit (PUT) dan Hapus (DELETE) untuk setiap kelas, serta tombol Hapus Modul dari kelas.
Stop Polling, Mulai Pakai SSE: Untuk fitur upload soal, ganti logika setInterval (polling) dengan EventSource ke endpoint /modul/:id/stream agar lebih efisien, real-time, dan tidak membebani server.
Asimetri Nama Field Tetap Ada: Ingat, request pakai nama, response balikin nama_kelas. Ini sudah final.