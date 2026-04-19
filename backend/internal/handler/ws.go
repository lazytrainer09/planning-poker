package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Client struct {
	RoomID        int64
	ParticipantID int64
	mu            sync.Mutex
	Conn          *websocket.Conn
	Send          chan []byte
}

// InMemParticipant holds participant data managed by Hub.
// Participants persist until server restart; disconnect does not remove them.
type InMemParticipant struct {
	ID     int64
	RoomID int64
	Name   string
}

// InMemSession holds session data managed by Hub.
type InMemSession struct {
	ID            int64
	RoomID        int64
	QuestionSetID int64
	Status        string // "voting" | "revealed"
}

type Hub struct {
	mu    sync.RWMutex
	rooms map[int64]map[*Client]bool

	// participants: participantID -> InMemParticipant (IDs are globally unique via atomic counter)
	participants     map[int64]*InMemParticipant
	roomParticipants map[int64][]int64 // roomID -> []participantID

	// sessions: sessionID -> InMemSession
	sessions map[int64]*InMemSession

	// answers: sessionID -> participantID -> questionID -> text
	answers map[int64]map[int64]map[int64]string

	nextParticipantID atomic.Int64
	nextSessionID     atomic.Int64
}

func NewHub() *Hub {
	return &Hub{
		rooms:            make(map[int64]map[*Client]bool),
		participants:     make(map[int64]*InMemParticipant),
		roomParticipants: make(map[int64][]int64),
		sessions:         make(map[int64]*InMemSession),
		answers:          make(map[int64]map[int64]map[int64]string),
	}
}

// AddParticipant registers a participant in a room and returns their generated ID.
func (h *Hub) AddParticipant(roomID int64, name string) int64 {
	id := h.nextParticipantID.Add(1)
	h.mu.Lock()
	defer h.mu.Unlock()
	h.participants[id] = &InMemParticipant{ID: id, RoomID: roomID, Name: name}
	h.roomParticipants[roomID] = append(h.roomParticipants[roomID], id)
	return id
}

// GetParticipant returns the name of a participant, validating room ownership.
func (h *Hub) GetParticipant(roomID, participantID int64) (name string, ok bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	p, exists := h.participants[participantID]
	if !exists || p.RoomID != roomID {
		return "", false
	}
	return p.Name, true
}

// GetParticipantName returns the participant's name by ID only (no room check).
func (h *Hub) GetParticipantName(participantID int64) (string, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	p, ok := h.participants[participantID]
	if !ok {
		return "", false
	}
	return p.Name, true
}

// GetParticipantsForRoom returns all participants registered in a room.
func (h *Hub) GetParticipantsForRoom(roomID int64) []*InMemParticipant {
	h.mu.RLock()
	defer h.mu.RUnlock()
	ids := h.roomParticipants[roomID]
	out := make([]*InMemParticipant, 0, len(ids))
	for _, id := range ids {
		if p, ok := h.participants[id]; ok {
			out = append(out, p)
		}
	}
	return out
}

// StartSession creates a new in-memory session and returns its ID.
func (h *Hub) StartSession(roomID, questionSetID int64) int64 {
	id := h.nextSessionID.Add(1)
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sessions[id] = &InMemSession{
		ID:            id,
		RoomID:        roomID,
		QuestionSetID: questionSetID,
		Status:        "voting",
	}
	h.answers[id] = make(map[int64]map[int64]string)
	return id
}

// GetSession retrieves a session by ID.
func (h *Hub) GetSession(sessionID int64) (*InMemSession, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	s, ok := h.sessions[sessionID]
	return s, ok
}

// SetSessionStatus updates a session's status.
func (h *Hub) SetSessionStatus(sessionID int64, status string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if s, ok := h.sessions[sessionID]; ok {
		s.Status = status
	}
}

// SubmitAnswer records an answer (upsert by participantID+questionID).
func (h *Hub) SubmitAnswer(sessionID, participantID, questionID int64, text string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.answers[sessionID] == nil {
		h.answers[sessionID] = make(map[int64]map[int64]string)
	}
	if h.answers[sessionID][participantID] == nil {
		h.answers[sessionID][participantID] = make(map[int64]string)
	}
	h.answers[sessionID][participantID][questionID] = text
}

// CountVotes returns (voted, total) participants for a session.
func (h *Hub) CountVotes(sessionID, roomID int64) (voted, total int) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	voted = len(h.answers[sessionID])
	total = len(h.roomParticipants[roomID])
	return
}

// HasVoted reports whether a participant has submitted any answer in a session.
func (h *Hub) HasVoted(sessionID, participantID int64) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.answers[sessionID][participantID]) > 0
}

// GetAnswers returns a copy of all answers for a session: participantID -> questionID -> text.
func (h *Hub) GetAnswers(sessionID int64) map[int64]map[int64]string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make(map[int64]map[int64]string)
	for pid, qs := range h.answers[sessionID] {
		inner := make(map[int64]string)
		for qid, text := range qs {
			inner[qid] = text
		}
		out[pid] = inner
	}
	return out
}

// ClearAnswers removes all answers for a session (used by reset).
func (h *Hub) ClearAnswers(sessionID int64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.answers[sessionID] = make(map[int64]map[int64]string)
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
	roomID, err := strconv.ParseInt(r.URL.Query().Get("room_id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid room_id", http.StatusBadRequest)
		return
	}
	participantID, err := strconv.ParseInt(r.URL.Query().Get("participant_id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid participant_id", http.StatusBadRequest)
		return
	}

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
		c.mu.Lock()
		err := c.Conn.WriteMessage(websocket.TextMessage, msg)
		c.mu.Unlock()
		if err != nil {
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
	}
}
