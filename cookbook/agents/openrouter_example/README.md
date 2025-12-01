# OpenRouter Example

Este exemplo demonstra como usar a integração do OpenRouter com o Agno.

## O que é OpenRouter?

[OpenRouter](https://openrouter.ai/) é uma API unificada que fornece acesso a múltiplos provedores de LLM através de um único endpoint. Ele é **totalmente compatível com a API da OpenAI**, o que significa que podemos usar a implementação existente do OpenAI-like internamente.

## Configuração

### 1. Obter uma API Key

1. Acesse [openrouter.ai](https://openrouter.ai/)
2. Crie uma conta ou faça login
3. Vá para a seção de API Keys
4. Crie uma nova API Key

### 2. Configurar a variável de ambiente

```bash
export OPENROUTER_API_KEY="sua-api-key-aqui"
```

## Uso Básico

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/devalexandre/agno-golang/agno/models"
    "github.com/devalexandre/agno-golang/agno/models/openrouter"
)

func main() {
    // Criar instância do OpenRouter
    chat, err := openrouter.NewOpenRouterChat(
        models.WithID(openrouter.ModelGPT4oMini),
    )
    if err != nil {
        log.Fatal(err)
    }

    // Criar mensagens
    messages := []models.Message{
        {
            Role:    models.TypeUserRole,
            Content: "Hello, how are you?",
        },
    }

    // Invocar o modelo
    ctx := context.Background()
    response, err := chat.Invoke(ctx, messages)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(response.Content)
}
```

## Usando com Agent

```go
package main

import (
    "fmt"
    "log"

    "github.com/devalexandre/agno-golang/agno/agent"
    "github.com/devalexandre/agno-golang/agno/models"
    "github.com/devalexandre/agno-golang/agno/models/openrouter"
)

func main() {
    // Criar instância do OpenRouter
    chat, err := openrouter.NewOpenRouterChat(
        models.WithID(openrouter.ModelGPT4oMini),
    )
    if err != nil {
        log.Fatal(err)
    }

    // Criar um agent
    myAgent, err := agent.NewAgent(agent.AgentConfig{
        Model:        chat,
        Name:         "My Agent",
        Instructions: "You are a helpful assistant.",
    })
    if err != nil {
        log.Fatal(err)
    }

    // Executar o agent
    response, err := myAgent.Run("What is 2 + 2?")
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(response.TextContent)
}
```

## Modelos Disponíveis

O OpenRouter fornece acesso a diversos modelos. Aqui estão algumas constantes pré-definidas:

### OpenAI
- `openrouter.ModelGPT4Turbo` - `openai/gpt-4-turbo`
- `openrouter.ModelGPT4` - `openai/gpt-4`
- `openrouter.ModelGPT4o` - `openai/gpt-4o`
- `openrouter.ModelGPT4oMini` - `openai/gpt-4o-mini`
- `openrouter.ModelGPT35Turbo` - `openai/gpt-3.5-turbo`
- `openrouter.ModelO1Preview` - `openai/o1-preview`
- `openrouter.ModelO1Mini` - `openai/o1-mini`

### Anthropic
- `openrouter.ModelClaude3Opus` - `anthropic/claude-3-opus`
- `openrouter.ModelClaude3Sonnet` - `anthropic/claude-3-sonnet`
- `openrouter.ModelClaude3Haiku` - `anthropic/claude-3-haiku`
- `openrouter.ModelClaude35Sonnet` - `anthropic/claude-3.5-sonnet`

### Google
- `openrouter.ModelGeminiPro` - `google/gemini-pro`
- `openrouter.ModelGemini15Pro` - `google/gemini-1.5-pro`
- `openrouter.ModelGemini15Flash` - `google/gemini-1.5-flash`

### Meta (Llama)
- `openrouter.ModelLlama370B` - `meta-llama/llama-3-70b-instruct`
- `openrouter.ModelLlama38B` - `meta-llama/llama-3-8b-instruct`
- `openrouter.ModelLlama3170B` - `meta-llama/llama-3.1-70b-instruct`
- `openrouter.ModelLlama318B` - `meta-llama/llama-3.1-8b-instruct`
- `openrouter.ModelLlama31405B` - `meta-llama/llama-3.1-405b-instruct`

### Mistral
- `openrouter.ModelMistralLarge` - `mistralai/mistral-large`
- `openrouter.ModelMistralMedium` - `mistralai/mistral-medium`
- `openrouter.ModelMistral7B` - `mistralai/mistral-7b-instruct`
- `openrouter.ModelMixtral8x7B` - `mistralai/mixtral-8x7b-instruct`
- `openrouter.ModelMixtral8x22B` - `mistralai/mixtral-8x22b-instruct`

### Outros
- `openrouter.ModelCommandR` - `cohere/command-r`
- `openrouter.ModelCommandRPlus` - `cohere/command-r-plus`
- `openrouter.ModelDeepSeekChat` - `deepseek/deepseek-chat`
- `openrouter.ModelDeepSeekCoder` - `deepseek/deepseek-coder`
- `openrouter.ModelQwen72B` - `qwen/qwen-72b-chat`
- `openrouter.ModelQwen25Coder` - `qwen/qwen-2.5-coder-32b-instruct`

Para ver a lista completa de modelos disponíveis, acesse: https://openrouter.ai/models

## Executando o Exemplo

```bash
# Configurar a API key
export OPENROUTER_API_KEY="sua-api-key"

# Executar o exemplo
go run cookbook/agents/openrouter_example/main.go
```

## Streaming

O OpenRouter também suporta streaming de respostas:

```go
err = chat.InvokeStream(ctx, messages, models.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
    fmt.Print(string(chunk))
    return nil
}))
```

## Implementação

Como o OpenRouter é totalmente compatível com a API da OpenAI, a implementação usa internamente o `openai/like/openailike.go` existente, apenas configurando a URL base para `https://openrouter.ai/api/v1`.

Isso significa que todas as funcionalidades disponíveis para o OpenAI também funcionam com o OpenRouter, incluindo:
- Chat completions
- Streaming
- Tool calling
- Todas as opções de configuração (temperature, max_tokens, etc.)
