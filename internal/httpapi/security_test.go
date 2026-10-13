package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/cache"
	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/models"
	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/repository"
)

// httptest requests have Host "example.com" and RemoteAddr "192.0.2.1:1234".
func request(s *Server, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	return rec
}

func securedServer(opts SecurityOptions) *Server {
	s := &Server{applianceRepo: newFakeApplianceStore(), runRepo: &fakeRunStore{}}
	s.ConfigureSecurity(opts)
	return s
}

const newAppliance = `{"name":"Kettle","durationHours":1}`

func TestCORSOnlyForAllowlistedOrigins(t *testing.T) {
	s := securedServer(SecurityOptions{AllowedOrigins: []string{"https://friend.example"}})

	if got := request(s, http.MethodGet, "/appliances", "", map[string]string{"Origin": "https://evil.example"}).Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("unlisted origin got CORS access: %q", got)
	}
	if got := request(s, http.MethodGet, "/appliances", "", map[string]string{"Origin": "https://friend.example"}).Header().Get("Access-Control-Allow-Origin"); got != "https://friend.example" {
		t.Errorf("listed origin should be echoed back, got %q", got)
	}
	if got := request(securedServer(SecurityOptions{}), http.MethodGet, "/appliances", "", map[string]string{"Origin": "https://friend.example"}).Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("with no allowlist, nothing cross-origin should get CORS access, got %q", got)
	}
}

func TestCrossSiteWritesAreRejected(t *testing.T) {
	s := securedServer(SecurityOptions{AllowedOrigins: []string{"https://friend.example"}})

	for name, c := range map[string]struct {
		origin string
		want   int
	}{
		"other site":            {"https://evil.example", http.StatusForbidden},
		"sandboxed page":        {"null", http.StatusForbidden},
		"same site":             {"http://example.com", http.StatusCreated},
		"allowlisted origin":    {"https://friend.example", http.StatusCreated},
		"no Origin (curl etc.)": {"", http.StatusCreated},
	} {
		headers := map[string]string{}
		if c.origin != "" {
			headers["Origin"] = c.origin
		}
		body := strings.Replace(newAppliance, "Kettle", "Kettle "+name, 1)
		if rec := request(s, http.MethodPost, "/appliances", body, headers); rec.Code != c.want {
			t.Errorf("%s: expected %d, got %d: %s", name, c.want, rec.Code, rec.Body.String())
		}
	}

	if rec := request(s, http.MethodDelete, "/appliances/1", "", map[string]string{"Origin": "https://evil.example"}); rec.Code != http.StatusForbidden {
		t.Errorf("cross-site DELETE: expected 403, got %d", rec.Code)
	}
}

func TestSameSiteBehindProxyUsesForwardedHost(t *testing.T) {
	headers := map[string]string{"Origin": "https://app.example", "X-Forwarded-Host": "app.example"}

	if rec := request(securedServer(SecurityOptions{TrustProxyHeaders: true}), http.MethodPost, "/appliances", newAppliance, headers); rec.Code != http.StatusCreated {
		t.Errorf("behind a trusted proxy, the forwarded host is this site: expected 201, got %d", rec.Code)
	}
	if rec := request(securedServer(SecurityOptions{}), http.MethodPost, "/appliances", newAppliance, headers); rec.Code != http.StatusForbidden {
		t.Errorf("without proxy trust, X-Forwarded-Host must be ignored: expected 403, got %d", rec.Code)
	}
}

func TestWriteRateLimitPerClient(t *testing.T) {
	s := securedServer(SecurityOptions{WritesPerMinute: 2, TrustProxyHeaders: true})
	post := func(ip string, n int) *httptest.ResponseRecorder {
		body := strings.Replace(newAppliance, "Kettle", "Kettle "+ip+" "+string(rune('a'+n)), 1)
		return request(s, http.MethodPost, "/appliances", body, map[string]string{"X-Forwarded-For": ip + ", 10.0.0.1"})
	}

	post("203.0.113.1", 0)
	post("203.0.113.1", 1)
	rec := post("203.0.113.1", 2)
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("third write in a minute: expected 429 with Retry-After, got %d %q", rec.Code, rec.Header().Get("Retry-After"))
	}
	if rec := post("203.0.113.2", 0); rec.Code != http.StatusCreated {
		t.Errorf("another client has its own budget: expected 201, got %d", rec.Code)
	}
	if rec := request(s, http.MethodGet, "/appliances", "", map[string]string{"X-Forwarded-For": "203.0.113.1"}); rec.Code != http.StatusOK {
		t.Errorf("reads are never rate limited: expected 200, got %d", rec.Code)
	}
}

func TestRedisLimiterSharesCountsAcrossInstances(t *testing.T) {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379/0"
	}
	client, err := cache.NewClient(redisURL)
	if err != nil {
		t.Skipf("redis not reachable at %s, skipping: %v", redisURL, err)
	}
	defer client.Close()

	key := "test-client-" + strings.ReplaceAll(t.Name(), "/", "-")
	a := &redisLimiter{client: client, limit: 2} // two "instances" sharing one Redis
	b := &redisLimiter{client: client, limit: 2}

	ok1, _ := a.Allow(context.Background(), key)
	ok2, _ := b.Allow(context.Background(), key)
	ok3, retry := a.Allow(context.Background(), key)
	if !ok1 || !ok2 || ok3 || retry <= 0 {
		t.Errorf("expected allow, allow, deny-with-retry across instances; got %v %v %v (retry %v)", ok1, ok2, ok3, retry)
	}
}

// deleteStore returns a fixed error from Delete.
type deleteStore struct {
	failingApplianceStore
}

func TestDeleteRulesMapToStatuses(t *testing.T) {
	for err, want := range map[error]int{
		repository.ErrProtected:      http.StatusForbidden,
		repository.ErrHasPendingRuns: http.StatusConflict,
		repository.ErrNotFound:       http.StatusNotFound,
	} {
		s := &Server{applianceRepo: deleteStore{failingApplianceStore{err: err}}, runRepo: &fakeRunStore{}}
		if rec := serve(s, http.MethodDelete, "/appliances/1", ""); rec.Code != want {
			t.Errorf("%v: expected %d, got %d", err, want, rec.Code)
		}
	}
}

func TestApplianceNameLengthAndProtectedFlag(t *testing.T) {
	s := &Server{applianceRepo: newFakeApplianceStore(), runRepo: &fakeRunStore{}}

	long := strings.Repeat("x", maxNameLength+1)
	if rec := serve(s, http.MethodPost, "/appliances", `{"name":"`+long+`","durationHours":1}`); rec.Code != http.StatusBadRequest {
		t.Errorf("overlong name: expected 400, got %d", rec.Code)
	}

	rec := serve(s, http.MethodPost, "/appliances", `{"name":"Sneaky","durationHours":1,"protected":true}`)
	if rec.Code != http.StatusCreated || strings.Contains(rec.Body.String(), `"protected":true`) {
		t.Errorf("a client must not be able to create a protected appliance, got %d %s", rec.Code, rec.Body.String())
	}
	_ = models.Appliance{}
}
