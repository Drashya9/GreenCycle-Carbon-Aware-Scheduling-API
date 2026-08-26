package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/grid"
	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/models"
	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/repository"
)

// ── fakes ────────────────────────────────────────────────────────────────────────

type fakeApplianceStore struct {
	byName map[string]*models.Appliance
	all    []models.Appliance
	nextID int64
}

func newFakeApplianceStore(appliances ...models.Appliance) *fakeApplianceStore {
	s := &fakeApplianceStore{byName: map[string]*models.Appliance{}, nextID: int64(len(appliances) + 1)}
	for i := range appliances {
		a := appliances[i]
		s.all = append(s.all, a)
		s.byName[lower(a.Name)] = &a
	}
	return s
}

func lower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

func (f *fakeApplianceStore) FindAll(ctx context.Context) ([]models.Appliance, error) {
	return f.all, nil
}

func (f *fakeApplianceStore) FindByNameIgnoreCase(ctx context.Context, name string) (*models.Appliance, error) {
	if a, ok := f.byName[lower(name)]; ok {
		return a, nil
	}
	return nil, repository.ErrNotFound
}

func (f *fakeApplianceStore) Create(ctx context.Context, a *models.Appliance) error {
	a.ID = f.nextID
	f.nextID++
	f.all = append(f.all, *a)
	f.byName[lower(a.Name)] = a
	return nil
}

func (f *fakeApplianceStore) Delete(ctx context.Context, id int64) error {
	for i, a := range f.all {
		if a.ID == id {
			f.all = append(f.all[:i], f.all[i+1:]...)
			return nil
		}
	}
	return repository.ErrNotFound
}

type fakeRunStore struct {
	runs   []models.ScheduledRun
	nextID int64
}

func (f *fakeRunStore) Create(ctx context.Context, run *models.ScheduledRun) error {
	f.nextID++
	run.ID = f.nextID
	run.Status = models.StatusPending
	f.runs = append(f.runs, *run)
	return nil
}

func (f *fakeRunStore) FindAll(ctx context.Context, status string) ([]models.ScheduledRun, error) {
	if status == "" {
		return f.runs, nil
	}
	var out []models.ScheduledRun
	for _, r := range f.runs {
		if string(r.Status) == status {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeRunStore) FindByID(ctx context.Context, id int64) (*models.ScheduledRun, error) {
	for _, r := range f.runs {
		if r.ID == id {
			return &r, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (f *fakeRunStore) Cancel(ctx context.Context, id int64) error {
	for i, r := range f.runs {
		if r.ID == id && r.Status == models.StatusPending {
			f.runs[i].Status = models.StatusCancelled
			return nil
		}
	}
	return repository.ErrNotFound
}

type fakeProvider struct {
	name     string
	forecast []models.GridDataPoint
}

func (f *fakeProvider) Name() string { return f.name }

func (f *fakeProvider) GetForecast(ctx context.Context, zip string) ([]models.GridDataPoint, error) {
	return f.forecast, nil
}

func canned24hForecast() []models.GridDataPoint {
	base := time.Now().Truncate(time.Hour)
	forecast := make([]models.GridDataPoint, 24)
	for i := range forecast {
		v := 300.0
		if i == 3 || i == 4 {
			v = 90.0 // cheapest window
		}
		forecast[i] = models.GridDataPoint{Timestamp: base.Add(time.Duration(i) * time.Hour), CarbonIntensity: v}
	}
	return forecast
}

// ── tests ────────────────────────────────────────────────────────────────────────

func TestHandleCalculate_HappyPath(t *testing.T) {
	appliances := newFakeApplianceStore(models.Appliance{ID: 1, Name: "Dryer", DurationHours: 1.0})
	s := &Server{
		applianceRepo:    appliances,
		runRepo:          &fakeRunStore{},
		providers:        []grid.Provider{&fakeProvider{name: "mock", forecast: canned24hForecast()}},
		raceTimeout:      2 * time.Second,
		multiZoneWorkers: 3,
	}

	req := httptest.NewRequest(http.MethodGet, "/calculate?appliance=Dryer&zip=85281", nil)
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var window models.OptimalWindow
	if err := json.Unmarshal(rec.Body.Bytes(), &window); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if window.ApplianceName != "Dryer" {
		t.Errorf("expected applianceName 'Dryer', got %q", window.ApplianceName)
	}
	if window.AverageCarbonIntensity != 90.0 {
		t.Errorf("expected avg 90.0, got %v", window.AverageCarbonIntensity)
	}
	if window.Provider != "mock" {
		t.Errorf("expected provider 'mock', got %q", window.Provider)
	}
}

func TestHandleCalculate_UnknownAppliance(t *testing.T) {
	appliances := newFakeApplianceStore()
	s := &Server{
		applianceRepo:    appliances,
		runRepo:          &fakeRunStore{},
		providers:        []grid.Provider{&fakeProvider{name: "mock", forecast: canned24hForecast()}},
		raceTimeout:      2 * time.Second,
		multiZoneWorkers: 3,
	}

	req := httptest.NewRequest(http.MethodGet, "/calculate?appliance=Toaster&zip=85281", nil)
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}

	var body errorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode error body: %v", err)
	}
	if body.Status != http.StatusBadRequest || body.Error == "" {
		t.Errorf("expected a well-formed error body, got: %+v", body)
	}
}

func TestApplianceCRUD(t *testing.T) {
	appliances := newFakeApplianceStore()
	s := &Server{applianceRepo: appliances, runRepo: &fakeRunStore{}}

	// Create
	createBody, _ := json.Marshal(map[string]any{"name": "Hot Tub", "durationHours": 3.0})
	req := httptest.NewRequest(http.MethodPost, "/appliances", bytes.NewReader(createBody))
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	// List
	req = httptest.NewRequest(http.MethodGet, "/appliances", nil)
	rec = httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	var list []models.Appliance
	json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 1 || list[0].Name != "Hot Tub" {
		t.Fatalf("expected 1 appliance 'Hot Tub', got %+v", list)
	}

	// Delete missing id -> 404
	req = httptest.NewRequest(http.MethodDelete, "/appliances/999", nil)
	rec = httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing appliance, got %d", rec.Code)
	}
}

func TestHandleCreateSchedule(t *testing.T) {
	appliances := newFakeApplianceStore(models.Appliance{ID: 1, Name: "Pool Pump", DurationHours: 2.0})
	runs := &fakeRunStore{}
	s := &Server{
		applianceRepo:    appliances,
		runRepo:          runs,
		providers:        []grid.Provider{&fakeProvider{name: "mock", forecast: canned24hForecast()}},
		raceTimeout:      2 * time.Second,
		multiZoneWorkers: 3,
	}

	body, _ := json.Marshal(map[string]any{"applianceName": "Pool Pump", "zip": "85281"})
	req := httptest.NewRequest(http.MethodPost, "/schedules", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var run models.ScheduledRun
	if err := json.Unmarshal(rec.Body.Bytes(), &run); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if run.Status != models.StatusPending {
		t.Errorf("expected status 'pending', got %q", run.Status)
	}
	if run.ApplianceName != "Pool Pump" {
		t.Errorf("expected applianceName 'Pool Pump', got %q", run.ApplianceName)
	}
	if len(runs.runs) != 1 {
		t.Fatalf("expected the run to be persisted, got %d runs", len(runs.runs))
	}
}
