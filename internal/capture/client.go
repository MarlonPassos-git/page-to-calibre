// Package capture fetches public HTTP resources without trusting DNS or redirects.
package capture

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

type Limits struct {
	Timeout   time.Duration
	MaxBytes  int64
	Redirects int
}
type Resource struct {
	URL         *url.URL
	Body        []byte
	ContentType string
}
type Client struct {
	http     *http.Client
	maxBytes int64
}

// PublicAddress excludes non-public destinations, including IPv4 mapped into IPv6.
func PublicAddress(addr netip.Addr) bool {
	addr = addr.Unmap()
	if addr.Is6() && !netip.MustParsePrefix("2000::/3").Contains(addr) {
		return false
	}
	if !addr.IsGlobalUnicast() || addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() {
		return false
	}
	for _, block := range []string{"100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001:db8::/32", "2001::/23", "64:ff9b::/96", "64:ff9b:1::/48", "2002::/16"} {
		if netip.MustParsePrefix(block).Contains(addr) {
			return false
		}
	}
	return true
}

func ValidateURL(raw string) (*url.URL, error) {
	if len(raw) > 4096 {
		return nil, errors.New("campo url: esperado endereço de até 4096 bytes")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil {
		return nil, errors.New("campo url: esperado endereço HTTP(S) absoluto sem credenciais")
	}
	if port := u.Port(); port != "" && port != "80" && port != "443" {
		return nil, errors.New("campo url: porta esperada 80 ou 443")
	}
	u.Fragment = ""
	return u, nil
}

func New(l Limits) *Client {
	dialer := &net.Dialer{Timeout: l.Timeout}
	transport := &http.Transport{Proxy: nil, TLSHandshakeTimeout: l.Timeout, ResponseHeaderTimeout: l.Timeout, MaxIdleConns: 10, IdleConnTimeout: 30 * time.Second}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		addresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		if err != nil {
			return nil, err
		}
		if len(addresses) == 0 {
			return nil, errors.New("DNS sem endereços")
		}
		// Validate every answer and dial a checked literal, closing the DNS-rebinding gap.
		for _, ip := range addresses {
			if !PublicAddress(ip) {
				return nil, errors.New("destino não público bloqueado")
			}
		}
		var last error
		for _, ip := range addresses {
			conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if err == nil {
				return conn, nil
			}
			last = err
		}
		return nil, last
	}
	return &Client{maxBytes: l.MaxBytes, http: &http.Client{Transport: transport, Timeout: l.Timeout, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > l.Redirects {
			return errors.New("limite de redirecionamentos excedido")
		}
		_, err := ValidateURL(req.URL.String())
		return err
	}}}
}

func (c *Client) Fetch(ctx context.Context, raw string) (Resource, error) {
	u, err := ValidateURL(raw)
	if err != nil {
		return Resource{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return Resource{}, err
	}
	req.Header.Set("User-Agent", "page-to-calibre/0.1 (+https://github.com/MarlonPassos-git/page-to-calibre)")
	res, err := c.http.Do(req)
	if err != nil {
		return Resource{}, fmt.Errorf("captura falhou: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return Resource{}, fmt.Errorf("origem respondeu HTTP %d", res.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, c.maxBytes+1))
	if err != nil {
		return Resource{}, err
	}
	if int64(len(body)) > c.maxBytes {
		return Resource{}, errors.New("recurso excede limite de bytes")
	}
	return Resource{URL: res.Request.URL, Body: body, ContentType: strings.ToLower(res.Header.Get("Content-Type"))}, nil
}
