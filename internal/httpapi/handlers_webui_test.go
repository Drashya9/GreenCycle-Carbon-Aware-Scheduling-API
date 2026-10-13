package httpapi

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/web"
)

func TestFaviconIsServedAsSVG(t *testing.T) {
	s := &Server{applianceRepo: newFakeApplianceStore(), runRepo: &fakeRunStore{}, webUI: web.FS}

	for _, path := range []string{"/favicon.svg", "/favicon.ico"} {
		rec := serve(s, http.MethodGet, path, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s: expected 200, got %d", path, rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "image/svg+xml" {
			t.Errorf("GET %s: expected image/svg+xml, got %q", path, ct)
		}
		if !strings.Contains(rec.Body.String(), "<svg") {
			t.Errorf("GET %s: body is not an SVG", path)
		}
	}

	page := serve(s, http.MethodGet, "/", "").Body.String()
	if !strings.Contains(page, `rel="icon" href="/favicon.svg"`) {
		t.Error("index.html does not link the favicon")
	}
}
