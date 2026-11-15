// Package openweather ports Agno's OpenWeatherTools (OpenWeatherMap API).
package openweather

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restapi"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// Default endpoints.
const (
	DefaultBaseURL = "https://api.openweathermap.org/data/2.5"
	DefaultGeoURL  = "https://api.openweathermap.org/geo/1.0"
)

// OpenWeatherTools accesses current weather, forecasts, air pollution and geocoding.
type OpenWeatherTools struct {
	toolkit.Toolkit
	apiKey     string
	units      string
	baseURL    string
	geoURL     string
	httpClient *http.Client

	enableCurrent, enableForecast, enableAir, enableGeocode bool
}

// Option configures OpenWeatherTools.
type Option func(*OpenWeatherTools)

// WithAPIKey sets the API key (default OPENWEATHER_API_KEY).
func WithAPIKey(k string) Option { return func(o *OpenWeatherTools) { o.apiKey = k } }

// WithUnits sets units: standard, metric (default) or imperial.
func WithUnits(u string) Option { return func(o *OpenWeatherTools) { o.units = u } }

// WithBaseURLs overrides the data and geocoding base URLs.
func WithBaseURLs(dataURL, geoURL string) Option {
	return func(o *OpenWeatherTools) { o.baseURL, o.geoURL = dataURL, geoURL }
}

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(o *OpenWeatherTools) { o.httpClient = c } }

// WithTimeout sets the per-request timeout.
func WithTimeout(d time.Duration) Option { return func(o *OpenWeatherTools) { o.Toolkit.Timeout = d } }

// WithCurrentWeather toggles GetCurrentWeather (default true).
func WithCurrentWeather(b bool) Option { return func(o *OpenWeatherTools) { o.enableCurrent = b } }

// WithForecast toggles GetForecast (default true).
func WithForecast(b bool) Option { return func(o *OpenWeatherTools) { o.enableForecast = b } }

// WithAirPollution toggles GetAirPollution (default true).
func WithAirPollution(b bool) Option { return func(o *OpenWeatherTools) { o.enableAir = b } }

// WithGeocoding toggles GeocodeLocation (default true).
func WithGeocoding(b bool) Option { return func(o *OpenWeatherTools) { o.enableGeocode = b } }

// WithAll enables every method.
func WithAll() Option {
	return func(o *OpenWeatherTools) {
		o.enableCurrent, o.enableForecast, o.enableAir, o.enableGeocode = true, true, true, true
	}
}

// GeocodeParams for GeocodeLocation.
type GeocodeParams struct {
	Location string `json:"location" description:"The name of the city, e.g., \"London\", \"Paris\", \"New York\"." required:"true"`
	Limit    int    `json:"limit,omitempty" description:"Maximum number of location results. Default is 1."`
}

// LocationParams has a location.
type LocationParams struct {
	Location string `json:"location" description:"The name of the city, e.g., \"London\", \"Paris\", \"New York\"." required:"true"`
}

// ForecastParams for GetForecast.
type ForecastParams struct {
	Location string `json:"location" description:"The name of the city, e.g., \"London\", \"Paris\", \"New York\"." required:"true"`
	Days     int    `json:"days,omitempty" description:"Number of days for forecast (max 5). Default is 5."`
}

// NewOpenWeatherTools creates the toolkit. Unlike Python it does not panic
// without an API key; methods report the missing key instead.
func NewOpenWeatherTools(opts ...Option) *OpenWeatherTools {
	o := &OpenWeatherTools{
		apiKey: os.Getenv("OPENWEATHER_API_KEY"), units: "metric",
		baseURL: DefaultBaseURL, geoURL: DefaultGeoURL,
		enableCurrent: true, enableForecast: true, enableAir: true, enableGeocode: true,
	}
	o.Toolkit = toolkit.NewToolkit()
	o.Toolkit.Name = "openweather_tools"
	o.Toolkit.Description = "Weather data from the OpenWeatherMap API."
	for _, opt := range opts {
		opt(o)
	}
	if o.httpClient == nil {
		o.httpClient = o.Toolkit.HTTPClient()
	}
	if o.enableCurrent {
		o.Toolkit.Register("GetCurrentWeather", "Get current weather data for a location.", o, o.GetCurrentWeather, LocationParams{})
	}
	if o.enableForecast {
		o.Toolkit.Register("GetForecast", "Get weather forecast for a location.", o, o.GetForecast, ForecastParams{})
	}
	if o.enableAir {
		o.Toolkit.Register("GetAirPollution", "Get current air pollution data for a location.", o, o.GetAirPollution, LocationParams{})
	}
	if o.enableGeocode {
		o.Toolkit.Register("GeocodeLocation", "Convert a location name to geographic coordinates.", o, o.GeocodeLocation, GeocodeParams{})
	}
	return o
}

func errJSON(msg string) string { return restapi.ToJSON(map[string]string{"error": msg}) }

func (o *OpenWeatherTools) get(u string, q url.Values, out interface{}) error {
	if o.apiKey == "" {
		return fmt.Errorf("OpenWeather API key is required. Set the OPENWEATHER_API_KEY environment variable")
	}
	q.Set("appid", o.apiKey)
	return restapi.New("", o.httpClient, nil).JSON(context.Background(), http.MethodGet, u, q, nil, out)
}

func (o *OpenWeatherTools) geocode(location string, limit int) ([]map[string]interface{}, error) {
	if limit <= 0 {
		limit = 1
	}
	var res []map[string]interface{}
	err := o.get(o.geoURL+"/direct", url.Values{"q": {location}, "limit": {strconv.Itoa(limit)}}, &res)
	return res, err
}

// GeocodeLocation converts a location name to coordinates.
func (o *OpenWeatherTools) GeocodeLocation(p GeocodeParams) (interface{}, error) {
	res, err := o.geocode(p.Location, p.Limit)
	if err != nil {
		return errJSON(err.Error()), nil
	}
	if len(res) == 0 {
		return errJSON(fmt.Sprintf("No location found for '%s'", p.Location)), nil
	}
	return restapi.ToJSON(res), nil
}

func (o *OpenWeatherTools) atLocation(location, path string, extra url.Values) (interface{}, error) {
	locs, err := o.geocode(location, 1)
	if err != nil {
		return errJSON(err.Error()), nil
	}
	if len(locs) == 0 {
		return errJSON(fmt.Sprintf("No location found for '%s'", location)), nil
	}
	loc := locs[0]
	q := url.Values{"lat": {fmt.Sprint(loc["lat"])}, "lon": {fmt.Sprint(loc["lon"])}}
	for k, v := range extra {
		q[k] = v
	}
	var result map[string]interface{}
	if err := o.get(o.baseURL+path, q, &result); err != nil {
		return errJSON(err.Error()), nil
	}
	if result == nil {
		result = map[string]interface{}{}
	}
	name, _ := loc["name"].(string)
	if name == "" {
		name = location
	}
	country, _ := loc["country"].(string)
	result["location_name"] = name
	result["country"] = country
	out, _ := json.MarshalIndent(result, "", "  ")
	return string(out), nil
}

// GetCurrentWeather returns current weather for a location.
func (o *OpenWeatherTools) GetCurrentWeather(p LocationParams) (interface{}, error) {
	return o.atLocation(p.Location, "/weather", url.Values{"units": {o.units}})
}

// GetForecast returns a forecast (3-hour steps, up to 5 days).
func (o *OpenWeatherTools) GetForecast(p ForecastParams) (interface{}, error) {
	days := p.Days
	if days <= 0 {
		days = 5
	}
	cnt := days * 8
	if cnt > 40 {
		cnt = 40
	}
	return o.atLocation(p.Location, "/forecast", url.Values{"units": {o.units}, "cnt": {strconv.Itoa(cnt)}})
}

// GetAirPollution returns current air pollution data.
func (o *OpenWeatherTools) GetAirPollution(p LocationParams) (interface{}, error) {
	return o.atLocation(p.Location, "/air_pollution", nil)
}
