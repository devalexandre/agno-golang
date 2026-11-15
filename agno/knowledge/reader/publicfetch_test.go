package reader

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
)

func TestIsPublicIP(t *testing.T) {
	blocked := []string{
		"127.0.0.1", "10.1.2.3", "172.16.0.1", "192.168.1.1", "169.254.169.254",
		"100.64.0.1", "0.0.0.0", "224.0.0.1", "255.255.255.255", "198.18.0.1",
		"::1", "::", "fc00::1", "fd12:3456::1", "fe80::1", "ff02::1",
		"::ffff:127.0.0.1", "::ffff:8.8.8.8", "64:ff9b::a00:1", "2001:db8::1",
	}
	for _, s := range blocked {
		if IsPublicIP(netip.MustParseAddr(s)) {
			t.Errorf("%s should be blocked", s)
		}
	}
	allowed := []string{"8.8.8.8", "1.1.1.1", "93.184.216.34", "2606:4700:4700::1111", "64:ff9b::808:808"}
	for _, s := range allowed {
		if !IsPublicIP(netip.MustParseAddr(s)) {
			t.Errorf("%s should be allowed", s)
		}
	}
}

func TestValidatePublicURL(t *testing.T) {
	bad := map[string]error{
		"ftp://example.com/":         ErrNonPublicURL,
		"http://user:pw@example.com": ErrNonPublicURL,
		"http://example.com:8080/":   ErrNonPublicURL,
		"https://example.com:80/":    ErrNonPublicURL,
		"http://localhost/":          ErrNonPublicAddress,
		"http://api.internal/":       ErrNonPublicAddress,
		"http://printer.local./":     ErrNonPublicAddress,
		"http://127.0.0.1/":          ErrNonPublicAddress,
		"http://[::ffff:10.0.0.1]/":  ErrNonPublicAddress,
	}
	for raw, want := range bad {
		if _, err := ValidatePublicURL(raw); !errors.Is(err, want) {
			t.Errorf("%s: got %v want %v", raw, err, want)
		}
	}
	for _, ok := range []string{"https://example.com/a", "http://example.com:80/", "https://8.8.8.8/"} {
		if _, err := ValidatePublicURL(ok); err != nil {
			t.Errorf("%s: unexpected %v", ok, err)
		}
	}
}

// The dial-time check is what defeats DNS rebinding: even with URL validation
// skipped, a connection to a loopback address (literal or resolved from a
// hostname) is refused.
func TestPublicClientRejectsAtDialTime(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "secret")
	}))
	defer srv.Close()

	client := newPublicClient(publicPolicy{allowIP: IsPublicIP})
	for _, target := range []string{srv.URL, strings.Replace(srv.URL, "127.0.0.1", "localhost", 1)} {
		resp, err := client.Get(target)
		if err == nil {
			resp.Body.Close()
			t.Fatalf("%s: expected dial to be refused", target)
		}
		if !errors.Is(err, ErrNonPublicAddress) {
			t.Fatalf("%s: expected ErrNonPublicAddress, got %v", target, err)
		}
	}
}

func TestPublicClientRedirectPolicy(t *testing.T) {
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "other")
	}))
	defer other.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/same", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/final", http.StatusFound) })
	mux.HandleFunc("/final", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "ok") })
	mux.HandleFunc("/cross", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, other.URL, http.StatusFound) })
	mux.HandleFunc("/loop", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/loop", http.StatusFound) })
	mux.HandleFunc("/big", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, strings.Repeat("x", 100)) })
	srv := httptest.NewServer(mux)
	defer srv.Close()

	allowAll := publicPolicy{allowIP: func(netip.Addr) bool { return true }}
	client := newPublicClient(allowAll)
	ctx := context.Background()

	final, body, err := fetchPublicPage(ctx, client, allowAll, srv.URL+"/same", 0, nil)
	if err != nil || string(body) != "ok" || !strings.HasSuffix(final, "/final") {
		t.Fatalf("same-origin redirect: %q %q %v", final, body, err)
	}
	if _, _, err := fetchPublicPage(ctx, client, allowAll, srv.URL+"/cross", 0, nil); !errors.Is(err, ErrCrossOriginRedirect) {
		t.Fatalf("cross-origin: got %v", err)
	}
	if _, _, err := fetchPublicPage(ctx, client, allowAll, srv.URL+"/loop", 0, nil); !errors.Is(err, ErrTooManyRedirects) {
		t.Fatalf("loop: got %v", err)
	}
	if _, _, err := fetchPublicPage(ctx, client, allowAll, srv.URL+"/big", 10, nil); !errors.Is(err, ErrBodyTooLarge) {
		t.Fatalf("size cap: got %v", err)
	}
}

func TestWebsiteReaderPublicOnly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "<html><head><title>Internal</title></head><body>secret data</body></html>")
	}))
	defer srv.Close()

	// Default (PublicOnly off): local servers are readable, as before.
	docs, err := NewWebsiteReader().Read(context.Background(), srv.URL)
	if err != nil || len(docs) != 1 || docs[0].Content != "Internal secret data" {
		t.Fatalf("default reader: %v %v", docs, err)
	}

	// PublicOnly: the httptest server on 127.0.0.1 is refused.
	_, err = NewWebsiteReader().WithPublicOnly(true).Read(context.Background(), srv.URL)
	if err == nil || !(errors.Is(err, ErrNonPublicAddress) || errors.Is(err, ErrNonPublicURL)) {
		t.Fatalf("public-only reader should refuse loopback, got %v", err)
	}
	_, err = NewWebsiteReader().WithPublicOnly(true).Read(context.Background(), "http://127.0.0.1/")
	if !errors.Is(err, ErrNonPublicAddress) {
		t.Fatalf("expected ErrNonPublicAddress, got %v", err)
	}
}
