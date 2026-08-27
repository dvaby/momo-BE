Markdown
# MOMO-BE: Project Handoff & State Summary
**Date:** 26 August 2026
**Project:** Momo Backend (Golang)
**Status:** Core Backend MVP Completed (100%)

## 1. Tech Stack & Environment
* **Language:** Golang (1.22+)
* **Web Framework:** Gin Web Framework (`github.com/gin-gonic/gin`)
* **ORM:** GORM (`gorm.io/gorm`, `gorm.io/driver/postgres`)
* **Database:** PostgreSQL
* **Security:** `golang-jwt/jwt/v5`, `golang.org/x/crypto/bcrypt`
* **External Dependency:** `poppler-utils` (specifically `pdftotext` for PDF extraction)
* **Deployment:** Docker (Multi-stage Alpine)

## 2. Architecture Structure (Clean Architecture Pattern)
Proyek menggunakan pendekatan Clean Architecture dengan Dependency Injection manual di `main.go`.
```text
momo-BE/
├── cmd/api/
│   └── main.go                 # Entry point, Dependency Injection, Server Init
├── internal/
│   ├── config/                 # Load .env variables
│   ├── database/               # PostgreSQL Connection & AutoMigrate
│   ├── model/                  # GORM Structs & DTOs (Request/Response)
│   ├── repository/             # Database queries (GORM interfaces)
│   ├── service/                # Business logic
│   ├── handler/                # Gin HTTP handlers
│   ├── middleware/             # GuruAuthMiddleware & AuthMiddleware (Siswa)
│   └── router/                 # Gin route groupings
├── pkg/
│   ├── aiclient/               # HTTP client for external AI service
│   └── jwtutil/                # Token generation & verification
├── Dockerfile                  # Multi-stage build (golang:alpine -> alpine:3.19)
└── .dockerignore
3. Database Schema (GORM AutoMigrate)
Entitas yang telah dimigrasikan:

Guru: id, nama, email (unique), password (bcrypt).

Modul: id, judul, deskripsi.

Materi: id, modul_id, judul, konten_teks (hasil ekstrak PDF).

Soal: id, modul_id, jenis (harian/ujian), pertanyaan, kriteria_jawaban.

Kelas: id, nama_kelas, kode_kelas (unique).

Siswa: id, kelas_id, nama.

JawabanSiswa: id, siswa_id, soal_id, jawaban_teks, skor, feedback_ai.

4. Authentication Flow (Dual-Role JWT)
Sistem menggunakan dua entitas berbeda untuk otentikasi, di-handle oleh 2 middleware terpisah:

Guru (Role: Admin/Teacher):

Auth via Email + Password (Bcrypt).

Claims: GuruID, Role: "guru".

Middleware: middleware.GuruAuthMiddleware().

Siswa (Role: Student):

Auth via Kode Kelas + Nama Siswa (Join method).

Claims: SiswaID, KelasID.

Middleware: middleware.AuthMiddleware().

5. API Routes & Endpoints
Public Routes:

GET /health : Health check

POST /api/v1/guru/register : Registrasi Guru

POST /api/v1/guru/login : Login Guru (Returns JWT)

POST /api/v1/join : Siswa join kelas (Returns JWT)

GET /api/v1/modul : List modul

GET /api/v1/modul/:id : Detail modul

POST /api/v1/test-extract-pdf : Utility test pdftotext

Protected Routes - Guru (GuruAuthMiddleware):

POST /api/v1/modul : Create modul

POST /api/v1/modul/:id/materi : Upload PDF Materi & Extract to DB

POST /api/v1/modul/:id/soal : Upload PDF Soal & Extract to DB

POST /api/v1/kelas : Create kelas (generates kode_kelas)

GET /api/v1/kelas/:id : Get detail kelas

POST /api/v1/kelas/:id/siswa : Bulk register siswa ke kelas

GET /api/v1/kelas/:id/nilai?modul_id=X&jenis=Y : Dashboard Guru (Rekapitulasi Nilai Siswa)

Protected Routes - Siswa (AuthMiddleware):

GET /api/v1/modul/:id/soal : Get daftar soal

POST /api/v1/submit-jawaban : Submit jawaban essay. Trigger evaluasi ke AI service, simpan skor & feedback_ai ke DB.

6. Key Integrations & Logic Rules
AI Evaluator (pkg/aiclient):
Ketika siswa submit jawaban, BE mengirim payload (pertanyaan, jawaban_siswa, kriteria_jawaban) ke External AI Microservice (POST /evaluate). AI mengembalikan JSON {skor, feedback} yang langsung disimpan ke tabel JawabanSiswa.

PDF Extraction:
Menggunakan os/exec memanggil command pdftotext -layout <pdf_path> -. Wajib memastikan environment / OS / Docker memiliki library poppler-utils.

Dashboard Nilai Logic:
Endpoint /kelas/:id/nilai menangani pengambilan nilai siswa berdasarkan modul_id dan jenis (opsional). Jika siswa mengerjakan berkali-kali, query GORM mengambil percobaan terakhir (latest attempt) berdasarkan urutan ID terbesar per soal. Siswa yang belum mengerjakan mengembalikan nilai 0.

7. Docker / Deployment State
Dockerfile: Multi-stage build menggunakan golang:alpine sebagai builder dan alpine:3.19 sebagai runtime.

System Dependencies: Runtime image menginstall poppler-utils (untuk PDF extract), ca-certificates, dan tzdata (Timezone set ke Asia/Jakarta).

Status: Image berhasil di-build dan siap dijalankan (production-ready).

8. Next Action for Context (For the AI)
Proyek Backend Golang ini sudah beroperasi penuh secara fungsionalitas dan keamanan. Langkah selanjutnya yang bisa dilakukan dalam konteks sistem terdistribusi ini adalah:

Membangun Frontend (Web / Mobile) mengacu pada endpoints di atas.

Membangun dan menyempurnakan AI Microservice (Python FastAPI/Flask) yang memproses endpoint /evaluate.

Membuat docker-compose.yml untuk menggabungkan DB PostgreSQL, Golang BE, dan AI Microservice.