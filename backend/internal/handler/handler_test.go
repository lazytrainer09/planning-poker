package handler_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"planning-poker/internal/db"
	"planning-poker/internal/handler"
)

// testEnv holds all dependencies for a test.
type testEnv struct {
	mux *http.ServeMux
}

func setup(t *testing.T) *testEnv {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() {
		database.Close()
		os.Remove(dbPath)
	})

	hub := handler.NewHub()
	roomH := &handler.RoomHandler{DB: database, Hub: hub}
	questionH := &handler.QuestionHandler{DB: database}
	sessionH := &handler.SessionHandler{DB: database, Hub: hub}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/rooms", roomH.CreateRoom)
	mux.HandleFunc("GET /api/rooms", roomH.ListRooms)
	mux.HandleFunc("POST /api/rooms/login", roomH.Login)
	mux.HandleFunc("GET /api/rooms/{roomID}/participants", roomH.GetParticipants)
	mux.HandleFunc("GET /api/rooms/{roomID}/participants/{participantID}/validate", roomH.ValidateParticipant)

	mux.HandleFunc("GET /api/rooms/{roomID}/question-sets", questionH.ListQuestionSets)
	mux.HandleFunc("POST /api/rooms/{roomID}/question-sets", questionH.CreateQuestionSet)
	mux.HandleFunc("PUT /api/question-sets/{qsID}", questionH.UpdateQuestionSet)
	mux.HandleFunc("DELETE /api/question-sets/{qsID}", questionH.DeleteQuestionSet)

	mux.HandleFunc("POST /api/rooms/{roomID}/sessions", sessionH.StartSession)
	mux.HandleFunc("POST /api/sessions/{sessionID}/answers", sessionH.SubmitAnswers)
	mux.HandleFunc("POST /api/sessions/{sessionID}/reveal", sessionH.RevealResults)
	mux.HandleFunc("POST /api/sessions/{sessionID}/reset", sessionH.ResetSession)
	mux.HandleFunc("GET /api/sessions/{sessionID}/questions", sessionH.GetSessionQuestions)
	mux.HandleFunc("GET /api/sessions/{sessionID}/status", sessionH.GetVoteStatus)
	mux.HandleFunc("GET /api/sessions/{sessionID}/results", sessionH.GetResults)

	return &testEnv{mux: mux}
}

func (e *testEnv) do(t *testing.T, method, path string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	e.mux.ServeHTTP(w, req)
	return w
}

func (e *testEnv) decode(t *testing.T, w *httptest.ResponseRecorder, v interface{}) {
	t.Helper()
	if err := json.NewDecoder(w.Body).Decode(v); err != nil {
		t.Fatalf("decode response: %v (body: %s)", err, w.Body.String())
	}
}

// --- Room Tests ---

func TestCreateRoom(t *testing.T) {
	env := setup(t)

	w := env.do(t, "POST", "/api/rooms", map[string]string{
		"name": "TestRoom", "passphrase": "secret",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var res map[string]interface{}
	env.decode(t, w, &res)
	if res["name"] != "TestRoom" {
		t.Errorf("expected name TestRoom, got %v", res["name"])
	}
	if res["id"] == nil || res["id"].(float64) <= 0 {
		t.Errorf("expected positive id, got %v", res["id"])
	}
}

func TestCreateRoom_Validation(t *testing.T) {
	env := setup(t)

	// Missing name
	w := env.do(t, "POST", "/api/rooms", map[string]string{"passphrase": "secret"})
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing name, got %d", w.Code)
	}

	// Missing passphrase
	w = env.do(t, "POST", "/api/rooms", map[string]string{"name": "Room"})
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing passphrase, got %d", w.Code)
	}
}

func TestListRooms(t *testing.T) {
	env := setup(t)

	// Create two rooms
	env.do(t, "POST", "/api/rooms", map[string]string{"name": "Room1", "passphrase": "p1"})
	env.do(t, "POST", "/api/rooms", map[string]string{"name": "Room2", "passphrase": "p2"})

	w := env.do(t, "GET", "/api/rooms", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var rooms []map[string]interface{}
	env.decode(t, w, &rooms)
	if len(rooms) != 2 {
		t.Errorf("expected 2 rooms, got %d", len(rooms))
	}
}

func TestListRooms_Empty(t *testing.T) {
	env := setup(t)

	w := env.do(t, "GET", "/api/rooms", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var rooms []map[string]interface{}
	env.decode(t, w, &rooms)
	if len(rooms) != 0 {
		t.Errorf("expected empty array, got %d rooms", len(rooms))
	}
}

// --- Login Tests ---

func TestLogin_ByRoomID(t *testing.T) {
	env := setup(t)

	// Create room
	w := env.do(t, "POST", "/api/rooms", map[string]string{"name": "MyRoom", "passphrase": "pass123"})
	var room map[string]interface{}
	env.decode(t, w, &room)
	roomID := room["id"].(float64)

	// Login
	w = env.do(t, "POST", "/api/rooms/login", map[string]interface{}{
		"room_id": roomID, "passphrase": "pass123", "name": "Alice",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var login map[string]interface{}
	env.decode(t, w, &login)
	if login["room_id"].(float64) != roomID {
		t.Errorf("room_id mismatch")
	}
	if login["room_name"] != "MyRoom" {
		t.Errorf("expected room_name MyRoom, got %v", login["room_name"])
	}
	if login["participant_id"].(float64) <= 0 {
		t.Errorf("expected positive participant_id")
	}
}

func TestLogin_ByRoomName(t *testing.T) {
	env := setup(t)

	env.do(t, "POST", "/api/rooms", map[string]string{"name": "NamedRoom", "passphrase": "pass"})

	w := env.do(t, "POST", "/api/rooms/login", map[string]interface{}{
		"room_name": "NamedRoom", "passphrase": "pass", "name": "Bob",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var login map[string]interface{}
	env.decode(t, w, &login)
	if login["room_name"] != "NamedRoom" {
		t.Errorf("expected room_name NamedRoom, got %v", login["room_name"])
	}
}

func TestLogin_WrongPassphrase(t *testing.T) {
	env := setup(t)

	w := env.do(t, "POST", "/api/rooms", map[string]string{"name": "Secured", "passphrase": "correct"})
	var room map[string]interface{}
	env.decode(t, w, &room)

	w = env.do(t, "POST", "/api/rooms/login", map[string]interface{}{
		"room_id": room["id"], "passphrase": "wrong", "name": "Eve",
	})
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestLogin_RoomNotFound(t *testing.T) {
	env := setup(t)

	w := env.do(t, "POST", "/api/rooms/login", map[string]interface{}{
		"room_id": 9999, "passphrase": "x", "name": "Nobody",
	})
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// --- Participant Validation ---

func TestValidateParticipant(t *testing.T) {
	env := setup(t)

	// Create room + login
	w := env.do(t, "POST", "/api/rooms", map[string]string{"name": "ValRoom", "passphrase": "p"})
	var room map[string]interface{}
	env.decode(t, w, &room)
	roomID := int64(room["id"].(float64))

	w = env.do(t, "POST", "/api/rooms/login", map[string]interface{}{
		"room_id": roomID, "passphrase": "p", "name": "Tester",
	})
	var login map[string]interface{}
	env.decode(t, w, &login)
	pid := int64(login["participant_id"].(float64))

	// Validate
	w = env.do(t, "GET", fmt.Sprintf("/api/rooms/%d/participants/%d/validate", roomID, pid), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var val map[string]interface{}
	env.decode(t, w, &val)
	if val["valid"] != true {
		t.Errorf("expected valid=true")
	}
	if val["name"] != "Tester" {
		t.Errorf("expected name Tester, got %v", val["name"])
	}
}

func TestValidateParticipant_Invalid(t *testing.T) {
	env := setup(t)

	w := env.do(t, "POST", "/api/rooms", map[string]string{"name": "R", "passphrase": "p"})
	var room map[string]interface{}
	env.decode(t, w, &room)
	roomID := int64(room["id"].(float64))

	w = env.do(t, "GET", fmt.Sprintf("/api/rooms/%d/participants/9999/validate", roomID), nil)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// --- Question Set Tests ---

func TestQuestionSetCRUD(t *testing.T) {
	env := setup(t)

	// Create room
	w := env.do(t, "POST", "/api/rooms", map[string]string{"name": "QRoom", "passphrase": "p"})
	var room map[string]interface{}
	env.decode(t, w, &room)
	roomID := int64(room["id"].(float64))

	// Create question set
	w = env.do(t, "POST", fmt.Sprintf("/api/rooms/%d/question-sets", roomID), map[string]interface{}{
		"name": "Sprint 1",
		"questions": []map[string]interface{}{
			{"text": "How complex?", "sort_order": 0},
			{"text": "How risky?", "sort_order": 1},
		},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create qs: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var qs map[string]interface{}
	env.decode(t, w, &qs)
	qsID := int64(qs["id"].(float64))

	// List question sets
	w = env.do(t, "GET", fmt.Sprintf("/api/rooms/%d/question-sets", roomID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list qs: expected 200, got %d", w.Code)
	}
	var sets []map[string]interface{}
	env.decode(t, w, &sets)
	if len(sets) != 1 {
		t.Fatalf("expected 1 question set, got %d", len(sets))
	}
	questions := sets[0]["questions"].([]interface{})
	if len(questions) != 2 {
		t.Errorf("expected 2 questions, got %d", len(questions))
	}

	// Update question set
	w = env.do(t, "PUT", fmt.Sprintf("/api/question-sets/%d", qsID), map[string]interface{}{
		"name": "Sprint 1 Updated",
		"questions": []map[string]interface{}{
			{"text": "Complexity?", "sort_order": 0},
			{"text": "Risk?", "sort_order": 1},
			{"text": "Effort?", "sort_order": 2},
		},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("update qs: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify update: list again
	w = env.do(t, "GET", fmt.Sprintf("/api/rooms/%d/question-sets", roomID), nil)
	env.decode(t, w, &sets)
	if sets[0]["name"] != "Sprint 1 Updated" {
		t.Errorf("expected updated name, got %v", sets[0]["name"])
	}
	questions = sets[0]["questions"].([]interface{})
	if len(questions) != 3 {
		t.Errorf("expected 3 questions after update, got %d", len(questions))
	}

	// Delete
	w = env.do(t, "DELETE", fmt.Sprintf("/api/question-sets/%d", qsID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete qs: expected 200, got %d", w.Code)
	}

	// Verify deleted
	w = env.do(t, "GET", fmt.Sprintf("/api/rooms/%d/question-sets", roomID), nil)
	env.decode(t, w, &sets)
	if len(sets) != 0 {
		t.Errorf("expected 0 question sets after delete, got %d", len(sets))
	}
}

func TestQuestionSetUpdate_NoDuplication(t *testing.T) {
	env := setup(t)

	// Create room
	w := env.do(t, "POST", "/api/rooms", map[string]string{"name": "DupRoom", "passphrase": "p"})
	var room map[string]interface{}
	env.decode(t, w, &room)
	roomID := int64(room["id"].(float64))

	// Create question set with 2 questions
	w = env.do(t, "POST", fmt.Sprintf("/api/rooms/%d/question-sets", roomID), map[string]interface{}{
		"name": "Original",
		"questions": []map[string]interface{}{
			{"text": "Q1", "sort_order": 0},
			{"text": "Q2", "sort_order": 1},
		},
	})
	var qs map[string]interface{}
	env.decode(t, w, &qs)
	qsID := int64(qs["id"].(float64))

	// Update with same 2 questions (different text)
	env.do(t, "PUT", fmt.Sprintf("/api/question-sets/%d", qsID), map[string]interface{}{
		"name": "Updated",
		"questions": []map[string]interface{}{
			{"text": "Q1 edited", "sort_order": 0},
			{"text": "Q2 edited", "sort_order": 1},
		},
	})

	// Verify: should still be exactly 2 questions, not 4
	w = env.do(t, "GET", fmt.Sprintf("/api/rooms/%d/question-sets", roomID), nil)
	var sets []map[string]interface{}
	env.decode(t, w, &sets)
	questions := sets[0]["questions"].([]interface{})
	if len(questions) != 2 {
		t.Errorf("question duplication bug: expected 2 questions, got %d", len(questions))
	}
	q1 := questions[0].(map[string]interface{})
	if q1["text"] != "Q1 edited" {
		t.Errorf("expected updated text 'Q1 edited', got %v", q1["text"])
	}
}

func TestQuestionSetUpdate_RemoveQuestion(t *testing.T) {
	env := setup(t)

	w := env.do(t, "POST", "/api/rooms", map[string]string{"name": "DelRoom", "passphrase": "p"})
	var room map[string]interface{}
	env.decode(t, w, &room)
	roomID := int64(room["id"].(float64))

	// Create with 3 questions
	w = env.do(t, "POST", fmt.Sprintf("/api/rooms/%d/question-sets", roomID), map[string]interface{}{
		"name": "ThreeQs",
		"questions": []map[string]interface{}{
			{"text": "Q1", "sort_order": 0},
			{"text": "Q2", "sort_order": 1},
			{"text": "Q3", "sort_order": 2},
		},
	})
	var qs map[string]interface{}
	env.decode(t, w, &qs)
	qsID := int64(qs["id"].(float64))

	// Update: remove Q2, keep Q1 and Q3
	w = env.do(t, "PUT", fmt.Sprintf("/api/question-sets/%d", qsID), map[string]interface{}{
		"name": "TwoQs",
		"questions": []map[string]interface{}{
			{"text": "Q1", "sort_order": 0},
			{"text": "Q3", "sort_order": 1},
		},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify: should be exactly 2 questions
	w = env.do(t, "GET", fmt.Sprintf("/api/rooms/%d/question-sets", roomID), nil)
	var sets []map[string]interface{}
	env.decode(t, w, &sets)
	questions := sets[0]["questions"].([]interface{})
	if len(questions) != 2 {
		t.Errorf("expected 2 questions after removing one, got %d", len(questions))
	}
	q1 := questions[0].(map[string]interface{})
	q2 := questions[1].(map[string]interface{})
	if q1["text"] != "Q1" || q2["text"] != "Q3" {
		t.Errorf("expected Q1 and Q3, got %v and %v", q1["text"], q2["text"])
	}
}

func TestQuestionSetUpdate_RemoveQuestionAfterVoting(t *testing.T) {
	env := setup(t)
	roomID, pid, qsID := setupVotingScenario(t, env)

	// Start a session and submit answers (creates answer rows referencing questions)
	w := env.do(t, "POST", fmt.Sprintf("/api/rooms/%d/sessions", roomID), map[string]interface{}{
		"question_set_id": qsID,
	})
	var sess map[string]interface{}
	env.decode(t, w, &sess)
	sessionID := int64(sess["session_id"].(float64))
	questions := sess["questions"].([]interface{})
	q1ID := fmt.Sprintf("%.0f", questions[0].(map[string]interface{})["id"].(float64))
	q2ID := fmt.Sprintf("%.0f", questions[1].(map[string]interface{})["id"].(float64))

	env.do(t, "POST", fmt.Sprintf("/api/sessions/%d/answers", sessionID), map[string]interface{}{
		"participant_id": pid,
		"answers":        map[string]string{q1ID: "3", q2ID: "5"},
	})

	// Now update the question set: remove the second question
	w = env.do(t, "PUT", fmt.Sprintf("/api/question-sets/%d", qsID), map[string]interface{}{
		"name": "Reduced",
		"questions": []map[string]interface{}{
			{"text": "Complexity only", "sort_order": 0},
		},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("update after voting: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify: should be 1 question, NOT 3 (1 new + 2 old stuck by FK)
	w = env.do(t, "GET", fmt.Sprintf("/api/rooms/%d/question-sets", roomID), nil)
	var sets []map[string]interface{}
	env.decode(t, w, &sets)
	qs := sets[0]["questions"].([]interface{})
	if len(qs) != 1 {
		t.Errorf("expected 1 question after update, got %d (FK constraint blocking DELETE?)", len(qs))
	}
}

func TestQuestionSetList_Empty(t *testing.T) {
	env := setup(t)

	w := env.do(t, "POST", "/api/rooms", map[string]string{"name": "Empty", "passphrase": "p"})
	var room map[string]interface{}
	env.decode(t, w, &room)
	roomID := int64(room["id"].(float64))

	w = env.do(t, "GET", fmt.Sprintf("/api/rooms/%d/question-sets", roomID), nil)
	var sets []map[string]interface{}
	env.decode(t, w, &sets)
	if len(sets) != 0 {
		t.Errorf("expected empty array, got %d", len(sets))
	}
}

// --- Session / Voting Tests ---

// helper: create a room, login a participant, create a question set. Returns roomID, participantID, qsID.
func setupVotingScenario(t *testing.T, env *testEnv) (roomID, participantID, qsID int64) {
	t.Helper()

	w := env.do(t, "POST", "/api/rooms", map[string]string{"name": "VoteRoom", "passphrase": "p"})
	var room map[string]interface{}
	env.decode(t, w, &room)
	roomID = int64(room["id"].(float64))

	w = env.do(t, "POST", "/api/rooms/login", map[string]interface{}{
		"room_id": roomID, "passphrase": "p", "name": "Voter1",
	})
	var login map[string]interface{}
	env.decode(t, w, &login)
	participantID = int64(login["participant_id"].(float64))

	w = env.do(t, "POST", fmt.Sprintf("/api/rooms/%d/question-sets", roomID), map[string]interface{}{
		"name": "Estimation",
		"questions": []map[string]interface{}{
			{"text": "Complexity", "sort_order": 0},
			{"text": "Risk", "sort_order": 1},
		},
	})
	var qs map[string]interface{}
	env.decode(t, w, &qs)
	qsID = int64(qs["id"].(float64))

	return
}

func TestStartSession(t *testing.T) {
	env := setup(t)
	roomID, _, qsID := setupVotingScenario(t, env)

	w := env.do(t, "POST", fmt.Sprintf("/api/rooms/%d/sessions", roomID), map[string]interface{}{
		"question_set_id": qsID,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var sess map[string]interface{}
	env.decode(t, w, &sess)
	if sess["session_id"].(float64) <= 0 {
		t.Errorf("expected positive session_id")
	}
	questions := sess["questions"].([]interface{})
	if len(questions) != 2 {
		t.Errorf("expected 2 questions, got %d", len(questions))
	}
}

func TestGetSessionQuestions(t *testing.T) {
	env := setup(t)
	roomID, _, qsID := setupVotingScenario(t, env)

	// Start session
	w := env.do(t, "POST", fmt.Sprintf("/api/rooms/%d/sessions", roomID), map[string]interface{}{
		"question_set_id": qsID,
	})
	var sess map[string]interface{}
	env.decode(t, w, &sess)
	sessionID := int64(sess["session_id"].(float64))

	// Get questions
	w = env.do(t, "GET", fmt.Sprintf("/api/sessions/%d/questions", sessionID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var questions []map[string]interface{}
	env.decode(t, w, &questions)
	if len(questions) != 2 {
		t.Errorf("expected 2 questions, got %d", len(questions))
	}
}

func TestSubmitAnswers(t *testing.T) {
	env := setup(t)
	roomID, pid, qsID := setupVotingScenario(t, env)

	// Start session
	w := env.do(t, "POST", fmt.Sprintf("/api/rooms/%d/sessions", roomID), map[string]interface{}{
		"question_set_id": qsID,
	})
	var sess map[string]interface{}
	env.decode(t, w, &sess)
	sessionID := int64(sess["session_id"].(float64))

	// Get question IDs
	questions := sess["questions"].([]interface{})
	q1ID := fmt.Sprintf("%.0f", questions[0].(map[string]interface{})["id"].(float64))
	q2ID := fmt.Sprintf("%.0f", questions[1].(map[string]interface{})["id"].(float64))

	// Submit answers
	w = env.do(t, "POST", fmt.Sprintf("/api/sessions/%d/answers", sessionID), map[string]interface{}{
		"participant_id": pid,
		"answers": map[string]string{
			q1ID: "3",
			q2ID: "5",
		},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var result map[string]interface{}
	env.decode(t, w, &result)
	if result["status"] != "ok" {
		t.Errorf("expected status ok, got %v", result["status"])
	}
	if result["voted"].(float64) != 1 {
		t.Errorf("expected voted=1, got %v", result["voted"])
	}
}

func TestVoteStatus(t *testing.T) {
	env := setup(t)
	roomID, pid1, qsID := setupVotingScenario(t, env)

	// Add second participant so auto-reveal doesn't fire when only pid1 votes
	w := env.do(t, "POST", "/api/rooms/login", map[string]interface{}{
		"room_id": roomID, "passphrase": "p", "name": "Voter2",
	})
	var login2 map[string]interface{}
	env.decode(t, w, &login2)

	// Start session
	w = env.do(t, "POST", fmt.Sprintf("/api/rooms/%d/sessions", roomID), map[string]interface{}{
		"question_set_id": qsID,
	})
	var sess map[string]interface{}
	env.decode(t, w, &sess)
	sessionID := int64(sess["session_id"].(float64))

	// Check status before voting
	w = env.do(t, "GET", fmt.Sprintf("/api/sessions/%d/status", sessionID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var status map[string]interface{}
	env.decode(t, w, &status)
	if status["status"] != "voting" {
		t.Errorf("expected status voting, got %v", status["status"])
	}

	// Participant 1 submits answer
	questions := sess["questions"].([]interface{})
	q1ID := fmt.Sprintf("%.0f", questions[0].(map[string]interface{})["id"].(float64))
	env.do(t, "POST", fmt.Sprintf("/api/sessions/%d/answers", sessionID), map[string]interface{}{
		"participant_id": pid1,
		"answers":        map[string]string{q1ID: "5"},
	})

	// Check status after pid1 voted (pid2 hasn't voted yet)
	w = env.do(t, "GET", fmt.Sprintf("/api/sessions/%d/status", sessionID), nil)
	var status2 map[string]interface{}
	env.decode(t, w, &status2)
	if status2["status"] != "voting" {
		t.Errorf("expected status still voting, got %v", status2["status"])
	}
	participants := status2["participants"].([]interface{})
	if len(participants) != 2 {
		t.Fatalf("expected 2 participants, got %d", len(participants))
	}
	// Find pid1's status
	var found bool
	for _, pp := range participants {
		p := pp.(map[string]interface{})
		if int64(p["participant_id"].(float64)) == pid1 {
			if p["has_voted"] != true {
				t.Errorf("expected pid1 has_voted=true")
			}
			found = true
		}
	}
	if !found {
		t.Errorf("pid1 not found in status")
	}
}

func TestAutoReveal_AllVoted(t *testing.T) {
	env := setup(t)
	roomID, pid, qsID := setupVotingScenario(t, env)

	// Start session
	w := env.do(t, "POST", fmt.Sprintf("/api/rooms/%d/sessions", roomID), map[string]interface{}{
		"question_set_id": qsID,
	})
	var sess map[string]interface{}
	env.decode(t, w, &sess)
	sessionID := int64(sess["session_id"].(float64))

	questions := sess["questions"].([]interface{})
	q1ID := fmt.Sprintf("%.0f", questions[0].(map[string]interface{})["id"].(float64))

	// Only 1 participant, so submitting should auto-reveal
	w = env.do(t, "POST", fmt.Sprintf("/api/sessions/%d/answers", sessionID), map[string]interface{}{
		"participant_id": pid,
		"answers":        map[string]string{q1ID: "8"},
	})
	var result map[string]interface{}
	env.decode(t, w, &result)
	if result["all_voted"] != true {
		t.Errorf("expected all_voted=true with single participant")
	}

	// Status should be revealed
	w = env.do(t, "GET", fmt.Sprintf("/api/sessions/%d/status", sessionID), nil)
	var status map[string]interface{}
	env.decode(t, w, &status)
	if status["status"] != "revealed" {
		t.Errorf("expected status revealed after all voted, got %v", status["status"])
	}
}

func TestRevealResults(t *testing.T) {
	env := setup(t)
	roomID, pid, qsID := setupVotingScenario(t, env)

	// Start session + submit
	w := env.do(t, "POST", fmt.Sprintf("/api/rooms/%d/sessions", roomID), map[string]interface{}{
		"question_set_id": qsID,
	})
	var sess map[string]interface{}
	env.decode(t, w, &sess)
	sessionID := int64(sess["session_id"].(float64))
	questions := sess["questions"].([]interface{})
	q1ID := fmt.Sprintf("%.0f", questions[0].(map[string]interface{})["id"].(float64))
	q2ID := fmt.Sprintf("%.0f", questions[1].(map[string]interface{})["id"].(float64))

	env.do(t, "POST", fmt.Sprintf("/api/sessions/%d/answers", sessionID), map[string]interface{}{
		"participant_id": pid,
		"answers":        map[string]string{q1ID: "3", q2ID: "5"},
	})

	// Get results
	w = env.do(t, "GET", fmt.Sprintf("/api/sessions/%d/results", sessionID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var results []map[string]interface{}
	env.decode(t, w, &results)
	if len(results) != 1 {
		t.Fatalf("expected 1 result entry, got %d", len(results))
	}
	answers := results[0]["answers"].(map[string]interface{})
	if answers[q1ID] != "3" {
		t.Errorf("expected answer '3' for q1, got %v", answers[q1ID])
	}
	if answers[q2ID] != "5" {
		t.Errorf("expected answer '5' for q2, got %v", answers[q2ID])
	}
}

func TestResetSession(t *testing.T) {
	env := setup(t)
	roomID, pid, qsID := setupVotingScenario(t, env)

	// Start + submit
	w := env.do(t, "POST", fmt.Sprintf("/api/rooms/%d/sessions", roomID), map[string]interface{}{
		"question_set_id": qsID,
	})
	var sess map[string]interface{}
	env.decode(t, w, &sess)
	sessionID := int64(sess["session_id"].(float64))
	questions := sess["questions"].([]interface{})
	q1ID := fmt.Sprintf("%.0f", questions[0].(map[string]interface{})["id"].(float64))

	env.do(t, "POST", fmt.Sprintf("/api/sessions/%d/answers", sessionID), map[string]interface{}{
		"participant_id": pid,
		"answers":        map[string]string{q1ID: "8"},
	})

	// Reset
	w = env.do(t, "POST", fmt.Sprintf("/api/sessions/%d/reset", sessionID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Status should be voting again
	w = env.do(t, "GET", fmt.Sprintf("/api/sessions/%d/status", sessionID), nil)
	var status map[string]interface{}
	env.decode(t, w, &status)
	if status["status"] != "voting" {
		t.Errorf("expected status voting after reset, got %v", status["status"])
	}

	// Results should be empty
	w = env.do(t, "GET", fmt.Sprintf("/api/sessions/%d/results", sessionID), nil)
	var results []map[string]interface{}
	env.decode(t, w, &results)
	if len(results) != 0 {
		t.Errorf("expected 0 results after reset, got %d", len(results))
	}
}

func TestMultipleParticipants_Voting(t *testing.T) {
	env := setup(t)
	roomID, pid1, qsID := setupVotingScenario(t, env)

	// Login second participant
	w := env.do(t, "POST", "/api/rooms/login", map[string]interface{}{
		"room_id": roomID, "passphrase": "p", "name": "Voter2",
	})
	var login map[string]interface{}
	env.decode(t, w, &login)
	pid2 := int64(login["participant_id"].(float64))

	// Start session
	w = env.do(t, "POST", fmt.Sprintf("/api/rooms/%d/sessions", roomID), map[string]interface{}{
		"question_set_id": qsID,
	})
	var sess map[string]interface{}
	env.decode(t, w, &sess)
	sessionID := int64(sess["session_id"].(float64))
	questions := sess["questions"].([]interface{})
	q1ID := fmt.Sprintf("%.0f", questions[0].(map[string]interface{})["id"].(float64))

	// Participant 1 votes — should NOT auto-reveal
	w = env.do(t, "POST", fmt.Sprintf("/api/sessions/%d/answers", sessionID), map[string]interface{}{
		"participant_id": pid1,
		"answers":        map[string]string{q1ID: "3"},
	})
	var r1 map[string]interface{}
	env.decode(t, w, &r1)
	if r1["all_voted"] != false {
		t.Errorf("expected all_voted=false after first participant votes")
	}

	// Participant 2 votes — should auto-reveal
	w = env.do(t, "POST", fmt.Sprintf("/api/sessions/%d/answers", sessionID), map[string]interface{}{
		"participant_id": pid2,
		"answers":        map[string]string{q1ID: "8"},
	})
	var r2 map[string]interface{}
	env.decode(t, w, &r2)
	if r2["all_voted"] != true {
		t.Errorf("expected all_voted=true after all participants voted")
	}

	// Results should contain both participants
	w = env.do(t, "GET", fmt.Sprintf("/api/sessions/%d/results", sessionID), nil)
	var results []map[string]interface{}
	env.decode(t, w, &results)
	if len(results) != 2 {
		t.Errorf("expected 2 result entries, got %d", len(results))
	}
}

func TestPassphraseIsHashed(t *testing.T) {
	env := setup(t)

	w := env.do(t, "POST", "/api/rooms", map[string]string{"name": "HashCheck", "passphrase": "mysecret"})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Login succeeds with correct passphrase (bcrypt comparison)
	w = env.do(t, "POST", "/api/rooms/login", map[string]interface{}{
		"room_name": "HashCheck", "passphrase": "mysecret", "name": "User1",
	})
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for correct passphrase, got %d", w.Code)
	}

	// Login fails with wrong passphrase
	w = env.do(t, "POST", "/api/rooms/login", map[string]interface{}{
		"room_name": "HashCheck", "passphrase": "wrong", "name": "User2",
	})
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for wrong passphrase, got %d", w.Code)
	}
}

func TestAnswerUpsert(t *testing.T) {
	env := setup(t)
	roomID, pid, qsID := setupVotingScenario(t, env)

	// Start session
	w := env.do(t, "POST", fmt.Sprintf("/api/rooms/%d/sessions", roomID), map[string]interface{}{
		"question_set_id": qsID,
	})
	var sess map[string]interface{}
	env.decode(t, w, &sess)
	sessionID := int64(sess["session_id"].(float64))
	questions := sess["questions"].([]interface{})
	q1ID := fmt.Sprintf("%.0f", questions[0].(map[string]interface{})["id"].(float64))

	// Submit once
	env.do(t, "POST", fmt.Sprintf("/api/sessions/%d/answers", sessionID), map[string]interface{}{
		"participant_id": pid,
		"answers":        map[string]string{q1ID: "3"},
	})

	// Reset and re-submit (simulating re-vote)
	env.do(t, "POST", fmt.Sprintf("/api/sessions/%d/reset", sessionID), nil)
	env.do(t, "POST", fmt.Sprintf("/api/sessions/%d/answers", sessionID), map[string]interface{}{
		"participant_id": pid,
		"answers":        map[string]string{q1ID: "13"},
	})

	// Result should show updated value
	w = env.do(t, "GET", fmt.Sprintf("/api/sessions/%d/results", sessionID), nil)
	var results []map[string]interface{}
	env.decode(t, w, &results)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	answers := results[0]["answers"].(map[string]interface{})
	if answers[q1ID] != "13" {
		t.Errorf("expected updated answer '13', got %v", answers[q1ID])
	}
}
