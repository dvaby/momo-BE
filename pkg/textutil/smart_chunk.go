package textutil

import "strings"

// SmartChunkText memecah teks menjadi chunks yang balanced (karakter merata)
// dengan tetap menghormati boundary kalimat.
// Target: semua chunks punya ukuran mirip untuk optimal parallel processing.
func SmartChunkText(text string, targetChunks int, minChunkSize, maxChunkSize int) []string {
	textLen := len(text)
	if textLen <= maxChunkSize {
		return []string{text}
	}

	// Hitung target size per chunk
	targetSize := textLen / targetChunks
	if targetSize < minChunkSize {
		targetSize = minChunkSize
	}
	if targetSize > maxChunkSize {
		targetSize = maxChunkSize
	}

	var chunks []string
	start := 0

	for start < textLen {
		// Target end untuk chunk ini
		targetEnd := start + targetSize
		if targetEnd > textLen {
			targetEnd = textLen
		}

		// Absolute max end untuk chunk ini (FIX: pakai start + maxChunkSize, bukan angka statis)
		maxEnd := start + maxChunkSize
		if maxEnd > textLen {
			maxEnd = textLen
		}

		// Kalau sudah sampai akhir
		if targetEnd <= start {
			chunks = append(chunks, text[start:])
			break
		}

		// Cari boundary yang bagus
		bestBoundary := findBestBoundary(text, start, targetEnd, maxEnd)

		// Fallback pencegah infinite loop: jika bestBoundary tidak maju, paksa maju minimal 1 karakter
		if bestBoundary <= start {
			bestBoundary = start + 1
			if bestBoundary > textLen {
				bestBoundary = textLen
			}
		}

		chunks = append(chunks, text[start:bestBoundary])
		start = bestBoundary
	}

	return chunks
}

// findBestBoundary cari posisi terbaik untuk memotong teks
func findBestBoundary(text string, start, targetEnd, maxEnd int) int {
	textLen := len(text)

	// Ensure bounds aman
	if start >= textLen {
		return textLen
	}
	if targetEnd > textLen {
		targetEnd = textLen
	}
	if maxEnd > textLen {
		maxEnd = textLen
	}

	// Cari dalam range [targetEnd - 200, targetEnd + 200]
	searchStart := targetEnd - 200
	if searchStart < start {
		searchStart = start
	}

	searchEnd := targetEnd + 200
	if searchEnd > maxEnd {
		searchEnd = maxEnd
	}
	if searchEnd > textLen {
		searchEnd = textLen
	}

	// 🛡️ CRITICAL FIX: Cegah slice bounds out of range
	// Jika searchStart >= searchEnd, berarti tidak ada ruang untuk mencari boundary.
	// Langsung return targetEnd (atau maxEnd) tanpa melakukan slicing.
	if searchStart >= searchEnd {
		if targetEnd <= maxEnd {
			return targetEnd
		}
		return maxEnd
	}

	searchRegion := text[searchStart:searchEnd]

	// Prioritas boundary (dari terbaik ke terburuk):
	// 1. Double newline (paragraph break)
	if idx := strings.LastIndex(searchRegion, "\n\n"); idx >= 0 {
		return searchStart + idx + 2
	}

	// 2. Single newline
	if idx := strings.LastIndex(searchRegion, "\n"); idx >= 0 {
		return searchStart + idx + 1
	}

	// 3. Titik + space
	if idx := strings.LastIndex(searchRegion, ". "); idx >= 0 {
		return searchStart + idx + 2
	}

	// 4. Tanda tanya/seru + space
	if idx := strings.LastIndex(searchRegion, "? "); idx >= 0 {
		return searchStart + idx + 2
	}
	if idx := strings.LastIndex(searchRegion, "! "); idx >= 0 {
		return searchStart + idx + 2
	}

	// 5. Spasi (word boundary)
	if idx := strings.LastIndex(searchRegion, " "); idx >= 0 {
		return searchStart + idx + 1
	}

	// Fallback: potong di targetEnd (pastikan tidak melebihi maxEnd)
	if targetEnd <= maxEnd {
		return targetEnd
	}
	return maxEnd
}