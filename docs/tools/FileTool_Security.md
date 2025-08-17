# FileTool - Sistema de Segurança

## 🛡️ Visão Geral

O **FileTool** implementa um sistema de segurança que **desabilita operações de escrita por padrão**. Esta é uma medida de proteção para prevenir modificações acidentais no sistema de arquivos.

## 🔒 Comportamento Padrão

### Operações Permitidas (Sempre)
- ✅ **ReadFile**: Leitura de arquivos
- ✅ **GetFileInfo**: Informações sobre arquivos/diretórios  
- ✅ **ListDirectory**: Listagem de diretórios
- ✅ **SearchFiles**: Busca de arquivos

### Operações Restritas (Desabilitadas por padrão)
- ❌ **WriteFile**: Escrita/criação de arquivos
- ❌ **CreateDirectory**: Criação de diretórios
- ❌ **DeleteFile**: Exclusão de arquivos/diretórios

## 🔧 Como Usar

### 1. FileTool Padrão (Somente Leitura)
```go
import "github.com/devalexandre/agno-golang/agno/tools"

// Criar FileTool com escrita desabilitada
fileTool := tools.NewFileTool()
fmt.Println(fileTool.IsWriteEnabled()) // false

// Operações de leitura funcionam normalmente
content, err := fileTool.ReadFile(ReadFileParams{Path: "/etc/hostname"})

// Operações de escrita falham com mensagem de segurança
result, err := fileTool.WriteFile(WriteFileParams{
    Path: "/tmp/test.txt", 
    Content: "test"
})
// Retorna: "write operations are disabled for security"
```

### 2. Habilitando Escrita Manualmente
```go
// Criar FileTool padrão
fileTool := tools.NewFileTool()

// Habilitar escrita quando necessário
fileTool.EnableWrite()
fmt.Println(fileTool.IsWriteEnabled()) // true

// Agora operações de escrita funcionam
result, err := fileTool.WriteFile(WriteFileParams{
    Path: "/tmp/test.txt", 
    Content: "test"
})

// Desabilitar novamente se necessário
fileTool.DisableWrite()
```

### 3. FileTool com Escrita Pré-habilitada
```go
// Criar FileTool já com escrita habilitada
fileTool := tools.NewFileToolWithWrite()
fmt.Println(fileTool.IsWriteEnabled()) // true

// Todas as operações funcionam imediatamente
result, err := fileTool.WriteFile(WriteFileParams{
    Path: "/tmp/test.txt", 
    Content: "test"
})
```

## 📊 Métodos de Controle

### Verificação de Status
```go
enabled := fileTool.IsWriteEnabled() // bool
```

### Habilitação/Desabilitação
```go
fileTool.EnableWrite()   // Habilita escrita
fileTool.DisableWrite()  // Desabilita escrita
```

### Construtores
```go
// Escrita desabilitada (padrão)
fileTool := tools.NewFileTool()

// Escrita habilitada
fileTool := tools.NewFileToolWithWrite()
```

## 🛠️ Uso com Agentes

### Agente Somente Leitura (Seguro)
```go
agent := agent.NewAgent(model)
agent.AddTool(tools.NewFileTool()) // Apenas leitura

// O agente pode ler arquivos mas não modificar
agent.PrintResponse("Read the contents of /etc/hostname", false, true)
```

### Agente com Escrita (Cuidado)
```go
agent := agent.NewAgent(model)
agent.AddTool(tools.NewFileToolWithWrite()) // Escrita habilitada

// O agente pode modificar arquivos
agent.PrintResponse("Create a file called test.txt with 'Hello World'", false, true)
```

### Controle Dinâmico
```go
fileTool := tools.NewFileTool()
agent.AddTool(fileTool)

// Habilitar escrita apenas quando necessário
fileTool.EnableWrite()
agent.PrintResponse("Create a backup file", false, true)

// Desabilitar novamente
fileTool.DisableWrite()
```

## ⚠️ Mensagens de Erro

Quando operações de escrita são tentadas com escrita desabilitada:

```json
{
  "path": "/tmp/test.txt",
  "success": false,
  "error": "write operations are disabled for security. Use EnableWrite() to enable them",
  "operation": "WriteFile"
}
```

## 🎯 Casos de Uso

### Desenvolvimento/Teste (Seguro)
```go
// Para desenvolvimento, use FileTool padrão
fileTool := tools.NewFileTool()
// Agente pode analisar arquivos mas não modificar nada
```

### Produção com Controle
```go
// Em produção, habilite escrita apenas quando necessário
fileTool := tools.NewFileTool()

if allowFileWrites {
    fileTool.EnableWrite()
}
```

### Automação/Scripts
```go
// Para scripts de automação que precisam modificar arquivos
fileTool := tools.NewFileToolWithWrite()
// Todas as operações habilitadas desde o início
```

## 🏆 Benefícios

1. **Segurança por Padrão**: Previne modificações acidentais
2. **Controle Granular**: Habilite escrita apenas quando necessário
3. **Auditoria**: Claro quando escrita está habilitada ou não
4. **Flexibilidade**: Múltiplas formas de controlar o comportamento
5. **Transparência**: Mensagens claras sobre restrições

---

**💡 Dica**: Para máxima segurança em produção, sempre use `NewFileTool()` e habilite escrita apenas temporariamente quando necessário.
