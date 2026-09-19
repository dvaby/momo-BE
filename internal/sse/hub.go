package sse

import (
	"encoding/json"
	"sync"
)

// Role subscriber
type Role string

const (
	RoleGuru   Role = "guru"
	RoleSiswa  Role = "siswa"
	RolePublic Role = "public" // BARU: listener tanpa token (mode demo/stream terbuka)
)

// Scope event (siapa yang boleh terima)
type Scope string

const (
	ScopeGuru  Scope = "guru"
	ScopeSiswa Scope = "siswa"
	ScopeAll   Scope = "all"
)

// Client adalah subscriber SSE
type Client struct {
	Role    Role
	SiswaID uint // untuk tutor-reply (khusus siswa)
	Events  chan Event
	Done    chan struct{}
}

// Event yang akan di-broadcast
type Event struct {
	Type  string
	Data  interface{}
	Scope Scope
}

// Hub manages clients and broadcasts events (role-aware)
type Hub struct {
	clients    map[*Client]bool
	mu         sync.RWMutex
	register   chan *Client
	unregister chan *Client
	broadcast  chan Event
}

func NewHub() *Hub {
	h := &Hub{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan Event, 100),
	}
	go h.run()
	return h
}

func (h *Hub) run() {
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
				close(client.Events)
			}
			h.mu.Unlock()
		case event := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				if shouldDeliver(client, event) {
					select {
					case client.Events <- event:
					default:
						// client lambat, skip
					}
				}
			}
			h.mu.RUnlock()
		}
	}
}

// shouldDeliver cek apakah client boleh terima event berdasarkan role.
// RolePublic (listener tanpa token) menerima event scope siswa + all,
// TIDAK menerima event scope guru (nilai/jawaban siswa tetap aman).
func shouldDeliver(client *Client, event Event) bool {
	switch event.Scope {
	case ScopeAll:
		return true
	case ScopeGuru:
		return client.Role == RoleGuru
	case ScopeSiswa:
		return client.Role == RoleSiswa || client.Role == RolePublic
	}
	return false
}

// Register menambah client ke hub (dipanggil dari handler)
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Unregister menghapus client dari hub (dipanggil saat disconnect)
func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

// Broadcast kirim event ke semua subscriber yang sesuai scope
func (h *Hub) Broadcast(eventType string, data interface{}, scope Scope) {
	h.broadcast <- Event{Type: eventType, Data: data, Scope: scope}
}

// BroadcastToGuru kirim event khusus guru
func (h *Hub) BroadcastToGuru(eventType string, data interface{}) {
	h.Broadcast(eventType, data, ScopeGuru)
}

// BroadcastToSiswa kirim event khusus siswa (ke semua siswa + listener public)
func (h *Hub) BroadcastToSiswa(eventType string, data interface{}) {
	h.Broadcast(eventType, data, ScopeSiswa)
}

// BroadcastToAll kirim event ke semua role
func (h *Hub) BroadcastToAll(eventType string, data interface{}) {
	h.Broadcast(eventType, data, ScopeAll)
}

// BroadcastToSiswaByID kirim event ke siswa tertentu (untuk tutor-reply).
// Listener public (tanpa token) ikut menerima sebagai fallback mode demo,
// supaya alur tutor bisa dipertontonkan tanpa auth.
func (h *Hub) BroadcastToSiswaByID(eventType string, data interface{}, siswaID uint) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		match := (client.Role == RoleSiswa && client.SiswaID == siswaID) ||
			client.Role == RolePublic
		if match {
			select {
			case client.Events <- Event{Type: eventType, Data: data, Scope: ScopeSiswa}:
			default:
				// client lambat, skip
			}
		}
	}
}

// WriteEvent helper menulis event ke ResponseWriter dalam format SSE
func WriteEvent(w interface {
	Write(p []byte) (int, error)
	Flush()
}, eventType string, data interface{}) {
	jsonBytes, _ := json.Marshal(data)
	w.Write([]byte("event: " + eventType + "\n"))
	w.Write([]byte("data: " + string(jsonBytes) + "\n\n"))
	w.Flush()
}