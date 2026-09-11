package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"momo-be/internal/model"
	"momo-be/internal/service"
)

// ===========================
// SSE HUB (Real-time Engine)
// ===========================

type SSEClient struct {
	events chan SSEEvent
	done   chan struct{}
}

type SSEEvent struct {
	Type string
	Data interface{}
}

type SSEHub struct {
	clients    map[*SSEClient]bool
	mu         sync.RWMutex
	register   chan *SSEClient
	unregister chan *SSEClient
	broadcast  chan SSEEvent
}

func NewSSEHub() *SSEHub {
	hub := &SSEHub{
		clients:    make(map[*SSEClient]bool),
		register:   make(chan *SSEClient),
		unregister: make(chan *SSEClient),
		broadcast:  make(chan SSEEvent, 100),
	}
	go hub.run()
	return hub
}

func (h *SSEHub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.events)
			}
			h.mu.Unlock()
		case event := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.events <- event:
				default:
					// Client lambat, skip
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Broadcast mengirim event ke semua subscriber
func (h *SSEHub) Broadcast(eventType string, data interface{}) {
	h.broadcast <- SSEEvent{Type: eventType, Data: data}
}

// ===========================
// KELAS HANDLER
// ===========================

type KelasHandler struct {
	service *service.KelasService
	sseHub  *SSEHub
}

func NewKelasHandler(service *service.KelasService, sseHub *SSEHub) *KelasHandler {
	return &KelasHandler{service: service, sseHub: sseHub}
}

// --- Request DTOs ---

type createKelasRequest struct {
	Nama          string `json:"nama" binding:"required"`
	MataPelajaran string `json:"mata_pelajaran" binding:"required"`
}

type updateKelasRequest struct {
	Nama          string `json:"nama"`
	MataPelajaran string `json:"mata_pelajaran"`
}

type assignModulRequest struct {
	ModulID uint `json:"modul_id" binding:"required"`
}

// --- Helper: kirim SSE event ke client ---

func writeSSEvent(w gin.ResponseWriter, eventType string, data interface{}) {
	jsonBytes, _ := json.Marshal(data)
	w.Write([]byte("event: " + eventType + "\n"))
	w.Write([]byte("data: " + string(jsonBytes) + "\n\n"))
	w.Flush()
}

// ===========================
// ENDPOINT: Subscribe SSE Stream
// GET /api/v1/kelas/stream
// ===========================

func (h *KelasHandler) StreamKelas(c *gin.Context) {
	// Set headers wajib untuk SSE
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("X-Accel-Buffering", "no")

	// Buat client baru
	client := &SSEClient{
		events: make(chan SSEEvent, 50),
		done:   make(chan struct{}),
	}

	// Register ke hub
	h.sseHub.register <- client

	// Pastikan unregister saat client disconnect
	defer func() {
		h.sseHub.unregister <- client
	}()

	// Kirim event welcome
	writeSSEvent(c.Writer, "connected", map[string]string{
		"message": "Terhubung ke stream kelas",
		"time":    time.Now().Format(time.RFC3339),
	})

	// Heartbeat ticker (jaga koneksi tetap hidup)
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	// Loop utama: dengarkan event dari hub atau context cancel
	for {
		select {
		case event, ok := <-client.events:
			if !ok {
				return
			}
			writeSSEvent(c.Writer, event.Type, event.Data)

		case <-heartbeat.C:
			c.Writer.Write([]byte(": heartbeat\n\n"))
			c.Writer.Flush()

		case <-c.Request.Context().Done():
			return
		}
	}
}

// ===========================
// CREATE: Buat Kelas Baru
// POST /api/v1/kelas
// ===========================

func (h *KelasHandler) CreateKelas(c *gin.Context) {
	guruIDVal, exists := c.Get("guru_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Akses khusus guru"})
		return
	}
	guruID := guruIDVal.(uint)

	var req createKelasRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	kelas, err := h.service.CreateKelas(guruID, req.Nama, req.MataPelajaran)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 📡 Broadcast event ke semua subscriber SSE
	h.sseHub.Broadcast("kelas-created", kelas)

	c.JSON(http.StatusCreated, kelas)
}

// ===========================
// READ ALL: List Semua Kelas Guru
// GET /api/v1/kelas
// ===========================

func (h *KelasHandler) GetKelasGuru(c *gin.Context) {
	guruIDVal, exists := c.Get("guru_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Akses khusus guru"})
		return
	}
	guruID := guruIDVal.(uint)

	kelass, err := h.service.GetKelasByGuruID(guruID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, kelass)
}

// ===========================
// READ BY ID: Detail 1 Kelas
// GET /api/v1/kelas/:id
// ===========================

func (h *KelasHandler) GetKelasByID(c *gin.Context) {
	guruIDVal, exists := c.Get("guru_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Akses khusus guru"})
		return
	}
	guruID := guruIDVal.(uint)

	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID kelas tidak valid"})
		return
	}

	kelas, err := h.service.GetKelasByID(uint(id), guruID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, kelas)
}

// ===========================
// UPDATE: Edit Kelas
// PUT /api/v1/kelas/:id
// ===========================

func (h *KelasHandler) UpdateKelas(c *gin.Context) {
	guruIDVal, exists := c.Get("guru_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Akses khusus guru"})
		return
	}
	guruID := guruIDVal.(uint)

	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID kelas tidak valid"})
		return
	}

	var req updateKelasRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	kelas, err := h.service.UpdateKelas(uint(id), guruID, req.Nama, req.MataPelajaran)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 📡 Broadcast event update
	h.sseHub.Broadcast("kelas-updated", kelas)

	c.JSON(http.StatusOK, gin.H{
		"message": "Kelas berhasil diperbarui",
		"data":    kelas,
	})
}

// ===========================
// DELETE: Hapus Kelas
// DELETE /api/v1/kelas/:id
// ===========================

func (h *KelasHandler) DeleteKelas(c *gin.Context) {
	guruIDVal, exists := c.Get("guru_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Akses khusus guru"})
		return
	}
	guruID := guruIDVal.(uint)

	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID kelas tidak valid"})
		return
	}

	err = h.service.DeleteKelas(uint(id), guruID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 📡 Broadcast event delete
	h.sseHub.Broadcast("kelas-deleted", map[string]uint{"id": uint(id)})

	c.JSON(http.StatusOK, gin.H{"message": "Kelas berhasil dihapus"})
}

// ===========================
// ASSIGN MODUL: Tautkan Modul ke Kelas
// POST /api/v1/kelas/:id/modul
// ===========================

func (h *KelasHandler) AssignModul(c *gin.Context) {
	guruIDVal, exists := c.Get("guru_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Akses khusus guru"})
		return
	}
	guruID := guruIDVal.(uint)

	kelasIDParam := c.Param("id")
	kelasID, err := strconv.ParseUint(kelasIDParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID kelas tidak valid"})
		return
	}

	var req assignModulRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = h.service.AssignModul(uint(kelasID), req.ModulID, guruID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Modul berhasil ditautkan ke kelas"})
}

// ===========================
// REMOVE MODUL: Lepas Modul dari Kelas
// DELETE /api/v1/kelas/:id/modul/:modul_id
// ===========================

func (h *KelasHandler) RemoveModul(c *gin.Context) {
	guruIDVal, exists := c.Get("guru_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Akses khusus guru"})
		return
	}
	guruID := guruIDVal.(uint)

	kelasIDParam := c.Param("id")
	kelasID, err := strconv.ParseUint(kelasIDParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID kelas tidak valid"})
		return
	}

	modulIDParam := c.Param("modul_id")
	modulID, err := strconv.ParseUint(modulIDParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID modul tidak valid"})
		return
	}

	err = h.service.RemoveModul(uint(kelasID), uint(modulID), guruID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Modul berhasil dihapus dari kelas"})
}

// Pastikan import model terpakai (untuk referensi tipe jika diperlukan)
var _ = model.Kelas{}