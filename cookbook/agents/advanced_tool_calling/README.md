# Advanced Tool Calling - Agno-Golang

Demonstração completa das funcionalidades avançadas de tool calling implementadas no P1.2 do roadmap.

## 📋 Funcionalidades Implementadas

### 1. **Execução Paralela de Ferramentas**
Executa múltiplas chamadas de ferramentas simultaneamente com controle de concorrência.

```go
config := agent.ToolCallConfig{
    MaxParallelCalls: 2,  // Máximo de 2 chamadas simultâneas
    RetryAttempts: 0,
    ValidateArguments: true,
}

results := ag.ExecuteToolCallsParallel(ctx, requests, config)
```

**Benefícios:**
- Melhor performance em operações independentes
- Controle de concorrência com semáforo
- Timeout por chamada configurável

### 2. **Retry Automático com Backoff Exponencial**
Implementa retry automático com backoff exponencial e jitter.

```go
config := agent.ToolCallConfig{
    RetryAttempts: 3,
    RetryDelay: 100,  // 100ms inicial
    UseExponentialBackoff: true,
}
```

**Estratégia de Backoff:**
- Tentativa 1: 100ms
- Tentativa 2: 200ms (2^1 * 100)
- Tentativa 3: 400ms (2^2 * 100)
- Jitter: ±10% para evitar thundering herd

### 3. **Validação de Argumentos**
Valida argumentos contra o schema da ferramenta antes da execução.

```go
config := agent.ToolCallConfig{
    ValidateArguments: true,
}
```

**Validações:**
- Campos obrigatórios presentes
- Tipos de dados corretos
- Conversão automática de tipos compatíveis

### 4. **Tratamento de Erros**
Handler customizável para tratamento de erros de ferramentas.

```go
handler := agent.NewDefaultToolCallErrorHandler(debug bool)
err := handler.HandleError(result)
```

### 5. **Execução em Batch**
Agrupa múltiplas chamadas em um batch com rastreamento de status.

```go
batch := &agent.ToolCallBatch{
    ID: "batch-001",
    Requests: requests,
    Config: config,
}

err := ag.ExecuteToolCallBatch(ctx, batch)
```

## 🏗️ Estruturas Principais

### ToolCallConfig
```go
type ToolCallConfig struct {
    MaxParallelCalls      int  // Máximo de chamadas paralelas
    RetryAttempts         int  // Número de tentativas
    RetryDelay            int  // Delay inicial em ms
    UseExponentialBackoff bool // Ativar backoff exponencial
    ValidateArguments     bool // Validar argumentos
    TimeoutPerCall        int  // Timeout por chamada em segundos
}
```

### ToolCallResult
```go
type ToolCallResult struct {
    ToolName   string
    MethodName string
    Arguments  map[string]interface{}
    Result     interface{}
    Error      error
    Duration   time.Duration
    Attempt    int
    Success    bool
}
```

### ToolCallBatch
```go
type ToolCallBatch struct {
    ID       string
    Requests []ToolCallRequest
    Results  []ToolCallResult
    Config   ToolCallConfig
    Status   string // "pending", "running", "completed", "failed"
    Error    error
    Duration time.Duration
}
```

## 📊 Estatísticas

Obtenha estatísticas sobre as chamadas de ferramentas:

```go
stats := agent.GetToolCallStats(results)

// Retorna:
// - total_calls: número total de chamadas
// - successful: chamadas bem-sucedidas
// - failed: chamadas falhadas
// - total_duration: duração total
// - average_duration: duração média
// - max_duration: duração máxima
// - min_duration: duração mínima
// - total_retries: total de retries
```

## 🔧 Exemplos de Uso

### Exemplo 1: Execução Paralela
```go
requests := []agent.ToolCallRequest{
    {
        ToolName:   "math",
        MethodName: "add",
        Arguments:  json.RawMessage(`{"a": 10, "b": 5}`),
    },
    {
        ToolName:   "math",
        MethodName: "multiply",
        Arguments:  json.RawMessage(`{"a": 3, "b": 4}`),
    },
}

config := agent.ToolCallConfig{
    MaxParallelCalls: 2,
    ValidateArguments: true,
}

results := ag.ExecuteToolCallsParallel(ctx, requests, config)
```

### Exemplo 2: Retry com Backoff
```go
config := agent.ToolCallConfig{
    RetryAttempts: 3,
    RetryDelay: 100,
    UseExponentialBackoff: true,
    ValidateArguments: true,
}

results := ag.ExecuteToolCallsParallel(ctx, requests, config)
```

### Exemplo 3: Validação de Argumentos
```go
config := agent.ToolCallConfig{
    ValidateArguments: true,
}

// Argumentos inválidos serão detectados
results := ag.ExecuteToolCallsParallel(ctx, requests, config)

for _, result := range results {
    if !result.Success {
        fmt.Printf("Erro: %v\n", result.Error)
    }
}
```

### Exemplo 4: Batch de Chamadas
```go
batch := &agent.ToolCallBatch{
    ID: "batch-001",
    Requests: requests,
    Config: agent.ToolCallConfig{
        MaxParallelCalls: 2,
        RetryAttempts: 1,
        ValidateArguments: true,
    },
}

err := ag.ExecuteToolCallBatch(ctx, batch)

fmt.Printf("Status: %s\n", batch.Status)
fmt.Printf("Duração: %v\n", batch.Duration)
```

## 🚀 Executando o Exemplo

```bash
cd cookbook/agents/advanced_tool_calling
go run main.go
```

## 📈 Performance

### Execução Paralela vs Sequencial

**Paralela (2 simultâneas):**
- 4 chamadas de 100ms cada = ~200ms total

**Sequencial:**
- 4 chamadas de 100ms cada = ~400ms total

**Ganho:** 50% de redução no tempo total

## 🔍 Debug

Ative o modo debug para ver detalhes das chamadas:

```go
ag, err := agent.NewAgent(agent.AgentConfig{
    // ...
    Debug: true,
})
```

## 📝 Notas Importantes

1. **Timeout**: Configure `TimeoutPerCall` para evitar travamentos
2. **Validação**: Sempre ative `ValidateArguments` em produção
3. **Retry**: Use backoff exponencial para APIs externas
4. **Concorrência**: Ajuste `MaxParallelCalls` conforme recursos disponíveis
5. **Estatísticas**: Use `GetToolCallStats` para monitoramento

## 🔗 Referências

- [IMPLEMENTATION_ROADMAP.md](../../../docs/IMPLEMENTATION_ROADMAP.md) - P1.2 Tool Calling Avançado
- [agent.go](../../../agno/agent/agent.go) - Implementação do Agent
- [tool_calling.go](../../../agno/agent/tool_calling.go) - Implementação de Tool Calling Avançado
