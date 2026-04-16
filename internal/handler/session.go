package handler

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
)

type SessionHandler struct {
	DB  *sql.DB
	Hub *Hub
}

type startSessionReq struct {
	QuestionSetID int64 `json:"question_set_id"`
}

type submitAnswersReq struct {
	ParticipantID int64             `json:"participant_id"`
	Answers       map[string]string `json:"answers"` // question_id (string) -> text
}

func (h *SessionHandler) StartSession(w http.ResponseWriter, r *http.Request) {
	roomID, err := strconv.ParseInt(r.PathValue("roomID"), 10, 64)
	if err != nil {
		http.Error(w, "invalid room id", http.StatusBadRequest)
		return
	}

	var req startSessionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	res, err := h.DB.Exec(
		"INSERT INTO sessions (room_id, question_set_id, status) VALUES (?, ?, 'voting')",
		roomID, req.QuestionSetID,
	)
	if err != nil {
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}
	sessionID, _ := res.LastInsertId()

	// Load questions for the session
	rows, err := h.DB.Query(
		"SELECT id, text, sort_order FROM questions WHERE question_set_id = ? ORDER BY sort_order",
		req.QuestionSetID,
	)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type qItem struct {
		ID        int64  `json:"id"`
		Text      string `json:"text"`
		SortOrder int    `json:"sort_order"`
	}
	var questions []qItem
	for rows.Next() {
		var q qItem
		if err := rows.Scan(&q.ID, &q.Text, &q.SortOrder); err != nil {
			log.Printf("scan question: %v", err)
			continue
		}
		questions = append(questions, q)
	}

	// Broadcast session start to all participants in the room
	h.Hub.Broadcast(roomID, map[string]interface{}{
		"type": "session_started",
		"payload": map[string]interface{}{
			"session_id":      sessionID,
			"question_set_id": req.QuestionSetID,
			"questions":       questions,
		},
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"session_id": sessionID,
		"questions":  questions,
	})
}

func (h *SessionHandler) SubmitAnswers(w http.ResponseWriter, r *http.Request) {
	sessionID, err := strconv.ParseInt(r.PathValue("sessionID"), 10, 64)
	if err != nil {
		http.Error(w, "invalid session id", http.StatusBadRequest)
		return
	}

	var req submitAnswersReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	for qIDStr, text := range req.Answers {
		qID, err := strconv.ParseInt(qIDStr, 10, 64)
		if err != nil {
			continue
		}
		if _, err := h.DB.Exec(
			`INSERT INTO answers (session_id, participant_id, question_id, text)
			 VALUES (?, ?, ?, ?)
			 ON CONFLICT(session_id, participant_id, question_id) DO UPDATE SET text = ?`,
			sessionID, req.ParticipantID, qID, text, text,
		); err != nil {
			log.Printf("insert answer: %v", err)
		}
	}

	// Get room_id for broadcast
	var roomID int64
	if err := h.DB.QueryRow("SELECT room_id FROM sessions WHERE id = ?", sessionID).Scan(&roomID); err != nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	// Get participant name
	var participantName string
	h.DB.QueryRow("SELECT name FROM participants WHERE id = ?", req.ParticipantID).Scan(&participantName)

	// Broadcast vote status update
	h.Hub.Broadcast(roomID, map[string]interface{}{
		"type": "vote_submitted",
		"payload": map[string]interface{}{
			"participant_id":   req.ParticipantID,
			"participant_name": participantName,
		},
	})

	// Check if all participants have voted
	var totalParticipants, votedParticipants int
	h.DB.QueryRow("SELECT COUNT(*) FROM participants WHERE room_id = ?", roomID).Scan(&totalParticipants)
	h.DB.QueryRow(
		"SELECT COUNT(DISTINCT participant_id) FROM answers WHERE session_id = ?",
		sessionID,
	).Scan(&votedParticipants)

	if votedParticipants >= totalParticipants && totalParticipants > 0 {
		// Auto-reveal
		h.DB.Exec("UPDATE sessions SET status = 'revealed' WHERE id = ?", sessionID)
		h.broadcastResults(roomID, sessionID)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "ok",
		"voted":     votedParticipants,
		"total":     totalParticipants,
		"all_voted": votedParticipants >= totalParticipants && totalParticipants > 0,
	})
}

func (h *SessionHandler) RevealResults(w http.ResponseWriter, r *http.Request) {
	sessionID, err := strconv.ParseInt(r.PathValue("sessionID"), 10, 64)
	if err != nil {
		http.Error(w, "invalid session id", http.StatusBadRequest)
		return
	}

	h.DB.Exec("UPDATE sessions SET status = 'revealed' WHERE id = ?", sessionID)

	var roomID int64
	if err := h.DB.QueryRow("SELECT room_id FROM sessions WHERE id = ?", sessionID).Scan(&roomID); err != nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	h.broadcastResults(roomID, sessionID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "revealed"})
}

func (h *SessionHandler) ResetSession(w http.ResponseWriter, r *http.Request) {
	sessionID, err := strconv.ParseInt(r.PathValue("sessionID"), 10, 64)
	if err != nil {
		http.Error(w, "invalid session id", http.StatusBadRequest)
		return
	}

	var roomID int64
	if err := h.DB.QueryRow("SELECT room_id FROM sessions WHERE id = ?", sessionID).Scan(&roomID); err != nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	// Delete answers and reset status
	h.DB.Exec("DELETE FROM answers WHERE session_id = ?", sessionID)
	h.DB.Exec("UPDATE sessions SET status = 'voting' WHERE id = ?", sessionID)

	// Broadcast reset
	h.Hub.Broadcast(roomID, map[string]interface{}{
		"type": "session_reset",
		"payload": map[string]interface{}{
			"session_id": sessionID,
		},
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "reset"})
}

func (h *SessionHandler) GetVoteStatus(w http.ResponseWriter, r *http.Request) {
	sessionID, err := strconv.ParseInt(r.PathValue("sessionID"), 10, 64)
	if err != nil {
		http.Error(w, "invalid session id", http.StatusBadRequest)
		return
	}

	var roomID int64
	if err := h.DB.QueryRow("SELECT room_id FROM sessions WHERE id = ?", sessionID).Scan(&roomID); err != nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	// Get all participants
	rows, err := h.DB.Query("SELECT id, name FROM participants WHERE room_id = ?", roomID)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type statusItem struct {
		ParticipantID   int64  `json:"participant_id"`
		ParticipantName string `json:"participant_name"`
		HasVoted        bool   `json:"has_voted"`
	}

	var statuses []statusItem
	for rows.Next() {
		var s statusItem
		if err := rows.Scan(&s.ParticipantID, &s.ParticipantName); err != nil {
			log.Printf("scan participant: %v", err)
			continue
		}

		var count int
		h.DB.QueryRow(
			"SELECT COUNT(*) FROM answers WHERE session_id = ? AND participant_id = ?",
			sessionID, s.ParticipantID,
		).Scan(&count)
		s.HasVoted = count > 0

		statuses = append(statuses, s)
	}

	// Get session status
	var status string
	h.DB.QueryRow("SELECT status FROM sessions WHERE id = ?", sessionID).Scan(&status)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":       status,
		"participants": statuses,
	})
}

func (h *SessionHandler) GetSessionQuestions(w http.ResponseWriter, r *http.Request) {
	sessionID, err := strconv.ParseInt(r.PathValue("sessionID"), 10, 64)
	if err != nil {
		http.Error(w, "invalid session id", http.StatusBadRequest)
		return
	}

	var questionSetID int64
	if err := h.DB.QueryRow("SELECT question_set_id FROM sessions WHERE id = ?", sessionID).Scan(&questionSetID); err != nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	rows, err := h.DB.Query(
		"SELECT id, text, sort_order FROM questions WHERE question_set_id = ? ORDER BY sort_order",
		questionSetID,
	)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type qItem struct {
		ID        int64  `json:"id"`
		Text      string `json:"text"`
		SortOrder int    `json:"sort_order"`
	}
	var questions []qItem
	for rows.Next() {
		var q qItem
		if err := rows.Scan(&q.ID, &q.Text, &q.SortOrder); err != nil {
			log.Printf("scan question: %v", err)
			continue
		}
		questions = append(questions, q)
	}
	if questions == nil {
		questions = []qItem{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(questions)
}

func (h *SessionHandler) GetResults(w http.ResponseWriter, r *http.Request) {
	sessionID, err := strconv.ParseInt(r.PathValue("sessionID"), 10, 64)
	if err != nil {
		http.Error(w, "invalid session id", http.StatusBadRequest)
		return
	}

	results := h.loadResults(sessionID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func (h *SessionHandler) broadcastResults(roomID, sessionID int64) {
	results := h.loadResults(sessionID)
	h.Hub.Broadcast(roomID, map[string]interface{}{
		"type":    "results_revealed",
		"payload": results,
	})
}

func (h *SessionHandler) loadResults(sessionID int64) []map[string]interface{} {
	rows, err := h.DB.Query(`
		SELECT a.participant_id, p.name, a.question_id, a.text
		FROM answers a
		JOIN participants p ON p.id = a.participant_id
		WHERE a.session_id = ?
		ORDER BY a.participant_id, a.question_id
	`, sessionID)
	if err != nil {
		return []map[string]interface{}{}
	}
	defer rows.Close()

	resultMap := make(map[int64]map[string]interface{})
	for rows.Next() {
		var pid, qid int64
		var name, text string
		if err := rows.Scan(&pid, &name, &qid, &text); err != nil {
			log.Printf("scan result: %v", err)
			continue
		}

		if _, ok := resultMap[pid]; !ok {
			resultMap[pid] = map[string]interface{}{
				"participant_id":   pid,
				"participant_name": name,
				"answers":          map[string]string{},
			}
		}
		resultMap[pid]["answers"].(map[string]string)[strconv.FormatInt(qid, 10)] = text
	}

	var results []map[string]interface{}
	for _, v := range resultMap {
		results = append(results, v)
	}
	if results == nil {
		results = []map[string]interface{}{}
	}
	return results
}
