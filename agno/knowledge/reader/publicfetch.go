package reader

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
	"syscall"
	"time"
)

// SSRF-safe public fetching. Port of agno-python
// knowledge/reader/utils/public_fetch.py: only public HTTP(S) URLs on standard
// ports are read, every connection is checked at dial time AFTER DNS
// resolution (so DNS rebinding cannot reach a private address), redirects stay
// on the original origin, and work is bounded in time and size.

const (
	// PublicFetchTimeout bounds each phase (dial, TLS handshake, response headers).
	PublicFetchTimeout = 10 * time.Second
	// PublicFetchDeadline bounds the whole fetch, redirects and body included.
	PublicFetchDeadline = 30 * time.Second
	// PublicFetchMaxRedirects is the number of same-origin redirects followed.
	PublicFetchMaxRedirects = 3
	// PublicFetchMaxBytes is the default body size limit (4 MiB).
	PublicFetchMaxBytes = 4 * 1024 * 1024
)

var (
	// ErrNonPublicAddress is returned when a URL or connection targets a
	// non-global address (loopback, private, link-local, CGNAT, ...).
	ErrNonPublicAddress = errors.New("public fetch: cannot read private network addresses")
	// ErrNonPublicURL is returned for URLs that are not public HTTP(S) URLs on a standard port.
	ErrNonPublicURL = errors.New("public fetch: a public HTTP(S) URL on a standard port is required")
	// ErrCrossOriginRedirect is returned when a redirect leaves the original origin.
	ErrCrossOriginRedirect = errors.New("public fetch: redirects must stay on the configured origin")
	// ErrTooManyRedirects is returned after PublicFetchMaxRedirects redirects.
	ErrTooManyRedirects = errors.New("public fetch: too many redirects")
	// ErrBodyTooLarge is returned when the response exceeds the size limit.
	ErrBodyTooLarge = errors.New("public fetch: response exceeds the download limit")
)

// nonGlobalPrefixes mirrors the IANA special-purpose registries Python's
// ipaddress.is_global is built on, plus multicast.
var nonGlobalPrefixes = func() []netip.Prefix {
	cidrs := []string{
		// IPv4
		"0.0.0.0/8",          // "this network", unspecified
		"10.0.0.0/8",         // private
		"100.64.0.0/10",      // CGNAT shared address space
		"127.0.0.0/8",        // loopback
		"169.254.0.0/16",     // link-local (cloud metadata lives here)
		"172.16.0.0/12",      // private
		"192.0.0.0/24",       // IETF protocol assignments
		"192.0.2.0/24",       // TEST-NET-1
		"192.88.99.0/24",     // 6to4 relay anycast (deprecated)
		"192.168.0.0/16",     // private
		"198.18.0.0/15",      // benchmarking
		"198.51.100.0/24",    // TEST-NET-2
		"203.0.113.0/24",     // TEST-NET-3
		"224.0.0.0/4",        // multicast
		"240.0.0.0/4",        // reserved
		"255.255.255.255/32", // broadcast
		// IPv6
		"::/128",         // unspecified
		"::1/128",        // loopback
		"::ffff:0:0/96",  // IPv4-mapped
		"64:ff9b:1::/48", // local-use NAT64
		"100::/64",       // discard-only
		"2001::/23",      // IETF protocol assignments
		"2001:db8::/32",  // documentation
		"2002::/16",      // 6to4
		"fc00::/7",       // unique local (ULA)
		"fe80::/10",      // link-local
		"fec0::/10",      // site-local (deprecated)
		"ff00::/8",       // multicast
	}
	out := make([]netip.Prefix, len(cidrs))
	for i, c := range cidrs {
		out[i] = netip.MustParsePrefix(c)
	}
	return out
}()

var nat64Prefix = netip.MustParsePrefix("64:ff9b::/96")

// IsPublicIP reports whether ip is a globally routable unicast address.
// IPv4-mapped IPv6 addresses are always rejected, so an IPv6 literal cannot
// smuggle a private IPv4 address; NAT64 (64:ff9b::/96) addresses are judged by
// the IPv4 address they embed.
func IsPublicIP(ip netip.Addr) bool {
	if !ip.IsValid() {
		return false
	}
	ip = ip.WithZone("")
	if ip.Is6() && !ip.Is4In6() {
		if nat64Prefix.Contains(ip) {
			b := ip.As16()
			return IsPublicIP(netip.AddrFrom4([4]byte{b[12], b[13], b[14], b[15]}))
		}
	}
	for _, p := range nonGlobalPrefixes {
		if p.Contains(ip) {
			return false
		}
	}
	return true
}

// ValidatePublicURL checks that raw is an http(s) URL with no credentials, on
// the scheme's standard port, whose host is not a local name and, when it is
// an IP literal, is public. Hostnames are resolved and checked at dial time.
func ValidatePublicURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNonPublicURL, err)
	}
	if err := validatePublicURL(u); err != nil {
		return nil, err
	}
	return u, nil
}

func validatePublicURL(u *url.URL) error {
	if u.Scheme != "http" && u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
		return ErrNonPublicURL
	}
	if port := u.Port(); port != "" && port != standardPort(u.Scheme) {
		return ErrNonPublicURL
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if host == "localhost" || strings.HasSuffix(host, ".localhost") ||
		strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return ErrNonPublicAddress
	}
	if ip, err := netip.ParseAddr(host); err == nil && !IsPublicIP(ip) {
		return ErrNonPublicAddress
	}
	return nil
}

func standardPort(scheme string) string {
	if scheme == "https" {
		return "443"
	}
	return "80"
}

// publicPolicy lets tests swap the address and URL checks.
type publicPolicy struct {
	allowIP     func(netip.Addr) bool
	validateURL func(*url.URL) error
}

var defaultPublicPolicy = publicPolicy{allowIP: IsPublicIP, validateURL: validatePublicURL}

// NewPublicHTTPClient returns an *http.Client that only connects to public
// addresses. The check runs in the dialer's Control hook, i.e. on the IP
// actually being connected to after DNS resolution, so DNS rebinding cannot
// bypass it. Proxies from the environment are ignored (a proxy would connect on
// our behalf), redirects must stay on the original origin and are limited to
// PublicFetchMaxRedirects, each phase times out after PublicFetchTimeout and
// the whole request after PublicFetchDeadline.
func NewPublicHTTPClient() *http.Client {
	return newPublicClient(defaultPublicPolicy)
}

func newPublicClient(p publicPolicy) *http.Client {
	dialer := &net.Dialer{
		Timeout: PublicFetchTimeout,
		Control: func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return fmt.Errorf("%w: %v", ErrNonPublicAddress, err)
			}
			ip, err := netip.ParseAddr(host)
			if err != nil || !p.allowIP(ip) {
				return fmt.Errorf("%w: %s", ErrNonPublicAddress, host)
			}
			return nil
		},
	}
	transport := &http.Transport{
		Proxy:                 nil,
		DialContext:           dialer.DialContext,
		TLSHandshakeTimeout:   PublicFetchTimeout,
		ResponseHeaderTimeout: PublicFetchTimeout,
		MaxIdleConns:          10,
		IdleConnTimeout:       30 * time.Second,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   PublicFetchDeadline,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > PublicFetchMaxRedirects {
				return ErrTooManyRedirects
			}
			origin := via[0].URL
			if !sameOrigin(origin, req.URL) {
				return ErrCrossOriginRedirect
			}
			if p.validateURL != nil {
				return p.validateURL(req.URL)
			}
			return nil
		},
	}
}

func sameOrigin(a, b *url.URL) bool {
	port := func(u *url.URL) string {
		if p := u.Port(); p != "" {
			return p
		}
		return standardPort(u.Scheme)
	}
	return a.Scheme == b.Scheme &&
		strings.EqualFold(strings.TrimSuffix(a.Hostname(), "."), strings.TrimSuffix(b.Hostname(), ".")) &&
		port(a) == port(b)
}

// FetchPublicPage fetches a single public page with NewPublicHTTPClient and
// returns the final URL and body. maxBytes <= 0 means PublicFetchMaxBytes.
func FetchPublicPage(ctx context.Context, rawURL string, maxBytes int64) (string, []byte, error) {
	return fetchPublicPage(ctx, NewPublicHTTPClient(), defaultPublicPolicy, rawURL, maxBytes, nil)
}

func fetchPublicPage(ctx context.Context, client *http.Client, p publicPolicy, rawURL string, maxBytes int64, header http.Header) (string, []byte, error) {
	if maxBytes <= 0 {
		maxBytes = PublicFetchMaxBytes
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", nil, fmt.Errorf("%w: %v", ErrNonPublicURL, err)
	}
	if p.validateURL != nil {
		if err := p.validateURL(u); err != nil {
			return "", nil, err
		}
	}
	u.Fragment = ""
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", nil, err
	}
	for k, v := range header {
		req.Header[k] = v
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", nil, fmt.Errorf("public fetch: GET %s returned %d", rawURL, resp.StatusCode)
	}
	if resp.ContentLength > maxBytes {
		return "", nil, ErrBodyTooLarge
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return "", nil, err
	}
	if int64(len(body)) > maxBytes {
		return "", nil, ErrBodyTooLarge
	}
	return resp.Request.URL.String(), body, nil
}
