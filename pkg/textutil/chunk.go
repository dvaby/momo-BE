package textutil

import (
	"strings"
)

// ChunkText memecah teks panjang menjadi beberapa chunk berdasarkan batas karakter,
// tapi berusaha tidak memotong di tengah kalimat (cari newline terdekat).
// maxChars: batas maksimum karakter per chunk
// overlap: jumlah karakter overlap antar chunk (untuk konteks)
func ChunkText(text string, maxChars int, overlap int) []string {
	if len(text) <= maxChars {
		return []string{text}
	}

	var chunks []string
	start := 0

	for start < len(text) {
		end := start + maxChars

		// Kalau sudah sampai akhir teks
		if end >= len(text) {
			chunks = append(chunks, text[start:])
			break
		}

		// Cari newline terdekat sebelum 'end' untuk tidak memotong kalimat
		lastNewline := strings.LastIndex(text[start:end], "\n")
		if lastNewline > maxChars/2 { // kalau newline ada di separuh kedua chunk
			end = start + lastNewline + 1
		} else {
			// Tidak ada newline yang bagus, cari titik atau tanda baca lain
			for i := end - 1; i > start+maxChars/2; i-- {
				if text[i] == '.' || text[i] == '!' || text[i] == '?' {
					end = i + 1
					break
				}
			}
		}

		chunks = append(chunks, text[start:end])

		// Geser start dengan overlap
		start = end - overlap
		if start >= len(text) {
			break
		}
	}

	return chunks
}
