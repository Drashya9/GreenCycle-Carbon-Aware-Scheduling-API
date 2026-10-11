package httpapi

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"
	"time"
)

// cronTickTimeout bounds one cron-triggered scheduler pass.
const cronTickTimeout = 60 * time.Second

// handleCronTick runs one scheduler pass. Vercel Cron sends CRON_SECRET as
// "Authorization: Bearer <secret>"; any external pinger can do the same.
func (s *Server) handleCronTick(w http.ResponseWriter, r *http.Request) {
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || subtle.ConstantTimeCompare([]byte(token), []byte(s.cronSecret)) != 1 {
		writeError(w, http.StatusUnauthorized, "invalid cron secret")
		return
	}

	// Detach from the request: a caller that hangs up mid-pass must not abandon a run that
	// has already been claimed but not yet delivered.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), cronTickTimeout)
	defer cancel()

	writeJSON(w, http.StatusOK, map[string]int{"dueRuns": s.cronTick(ctx)})
}
