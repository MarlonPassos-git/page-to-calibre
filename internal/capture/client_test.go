package capture

import (
	"context"
	"io"
	"net/http"
	"net/netip"
	"strings"
	"testing"
	"time"
)

func TestRejectNonPublicAddresses(t *testing.T) {
	for _, raw := range []string{"127.0.0.1", "::1", "::ffff:127.0.0.1", "10.1.2.3", "172.16.2.3", "192.168.1.2", "100.100.1.2", "169.254.169.254", "fe80::1", "fc00::1", "0.0.0.0", "224.0.0.1", "2002:7f00:1::"} {
		t.Run(raw, func(t *testing.T) {
			if PublicAddress(netip.MustParseAddr(raw)) {
				t.Fatal("non-public destination allowed")
			}
		})
	}
	for _, raw := range []string{"8.8.8.8", "2606:4700:4700::1111"} {
		if !PublicAddress(netip.MustParseAddr(raw)) {
			t.Fatal("public destination blocked", raw)
		}
	}
}
func TestValidateURL(t *testing.T) {
	for _, raw := range []string{"file:///etc/passwd", "ftp://example.org/a", "http://user:password@example.org", "https://example.org:9090", "broken"} {
		if _, err := ValidateURL(raw); err == nil {
			t.Fatal("accepted", raw)
		}
	}
}
func TestFetchBlocksLoopback(t *testing.T) {
	c := New(Limits{Timeout: time.Second, MaxBytes: 100, Redirects: 2})
	for _, raw := range []string{"http://127.0.0.1/", "http://localhost/", "http://[::1]/"} {
		if _, err := c.Fetch(context.Background(), raw); err == nil {
			t.Fatal("fetched loopback", raw)
		}
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestResourceLimitsAndTimeout(t *testing.T) {
	c := New(Limits{Timeout: 20 * time.Millisecond, MaxBytes: 4, Redirects: 2})
	c.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("12345")), Request: r}, nil
	})
	if _, err := c.Fetch(context.Background(), "https://public.example/"); err == nil {
		t.Fatal("oversized resource accepted")
	}
	c.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) { <-r.Context().Done(); return nil, r.Context().Err() })
	if _, err := c.Fetch(context.Background(), "https://public.example/"); err == nil {
		t.Fatal("timeout ignored")
	}
}
func TestRedirectToPrivateDNSIsBlocked(t *testing.T) {
	for _, destination := range []string{"http://127.0.0.1/", "http://localhost/", "http://169.254.169.254/"} {
		t.Run(destination, func(t *testing.T) {
			c := New(Limits{Timeout: time.Second, MaxBytes: 100, Redirects: 2})
			real := c.http.Transport
			c.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.Host == "public.example" {
					return &http.Response{StatusCode: 302, Header: http.Header{"Location": []string{destination}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
				}
				return real.RoundTrip(r)
			})
			if _, err := c.Fetch(context.Background(), "https://public.example/"); err == nil {
				t.Fatal("redirect reached private destination")
			}
		})
	}
}
func TestRedirectCountIsBounded(t *testing.T) {
	c := New(Limits{Timeout: time.Second, MaxBytes: 100, Redirects: 2})
	c.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 302, Header: http.Header{"Location": []string{"https://public.example/again"}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
	})
	if _, err := c.Fetch(context.Background(), "https://public.example/"); err == nil {
		t.Fatal("redirect loop accepted")
	}
}
