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

	sessionID := h.Hub.StartSession(roomID, req.QuestionSetID)

	// Load questions for the session from DB (questions are still persisted)
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
		h.Hub.SubmitAnswer(sessionID, req.ParticipantID, qID, text)
	}

	sess, ok := h.Hub.GetSession(sessionID)
	if !ok {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	participantName, _ := h.Hub.GetParticipantName(req.ParticipantID)

	// Broadcast vote status update
	h.Hub.Broadcast(sess.RoomID, map[string]interface{}{
		"type": "vote_submitted",
		"payload": map[string]interface{}{
			"participant_id":   req.ParticipantID,
			"participant_name": participantName,
		},
	})

	voted, total := h.Hub.CountVotes(sessionID, sess.RoomID)

	if voted >= total && total > 0 {
		// Auto-reveal
		h.Hub.SetSessionStatus(sessionID, "revealed")
		h.broadcastResults(sess.RoomID, sessionID)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "ok",
		"voted":     voted,
		"total":     total,
		"all_voted": voted >= total && total > 0,
	})
}

func (h *SessionHandler) RevealResults(w http.ResponseWriter, r *http.Request) {
	sessionID, err := strconv.ParseInt(r.PathValue("sessionID"), 10, 64)
	if err != nil {
		http.Error(w, "invalid session id", http.StatusBadRequest)
		return
	}

	sess, ok := h.Hub.GetSession(sessionID)
	if !ok {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	h.Hub.SetSessionStatus(sessionID, "revealed")
	h.broadcastResults(sess.RoomID, sessionID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "revealed"})
}

func (h *SessionHandler) ResetSession(w http.ResponseWriter, r *http.Request) {
	sessionID, err := strconv.ParseInt(r.PathValue("sessionID"), 10, 64)
	if err != nil {
		http.Error(w, "invalid session id", http.StatusBadRequest)
		return
	}

	sess, ok := h.Hub.GetSession(sessionID)
	if !ok {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	h.Hub.ClearAnswers(sessionID)
	h.Hub.SetSessionStatus(sessionID, "voting")

	// Broadcast reset
	h.Hub.Broadcast(sess.RoomID, map[string]interface{}{
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

	sess, ok := h.Hub.GetSession(sessionID)
	if !ok {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	participants := h.Hub.GetParticipantsForRoom(sess.RoomID)

	type statusItem struct {
		ParticipantID   int64  `json:"participant_id"`
		ParticipantName string `json:"participant_name"`
		HasVoted        bool   `json:"has_voted"`
	}

	statuses := make([]statusItem, 0, len(participants))
	for _, p := range participants {
		statuses = append(statuses, statusItem{
			ParticipantID:   p.ID,
			ParticipantName: p.Name,
			HasVoted:        h.Hub.HasVoted(sessionID, p.ID),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":          sess.Status,
		"question_set_id": sess.QuestionSetID,
		"participants":    statuses,
	})
}

func (h *SessionHandler) GetSessionQuestions(w http.ResponseWriter, r *http.Request) {
	sessionID, err := strconv.ParseInt(r.PathValue("sessionID"), 10, 64)
	if err != nil {
		http.Error(w, "invalid session id", http.StatusBadRequest)
		return
	}

	sess, ok := h.Hub.GetSession(sessionID)
	if !ok {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	rows, err := h.DB.Query(
		"SELECT id, text, sort_order FROM questions WHERE question_set_id = ? ORDER BY sort_order",
		sess.QuestionSetID,
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
	allAnswers := h.Hub.GetAnswers(sessionID)

	var results []map[string]interface{}
	for pid, questionAnswers := range allAnswers {
		name, _ := h.Hub.GetParticipantName(pid)
		answersStr := make(map[string]string, len(questionAnswers))
		for qid, text := range questionAnswers {
			answersStr[strconv.FormatInt(qid, 10)] = text
		}
		results = append(results, map[string]interface{}{
			"participant_id":   pid,
			"participant_name": name,
			"answers":          answersStr,
		})
	}
	if results == nil {
		results = []map[string]interface{}{}
	}
	return results
}
