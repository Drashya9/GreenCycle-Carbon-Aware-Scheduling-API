package httpapi

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/grid"
	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/models"
	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/repository"
)

// dbInternals stands in for the kind of driver error text that must never reach a client.
var dbInternals = errors.New(`pq: duplicate key value violates unique constraint "secret_idx" (SQLSTATE 23505) host=db.internal`)

// failingApplianceStore returns err from every call.
type failingApplianceStore struct{ err error }

func (f failingApplianceStore) FindAll(context.Context) ([]models.Appliance, error) {
	return nil, f.err
}
func (f failingApplianceStore) FindByNameIgnoreCase(context.Context, string) (*models.Appliance, error) {
	return nil, f.err
}
func (f failingApplianceStore) Create(context.Context, *models.Appliance) error { return f.err }
func (f failingApplianceStore) Delete(context.Context, int64) error             { return f.err }

func serve(s *Server, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewReader([]byte(body)))
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	return rec
}

func TestEmptyListsAreArraysNotNull(t *testing.T) {
	s := &Server{applianceRepo: newFakeApplianceStore(), runRepo: &fakeRunStore{}}

	for _, path := range []string{"/appliances", "/schedules"} {
		rec := serve(s, http.MethodGet, path, "")
		if got := strings.TrimSpace(rec.Body.String()); got != "[]" {
			t.Errorf("GET %s with no rows: expected [], got %s", path, got)
		}
	}
}

func TestDuplicateApplianceIs409(t *testing.T) {
	s := &Server{applianceRepo: failingApplianceStore{err: repository.ErrConflict}, runRepo: &fakeRunStore{}}

	rec := serve(s, http.MethodPost, "/appliances", `{"name":"Dryer","durationHours":1}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `already exists`) {
		t.Errorf("expected a clean conflict message, got %s", rec.Body.String())
	}
}

func TestServerErrorsDoNotLeakInternals(t *testing.T) {
	s := &Server{
		applianceRepo:    failingApplianceStore{err: dbInternals},
		runRepo:          &fakeRunStore{},
		providers:        []grid.Provider{&fakeProvider{name: "mock", forecast: canned24hForecast()}},
		raceTimeout:      2 * time.Second,
		multiZoneWorkers: 1,
	}

	for _, c := range []struct{ method, path, body string }{
		{http.MethodGet, "/appliances", ""},
		{http.MethodPost, "/appliances", `{"name":"x","durationHours":1}`},
		{http.MethodDelete, "/appliances/1", ""},
		{http.MethodGet, "/calculate?appliance=Dryer&zip=85281", ""},
		{http.MethodGet, "/calculate/multi?appliance=Dryer&zips=85281", ""},
		{http.MethodPost, "/schedules", `{"applianceName":"Dryer","zip":"85281"}`},
	} {
		rec := serve(s, c.method, c.path, c.body)
		if rec.Code != http.StatusInternalServerError {
			t.Errorf("%s %s: expected 500 for a store failure, got %d", c.method, c.path, rec.Code)
		}
		if body := rec.Body.String(); strings.Contains(body, "secret_idx") || strings.Contains(body, "db.internal") {
			t.Errorf("%s %s: response leaked driver details: %s", c.method, c.path, body)
		}
	}
}

func TestScheduleResponseHasPlainNullableFields(t *testing.T) {
	appliances := newFakeApplianceStore(models.Appliance{ID: 1, Name: "Dryer", DurationHours: 1})
	s := &Server{
		applianceRepo:    appliances,
		runRepo:          &fakeRunStore{},
		providers:        []grid.Provider{&fakeProvider{name: "mock", forecast: canned24hForecast()}},
		raceTimeout:      2 * time.Second,
		multiZoneWorkers: 1,
	}

	rec := serve(s, http.MethodPost, "/schedules", `{"applianceName":"Dryer","zip":"85281","webhookUrl":"https://example.com/hook"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{`"webhookUrl":"https://example.com/hook"`, `"firedAt":null`, `"lastError":null`} {
		if !strings.Contains(body, want) {
			t.Errorf("expected %s in %s", want, body)
		}
	}
}
