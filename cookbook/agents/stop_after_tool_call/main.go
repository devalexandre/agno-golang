// Tool flags: stop_after_tool_call and show_result.
//
//   - toolkit.WithStopAfterCall(): the run ends right after the method runs
//     and its result becomes the run content (implies show_result).
//   - toolkit.WithShowResult(): the method result is prepended to the content
//     while the model still produces its answer.
//
// Runs offline with a scripted stub model; no env vars or services needed.
package main

import (
	"fmt"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/devalexandre/agno-golang/cookbook/internal/toolcallstub"
)

type QueryParams struct {
	Symbol string `json:"symbol" description:"Ticker" required:"true"`
}

type QuotesTool struct{ toolkit.Toolkit }

func (q *QuotesTool) Price(p QueryParams) (interface{}, error) { return p.Symbol + ": 101.25", nil }

func (q *QuotesTool) News(p QueryParams) (interface{}, error) {
	return p.Symbol + " beats estimates", nil
}

func main() {
	tool := &QuotesTool{Toolkit: toolkit.NewToolkit()}
	tool.Name, tool.Description = "Quotes", "Market data"
	tool.RegisterWithOptions("Price", "Latest price", tool, tool.Price, QueryParams{}, toolkit.WithStopAfterCall())
	tool.RegisterWithOptions("News", "Latest news", tool, tool.News, QueryParams{}, toolkit.WithShowResult())

	stop, err := agent.NewAgent(agent.AgentConfig{
		Model: toolcallstub.New("(never reached)", "Quotes_Price", `{"symbol":"ACME"}`),
		Tools: []toolkit.Tool{tool},
	})
	if err != nil {
		log.Fatal(err)
	}
	out, err := stop.Run("Price of ACME?")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("stop_after_tool_call -> %q\n", out.TextContent)

	show, err := agent.NewAgent(agent.AgentConfig{
		Model: toolcallstub.New("Summary written.", "Quotes_News", `{"symbol":"ACME"}`),
		Tools: []toolkit.Tool{tool},
	})
	if err != nil {
		log.Fatal(err)
	}
	out, err = show.Run("News about ACME?")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("show_result -> %q\n", out.TextContent)
}
