package ws

import (
	"encoding/json"
	"log"
	"sync"
)

type Hub struct {
	// clients maps userID → set of connections.
	clients    map[int64]map[*Client]bool
	mu         sync.RWMutex
	Register   chan *Client
	Unregister chan *Client

	// OnDisconnect is called when a user's last connection is removed.
	OnDisconnect func(userID int64)
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[int64]map[*Client]bool),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.mu.Lock()
			if h.clients[client.UserID] == nil {
				h.clients[client.UserID] = make(map[*Client]bool)
			}
			wasOffline := len(h.clients[client.UserID]) == 0
			h.clients[client.UserID][client] = true
			h.mu.Unlock()

			if wasOffline {
				// Broadcast online presence.
				evt, err := NewEvent("presence.update", PresenceUpdatePayload{
					UserID: client.UserID,
					Status: "online",
				})
				if err == nil {
					h.BroadcastAll(evt)
				}
			}

		case client := <-h.Unregister:
			h.mu.Lock()
			if conns, ok := h.clients[client.UserID]; ok {
				delete(conns, client)
				close(client.Send)
				if len(conns) == 0 {
					delete(h.clients, client.UserID)
					h.mu.Unlock()

					if h.OnDisconnect != nil {
						h.OnDisconnect(client.UserID)
					}

					evt, err := NewEvent("presence.update", PresenceUpdatePayload{
						UserID: client.UserID,
						Status: "offline",
					})
					if err == nil {
						h.BroadcastAll(evt)
					}
					continue
				}
			}
			h.mu.Unlock()
		}
	}
}

// Send sends an event to all connections for the given user IDs.
// If a client's send buffer is full, the client is disconnected; the
// read pump will then re-register after the client reconnects, forcing
// a fresh sync instead of silently dropping real-time state.
func (h *Hub) Send(userIDs []int64, evt Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, uid := range userIDs {
		if conns, ok := h.clients[uid]; ok {
			for client := range conns {
				select {
				case client.Send <- evt:
				default:
					log.Printf("ws: buffer full for user %d, disconnecting", uid)
					client.Disconnect()
				}
			}
		}
	}
}

// BroadcastAll sends an event to every connected client. Presence
// updates are lossier by nature; we still disconnect slow clients so
// they re-sync on reconnect.
func (h *Hub) BroadcastAll(evt Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, conns := range h.clients {
		for client := range conns {
			select {
			case client.Send <- evt:
			default:
				client.Disconnect()
			}
		}
	}
}

// IsOnline checks if a user has at least one active connection.
func (h *Hub) IsOnline(userID int64) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients[userID]) > 0
}

// SendJSON is a convenience that marshals the payload and sends.
func (h *Hub) SendJSON(userIDs []int64, eventType string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("ws: marshal error: %v", err)
		return
	}
	h.Send(userIDs, Event{Type: eventType, Payload: data})
}
