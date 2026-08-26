package httpapi

import (
	"encoding/json"
	"net/http"
	"time"
)

// errorBody mirrors the Java version's GlobalExceptionHandler shape, so any existing
// client code parsing {timestamp, status, error, message} keeps working.
type errorBody struct {
	Timestamp string `json:"timestamp"`
	Status    int    `json:"status"`
	Error     string `json:"error"`
	Message   string `json:"message"`
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorBody{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Status:    status,
		Error:     http.StatusText(status),
		Message:   message,
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
