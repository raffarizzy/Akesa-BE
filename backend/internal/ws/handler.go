package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/farhan-hamzah/Akesa/backend/internal/auth"
)

type Handler struct {
	hub *Hub
}

func NewHandler(hub *Hub) *Handler {
	return &Handler{hub: hub}
}

func (h *Handler) ServeWS(w http.ResponseWriter, r *http.Request) {
	authUser, ok := auth.GetAuthUser(r)
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[ws] upgrade error: %v", err)
		return
	}

	client := &Client{
		hub:    h.hub,
		conn:   conn,
		userID: authUser.ID,
		send:   make(chan []byte, 256),
	}

	h.hub.register <- client

	// Kirim pesan selamat datang sebagai konfirmasi koneksi siap
	welcomeMsg, _ := json.Marshal(RealtimeMessage{
		Event: "connected",
		Data: map[string]any{
			"userId": authUser.ID.String(),
			"role":   authUser.Role,
		},
		Timestamp: time.Now().Unix(),
	})
	select {
	case client.send <- welcomeMsg:
	default:
	}

	go client.writePump()
	go client.readPump()
}
