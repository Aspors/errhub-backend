package models

import (
	"errors"
	"time"
)

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
	Release     string       `json:"release,omitempty"` // git commit hash, set automatically by SDK via build-time env
	Level       string       `json:"level"`
	Error       ErrorDetail  `json:"error"`
	Context     ContextData  `json:"context"`
	Breadcrumbs []Breadcrumb `json:"breadcrumbs"`
}

var validLevels = map[string]struct{}{
	"error":   {},
	"warning": {},
	"info":    {},
}

func (p *EventPayload) Validate() error {
	if p.ProjectID == "" {
		return errors.New("project_id is required")
	}
	if p.Error.Type == "" {
		return errors.New("error.type is required")
	}
	if p.Error.Message == "" {
		return errors.New("error.message is required")
	}
	if _, ok := validLevels[p.Level]; !ok {
		return errors.New("level must be one of: error, warning, info")
	}
	return nil
}
