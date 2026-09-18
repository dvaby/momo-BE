package errors

import "net/http"

// ============================================
// STANDARD ERROR CODES UNTUK MOMO BE
// ============================================

// ---- Client Errors (FE salah - 4xx) ----
const (
	CodeInvalidRequest   = "INVALID_REQUEST"       // Request body invalid / tidak valid JSON
	CodeMissingField     = "MISSING_FIELD"         // Field wajib tidak diisi
	CodeInvalidFormat    = "INVALID_FORMAT"        // Format data salah (email, tanggal, dll)
	CodeInvalidFile      = "INVALID_FILE"          // File tidak valid (ukuran/tipe)
	CodeUnauthorized     = "UNAUTHORIZED"          // Tidak ada token / token invalid
	CodeForbidden        = "FORBIDDEN"             // Token valid tapi tidak punya akses
	CodeNotFound         = "NOT_FOUND"             // Resource tidak ditemukan
	CodeDuplicate        = "DUPLICATE"             // Data sudah ada (email, kode kelas, dll)
	CodeAlreadyAnswered  = "ALREADY_ANSWERED"      // Soal UTS/UAS sudah dijawab
)

// ---- Server/AI Errors (AI Service salah - 5xx) ----
const (
	CodeAIProcessingFailed = "AI_PROCESSING_FAILED"   // AI gagal proses (parse/ekstrak)
	CodeAITimeout          = "AI_TIMEOUT"             // AI Service timeout
	CodeAIUnavailable      = "AI_UNAVAILABLE"         // AI Service down / tidak reachable
	CodeAIInvalidResponse  = "AI_INVALID_RESPONSE"    // Response AI tidak valid/tidak dikenali
)

// ---- Internal Errors (Backend salah - 5xx) ----
const (
	CodeInternalError = "INTERNAL_ERROR" // Error internal server
	CodeDatabaseError = "DATABASE_ERROR" // Error database
)

// ErrorResponse adalah format error response standar
type ErrorResponse struct {
	Code    string `json:"code"`    // Kode error (contoh: "AI_PROCESSING_FAILED")
	Message string `json:"message"` // Pesan error untuk user
	Source  string `json:"source"`  // Sumber error: "client" | "ai_service" | "server"
}

// NewClientError buat error yang disebabkan oleh FE (4xx)
func NewClientError(code, message string) ErrorResponse {
	return ErrorResponse{
		Code:    code,
		Message: message,
		Source:  "client",
	}
}

// NewAIError buat error yang disebabkan oleh AI Service (5xx)
func NewAIError(code, message string) ErrorResponse {
	return ErrorResponse{
		Code:    code,
		Message: message,
		Source:  "ai_service",
	}
}

// NewServerError buat error internal server (5xx)
func NewServerError(code, message string) ErrorResponse {
	return ErrorResponse{
		Code:    code,
		Message: message,
		Source:  "server",
	}
}

// GetHTTPStatus return HTTP status code berdasarkan error code
func GetHTTPStatus(code string) int {
	switch code {
	// Client errors → 4xx
	case CodeInvalidRequest, CodeMissingField, CodeInvalidFormat, CodeInvalidFile:
		return http.StatusBadRequest
	case CodeUnauthorized:
		return http.StatusUnauthorized
	case CodeForbidden:
		return http.StatusForbidden
	case CodeNotFound:
		return http.StatusNotFound
	case CodeDuplicate, CodeAlreadyAnswered:
		return http.StatusConflict
	
	// AI/Server errors → 5xx
	case CodeAIProcessingFailed, CodeAITimeout, CodeAIUnavailable, CodeAIInvalidResponse:
		return http.StatusServiceUnavailable
	case CodeInternalError, CodeDatabaseError:
		return http.StatusInternalServerError
	
	default:
		return http.StatusInternalServerError
	}
}