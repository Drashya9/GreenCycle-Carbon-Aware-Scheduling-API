// Package netguard keeps user-supplied webhook URLs from reaching anything but the public
// internet, so a webhook can't be used to make the server call its own internal network or
// a cloud metadata endpoint (SSRF).
package netguard

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"
)

// sharedAddressSpace is 100.64.0.0/10 (carrier-grade NAT), which net.IP.IsPrivate omits.
var sharedAddressSpace = &net.IPNet{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)}

// IsPublicIP reports whether ip is routable on the public internet.
func IsPublicIP(ip net.IP) bool {
	return !(ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsMulticast() ||
		sharedAddressSpace.Contains(ip))
}

// ValidateWebhookURL rejects anything but an absolute https URL whose host isn't an obviously
// internal name or a non-public IP literal. Hostnames are checked again after DNS resolution
// at send time by PublicOnlyClient, since a public-looking name can resolve to a private IP.
func ValidateWebhookURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return errors.New("webhookUrl must be an absolute https:// URL")
	}
	if u.User != nil {
		return errors.New("webhookUrl must not contain credentials")
	}

	host := strings.ToLower(u.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".internal") {
		return errors.New("webhookUrl must point to a public host")
	}
	if ip := net.ParseIP(host); ip != nil && !IsPublicIP(ip) {
		return errors.New("webhookUrl must point to a public host")
	}
	return nil
}

// PublicOnlyClient returns an HTTP client that refuses to connect to non-public addresses —
// checked on the resolved IP at dial time, which also defeats DNS rebinding — and never
// follows redirects, so a public endpoint can't bounce the request somewhere internal.
func PublicOnlyClient() *http.Client {
	dialer := &net.Dialer{
		Timeout: 10 * time.Second,
		Control: func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			if ip := net.ParseIP(host); ip == nil || !IsPublicIP(ip) {
				return fmt.Errorf("refusing to connect to non-public address %s", host)
			}
			return nil
		},
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = dialer.DialContext
	// Through a proxy, the dial check would see the proxy's address instead of the target's.
	transport.Proxy = nil

	return &http.Client{Transport: transport, CheckRedirect: NoRedirects}
}

// NoRedirects is an http.Client CheckRedirect that returns the 3xx response as-is.
func NoRedirects(*http.Request, []*http.Request) error {
	return http.ErrUseLastResponse
}
