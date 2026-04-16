package handler

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"planning-poker/internal/model"
)

type QuestionHandler struct {
	DB *sql.DB
}

type createQSetReq struct {
	Name      string          `json:"name"`
	Questions []questionInput `json:"questions"`
}

type questionInput struct {
	Text      string `json:"text"`
	SortOrder int    `json:"sort_order"`
}

func (h *QuestionHandler) ListQuestionSets(w http.ResponseWriter, r *http.Request) {
	roomID, err := strconv.ParseInt(r.PathValue("roomID"), 10, 64)
	if err != nil {
		http.Error(w, "invalid room id", http.StatusBadRequest)
		return
	}

	rows, err := h.DB.Query(
		"SELECT id, room_id, name, created_at, updated_at FROM question_sets WHERE room_id = ? ORDER BY id",
		roomID,
	)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var sets []model.QuestionSet
	for rows.Next() {
		var qs model.QuestionSet
		if err := rows.Scan(&qs.ID, &qs.RoomID, &qs.Name, &qs.CreatedAt, &qs.UpdatedAt); err != nil {
			log.Printf("scan question_set: %v", err)
			continue
		}
		sets = append(sets, qs)
	}

	// load questions for each set
	for i := range sets {
		qrows, err := h.DB.Query(
			"SELECT id, question_set_id, text, sort_order FROM questions WHERE question_set_id = ? ORDER BY sort_order",
			sets[i].ID,
		)
		if err != nil {
			log.Printf("query questions for set %d: %v", sets[i].ID, err)
			continue
		}
		for qrows.Next() {
			var q model.Question
			if err := qrows.Scan(&q.ID, &q.QuestionSetID, &q.Text, &q.SortOrder); err != nil {
				log.Printf("scan question: %v", err)
				continue
			}
			sets[i].Questions = append(sets[i].Questions, q)
		}
		qrows.Close()
		if sets[i].Questions == nil {
			sets[i].Questions = []model.Question{}
		}
	}
	if sets == nil {
		sets = []model.QuestionSet{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sets)
}

func (h *QuestionHandler) CreateQuestionSet(w http.ResponseWriter, r *http.Request) {
	roomID, err := strconv.ParseInt(r.PathValue("roomID"), 10, 64)
	if err != nil {
		http.Error(w, "invalid room id", http.StatusBadRequest)
		return
	}

	var req createQSetReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	now := time.Now()
	res, err := h.DB.Exec(
		"INSERT INTO question_sets (room_id, name, created_at, updated_at) VALUES (?, ?, ?, ?)",
		roomID, req.Name, now, now,
	)
	if err != nil {
		http.Error(w, "failed to create question set", http.StatusInternalServerError)
		return
	}
	qsID, _ := res.LastInsertId()

	for _, q := range req.Questions {
		if _, err := h.DB.Exec(
			"INSERT INTO questions (question_set_id, text, sort_order) VALUES (?, ?, ?)",
			qsID, q.Text, q.SortOrder,
		); err != nil {
			log.Printf("insert question: %v", err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"id": qsID})
}

func (h *QuestionHandler) UpdateQuestionSet(w http.ResponseWriter, r *http.Request) {
	qsID, err := strconv.ParseInt(r.PathValue("qsID"), 10, 64)
	if err != nil {
		http.Error(w, "invalid question set id", http.StatusBadRequest)
		return
	}

	var req createQSetReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if _, err := h.DB.Exec("UPDATE question_sets SET name = ?, updated_at = ? WHERE id = ?", req.Name, time.Now(), qsID); err != nil {
		http.Error(w, "failed to update", http.StatusInternalServerError)
		return
	}
	h.DB.Exec("DELETE FROM questions WHERE question_set_id = ?", qsID)

	for _, q := range req.Questions {
		if _, err := h.DB.Exec(
			"INSERT INTO questions (question_set_id, text, sort_order) VALUES (?, ?, ?)",
			qsID, q.Text, q.SortOrder,
		); err != nil {
			log.Printf("insert question: %v", err)
		}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *QuestionHandler) DeleteQuestionSet(w http.ResponseWriter, r *http.Request) {
	qsID, err := strconv.ParseInt(r.PathValue("qsID"), 10, 64)
	if err != nil {
		http.Error(w, "invalid question set id", http.StatusBadRequest)
		return
	}

	h.DB.Exec("DELETE FROM question_sets WHERE id = ?", qsID)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
