package websocket

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	gorilla "github.com/gorilla/websocket"
)

const (
	// Max recent events kept per project for reconnection replay
	maxRecentEvents = 100
)

// Event represents a real-time event broadcast to connected clients.
type Event struct {
	ID        string      `json:"id"`
	Type      string      `json:"type"` // issue_created, issue_updated, issue_moved, comment_added, sprint_updated
	ProjectID uuid.UUID   `json:"project_id"`
	Payload   interface{} `json:"payload"`
	Timestamp time.Time   `json:"timestamp"`
}

// Client is a single websocket connection.
type Client struct {
	Hub       *Hub
	Conn      *gorilla.Conn
	Send      chan []byte
	ProjectID uuid.UUID
	UserID    uuid.UUID
}

// Hub manages all active websocket connections grouped by project.
type Hub struct {
	// project_id -> set of connected clients
	projects map[uuid.UUID]map[*Client]bool

	// presence: project_id -> user_id -> last_seen (for "who is viewing" feature)
	Presence map[uuid.UUID]map[uuid.UUID]time.Time

	// ring buffer of recent events per project for reconnection replay
	recentEvents map[uuid.UUID][]*Event

	Broadcast  chan *Event
	Register   chan *Client
	Unregister chan *Client
	mu         sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		projects:     make(map[uuid.UUID]map[*Client]bool),
		Presence:     make(map[uuid.UUID]map[uuid.UUID]time.Time),
		recentEvents: make(map[uuid.UUID][]*Event),
		Broadcast:    make(chan *Event, 256),
		Register:     make(chan *Client),
		Unregister:   make(chan *Client),
	}
}

// Run is the main event loop -- must be started as a goroutine.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.mu.Lock()
			if _, ok := h.projects[client.ProjectID]; !ok {
				h.projects[client.ProjectID] = make(map[*Client]bool)
			}
			h.projects[client.ProjectID][client] = true
			// Track presence
			if _, ok := h.Presence[client.ProjectID]; !ok {
				h.Presence[client.ProjectID] = make(map[uuid.UUID]time.Time)
			}
			h.Presence[client.ProjectID][client.UserID] = time.Now()
			h.mu.Unlock()

			log.Printf("ws: client registered for project %s (user %s)", client.ProjectID, client.UserID)

		case client := <-h.Unregister:
			h.mu.Lock()
			if clients, ok := h.projects[client.ProjectID]; ok {
				if _, exists := clients[client]; exists {
					delete(clients, client)
					close(client.Send)
				}
			}
			// Remove from presence
			if users, ok := h.Presence[client.ProjectID]; ok {
				delete(users, client.UserID)
			}
			h.mu.Unlock()

		case event := <-h.Broadcast:
			h.mu.Lock()
			// Store for replay on reconnection
			events := h.recentEvents[event.ProjectID]
			if len(events) >= maxRecentEvents {
				events = events[1:] // drop oldest
			}
			h.recentEvents[event.ProjectID] = append(events, event)
			h.mu.Unlock()

			// Broadcast to all clients watching this project
			h.mu.RLock()
			clients := h.projects[event.ProjectID]
			h.mu.RUnlock()

			data, err := json.Marshal(event)
			if err != nil {
				log.Printf("ws: failed to marshal event: %v", err)
				continue
			}

			for client := range clients {
				select {
				case client.Send <- data:
				default:
					// Client buffer full -- disconnect
					h.mu.Lock()
					delete(h.projects[event.ProjectID], client)
					close(client.Send)
					h.mu.Unlock()
				}
			}
		}
	}
}

// GetRecentEvents returns events after the given event ID for replay.
// If lastEventID is empty, returns all recent events for the project.
func (h *Hub) GetRecentEvents(projectID uuid.UUID, lastEventID string) []*Event {
	h.mu.RLock()
	defer h.mu.RUnlock()

	events := h.recentEvents[projectID]
	if lastEventID == "" {
		return events
	}

	// Find the event and return everything after it
	for i, e := range events {
		if e.ID == lastEventID {
			return events[i+1:]
		}
	}
	// ID not found -- return all (client missed too many events)
	return events
}

// GetPresence returns the list of users currently viewing a project.
func (h *Hub) GetPresence(projectID uuid.UUID) map[uuid.UUID]time.Time {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.Presence[projectID]
}

// PublishEvent is a helper to broadcast an event from service code.
func (h *Hub) PublishEvent(eventType string, projectID uuid.UUID, payload interface{}) {
	h.Broadcast <- &Event{
		ID:        uuid.New().String(),
		Type:      eventType,
		ProjectID: projectID,
		Payload:   payload,
		Timestamp: time.Now(),
	}
}
