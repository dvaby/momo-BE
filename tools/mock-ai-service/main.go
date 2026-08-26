package main

import (
	"encoding/json"
	"log"
	"net/http"
	"regexp"
	"strings"
)

type request struct {
	Tipe       string `json:"tipe"`
	TeksMentah string `json:"teks_mentah"`
}

// Pola ini mencari blok "a. ... b. ... c. ... d." yang munculberurutan
// dalam jarak dekat (maksimal 200 karakter antar penanda) - ciri khas
// satu set pilihan ganda, bukan sekadar kata yang kebetulan berakhiran huruf itu.
var polaPilihanGanda = regexp.MustCompile(`(?is)a\.\s*\S.{0,200}?b\.\s*\S.{0,200}?c\.\s*\S.{0,200}?d\.\s*\S`)

func handleProcess(w http.ResponseWriter, r *http.Request) {
	var req request
	json.NewDecoder(r.Body).Decode(&req)

	log.Printf("Menerima request tipe: %s, panjang teks: %d karakter\n", req.Tipe, len(req.TeksMentah))

	w.Header().Set("Content-Type", "application/json")

	if req.Tipe == "soal" {
		jumlahBlokDitemukan := len(polaPilihanGanda.FindAllString(req.TeksMentah, -1))

		if jumlahBlokDitemukan == 0 {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Teks tidak menunjukkan pola blok pilihan ganda (a./b./c./d. berurutan)",
				"data":    map[string]interface{}{},
			})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data": map[string]interface{}{
				"soal": []map[string]interface{}{
					{
						"pertanyaan":    "Ini soal dummy dari Mock AI Service, apa jawabannya?",
						"pilihan_a":     "Pilihan A",
						"pilihan_b":     "Pilihan B (ini kunci)",
						"pilihan_c":     "Pilihan C",
						"pilihan_d":     "Pilihan D",
						"kunci_jawaban": "B",
					},
				},
			},
		})
		return
	}

	if req.Tipe == "materi" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data": map[string]interface{}{
				"materi": []map[string]interface{}{
					{"urutan": 1, "judul": "Bagian 1 (dari Mock)", "konten": "Ini konten materi hasil dummy dari mock AI Service."},
					{"urutan": 2, "judul": "Bagian 2 (dari Mock)", "konten": "Konten kedua untuk memastikan multi-item tersimpan dengan benar."},
				},
			},
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"message": "Tipe tidak dikenali, harus 'materi' atau 'soal'",
		"data":    map[string]interface{}{},
	})
}

type evaluateRequest struct {
	Pertanyaan         string `json:"pertanyaan"`
	PilihanA           string `json:"pilihan_a"`
	PilihanB           string `json:"pilihan_b"`
	PilihanC           string `json:"pilihan_c"`
	PilihanD           string `json:"pilihan_d"`
	KunciJawaban       string `json:"kunci_jawaban"`
	JawabanSiswaMentah string `json:"jawaban_siswa_mentah"`
}

// handleEvaluate adalah versi SANGAT SEDERHANA untuk keperluan development
// lokal saja — bukan mencerminkan kecerdasan LLM sungguhan milik AI Service
// asli. Heuristiknya: cek apakah teks jawaban mentah siswa mengandung huruf
// kunci jawaban (misal "B") ATAU mengandung isi teks pilihan yang benar.
// Ini cukup untuk testing alur BE, TIDAK untuk menguji akurasi evaluasi.
func handleEvaluate(w http.ResponseWriter, r *http.Request) {
	var req evaluateRequest
	json.NewDecoder(r.Body).Decode(&req)

	log.Printf("Menerima request evaluasi, jawaban mentah: %q\n", req.JawabanSiswaMentah)

	w.Header().Set("Content-Type", "application/json")

	if req.KunciJawaban == "" || req.JawabanSiswaMentah == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "kunci_jawaban dan jawaban_siswa_mentah wajib diisi",
			"data":    map[string]interface{}{},
		})
		return
	}

	jawabanLower := strings.ToLower(req.JawabanSiswaMentah)

	// Ambil teks pilihan yang sesuai kunci jawaban, untuk dicocokkan
	// isinya, bukan cuma hurufnya.
	pilihanBenarTeks := ""
	switch strings.ToUpper(req.KunciJawaban) {
	case "A":
		pilihanBenarTeks = req.PilihanA
	case "B":
		pilihanBenarTeks = req.PilihanB
	case "C":
		pilihanBenarTeks = req.PilihanC
	case "D":
		pilihanBenarTeks = req.PilihanD
	}

	terdeteksiBenar := strings.Contains(jawabanLower, strings.ToLower(req.KunciJawaban)) ||
		(pilihanBenarTeks != "" && strings.Contains(jawabanLower, strings.ToLower(pilihanBenarTeks)))

	jawabanTerdeteksi := req.KunciJawaban
	feedback := "Jawaban kamu kurang tepat, coba lagi ya."
	if !terdeteksiBenar {
		jawabanTerdeteksi = "?"
	} else {
		feedback = "Betul! Jawaban kamu tepat."
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"jawaban_terdeteksi": jawabanTerdeteksi,
			"benar":              terdeteksiBenar,
			"feedback":           feedback,
		},
	})
}

func main() {
	http.HandleFunc("/process", handleProcess)
	http.HandleFunc("/evaluate", handleEvaluate)
	log.Println("Mock AI Service jalan di :8000")
	log.Fatal(http.ListenAndServe(":8000", nil))
}