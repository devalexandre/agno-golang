// Package openrouteservice ports Agno's OpenRouteServiceTools: geocoding,
// directions and distance matrices via api.openrouteservice.org.
package openrouteservice

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restapi"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// DefaultBaseURL is the OpenRouteService API base URL.
const DefaultBaseURL = "https://api.openrouteservice.org"

// ValidProfiles are the supported travel profiles.
var ValidProfiles = map[string]bool{
	"driving-car": true, "driving-hgv": true, "cycling-regular": true, "cycling-road": true,
	"cycling-mountain": true, "cycling-electric": true, "foot-walking": true, "foot-hiking": true, "wheelchair": true,
}

// OpenRouteServiceTools provides geocoding and road routing.
type OpenRouteServiceTools struct {
	toolkit.Toolkit
	apiKey     string
	baseURL    string
	httpClient *http.Client

	enableDirections, enableMatrix, enableGeocoding bool
}

// Option configures OpenRouteServiceTools.
type Option func(*OpenRouteServiceTools)

// WithAPIKey sets the API token (default ORS_API_KEY).
func WithAPIKey(k string) Option { return func(o *OpenRouteServiceTools) { o.apiKey = k } }

// WithBaseURL overrides the base URL.
func WithBaseURL(u string) Option { return func(o *OpenRouteServiceTools) { o.baseURL = u } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option {
	return func(o *OpenRouteServiceTools) { o.httpClient = c }
}

// WithTimeout sets the per-request timeout.
func WithTimeout(d time.Duration) Option {
	return func(o *OpenRouteServiceTools) { o.Toolkit.Timeout = d }
}

// WithDirections toggles GetDirections (default true).
func WithDirections(b bool) Option { return func(o *OpenRouteServiceTools) { o.enableDirections = b } }

// WithDistanceMatrix toggles GetDistanceMatrix (default true).
func WithDistanceMatrix(b bool) Option { return func(o *OpenRouteServiceTools) { o.enableMatrix = b } }

// WithGeocoding toggles GeocodeLocation (default true).
func WithGeocoding(b bool) Option { return func(o *OpenRouteServiceTools) { o.enableGeocoding = b } }

// WithAll enables every method.
func WithAll() Option {
	return func(o *OpenRouteServiceTools) {
		o.enableDirections, o.enableMatrix, o.enableGeocoding = true, true, true
	}
}

// GeocodeParams for GeocodeLocation.
type GeocodeParams struct {
	Location string `json:"location" description:"A place name or address, e.g. \"Berlin\", \"1600 Amphitheatre Pkwy\"." required:"true"`
}

// DirectionsParams for GetDirections.
type DirectionsParams struct {
	StartLocation string `json:"start_location" description:"Starting place name or address (e.g. \"Berlin\")." required:"true"`
	EndLocation   string `json:"end_location" description:"Destination place name or address (e.g. \"Amsterdam\")." required:"true"`
	Profile       string `json:"profile,omitempty" description:"Travel profile: driving-car (default), driving-hgv, cycling-regular, cycling-road, cycling-mountain, cycling-electric, foot-walking, foot-hiking, wheelchair. Public transit is not supported."`
}

// MatrixParams for GetDistanceMatrix.
type MatrixParams struct {
	Locations []string `json:"locations" description:"Place names or addresses (e.g. [\"Berlin\", \"Amsterdam\", \"Paris\"])." required:"true"`
	Profile   string   `json:"profile,omitempty" description:"Travel profile (see GetDirections). Defaults to driving-car."`
}

// NewOpenRouteServiceTools creates the toolkit.
func NewOpenRouteServiceTools(opts ...Option) *OpenRouteServiceTools {
	o := &OpenRouteServiceTools{apiKey: os.Getenv("ORS_API_KEY"), baseURL: DefaultBaseURL,
		enableDirections: true, enableMatrix: true, enableGeocoding: true}
	o.Toolkit = toolkit.NewToolkit()
	o.Toolkit.Name = "openrouteservice_tools"
	o.Toolkit.Description = "Geocoding and accurate road routing (distance and travel time) via OpenRouteService."
	for _, opt := range opts {
		opt(o)
	}
	o.baseURL = strings.TrimRight(o.baseURL, "/")
	if o.httpClient == nil {
		o.httpClient = o.Toolkit.HTTPClient()
	}
	if o.enableGeocoding {
		o.Toolkit.Register("GeocodeLocation", "Convert a place name or address into geographic coordinates.", o, o.GeocodeLocation, GeocodeParams{})
	}
	if o.enableDirections {
		o.Toolkit.Register("GetDirections", "Get the accurate routed distance and travel time between two locations. Place names are geocoded automatically.", o, o.GetDirections, DirectionsParams{})
	}
	if o.enableMatrix {
		o.Toolkit.Register("GetDistanceMatrix", "Compute a matrix of routed distances (km) and travel times (seconds) between many locations.", o, o.GetDistanceMatrix, MatrixParams{})
	}
	return o
}

type result = map[string]interface{}

func errRes(msg string) result { return result{"error": msg} }

func httpErr(err error) result {
	var apiErr *restapi.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.Status {
		case 401, 403:
			return errRes("Invalid or unauthorized OpenRouteService API key.")
		case 404:
			return errRes("No route found between the given locations for this profile.")
		case 429:
			return errRes("OpenRouteService rate limit exceeded. Try again later.")
		}
		return errRes(fmt.Sprintf("OpenRouteService API error: %d.", apiErr.Status))
	}
	return errRes(err.Error())
}

func validateProfile(p string) result {
	if ValidProfiles[p] {
		return nil
	}
	names := make([]string, 0, len(ValidProfiles))
	for k := range ValidProfiles {
		names = append(names, k)
	}
	sort.Strings(names)
	return errRes(fmt.Sprintf("Invalid profile '%s'. Valid profiles are: %s.", p, strings.Join(names, ", ")))
}

func round(v float64, d int) float64 {
	p := math.Pow(10, float64(d))
	return math.Round(v*p) / p
}

func (o *OpenRouteServiceTools) client() *restapi.Client {
	return restapi.New(o.baseURL, o.httpClient, map[string]string{"Authorization": o.apiKey})
}

func (o *OpenRouteServiceTools) geocode(location string) result {
	if o.apiKey == "" {
		return errRes("OpenRouteService API key is required. Set the ORS_API_KEY environment variable.")
	}
	var data struct {
		Features []struct {
			Geometry struct {
				Coordinates []float64 `json:"coordinates"`
			} `json:"geometry"`
			Properties map[string]interface{} `json:"properties"`
		} `json:"features"`
	}
	q := url.Values{"api_key": {o.apiKey}, "text": {location}, "size": {"1"}}
	if err := o.client().JSON(context.Background(), http.MethodGet, "/geocode/search", q, nil, &data); err != nil {
		return httpErr(err)
	}
	if len(data.Features) == 0 || len(data.Features[0].Geometry.Coordinates) < 2 {
		return errRes(fmt.Sprintf("No location found for '%s'.", location))
	}
	top := data.Features[0]
	label, _ := top.Properties["label"].(string)
	if label == "" {
		label = location
	}
	return result{"query": location, "label": label, "latitude": top.Geometry.Coordinates[1], "longitude": top.Geometry.Coordinates[0]}
}

// GeocodeLocation resolves a place to coordinates.
func (o *OpenRouteServiceTools) GeocodeLocation(p GeocodeParams) (interface{}, error) {
	return o.geocode(p.Location), nil
}

// GetDirections returns routed distance and duration between two places.
func (o *OpenRouteServiceTools) GetDirections(p DirectionsParams) (interface{}, error) {
	profile := p.Profile
	if profile == "" {
		profile = "driving-car"
	}
	if bad := validateProfile(profile); bad != nil {
		return bad, nil
	}
	start := o.geocode(p.StartLocation)
	if e, ok := start["error"]; ok {
		return errRes(fmt.Sprintf("Could not resolve start location: %v", e)), nil
	}
	end := o.geocode(p.EndLocation)
	if e, ok := end["error"]; ok {
		return errRes(fmt.Sprintf("Could not resolve end location: %v", e)), nil
	}
	payload := map[string]interface{}{"coordinates": [][]interface{}{
		{start["longitude"], start["latitude"]}, {end["longitude"], end["latitude"]}}}
	var data struct {
		Routes []struct {
			Summary struct {
				Distance float64 `json:"distance"`
				Duration float64 `json:"duration"`
			} `json:"summary"`
		} `json:"routes"`
	}
	if err := o.client().JSON(context.Background(), http.MethodPost, "/v2/directions/"+profile, nil, payload, &data); err != nil {
		return httpErr(err), nil
	}
	if len(data.Routes) == 0 {
		return errRes("No route found between the given locations for this profile."), nil
	}
	s := data.Routes[0].Summary
	return result{
		"start": start["label"], "end": end["label"], "profile": profile,
		"distance_km": round(s.Distance/1000, 2), "distance_meters": round(s.Distance, 1),
		"duration_seconds": round(s.Duration, 1), "duration_minutes": round(s.Duration/60, 1),
	}, nil
}

// GetDistanceMatrix computes distance/duration matrices.
func (o *OpenRouteServiceTools) GetDistanceMatrix(p MatrixParams) (interface{}, error) {
	profile := p.Profile
	if profile == "" {
		profile = "driving-car"
	}
	if bad := validateProfile(profile); bad != nil {
		return bad, nil
	}
	if len(p.Locations) < 2 {
		return errRes("Provide at least two locations for a distance matrix."), nil
	}
	var coords [][]interface{}
	var labels []interface{}
	for _, loc := range p.Locations {
		r := o.geocode(loc)
		if e, ok := r["error"]; ok {
			return errRes(fmt.Sprintf("Could not resolve location '%s': %v", loc, e)), nil
		}
		coords = append(coords, []interface{}{r["longitude"], r["latitude"]})
		labels = append(labels, r["label"])
	}
	payload := map[string]interface{}{"locations": coords, "metrics": []string{"distance", "duration"}, "units": "km"}
	var data map[string]interface{}
	if err := o.client().JSON(context.Background(), http.MethodPost, "/v2/matrix/"+profile, nil, payload, &data); err != nil {
		return httpErr(err), nil
	}
	return result{"profile": profile, "locations": labels, "distances_km": data["distances"], "durations_seconds": data["durations"]}, nil
}
