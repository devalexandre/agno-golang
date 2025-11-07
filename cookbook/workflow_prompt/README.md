# Workflow Prompt Example

Este exemplo demonstra como usar o workflow v2 do Agno para processar prompts do usuário através de uma pipeline de agentes.

## Funcionalidade

O workflow consiste em três etapas sequenciais:

1. **Análise** - Analisa a pergunta do usuário para entender o contexto e tipo de questão
2. **Processamento** - Gera uma resposta abrangente baseada na análise
3. **Revisão** - Revisa e refina a resposta final para garantir qualidade

## Pré-requisitos

- Go 1.25+ instalado
- Ollama rodando em `http://localhost:11434`
- Modelo `llama3.2:latest` baixado (ou outro modelo especificado)

## Configuração do Ollama

```bash
# Instalar e iniciar o Ollama
ollama serve

# Baixar o modelo (em outro terminal)
ollama pull llama3.2:latest
```

## Como usar

### Uso básico

```bash
go run main.go "Explique o que é inteligência artificial"
```

### Mais exemplos

```bash
go run main.go "Como funciona o machine learning?"
go run main.go "Qual a diferença entre Python e Go?"
go run main.go "O que são microserviços?"
go run main.go "Explique blockchain de forma simples"
go run main.go "Explique como o agno-go funciona"
```

## Configuração

O exemplo usa configurações fixas para simplicidade:

| Configuração | Valor |
|--------------|-------|
| **Modelo** | `llama3.2:latest` |
| **URL do Ollama** | `http://localhost:11434` |
| **Debug** | Desabilitado |

## Exemplo de saída

```bash
$ go run main.go "Explique como o agno-go funciona"

=== Workflow Prompt Example ===
Model: llama3.2:latest
Prompt: Explique como o agno-go funciona

🚀 Starting workflow execution...
------------------------------------------------------------
🔄 Starting step: analyze
✅ Completed step: analyze
🔄 Starting step: process
✅ Completed step: process
🔄 Starting step: review
✅ Completed step: review
🎉 Workflow completed successfully!
------------------------------------------------------------
📋 WORKFLOW RESULTS:
------------------------------------------------------------
📝 FINAL RESPONSE:
O agno-go é um framework Go para criação de agentes de IA...

🏁 WORKFLOW OUTPUT:
O agno-go é um framework Go para criação de agentes de IA...
============================================================
✨ Example completed successfully!
```

## Estrutura do código

O exemplo demonstra:

- **Configuração de agentes**: Como criar agentes especializados para cada etapa
- **Wrapper de funções**: Como adaptar agentes para funcionar com o workflow v2
- **Configuração do workflow**: Como configurar streaming, debug e manipuladores de eventos
- **Execução sequencial**: Como executar etapas em sequência passando dados entre elas
- **Tratamento de eventos**: Como capturar e exibir eventos do workflow em tempo real
- **Métricas**: Como acessar métricas de execução

## Personalização

Você pode facilmente personalizar este exemplo:

1. **Modificar as instruções dos agentes** para diferentes tipos de processamento
2. **Adicionar mais etapas** ao workflow
3. **Implementar processamento paralelo** usando `v2.Parallel`
4. **Adicionar condições** usando `v2.Condition`
5. **Integrar com diferentes modelos** (OpenAI, Google Gemini, etc.)

## Solução de problemas

### Erro de conexão com Ollama
```
Failed to create Ollama model: connection refused
```
- Verifique se o Ollama está rodando: `ollama serve`

### Modelo não encontrado
```
Failed to create Ollama model: model not found
```
- Baixe o modelo: `ollama pull llama3.2:latest`

### Prompt obrigatório
```
Usage: go run main.go "Your question here"
```
- Sempre forneça um prompt como argumento direto