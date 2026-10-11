package httpapi

import (
	"encoding/json"
	"log"
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

// writeInternalError logs err server-side and returns a generic 500, so database and driver
// details (constraint names, SQLSTATEs, hostnames) never reach a client.
func writeInternalError(w http.ResponseWriter, action string, err error) {
	log.Printf("httpapi: %s: %v", action, err)
	writeError(w, http.StatusInternalServerError, "failed to "+action)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
