package openrouteservice

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenRouteService(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/geocode/search":
			_, _ = w.Write([]byte(`{"features":[{"geometry":{"coordinates":[13.4,52.5]},"properties":{"label":"` + r.URL.Query().Get("text") + `, DE"}}]}`))
		case strings.HasPrefix(r.URL.Path, "/v2/directions/"):
			if r.Header.Get("Authorization") != "k" {
				w.WriteHeader(401)
				return
			}
			_, _ = w.Write([]byte(`{"routes":[{"summary":{"distance":654321,"duration":23456}}]}`))
		case strings.HasPrefix(r.URL.Path, "/v2/matrix/"):
			_, _ = w.Write([]byte(`{"distances":[[0,1],[1,0]],"durations":[[0,2],[2,0]]}`))
		}
	}))
	defer srv.Close()
	o := NewOpenRouteServiceTools(WithAPIKey("k"), WithBaseURL(srv.URL))
	out, _ := o.GetDirections(DirectionsParams{StartLocation: "Berlin", EndLocation: "Amsterdam"})
	r := out.(result)
	if r["distance_km"] != 654.32 || r["duration_minutes"] != 390.9 || r["start"] != "Berlin, DE" {
		t.Fatal(r)
	}
	out, _ = o.GetDirections(DirectionsParams{StartLocation: "a", EndLocation: "b", Profile: "train"})
	if !strings.Contains(out.(result)["error"].(string), "Invalid profile") {
		t.Fatal(out)
	}
	out, _ = o.GetDistanceMatrix(MatrixParams{Locations: []string{"a", "b"}})
	if out.(result)["distances_km"] == nil {
		t.Fatal(out)
	}
	out, _ = NewOpenRouteServiceTools(WithAPIKey("bad"), WithBaseURL(srv.URL)).GetDirections(DirectionsParams{StartLocation: "a", EndLocation: "b"})
	if out.(result)["error"] != "Invalid or unauthorized OpenRouteService API key." {
		t.Fatal(out)
	}
}
