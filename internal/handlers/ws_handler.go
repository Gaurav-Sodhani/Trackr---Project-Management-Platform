package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	gorilla "github.com/gorilla/websocket"

	"project-management-platform/internal/websocket"
)

var upgrader = gorilla.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true }, // allow all origins for demo
}

type WSHandler struct {
	hub *websocket.Hub
}

func NewWSHandler(hub *websocket.Hub) *WSHandler {
	return &WSHandler{hub: hub}
}

// HandleConnection upgrades HTTP to WebSocket.
// Query params: ?project_id=...&user_id=...&last_event_id=...
func (h *WSHandler) HandleConnection(c *gin.Context) {
	projectID, err := uuid.Parse(c.Query("project_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "project_id query param required"})
		return
	}
	userID, err := uuid.Parse(c.Query("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id query param required"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("ws: upgrade failed: %v", err)
		return
	}

	client := &websocket.Client{
		Hub:       h.hub,
		Conn:      conn,
		Send:      make(chan []byte, 256),
		ProjectID: projectID,
		UserID:    userID,
	}

	h.hub.Register <- client

	// Replay missed events if client provides last_event_id (reconnection support)
	lastEventID := c.Query("last_event_id")
	if lastEventID != "" {
		missed := h.hub.GetRecentEvents(projectID, lastEventID)
		for _, event := range missed {
			data, err := json.Marshal(event)
			if err == nil {
				client.Send <- data
			}
		}
	}

	// Start read/write pumps in separate goroutines
	go h.writePump(client)
	go h.readPump(client)
}

// writePump sends messages from the hub to the websocket connection.
func (h *WSHandler) writePump(client *websocket.Client) {
	defer client.Conn.Close()

	for message := range client.Send {
		client.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if err := client.Conn.WriteMessage(gorilla.TextMessage, message); err != nil {
			return
		}
	}
}

// readPump reads messages from the client (presence heartbeats).
func (h *WSHandler) readPump(client *websocket.Client) {
	defer func() {
		h.hub.Unregister <- client
		client.Conn.Close()
	}()

	client.Conn.SetReadLimit(512)
	client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	client.Conn.SetPongHandler(func(string) error {
		client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, _, err := client.Conn.ReadMessage()
		if err != nil {
			break
		}
	}
}
