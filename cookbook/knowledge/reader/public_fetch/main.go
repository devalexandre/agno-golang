// SSRF-safe web fetching with WebsiteReader.PublicOnly / NewPublicHTTPClient.
//
// Turn PublicOnly on whenever the URL comes from an untrusted source (users,
// LLM tool calls). The reader then only fetches public http(s) URLs on
// standard ports: private/loopback/link-local addresses are rejected at dial
// time (after DNS resolution, so DNS rebinding does not help), redirects must
// stay on the same origin (max 3) and the body is capped at MaxBytes.
//
// Shows:
//  1. a local test server is readable by the default reader but refused by a
//     PublicOnly reader (non-standard port, and http://127.0.0.1 itself);
//  2. NewPublicHTTPClient used as a plain *http.Client refuses loopback too;
//  3. a public page (https://example.com) is fetched with PublicOnly.
//
// Steps 1-2 run offline; step 3 needs internet access (it only prints the
// error when offline).
//
//	go run ./cookbook/knowledge/reader/public_fetch
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/devalexandre/agno-golang/agno/knowledge/reader"
)

func main() {
	ctx := context.Background()

	// A "private" service, e.g. an internal admin page.
	internal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "<html><title>Internal admin</title><body>secret token: 42</body></html>")
	}))
	defer internal.Close()

	// 1a. Default reader (PublicOnly off, as in Python): anything goes.
	docs, err := reader.NewWebsiteReader().Read(ctx, internal.URL)
	if err == nil {
		fmt.Printf("default reader  %s -> %q\n", internal.URL, docs[0].Content)
	}

	// 1b. PublicOnly reader: private targets are refused.
	public := reader.NewWebsiteReader().WithPublicOnly(true)
	public.MaxBytes = 1 << 20
	for _, u := range []string{internal.URL, "http://127.0.0.1/", "http://localhost/", "http://169.254.169.254/latest/meta-data/"} {
		_, err := public.Read(ctx, u)
		fmt.Printf("public-only     %s -> refused=%v (%v)\n", u, isRefusal(err), err)
	}

	// 2. The SSRF-safe client on its own (any *http.Client use).
	_, err = reader.NewPublicHTTPClient().Get(internal.URL)
	fmt.Printf("public client   %s -> non-public=%v\n", internal.URL, errors.Is(err, reader.ErrNonPublicAddress))

	// URL-only check, without fetching.
	_, err = reader.ValidatePublicURL("https://user:pass@example.com/")
	fmt.Println("ValidatePublicURL with credentials ->", err)

	// 3. A public page is fetched normally.
	docs, err = public.Read(ctx, "https://example.com/")
	if err != nil {
		fmt.Println("public-only     https://example.com/ -> error (offline?):", err)
		return
	}
	fmt.Printf("public-only     https://example.com/ -> title=%q, %d chars\n", docs[0].Metadata["title"], len(docs[0].Content))
}

func isRefusal(err error) bool {
	return errors.Is(err, reader.ErrNonPublicAddress) || errors.Is(err, reader.ErrNonPublicURL)
}
