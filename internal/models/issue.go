package models

import "time"

type Issue struct {
	ID           string    `json:"id"`
	ProjectID    string    `json:"project_id"`
	Fingerprint  string    `json:"fingerprint"`
	Level        string    `json:"level"`
	ErrorType    string    `json:"error_type"`
	ErrorMessage string    `json:"error_message"`
	Occurrences  int       `json:"occurrences"`
	FirstSeen    time.Time `json:"first_seen"`
	LastSeen     time.Time `json:"last_seen"`
	Status       string    `json:"status"`
}

type Project struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	APIKey    string    `json:"api_key"`
	CreatedAt time.Time `json:"created_at"`
}
