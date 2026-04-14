package models

import "time"

type Breadcrumb struct {
	Type      string                 `json:"type"`
	Message   string                 `json:"message"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

type ErrorDetail struct {
	Type       string `json:"type"`
	Message    string `json:"message"`
	Stacktrace string `json:"stacktrace,omitempty"`
}

type ContextData struct {
	URL       string `json:"url"`
	UserAgent string `json:"user_agent"`
	Framework string `json:"framework,omitempty"`
}


type EventPayload struct {
	ProjectID   string       `json:"project_id"`
	Level       string       `json:"level"`
	Error       ErrorDetail  `json:"error"`
	Context     ContextData  `json:"context"`
	Breadcrumbs []Breadcrumb `json:"breadcrumbs"`
}
