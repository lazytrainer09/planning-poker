package main

import (
	"log"
	"net/http"
	"os"

	"planning-poker/internal/db"
	"planning-poker/internal/handler"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "planning_poker.db"
	}

	database, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	hub := handler.NewHub()

	roomH := &handler.RoomHandler{DB: database, Hub: hub}
	questionH := &handler.QuestionHandler{DB: database}
	sessionH := &handler.SessionHandler{DB: database, Hub: hub}

	mux := http.NewServeMux()

	// API routes
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

	// WebSocket
	mux.HandleFunc("/ws", hub.HandleWS)

	log.Printf("Starting API server on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
