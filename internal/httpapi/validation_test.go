package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/grid"
	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/models"
)

type failingProvider struct{}

func (failingProvider) Name() string { return "broken" }
func (failingProvider) GetForecast(context.Context, string) ([]models.GridDataPoint, error) {
	return nil, errors.New("upstream returned 502")
}

func validationServer(providers ...grid.Provider) *Server {
	if len(providers) == 0 {
		providers = []grid.Provider{&fakeProvider{name: "mock", forecast: canned24hForecast()}}
	}
	return &Server{
		applianceRepo:    newFakeApplianceStore(models.Appliance{ID: 1, Name: "Dryer", DurationHours: 1}),
		runRepo:          &fakeRunStore{},
		providers:        providers,
		raceTimeout:      2 * time.Second,
		multiZoneWorkers: 2,
	}
}

func TestZipCodesAreValidated(t *testing.T) {
	s := validationServer()
	for _, c := range []struct{ method, path, body string }{
		{http.MethodGet, "/calculate?appliance=Dryer&zip=!!!", ""},
		{http.MethodGet, "/calculate?appliance=Dryer&zip=8528", ""},
		{http.MethodGet, "/calculate/multi?appliance=Dryer&zips=85281,not-a-zip%3Cscript%3E", ""},
		{http.MethodPost, "/schedules", `{"applianceName":"Dryer","zip":"abcde"}`},
	} {
		if rec := serve(s, c.method, c.path, c.body); rec.Code != http.StatusBadRequest {
			t.Errorf("%s %s: expected 400, got %d: %s", c.method, c.path, rec.Code, rec.Body.String())
		}
	}
}

func TestMultiZoneDedupesAndCaps(t *testing.T) {
	s := validationServer()

	rec := serve(s, http.MethodGet, "/calculate/multi?appliance=Dryer&zips=85281,85281,%2085281,90001", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body struct{ Results []models.ZoneResult }
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Results) != 2 {
		t.Errorf("expected duplicates collapsed to 2 results, got %d", len(body.Results))
	}

	eleven := "10001,10002,10003,10004,10005,10006,10007,10008,10009,10010,10011"
	if rec := serve(s, http.MethodGet, "/calculate/multi?appliance=Dryer&zips="+eleven, ""); rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for 11 zips, got %d", rec.Code)
	}
}

func TestApplianceDurationAndNameRules(t *testing.T) {
	s := validationServer()

	for _, body := range []string{
		`{"name":"Long","durationHours":24.5}`,
		`{"name":"Huge","durationHours":1e308}`,
		`{"name":"Zero","durationHours":0}`,
		`{"name":"   ","durationHours":1}`,
	} {
		if rec := serve(s, http.MethodPost, "/appliances", body); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: expected 400, got %d", body, rec.Code)
		}
	}

	rec := serve(s, http.MethodPost, "/appliances", `{"name":"  Hot Tub  ","durationHours":24}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for a 24h appliance, got %d: %s", rec.Code, rec.Body.String())
	}
	var created models.Appliance
	json.Unmarshal(rec.Body.Bytes(), &created)
	if created.Name != "Hot Tub" {
		t.Errorf("expected the name to be stored trimmed, got %q", created.Name)
	}
}

func TestWebhookURLIsValidated(t *testing.T) {
	s := validationServer()
	for _, hook := range []string{"not a url", "http://example.com/hook", "https://127.0.0.1/hook", "https://169.254.169.254/latest/meta-data"} {
		body := `{"applianceName":"Dryer","zip":"85281","webhookUrl":"` + hook + `"}`
		if rec := serve(s, http.MethodPost, "/schedules", body); rec.Code != http.StatusBadRequest {
			t.Errorf("webhook %q: expected 400, got %d", hook, rec.Code)
		}
	}
}

func TestProviderFailureIs503(t *testing.T) {
	s := validationServer(failingProvider{})

	for _, c := range []struct{ method, path, body string }{
		{http.MethodGet, "/calculate?appliance=Dryer&zip=85281", ""},
		{http.MethodPost, "/schedules", `{"applianceName":"Dryer","zip":"85281"}`},
	} {
		rec := serve(s, c.method, c.path, c.body)
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("%s %s: expected 503, got %d", c.method, c.path, rec.Code)
		}
		if strings.Contains(rec.Body.String(), "upstream returned 502") {
			t.Errorf("%s %s: leaked provider error detail: %s", c.method, c.path, rec.Body.String())
		}
	}
}

func TestNotFoundAndHeadReturnUsefulResponses(t *testing.T) {
	s := validationServer()

	for _, path := range []string{"/no-such-route", "/schedules/999999"} {
		rec := serve(s, http.MethodGet, path, "")
		if rec.Code != http.StatusNotFound || !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
			t.Errorf("GET %s: expected a JSON 404, got %d %q: %s", path, rec.Code, rec.Header().Get("Content-Type"), rec.Body.String())
		}
	}
	if rec := serve(s, http.MethodDelete, "/appliances/999", ""); rec.Code != http.StatusNotFound || rec.Body.Len() == 0 {
		t.Errorf("DELETE missing appliance: expected a JSON 404 body, got %d %q", rec.Code, rec.Body.String())
	}
	if rec := serve(s, http.MethodHead, "/appliances", ""); rec.Code != http.StatusOK {
		t.Errorf("HEAD /appliances: expected 200, got %d", rec.Code)
	}
}
