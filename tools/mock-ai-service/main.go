package main

import (
	"encoding/json"
	"log"
	"net/http"
	"regexp"
)

type request struct {
	Tipe       string `json:"tipe"`
	TeksMentah string `json:"teks_mentah"`
}

// Pola ini mencari blok "a. ... b. ... c. ... d." yang muncul berurutan
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

func main() {
	http.HandleFunc("/process", handleProcess)
	log.Println("Mock AI Service jalan di :8000")
	log.Fatal(http.ListenAndServe(":8000", nil))
}