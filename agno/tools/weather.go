package tools

import (
	"encoding/json"
	"fmt"
)

// GetCurrentWeatherHandler é um manipulador de ferramentas para obter o clima atual.
type GetCurrentWeatherHandler struct{}

// Estrutura que define os parâmetros esperados.
type WeatherParams struct {
	Location string `json:"location"`
	Unit     string `json:"unit,omitempty"`
}

// Name implementa a interface Tool.
func (h GetCurrentWeatherHandler) Name() string {
	return "get_weather"
}

// Description implementa a interface Tool.
func (h GetCurrentWeatherHandler) Description() string {
	return "Fetches the current weather for a given location."
}

// Execute implementa a interface Tool.
func (h GetCurrentWeatherHandler) Execute(params json.RawMessage) (interface{}, error) {
	var wp WeatherParams
	if err := json.Unmarshal(params, &wp); err != nil {
		return nil, fmt.Errorf("failed to parse parameters: %w", err)
	}

	response := map[string]interface{}{
		"location": wp.Location,
		"unit":     wp.Unit,
		"weather":  "Sunny",
		"temp":     25,
	}
	return response, nil
}

// GetParameterStruct implementa a interface Tool.
func (h GetCurrentWeatherHandler) GetParameterStruct() interface{} {
	return WeatherParams{}
}
