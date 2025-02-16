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
		return "", fmt.Errorf("erro ao parsear URL base: %v", err)
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
		return "", fmt.Errorf("erro ao buscar dados: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return "", fmt.Errorf("status HTTP inválido: %d. Resposta: %s", resp.StatusCode, string(body))
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("erro ao ler resposta: %v", err)
	}

	var forecast ForecastResponse
	err = json.Unmarshal(body, &forecast)
	if err != nil {
		// Se houver erro na conversão JSON, retorna a resposta bruta.
		return string(body), nil
	}

	output, err := json.MarshalIndent(forecast, "", "  ")
	if err != nil {
		return "", fmt.Errorf("erro ao formatar saída JSON: %v", err)
	}

	return string(output), nil
}

// WeatherTool implements the Tool interface for fetching weather data from the Open-Meteo API.
type WeatherTool struct{}

// Description returns a short description of the tool.
func (wt WeatherTool) Description() string {
	return "Fetches current weather data from the Open-Meteo API using query parameters."
}

func (wt WeatherTool) Execute(input json.RawMessage) (interface{}, error) {
	var params map[string]interface{}
	err := json.Unmarshal(input, &params)
	if err != nil {
		return nil, err
	}
	result, err := GetCurrentWeatherHandler(params)
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
	}
}

// Name returns the name of the tool.
func (wt WeatherTool) Name() string {
	return "WeatherTool"
}
