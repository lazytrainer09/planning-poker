package model

import "time"

type Room struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	Passphrase string    `json:"-"`
	CreatedAt  time.Time `json:"created_at"`
}

type QuestionSet struct {
	ID        int64      `json:"id"`
	RoomID    int64      `json:"room_id"`
	Name      string     `json:"name"`
	Questions []Question `json:"questions,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type Question struct {
	ID            int64  `json:"id"`
	QuestionSetID int64  `json:"question_set_id"`
	Text          string `json:"text"`
	SortOrder     int    `json:"sort_order"`
}

type Session struct {
	ID            int64     `json:"id"`
	RoomID        int64     `json:"room_id"`
	QuestionSetID int64     `json:"question_set_id"`
	Status        string    `json:"status"` // "voting" or "revealed"
	CreatedAt     time.Time `json:"created_at"`
}

type Participant struct {
	ID          int64     `json:"id"`
	RoomID      int64     `json:"room_id"`
	Name        string    `json:"name"`
	ConnectedAt time.Time `json:"connected_at"`
}

type Answer struct {
	ID            int64  `json:"id"`
	SessionID     int64  `json:"session_id"`
	ParticipantID int64  `json:"participant_id"`
	QuestionID    int64  `json:"question_id"`
	Text          string `json:"text"`
}

// WebSocket message types
type WSMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

type VoteStatus struct {
	ParticipantID   int64  `json:"participant_id"`
	ParticipantName string `json:"participant_name"`
	HasVoted        bool   `json:"has_voted"`
}

type RevealResult struct {
	ParticipantID   int64             `json:"participant_id"`
	ParticipantName string            `json:"participant_name"`
	Answers         map[int64]string  `json:"answers"` // question_id -> text
}
