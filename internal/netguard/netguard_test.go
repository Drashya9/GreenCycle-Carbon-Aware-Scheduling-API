package netguard

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidateWebhookURL(t *testing.T) {
	ok := []string{
		"https://webhook.site/abc-123",
		"https://example.com:8443/hooks?x=1",
		"https://8.8.8.8/hook",
	}
	for _, u := range ok {
		if err := ValidateWebhookURL(u); err != nil {
			t.Errorf("%q: expected valid, got %v", u, err)
		}
	}

	bad := []string{
		"not a url",
		"http://example.com/hook", // plain http
		"ftp://example.com",
		"https://",
		"https://user:pass@example.com/hook",
		"https://localhost/hook",
		"https://api.localhost/hook",
		"https://metadata.google.internal/computeMetadata",
		"https://127.0.0.1/hook",
		"https://10.0.0.5/hook",
		"https://192.168.1.1/hook",
		"https://169.254.169.254/latest/meta-data", // cloud metadata
		"https://100.64.0.1/hook",                  // carrier-grade NAT
		"https://[::1]/hook",
		"https://0.0.0.0/hook",
	}
	for _, u := range bad {
		if err := ValidateWebhookURL(u); err == nil {
			t.Errorf("%q: expected rejection, got nil", u)
		}
	}
}

func TestIsPublicIP(t *testing.T) {
	for _, s := range []string{"8.8.8.8", "1.1.1.1", "2606:4700:4700::1111"} {
		if !IsPublicIP(net.ParseIP(s)) {
			t.Errorf("%s should be public", s)
		}
	}
	for _, s := range []string{"127.0.0.1", "10.1.2.3", "172.16.0.1", "192.168.0.1", "169.254.169.254", "100.100.100.100", "::1", "fe80::1", "fd00::1", "0.0.0.0", "224.0.0.1"} {
		if IsPublicIP(net.ParseIP(s)) {
			t.Errorf("%s should not be public", s)
		}
	}
}

// TestPublicOnlyClientBlocksLoopback proves the dial-time check: even a URL that passed
// validation can't reach a non-public address once it resolves to one.
func TestPublicOnlyClientBlocksLoopback(t *testing.T) {
	reached := false
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached = true }))
	defer srv.Close()

	_, err := PublicOnlyClient().Get(srv.URL) // srv listens on 127.0.0.1
	if err == nil || !strings.Contains(err.Error(), "non-public address") {
		t.Fatalf("expected the dial to be refused, got %v", err)
	}
	if reached {
		t.Error("the request reached the loopback server")
	}
}

func TestNoRedirectsReturnsTheRedirectResponse(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("redirect was followed")
	}))
	defer target.Close()
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer redirector.Close()

	resp, err := (&http.Client{CheckRedirect: NoRedirects}).Post(redirector.URL, "application/json", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTemporaryRedirect {
		t.Errorf("expected the 307 itself, got %d", resp.StatusCode)
	}
}
