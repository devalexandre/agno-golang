// ✅ Fixed file: Pure Golang, no mixing, ready to run main.go
// agent.go updated for dynamic streaming with dual panel, compilable Go code

package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/devalexandre/agno-golang/agno/utils"
)

type AgentConfig struct {
	Context        context.Context
	Model          models.AgnoModelInterface
	Description    string
	Goal           string
	Instructions   string
	ContextData    map[string]interface{}
	ExpectedOutput string
	Tools          []toolkit.Tool
	Stream         bool
	Markdown       bool
	ShowToolsCall  bool
	Debug          bool
}

type Agent struct {
	ctx                    context.Context
	model                  models.AgnoModelInterface
	description            string
	goal                   string
	instructions           string
	additional_information []string
	contextData            map[string]interface{}
	expected_output        string
	tools                  []toolkit.Tool
	stream                 bool
	markdown               bool
	showToolsCall          bool
	debug                  bool
}

func NewAgent(config AgentConfig) *Agent {
	config.Context = context.WithValue(config.Context, models.DebugKey, config.Debug)
	config.Context = context.WithValue(config.Context, models.ShowToolsCallKey, config.ShowToolsCall)
	return &Agent{
		ctx:             config.Context,
		model:           config.Model,
		description:     config.Description,
		goal:            config.Goal,
		instructions:    config.Instructions,
		expected_output: config.ExpectedOutput,
		contextData:     config.ContextData,
		tools:           config.Tools,
		stream:          config.Stream,
		markdown:        config.Markdown,
		showToolsCall:   config.ShowToolsCall,
		debug:           config.Debug,
	}
}

func (a *Agent) Run(prompt string) (models.RunResponse, error) {
	messages := a.prepareMessages(prompt)

	bx := utils.ThinkingPanel(prompt)

	resp, err := a.model.Invoke(a.ctx, messages, models.WithTools(a.tools))
	if err != nil {
		return models.RunResponse{}, err
	}

	utils.ResponsePanel(resp.Content, bx, time.Now(), a.markdown)

	return models.RunResponse{
		TextContent: resp.Content,
		ContentType: "text",
		Event:       "RunResponse",
		Messages: []models.Message{
			{
				Role:    models.Role(resp.Role),
				Content: resp.Content,
			},
		},
		Model:     resp.Model,
		CreatedAt: time.Now().Unix(),
	}, nil
}

func (a *Agent) RunStream(prompt string) (<-chan models.RunResponse, <-chan error) {
	start := time.Now()
	messages := a.prepareMessages(prompt)

	spinnerResponse := utils.ThinkingPanel(prompt)
	contentChan := utils.StartSimplePanel(spinnerResponse, start)
	defer close(contentChan)

	// Thinking
	contentChan <- utils.ContentUpdateMsg{
		PanelName: "Thinking",
		Content:   prompt,
	}

	respStream, errChan := a.model.AInvokeStream(a.ctx, messages, models.WithTools(a.tools))

	out := make(chan models.RunResponse)
	errOut := make(chan error, 1)

	go func() {
		defer close(out)
		defer close(errOut)
		defer close(contentChan)

		for msg := range respStream {
			contentChan <- utils.ContentUpdateMsg{
				PanelName: "Response",
				Content:   fmt.Sprintf("Response (%.1fs)\n\n%s", time.Since(start).Seconds(), msg.Content),
			}

			out <- models.RunResponse{
				TextContent: msg.Content,
				ContentType: "text",
				Event:       "RunResponse",
				Messages: []models.Message{
					{
						Role:    models.Role(msg.Role),
						Content: msg.Content,
					},
				},
				Model:     msg.Model,
				CreatedAt: time.Now().Unix(),
			}
		}

		if err, ok := <-errChan; ok && err != nil {
			errOut <- err
		}
	}()

	return out, errOut
}

func (a *Agent) PrintResponse(prompt string, stream bool, markdown bool) {
	start := time.Now()
	messages := a.prepareMessages(prompt)

	spinnerResponse := utils.ThinkingPanel(prompt)

	resp, err := a.model.Invoke(a.ctx, messages, models.WithTools(a.tools))
	if err != nil {
		fmt.Println(err)
		return
	}

	utils.ResponsePanel(resp.Content, spinnerResponse, start, markdown)

}

func (a *Agent) PrintStreamResponse(prompt string, stream bool, markdown bool) {
	start := time.Now()
	messages := a.prepareMessages(prompt)
	// Thinking
	spinnerResponse := utils.ThinkingPanel(prompt)
	contentChan := utils.StartSimplePanel(spinnerResponse, start)
	defer close(contentChan)

	// Response
	responseTile := fmt.Sprintf("Response (%.1fs)\n\n", time.Since(start).Seconds())
	showResponse := false
	respStream, err := a.model.InvokeStream(a.ctx, messages, models.WithTools(a.tools))
	if err != nil {

		contentChan <- utils.ContentUpdateMsg{
			PanelName: "Error",
			Content:   err.Error(),
		}
		return
	}

	var fullResponse string
	for msg := range respStream {
		if !showResponse {
			contentChan <- utils.ContentUpdateMsg{
				PanelName: "Response",
				Content:   responseTile,
			}
			showResponse = true
		}

		fullResponse += msg.Content

		// Final response
		contentChan <- utils.ContentUpdateMsg{
			PanelName: "Response",
			Content:   msg.Content,
		}
	}

}

func (a *Agent) prepareMessages(prompt string) []models.Message {
	systemMessage := ""

	if a.goal != "" {
		systemMessage += fmt.Sprintf("<goal>\n%s\n</goal>\n", a.goal)
	}

	if a.description != "" {
		systemMessage += fmt.Sprintf("<description>\n%s\n</description>\n", a.description)
	}

	if a.instructions != "" {
		systemMessage += fmt.Sprintf("<instructions>\n%s\n</instructions>\n", a.instructions)
	}

	if a.expected_output != "" {
		systemMessage += fmt.Sprintf("<expected_output>\n%s\n</expected_output>\n", a.expected_output)
	}
	if a.markdown {
		a.additional_information = append(a.additional_information, "Use markdown to format your answers.")
	}

	if len(a.additional_information) > 0 {
		systemMessage += fmt.Sprintf("<additional_information>\n%s\n</additional_information>\n", strings.Join(a.additional_information, "\n"))
	}

	if len(a.contextData) > 0 {
		contextStr := utils.PrettyPrintMap(a.contextData)
		systemMessage += fmt.Sprintf("<context>\n%s\n</context>\n", contextStr)
	}

	if a.debug {
		utils.DebugPanel(systemMessage)
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
