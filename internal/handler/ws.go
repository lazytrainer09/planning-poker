package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Client struct {
	RoomID        int64
	ParticipantID int64
	Conn          *websocket.Conn
	Send          chan []byte
}

type Hub struct {
	mu      sync.RWMutex
	rooms   map[int64]map[*Client]bool
}

func NewHub() *Hub {
	return &Hub{
		rooms: make(map[int64]map[*Client]bool),
	}
}

func (h *Hub) Register(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.rooms[client.RoomID] == nil {
		h.rooms[client.RoomID] = make(map[*Client]bool)
	}
	h.rooms[client.RoomID][client] = true
	log.Printf("Client registered: room=%d participant=%d (total in room: %d)",
		client.RoomID, client.ParticipantID, len(h.rooms[client.RoomID]))
}

func (h *Hub) Unregister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if clients, ok := h.rooms[client.RoomID]; ok {
		delete(clients, client)
		if len(clients) == 0 {
			delete(h.rooms, client.RoomID)
		}
	}
	close(client.Send)
	log.Printf("Client unregistered: room=%d participant=%d", client.RoomID, client.ParticipantID)
}

func (h *Hub) Broadcast(roomID int64, msg interface{}) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	if clients, ok := h.rooms[roomID]; ok {
		for client := range clients {
			select {
			case client.Send <- data:
			default:
				// Client buffer full, skip
			}
		}
	}
}

func (h *Hub) HandleWS(w http.ResponseWriter, r *http.Request) {
	roomID, _ := strconv.ParseInt(r.URL.Query().Get("room_id"), 10, 64)
	participantID, _ := strconv.ParseInt(r.URL.Query().Get("participant_id"), 10, 64)

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	client := &Client{
		RoomID:        roomID,
		ParticipantID: participantID,
		Conn:          conn,
		Send:          make(chan []byte, 256),
	}

	h.Register(client)

	// Notify others that a participant joined
	h.Broadcast(roomID, map[string]interface{}{
		"type": "participant_joined",
		"payload": map[string]interface{}{
			"participant_id": participantID,
		},
	})

	go client.writePump()
	go client.readPump(h)
}

func (c *Client) writePump() {
	defer c.Conn.Close()
	for msg := range c.Send {
		if err := c.Conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			return
		}
	}
}

func (c *Client) readPump(hub *Hub) {
	defer func() {
		hub.Unregister(c)
		hub.Broadcast(c.RoomID, map[string]interface{}{
			"type": "participant_left",
			"payload": map[string]interface{}{
				"participant_id": c.ParticipantID,
			},
		})
		c.Conn.Close()
	}()

	for {
		_, _, err := c.Conn.ReadMessage()
		if err != nil {
			break
		}
		// We don't process incoming WS messages from clients for now
		// All mutations go through REST API
	}
}
