package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func cronRequest(s *Server, authHeader string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/cron/tick", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	return rec
}

func TestCronTick_NotRegisteredWithoutSecret(t *testing.T) {
	s := &Server{applianceRepo: newFakeApplianceStore(), runRepo: &fakeRunStore{}}
	s.EnableCron("", func(context.Context) int { return 0 })

	if rec := cronRequest(s, "Bearer anything"); rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 when no secret is configured, got %d", rec.Code)
	}
}

func TestCronTick_RejectsBadCredentials(t *testing.T) {
	calls := 0
	s := &Server{applianceRepo: newFakeApplianceStore(), runRepo: &fakeRunStore{}}
	s.EnableCron("s3cret", func(context.Context) int { calls++; return 1 })

	for name, header := range map[string]string{
		"missing header":        "",
		"wrong token":           "Bearer nope",
		"secret without Bearer": "s3cret",
	} {
		if rec := cronRequest(s, header); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: expected 401, got %d", name, rec.Code)
		}
	}
	if calls != 0 {
		t.Errorf("expected the scheduler pass never to run for rejected requests, ran %d times", calls)
	}
}

func TestCronTick_RunsPassWithValidToken(t *testing.T) {
	s := &Server{applianceRepo: newFakeApplianceStore(), runRepo: &fakeRunStore{}}
	s.EnableCron("s3cret", func(context.Context) int { return 3 })

	rec := cronRequest(s, "Bearer s3cret")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var body map[string]int
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["dueRuns"] != 3 {
		t.Errorf("expected dueRuns 3, got %v", body)
	}
}
