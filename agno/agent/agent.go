package agent

import (
	"context"
	"time"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/utils"
)

// AgentConfig define as opções de configuração para o Agent
type AgentConfig struct {
	Context      context.Context
	Model        models.AgnoModelInterface
	Description  string
	Instructions string
	Stream       bool
	Markdown     bool
}

// Agent estrutura principal que orquestra as chamadas de IA
type Agent struct {
	ctx          context.Context
	model        models.AgnoModelInterface
	description  string
	instructions string
	stream       bool
	markdown     bool
}

// NewAgent cria um novo agente com a configuração fornecida
func NewAgent(config AgentConfig) *Agent {
	return &Agent{
		ctx:          config.Context,
		model:        config.Model,
		description:  config.Description,
		instructions: config.Instructions,
		stream:       config.Stream,
		markdown:     config.Markdown,
	}
}

// Run executa o agente com o prompt fornecido
func (a *Agent) Run(prompt string, stream bool, markdown bool) {
	start := time.Now()

	// Prepara as mensagens iniciais: Description + Instructions + Prompt
	messages := []models.Message{}

	if a.description != "" {
		messages = append(messages, models.Message{
			Role:    models.TypeSystemRole,
			Content: a.description,
		})
	}

	if a.instructions != "" {
		messages = append(messages, models.Message{
			Role:    models.TypeSystemRole,
			Content: a.instructions,
		})
	}

	// Mensagem do usuário
	messages = append(messages, models.Message{
		Role:    models.TypeUserRole,
		Content: prompt,
	})

	// Usa stream se solicitado ou se default for stream
	useStream := stream || a.stream

	if useStream {
		utils.CreateThinkingPanel(prompt)

		respStream, err := a.model.InvokeStream(a.ctx, messages)
		if err != nil {
			utils.CreateErrorPanel(err.Error(), time.Since(start).Seconds())
			return
		}

		for msg := range respStream {
			utils.CreateResponsePanel(msg.Content, time.Since(start).Seconds())
		}

	} else {
		utils.CreateThinkingPanel(prompt)

		resp, err := a.model.Invoke(a.ctx, messages)
		if err != nil {
			utils.CreateErrorPanel(err.Error(), time.Since(start).Seconds())
			return
		}

		utils.CreateResponsePanel(resp.Content, time.Since(start).Seconds())
	}
}
