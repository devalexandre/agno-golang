package tools

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

// ForecastResponse represents the expected response from the Open-Meteo API.
type ForecastResponse struct {
	Current struct {
		Time          string  `json:"time"`
		Temperature2m float64 `json:"temperature_2m"`
		WindSpeed10m  float64 `json:"wind_speed_10m"`
	} `json:"current"`
	Hourly struct {
		Time               []string  `json:"time"`
		Temperature2m      []float64 `json:"temperature_2m"`
		RelativeHumidity2m []float64 `json:"relative_humidity_2m"`
		WindSpeed10m       []float64 `json:"wind_speed_10m"`
	} `json:"hourly"`
}

// GetCurrentWeatherHandler fetches weather forecast data from the Open-Meteo API
// using the provided query parameters and returns the formatted JSON output as a string.
func GetCurrentWeatherHandler(queryParams map[string]interface{}) (string, error) {
	baseURL := "https://api.open-meteo.com/v1/forecast"
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("error parsing base URL: %v", err)
	}
	q := u.Query()
	for key, value := range queryParams {
		q.Set(key, fmt.Sprintf("%v", value))
	}

	q.Set("current", "temperature_2m,wind_speed_10m")
	q.Set("hourly", "temperature_2m,relative_humidity_2m,wind_speed_10m")
	u.RawQuery = q.Encode()

	resp, err := http.Get(u.String())
	if err != nil {
		return "", fmt.Errorf("error fetching data: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return "", fmt.Errorf("invalid HTTP status: %d. Response: %s", resp.StatusCode, string(body))
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %v", err)
	}

	var forecast ForecastResponse
	err = json.Unmarshal(body, &forecast)
	if err != nil {
		// If there is an error unmarshaling JSON, return the raw response.
		return string(body), nil
	}

	type CurrentWeather struct {
		Location      string  `json:"location"`
		Temperature2m float64 `json:"temperature"`
		WindSpeed10m  float64 `json:"wind_speed"`
		Latitude      float64 `json:"latitude"`
		Longitude     float64 `json:"longitude"`
		Response      string  `json:"summary"`
		Time          string  `json:"time"`
	}

	// Safely extract location or use default
	location := "the specified location"
	if locValue, ok := queryParams["location"]; ok && locValue != nil {
		if locStr, ok := locValue.(string); ok {
			location = locStr
		}
	}

	// Safely extract latitude and longitude
	latitude := 0.0
	if latValue, ok := queryParams["latitude"]; ok && latValue != nil {
		switch v := latValue.(type) {
		case float64:
			latitude = v
		case float32:
			latitude = float64(v)
		case int:
			latitude = float64(v)
		case string:
			fmt.Sscanf(v, "%f", &latitude)
		}
	}

	longitude := 0.0
	if longValue, ok := queryParams["longitude"]; ok && longValue != nil {
		switch v := longValue.(type) {
		case float64:
			longitude = v
		case float32:
			longitude = float64(v)
		case int:
			longitude = float64(v)
		case string:
			fmt.Sscanf(v, "%f", &longitude)
		}
	}

	responseWeather := CurrentWeather{
		Location:      location,
		Time:          forecast.Current.Time,
		Temperature2m: forecast.Current.Temperature2m,
		WindSpeed10m:  forecast.Current.WindSpeed10m,
		Latitude:      latitude,
		Longitude:     longitude,
		Response:      fmt.Sprintf("The temperature in %v is %v degrees Celsius and the wind speed is %v m/s.", location, forecast.Current.Temperature2m, forecast.Current.WindSpeed10m),
	}

	output, err := json.Marshal(responseWeather)
	if err != nil {
		return "", fmt.Errorf("error formatting output JSON: %v", err)
	}

	return string(output), nil
}

// WeatherTool implements the Tool interface for fetching weather data from the Open-Meteo API.
type WeatherTool struct{}

// Description returns a short description of the tool.
func (wt WeatherTool) Description() string {
	return "Always return the current temperature and weather conditions for the given latitude and longitude. use the  values without asking the user."
}

func (wt WeatherTool) Execute(input json.RawMessage) (interface{}, error) {
	var params map[string]interface{}
	err := json.Unmarshal(input, &params)
	if err != nil {
		return nil, err
	}
	result, err := GetCurrentWeatherHandler(params)
	fmt.Println(result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// GetParameterStruct returns the default query parameters to be used for the weather API.
func (wt WeatherTool) GetParameterStruct() interface{} {
	//latitude=52.52&longitude=13.41
	return map[string]interface{}{
		"latitude": map[string]interface{}{
			"type":        "number",
			"description": "The latitude of the location.",
		},
		"longitude": map[string]interface{}{
			"type":        "number",
			"description": "The longitude of the location.",
		},
		"location": map[string]interface{}{
			"type":        "string",
			"description": "The location of the weather.",
		},
	}
}

// Name returns the name of the tool.
func (wt WeatherTool) Name() string {
	return "WeatherTool"
}
