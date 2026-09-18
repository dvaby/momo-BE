package textutil

import "strings"

// SmartChunkText memecah teks menjadi chunks yang balanced (karakter merata)
// dengan tetap menghormati boundary kalimat.
// Target: semua chunks punya ukuran mirip untuk optimal parallel processing.
func SmartChunkText(text string, targetChunks int, minChunkSize, maxChunkSize int) []string {
	if len(text) <= maxChunkSize {
		return []string{text}
	}

	// Hitung target size per chunk
	targetSize := len(text) / targetChunks
	if targetSize < minChunkSize {
		targetSize = minChunkSize
	}
	if targetSize > maxChunkSize {
		targetSize = maxChunkSize
	}

	var chunks []string
	start := 0

	for start < len(text) {
		end := start + targetSize

		// Kalau sudah sampai akhir
		if end >= len(text) {
			chunks = append(chunks, text[start:])
			break
		}

		// Cari boundary yang bagus (newline, titik, tanda baca)
		bestBoundary := findBestBoundary(text, start, end, maxChunkSize)
		chunks = append(chunks, text[start:bestBoundary])
		start = bestBoundary
	}

	return chunks
}

// findBestBoundary cari posisi terbaik untuk memotong teks
func findBestBoundary(text string, start, targetEnd, maxEnd int) int {
	// Cari dalam range [targetEnd - 200, targetEnd + 200]
	searchStart := targetEnd - 200
	if searchStart < start {
		searchStart = start + 100 // minimal 100 chars
	}
	searchEnd := targetEnd + 200
	if searchEnd > len(text) {
		searchEnd = len(text)
	}
	if searchEnd > maxEnd {
		searchEnd = maxEnd
	}

	// Prioritas boundary (dari terbaik ke terburuk):
	// 1. Double newline (paragraph break)
	// 2. Single newline
	// 3. Titik + space
	// 4. Tanda baca lain + space
	
	searchRegion := text[searchStart:searchEnd]
	
	// Cari double newline
	if idx := strings.LastIndex(searchRegion, "\n\n"); idx > 0 {
		return searchStart + idx + 2
	}
	
	// Cari single newline
	if idx := strings.LastIndex(searchRegion, "\n"); idx > 0 {
		return searchStart + idx + 1
	}
	
	// Cari titik + space
	if idx := strings.LastIndex(searchRegion, ". "); idx > 0 {
		return searchStart + idx + 2
	}
	
	// Cari tanda tanya/seru + space
	if idx := strings.LastIndex(searchRegion, "? "); idx > 0 {
		return searchStart + idx + 2
	}
	if idx := strings.LastIndex(searchRegion, "! "); idx > 0 {
		return searchStart + idx + 2
	}
	
	// Cari spasi (word boundary)
	if idx := strings.LastIndex(searchRegion, " "); idx > 0 {
		return searchStart + idx + 1
	}
	
	// Fallback: potong di targetEnd
	return targetEnd
}