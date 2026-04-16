package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"golang.org/x/crypto/bcrypt"
)

type RoomHandler struct {
	DB *sql.DB
}

type createRoomReq struct {
	Name       string `json:"name"`
	Passphrase string `json:"passphrase"`
}

type loginRoomReq struct {
	RoomID     int64  `json:"room_id"`
	RoomName   string `json:"room_name"`
	Passphrase string `json:"passphrase"`
	Name       string `json:"name"`
}

type loginResp struct {
	RoomID        int64  `json:"room_id"`
	RoomName      string `json:"room_name"`
	ParticipantID int64  `json:"participant_id"`
}

func (h *RoomHandler) CreateRoom(w http.ResponseWriter, r *http.Request) {
	var req createRoomReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if req.Name == "" || req.Passphrase == "" {
		http.Error(w, "name and passphrase required", http.StatusBadRequest)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Passphrase), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "failed to hash passphrase", http.StatusInternalServerError)
		return
	}

	res, err := h.DB.Exec("INSERT INTO rooms (name, passphrase) VALUES (?, ?)", req.Name, string(hash))
	if err != nil {
		http.Error(w, "failed to create room", http.StatusInternalServerError)
		return
	}
	id, _ := res.LastInsertId()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"id": id, "name": req.Name})
}

func (h *RoomHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRoomReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	var roomID int64
	var roomName, hashedPassphrase string
	if req.RoomID > 0 {
		err := h.DB.QueryRow("SELECT id, name, passphrase FROM rooms WHERE id = ?", req.RoomID).Scan(&roomID, &roomName, &hashedPassphrase)
		if err == sql.ErrNoRows {
			http.Error(w, "room not found", http.StatusNotFound)
			return
		} else if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
	} else if req.RoomName != "" {
		err := h.DB.QueryRow("SELECT id, name, passphrase FROM rooms WHERE name = ?", req.RoomName).Scan(&roomID, &roomName, &hashedPassphrase)
		if err == sql.ErrNoRows {
			http.Error(w, "room not found", http.StatusNotFound)
			return
		} else if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
	} else {
		http.Error(w, "room_id or room_name required", http.StatusBadRequest)
		return
	}

	if bcrypt.CompareHashAndPassword([]byte(hashedPassphrase), []byte(req.Passphrase)) != nil {
		http.Error(w, "invalid passphrase", http.StatusUnauthorized)
		return
	}

	res, err := h.DB.Exec("INSERT INTO participants (room_id, name) VALUES (?, ?)", roomID, req.Name)
	if err != nil {
		http.Error(w, "failed to join", http.StatusInternalServerError)
		return
	}
	pid, _ := res.LastInsertId()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(loginResp{
		RoomID:        roomID,
		RoomName:      roomName,
		ParticipantID: pid,
	})
}

func (h *RoomHandler) ListRooms(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.Query("SELECT id, name FROM rooms ORDER BY created_at DESC")
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type roomItem struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}
	var rooms []roomItem
	for rows.Next() {
		var r roomItem
		rows.Scan(&r.ID, &r.Name)
		rooms = append(rooms, r)
	}
	if rooms == nil {
		rooms = []roomItem{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rooms)
}

func (h *RoomHandler) ValidateParticipant(w http.ResponseWriter, r *http.Request) {
	roomID, err := strconv.ParseInt(r.PathValue("roomID"), 10, 64)
	if err != nil {
		http.Error(w, "invalid room id", http.StatusBadRequest)
		return
	}
	participantID, err := strconv.ParseInt(r.PathValue("participantID"), 10, 64)
	if err != nil {
		http.Error(w, "invalid participant id", http.StatusBadRequest)
		return
	}

	var name string
	err = h.DB.QueryRow("SELECT name FROM participants WHERE id = ? AND room_id = ?", participantID, roomID).Scan(&name)
	if err == sql.ErrNoRows {
		http.Error(w, "invalid session", http.StatusUnauthorized)
		return
	} else if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	var roomName string
	h.DB.QueryRow("SELECT name FROM rooms WHERE id = ?", roomID).Scan(&roomName)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"valid":     true,
		"name":      name,
		"room_name": roomName,
	})
}

func (h *RoomHandler) GetParticipants(w http.ResponseWriter, r *http.Request) {
	roomID, err := strconv.ParseInt(r.PathValue("roomID"), 10, 64)
	if err != nil {
		http.Error(w, "invalid room id", http.StatusBadRequest)
		return
	}

	rows, err := h.DB.Query("SELECT id, name FROM participants WHERE room_id = ?", roomID)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type p struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}
	var participants []p
	for rows.Next() {
		var pt p
		rows.Scan(&pt.ID, &pt.Name)
		participants = append(participants, pt)
	}
	if participants == nil {
		participants = []p{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(participants)
}
