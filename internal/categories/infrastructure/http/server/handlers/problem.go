package handlers

import (
	"encoding/json"
	"net/http"
)

const (
	contentTypeProblem = "application/problem+json"
	problemTypeBlank   = "about:blank"
	fallbackProblem    = `{"type":"about:blank","title":"Internal Server Error","status":500,"detail":"internal server error","version":0}`
)

type problemBody struct {
	Type    string         `json:"type"`
	Title   string         `json:"title"`
	Status  int            `json:"status"`
	Detail  string         `json:"detail,omitempty"`
	Errors  map[string]any `json:"errors,omitempty"`
	Version int64          `json:"version"`
}

func writeProblem(w http.ResponseWriter, statusCode int, message, code string, version int64) {
	body := problemBody{
		Type:    problemTypeBlank,
		Title:   http.StatusText(statusCode),
		Status:  statusCode,
		Detail:  message,
		Version: version,
	}
	if code != "" {
		body.Errors = map[string]any{"code": code}
	}
	b, err := json.Marshal(body)
	if err != nil {
		w.Header().Set("Content-Type", contentTypeProblem)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(fallbackProblem))
		return
	}
	w.Header().Set("Content-Type", contentTypeProblem)
	w.WriteHeader(statusCode)
	_, _ = w.Write(b)
}
