package main

import (
	"fmt"
	"os"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/dashscope"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

type ParallelDemoTool struct {
	toolkit.Toolkit
}

type WeatherParams struct {
	City string `json:"city" description:"Nome da cidade" required:"true"`
}

type FxRateParams struct {
	From string `json:"from" description:"Moeda base (ex.: USD)" required:"true"`
	To   string `json:"to" description:"Moeda destino (ex.: BRL)" required:"true"`
}

func NewParallelDemoTool() *ParallelDemoTool {
	t := &ParallelDemoTool{}
	t.Toolkit = toolkit.NewToolkit()
	t.Toolkit.Name = "ParallelDemo"
	t.Toolkit.Description = "Tools demo para testar parallel tool calls"

	t.Toolkit.Register("GetWeather", "Retorna um clima fictício para uma cidade", t, t.GetWeather, WeatherParams{})
	t.Toolkit.Register("GetFxRate", "Retorna um câmbio fictício entre duas moedas", t, t.GetFxRate, FxRateParams{})

	return t
}

func (t *ParallelDemoTool) GetWeather(params WeatherParams) (interface{}, error) {
	// Sem rede: resultado determinístico
	return map[string]any{
		"city":        params.City,
		"temperature": 27,
		"condition":   "sunny",
	}, nil
}

func (t *ParallelDemoTool) GetFxRate(params FxRateParams) (interface{}, error) {
	// Sem rede: resultado determinístico
	rate := 5.10
	if params.From == "EUR" && params.To == "BRL" {
		rate = 5.55
	}
	return map[string]any{
		"from": params.From,
		"to":   params.To,
		"rate": rate,
	}, nil
}

func main() {
	baseURL := os.Getenv("LLM_STUDIO_BASE_URL")
	if baseURL == "" {
		baseURL = os.Getenv("DASHSCOPE_BASE_URL")
	}
	if baseURL == "" {
		baseURL = "http://localhost:1234/v1"
	}

	modelID := os.Getenv("LLM_STUDIO_MODEL")
	if modelID == "" {
		modelID = os.Getenv("DASHSCOPE_MODEL")
	}
	if modelID == "" {
		modelID = "qwen2.5-3b-instruct"
	}

	options := []models.OptionClient{
		models.WithID(modelID),
		models.WithBaseURL(baseURL),
	}

	model, err := dashscope.NewDashScopeChat(options...)
	if err != nil {
		panic(err)
	}

	tool := NewParallelDemoTool()

	agt, err := agent.NewAgent(agent.AgentConfig{
		Name:          "Qwen Parallel Tools",
		Model:         model,
		Tools:         []toolkit.Tool{tool},
		Markdown:      true,
		ShowToolsCall: true,
		ModelOptions: []models.Option{
			models.WithRequestParams(map[string]any{
				"parallel_tool_calls": true,
			}),
		},
	})
	if err != nil {
		panic(err)
	}

	resp, err := agt.Run(`Use as tools para responder:
1) Pegue o clima em "São Paulo"
2) Pegue o câmbio "USD" -> "BRL"
Chame as DUAS tools no mesmo turno e depois responda em 2 linhas com os resultados.`)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Content:\n%s\n", resp.TextContent)
	if len(resp.Messages) > 0 && len(resp.Messages[0].ToolCalls) > 0 {
		fmt.Printf("\nToolCalls: %d\n", len(resp.Messages[0].ToolCalls))
	}
}
