package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"momo-be/internal/sse"
)

type StreamHandler struct {
	hub *sse.Hub
}

func NewStreamHandler(hub *sse.Hub) *StreamHandler {
	return &StreamHandler{hub: hub}
}

// HandleStream adalah endpoint unified stream: GET /api/v1/stream
// Role-aware: menerima token guru, token siswa, ATAU tanpa token (mode public).
// Token bisa via header Authorization ATAU query param ?token=.
func (h *StreamHandler) HandleStream(c *gin.Context) {
	roleVal, exists := c.Get("role")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Role tidak dikenali"})
		return
	}

	var role sse.Role
	switch roleVal.(string) {
	case "guru":
		role = sse.RoleGuru
	case "siswa":
		role = sse.RoleSiswa
	case "public":
		role = sse.RolePublic
	default:
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Role tidak valid"})
		return
	}

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("X-Accel-Buffering", "no")

	// Buat client baru
	client := &sse.Client{
		Role:   role,
		Events: make(chan sse.Event, 50),
		Done:   make(chan struct{}),
	}

	// Set SiswaID untuk role siswa (untuk tutor-reply routing)
	if role == sse.RoleSiswa {
		if siswaIDVal, exists := c.Get("siswa_id"); exists {
			client.SiswaID = siswaIDVal.(uint)
		}
	}

	// Register ke hub
	h.hub.Register(client)

	// Pastikan unregister saat client disconnect
	defer h.hub.Unregister(client)

	// Kirim event welcome
	sse.WriteEvent(c.Writer, "connected", map[string]string{
		"message": "Terhubung ke stream Momo",
		"role":    string(role),
		"time":    time.Now().Format(time.RFC3339),
	})

	// Heartbeat ticker (jaga koneksi tetap hidup)
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	// Loop utama
	for {
		select {
		case event, ok := <-client.Events:
			if !ok {
				return
			}
			sse.WriteEvent(c.Writer, event.Type, event.Data)

		case <-heartbeat.C:
			c.Writer.Write([]byte(": heartbeat\n\n"))
			c.Writer.Flush()

		case <-c.Request.Context().Done():
			return
		}
	}
}