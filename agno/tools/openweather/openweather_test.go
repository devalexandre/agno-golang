package openweather

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenWeather(t *testing.T) {
	var lastQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("appid") != "k" {
			w.WriteHeader(401)
			return
		}
		switch r.URL.Path {
		case "/geo/direct":
			if r.URL.Query().Get("q") == "Nowhere" {
				_, _ = w.Write([]byte(`[]`))
				return
			}
			_, _ = w.Write([]byte(`[{"name":"London","country":"GB","lat":51.5,"lon":-0.12}]`))
		default:
			lastQuery = r.URL.RawQuery
			_, _ = w.Write([]byte(`{"main":{"temp":12}}`))
		}
	}))
	defer srv.Close()
	o := NewOpenWeatherTools(WithAPIKey("k"), WithBaseURLs(srv.URL+"/data", srv.URL+"/geo"))
	out, _ := o.GetForecast(ForecastParams{Location: "London", Days: 9})
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(out.(string)), &m); err != nil {
		t.Fatal(err)
	}
	if m["location_name"] != "London" || m["country"] != "GB" || !strings.Contains(lastQuery, "cnt=40") || !strings.Contains(lastQuery, "units=metric") {
		t.Fatalf("%v %s", m, lastQuery)
	}
	out, _ = o.GetCurrentWeather(LocationParams{Location: "Nowhere"})
	if !strings.Contains(out.(string), "No location found") {
		t.Fatal(out)
	}
}
