# Memory-enabled Agent Example

Este exemplo demonstra como usar o sistema de memória persistente do Agno para criar um agente que lembra de conversas anteriores.

## Features

- ✅ **Memória Persistente**: Armazena conversas em SQLite
- ✅ **Contexto Conversacional**: Lembra informações pessoais do usuário
- ✅ **Recall Automático**: Recupera memórias relevantes em novas conversas
- ✅ **Classificação de Memória**: Identifica quais mensagens são importantes
- ✅ **Cloud LLM**: Usa modelo cloud para processamento

## Componentes

### 1. SQLite Memory Database
```go
memoryDB, err := memorysqlite.NewSqliteMemoryDb("user_memories", dbFile)
```
- Armazena memórias de forma persistente
- Organiza por user_id
- Suporta queries e filtros

### 2. Memory Manager
```go
memoryManager := memory.NewMemory(cloudModel, memoryDB)
```
- Extrai informações importantes das conversas
- Cria memórias estruturadas
- Gerencia ciclo de vida das memórias

### 3. Agent com Memory
```go
agt, err := agent.NewAgent(agent.AgentConfig{
    Memory: memoryManager,
    // ...
})
```
- Integra memória ao fluxo conversacional
- Usa memórias para personalizar respostas

## Como Executar

```bash
# 1. Certifique-se de ter o Ollama cloud configurado
# 2. Execute o exemplo
go run cookbook/agents/memory_example/main.go
```

## Fluxo do Exemplo

1. **Conversa 1**: Compartilha informações pessoais
   - Nome: Alexandre
   - Profissão: Desenvolvedor de software
   - País: Brasil
   - Linguagem favorita: Go
   - Paradigma favorito: Programação funcional

2. **Armazenamento**: Memórias são extraídas e salvas
   - Sistema identifica informações importantes
   - Cria registros estruturados no banco
   - Associa ao user_id

3. **Conversa 2**: Testa recall de memória
   - "What's my name?" → Alexandre
   - "What do you know about my programming interests?" → Go, AI, functional programming
   - "What country am I from?" → Brazil

4. **Visualização**: Mostra memórias armazenadas
   - Lista todas as memórias do usuário
   - Exibe timestamps e contexto
   - Permite busca semântica

## Estrutura da Memória

```go
type UserMemory struct {
    ID        string
    UserID    string
    Memory    string    // Informação extraída
    Input     string    // Mensagem original
    Summary   string    // Resumo (opcional)
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

## Casos de Uso

### 1. Assistentes Personalizados
- Lembrar preferências do usuário
- Manter contexto entre sessões
- Adaptar respostas ao histórico

### 2. Suporte ao Cliente
- Histórico de interações
- Problemas recorrentes
- Soluções anteriores

### 3. Agentes Educacionais
- Progresso do aluno
- Dificuldades identificadas
- Tópicos já estudados

### 4. Chatbots Empresariais
- Informações do funcionário
- Projetos em andamento
- Decisões tomadas

## APIs Principais

### CreateMemory
```go
memory, err := memoryManager.CreateMemory(ctx, userID, input, response)
```
Extrai e armazena uma memória da conversa.

### GetUserMemories
```go
memories, err := memoryManager.GetUserMemories(ctx, userID)
```
Recupera todas as memórias de um usuário.

### UpdateMemory
```go
memory, err := memoryManager.UpdateMemory(ctx, memoryID, newContent)
```
Atualiza uma memória existente.

### DeleteMemory
```go
err := memoryManager.DeleteMemory(ctx, memoryID)
```
Remove uma memória específica.

## Configuração

### Database
```go
dbFile := "agent_memory.db"  // Arquivo SQLite
tableName := "user_memories"  // Nome da tabela
```

### Model
```go
cloudModel, err := ollama.NewOllamaChat(
    models.WithID("kimi-k2:1t-cloud"),
    models.WithBaseURL("https://ollama.cloud.devalexandre.com.br"),
)
```

## Persistência

As memórias são salvas em `agent_memory.db` no diretório local. Execute o exemplo múltiplas vezes para ver o agente lembrar de conversas anteriores!

## Próximos Passos

- Adicionar busca semântica com embeddings
- Implementar sistema de importância/relevância
- Criar agregação temporal de memórias
- Adicionar suporte a múltiplos usuários com isolamento
