# Blog Post Generator Workflow# Workflow Prompt Example



This example demonstrates a **multi-agent workflow** that generates complete blog posts and saves them as Markdown files using Ollama Cloud.Este exemplo demonstra como usar o workflow v2 do Agno para processar prompts do usuário através de uma pipeline de agentes.



## 🎯 What It Does## Funcionalidade



The workflow uses **3 specialized agents** working in sequence:O workflow consiste em três etapas sequenciais:



1. **Researcher Agent** 🔍1. **Análise** - Analisa a pergunta do usuário para entender o contexto e tipo de questão

   - Analyzes the topic2. **Processamento** - Gera uma resposta abrangente baseada na análise

   - Creates a comprehensive outline3. **Revisão** - Revisa e refina a resposta final para garantir qualidade

   - Identifies target audience

   - Plans structure and key points## Pré-requisitos



2. **Writer Agent** ✍️- Go 1.25+ instalado

   - Writes complete blog post content- Ollama rodando em `http://localhost:11434`

   - Creates engaging introduction- Modelo `llama3.2:latest` baixado (ou outro modelo especificado)

   - Develops well-structured sections

   - Includes code examples and use cases## Configuração do Ollama



3. **Editor Agent** 📝```bash

   - Reviews and polishes the content# Instalar e iniciar o Ollama

   - Fixes grammar and styleollama serve

   - Optimizes for SEO

   - Adds frontmatter metadata# Baixar o modelo (em outro terminal)

   - Produces publication-ready Markdownollama pull llama3.2:latest

```

## 🚀 Features

## Como usar

- ✅ **Ollama Cloud Integration** - Uses `kimi-k2:1t-cloud` model

- ✅ **Multi-Agent Workflow** - Researcher → Writer → Editor pipeline### Uso básico

- ✅ **Markdown Output** - Generates `.md` files with frontmatter

- ✅ **File System Safe** - Auto-generates clean filenames```bash

- ✅ **Event Streaming** - Real-time progress updatesgo run main.go "Explique o que é inteligência artificial"

- ✅ **Production Ready** - Includes error handling and retries```

- ✅ **Debug Mode** - Optional detailed logging

### Mais exemplos

## 📋 Requirements

```bash

- Go 1.23 or highergo run main.go "Como funciona o machine learning?"

- Ollama Cloud API keygo run main.go "Qual a diferença entre Python e Go?"

- Internet connectiongo run main.go "O que são microserviços?"

go run main.go "Explique blockchain de forma simples"

## 🔧 Setupgo run main.go "Explique como o agno-go funciona"

```

1. **Set Ollama Cloud API Key:**

## Configuração

```bash

export OLLAMA_API_KEY="your-api-key-here"O exemplo usa configurações fixas para simplicidade:

```

| Configuração | Valor |

2. **Install dependencies:**|--------------|-------|

| **Modelo** | `llama3.2:latest` |

```bash| **URL do Ollama** | `http://localhost:11434` |

go mod download| **Debug** | Desabilitado |

```

## Exemplo de saída

## 💻 Usage

```bash

```bash$ go run main.go "Explique como o agno-go funciona"

# Basic usage

go run main.go "Best practices for building AI agents in Go"=== Workflow Prompt Example ===

Model: llama3.2:latest

# More examplesPrompt: Explique como o agno-go funciona

go run main.go "Introduction to Retrieval Augmented Generation"

go run main.go "How to build production-ready workflows"🚀 Starting workflow execution...

go run main.go "Understanding Go concurrency patterns"------------------------------------------------------------

```🔄 Starting step: analyze

✅ Completed step: analyze

## 📂 Output🔄 Starting step: process

✅ Completed step: process

Blog posts are saved to `blog_posts/` directory with auto-generated filenames:🔄 Starting step: review

✅ Completed step: review

```🎉 Workflow completed successfully!

blog_posts/------------------------------------------------------------

├── 2025-11-08-best-practices-for-building-ai-agents-in-go.md📋 WORKFLOW RESULTS:

├── 2025-11-08-introduction-to-retrieval-augmented-generation.md------------------------------------------------------------

└── 2025-11-08-how-to-build-production-ready-workflows.md📝 FINAL RESPONSE:

```O agno-go é um framework Go para criação de agentes de IA...



### Example Output File🏁 WORKFLOW OUTPUT:

O agno-go é um framework Go para criação de agentes de IA...

```markdown============================================================

---✨ Example completed successfully!

title: "Best Practices for Building AI Agents in Go"```

date: 2025-11-08

author: AI Writer## Estrutura do código

tags: ["golang", "ai", "agents", "best-practices"]

description: "Learn the best practices for building production-ready AI agents using Go..."O exemplo demonstra:

---

- **Configuração de agentes**: Como criar agentes especializados para cada etapa

# Best Practices for Building AI Agents in Go- **Wrapper de funções**: Como adaptar agentes para funcionar com o workflow v2

- **Configuração do workflow**: Como configurar streaming, debug e manipuladores de eventos

## Introduction- **Execução sequencial**: Como executar etapas em sequência passando dados entre elas

- **Tratamento de eventos**: Como capturar e exibir eventos do workflow em tempo real

Building AI agents in Go combines the power of...- **Métricas**: Como acessar métricas de execução



## 1. Design Principles## Personalização



### Single ResponsibilityVocê pode facilmente personalizar este exemplo:

Each agent should have a clear, well-defined purpose...

1. **Modificar as instruções dos agentes** para diferentes tipos de processamento

[... full blog post content ...]2. **Adicionar mais etapas** ao workflow

3. **Implementar processamento paralelo** usando `v2.Parallel`

## Conclusion4. **Adicionar condições** usando `v2.Condition`

5. **Integrar com diferentes modelos** (OpenAI, Google Gemini, etc.)

By following these best practices...

```## Solução de problemas



## 🔄 Workflow Steps### Erro de conexão com Ollama

```

### Step 1: Research (60s timeout)Failed to create Ollama model: connection refused

- Analyzes the topic```

- Creates structured outline- Verifique se o Ollama está rodando: `ollama serve`

- Defines target audience

- Plans content strategy### Modelo não encontrado

```

### Step 2: Write (120s timeout)Failed to create Ollama model: model not found

- Writes full blog post```

- Creates engaging content- Baixe o modelo: `ollama pull llama3.2:latest`

- Adds code examples

- Develops clear explanations### Prompt obrigatório

```

### Step 3: Edit (60s timeout)Usage: go run main.go "Your question here"

- Reviews and polishes```

- Fixes grammar/style- Sempre forneça um prompt como argumento direto
- Optimizes for SEO
- Adds metadata frontmatter

## 🎨 Customization

### Change the Model

```go
model, err := ollama.NewOllamaChat(
    models.WithID("llama3.2:latest"), // Use different model
    models.WithBaseURL("http://localhost:11434"), // Or local Ollama
)
```

### Adjust Timeouts

```go
researchStep, err := v2.NewStep(
    v2.WithName("research"),
    v2.WithTimeout(90), // Increase to 90 seconds
)
```

### Enable Debug Mode

```go
debug := true // Set to true for detailed logs
```

### Customize Agent Instructions

Modify the `Instructions` field in any agent:

```go
researcherAgent, err := agent.NewAgent(agent.AgentConfig{
    // ...
    Instructions: `Your custom instructions here...`,
})
```

## 📊 Example Output

```
=== Blog Post Generator Workflow ===
Topic: Best practices for building AI agents in Go

🚀 Starting blog post generation workflow...
------------------------------------------------------------
🔄 Starting step: research
✅ Completed step: research
🔄 Starting step: write
✅ Completed step: write
🔄 Starting step: edit
✅ Completed step: edit
🎉 Blog post generation completed!
------------------------------------------------------------
📋 BLOG POST GENERATION RESULTS:
------------------------------------------------------------
📝 FINAL BLOG POST:

---
title: "Best Practices for Building AI Agents in Go"
date: 2025-11-08
...

------------------------------------------------------------
💾 Blog post saved to: blog_posts/2025-11-08-best-practices-for-building-ai-agents-in-go.md
------------------------------------------------------------
✨ Blog post generation completed successfully!
📄 Open your blog post: blog_posts/2025-11-08-best-practices-for-building-ai-agents-in-go.md
```

## 🛠️ Advanced Usage

### Batch Generation

Create multiple blog posts:

```bash
for topic in \
    "Go vs Python for AI" \
    "RAG implementation guide" \
    "Agent orchestration patterns"; do
    go run main.go "$topic"
    sleep 2
done
```

### Custom Output Directory

Modify `outputDir` in code:

```go
outputDir := "content/blog" // Custom directory
```

## 🔍 Debug Mode Features

Enable debug mode to see:
- Research outline
- Initial draft
- Execution metrics
- Step durations
- Retry counts

```go
debug := true
```

## 📚 Code Structure

```go
main.go
├── Agents Setup
│   ├── Researcher (outline creation)
│   ├── Writer (content creation)
│   └── Editor (polishing)
├── Workflow Configuration
│   ├── Steps definition
│   ├── Event handlers
│   └── Execution flow
├── File Output
│   ├── Filename generation
│   ├── File writing
│   └── Success reporting
└── Helper Functions
    ├── generateFilename()
    └── truncateString()
```

## 🎯 Use Cases

1. **Content Marketing**
   - Generate blog posts at scale
   - Maintain consistent quality
   - SEO-optimized content

2. **Technical Documentation**
   - Create tutorial content
   - Generate how-to guides
   - Document best practices

3. **Knowledge Base**
   - Build internal wikis
   - Create learning materials
   - Document processes

4. **Content Ideation**
   - Explore topic variations
   - Generate multiple drafts
   - A/B test different approaches

## 🔗 Related Examples

- **[Agents Cookbook](../agents/)** - Individual agent examples
- **[Workflow V2](../../docs/WORKFLOW_V2_IMPLEMENTATION.md)** - Workflow documentation
- **[Ollama Cloud](../agents/ollama-cloud/)** - Cloud model usage

## 💡 Tips

1. **Specific Topics Work Best**: Provide clear, focused topics for better results
2. **Review Output**: Always review and edit generated content before publishing
3. **Iterate**: Run multiple times with different phrasings to get best results
4. **Customize Instructions**: Tailor agent instructions to your blog's style
5. **Use Debug Mode**: Enable debug to see intermediate steps

## 🐛 Troubleshooting

### API Key Issues
```bash
# Verify API key is set
echo $OLLAMA_API_KEY

# Set in current session
export OLLAMA_API_KEY="your-key"
```

### Timeout Errors
- Increase step timeouts for complex topics
- Use faster models for quicker generation

### File Permission Errors
```bash
# Ensure write permissions
chmod 755 blog_posts/
```

## 📝 License

Same license as the main agno-golang project.

---

**Last Updated:** November 8, 2025  
**Workflow:** Research → Write → Edit → Save  
**Model:** Ollama Cloud (kimi-k2:1t-cloud)
