// ✅ Fixed file: Pure Golang, no mixing, ready to run main.go
// agent.go updated for dynamic streaming with dual panel, compilable Go code

package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/tools"
	"github.com/devalexandre/agno-golang/agno/utils"
)

type AgentConfig struct {
	Context      context.Context
	Model        models.AgnoModelInterface
	Description  string
	Goal         string
	Instructions string
	ContextData  map[string]interface{}
	Tools        []tools.Tool
	Stream       bool
	Markdown     bool
	Debug        bool
}

type Agent struct {
	ctx          context.Context
	model        models.AgnoModelInterface
	description  string
	goal         string
	instructions string
	contextData  map[string]interface{}
	tools        []tools.Tool
	stream       bool
	markdown     bool
	debug        bool
}

func NewAgent(config AgentConfig) *Agent {
	config.Context = context.WithValue(config.Context, "debug", config.Debug)
	return &Agent{
		ctx:          config.Context,
		model:        config.Model,
		description:  config.Description,
		goal:         config.Goal,
		instructions: config.Instructions,
		contextData:  config.ContextData,
		tools:        config.Tools,
		stream:       config.Stream,
		markdown:     config.Markdown,
		debug:        config.Debug,
	}
}

func (a *Agent) PrintStreamResponse(prompt string, stream bool, markdown bool) {
	start := time.Now()
	messages := a.prepareMessages(prompt)

	// Fixed initial panel
	initialTop := fmt.Sprintf("Thinking...\n\n%s", prompt)

	contentChan, _ := utils.StartDynamicDualPanel(initialTop, "", utils.ColorGreen, utils.ColorCyan)
	defer close(contentChan)

	respStream, err := a.model.InvokeStream(a.ctx, messages, models.WithTools(a.tools))
	if err != nil {
		utils.CreateErrorPanel(err.Error(), time.Since(start).Seconds())
		return
	}

	var fullResponse string
	for msg := range respStream {
		fullResponse += msg.Content
		contentChan <- utils.ContentUpdateMsg{
			BottomPanel: fmt.Sprintf("Response (%.1fs)\n\n%s", time.Since(start).Seconds(), fullResponse),
		}
	}

}

func (a *Agent) prepareMessages(prompt string) []models.Message {
	var systemMessage string

	if a.description != "" {
		systemMessage += fmt.Sprintf("<goal>\n%s\n</goal>\n", a.description)
	}

	if a.goal != "" {
		systemMessage += fmt.Sprintf("<goal>\n%s\n</goal>\n", a.goal)
	}

	if a.instructions != "" {
		systemMessage += fmt.Sprintf("<instructions>\n%s\n</instructions>\n", a.instructions)
	}

	if len(a.contextData) > 0 {
		contextStr := utils.PrettyPrintMap(a.contextData)
		systemMessage += fmt.Sprintf("<context>\n%s\n</context>\n", contextStr)
	}

	messages := []models.Message{}

	if systemMessage != "" {
		messages = append(messages, models.Message{
			Role:    models.TypeSystemRole,
			Content: systemMessage,
		})
	}

	messages = append(messages, models.Message{
		Role:    models.TypeUserRole,
		Content: prompt,
	})

	return messages
}
