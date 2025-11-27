# Terminal Panel System - README

## 🎨 Novo Sistema de Painéis com Lipgloss

Sistema de painéis completamente redesenhado para o agno-golang, oferecendo uma experiência visual moderna e confiável.

## ✨ Features

- 🎨 **10 tipos de painéis** (Thinking, Response, Tool Call, Debug, Error, Success, Warning, Info, Reasoning, Custom)
- 🌈 **Cores vibrantes** com paleta moderna
- 📝 **Suporte a Markdown** via glamour
- 😀 **Emojis nativos** em todos os painéis
- 🔄 **Bordas arredondadas** elegantes
- 📏 **Detecção automática** de tamanho do terminal
- ⚡ **Performance** otimizada
- 🔧 **API simples** e intuitiva

## 🚀 Quick Start

```go
package main

import (
    "time"
    "github.com/devalexandre/agno-golang/agno/utils"
)

func main() {
    // Habilitar markdown
    utils.SetMarkdownMode(true)
    
    // Mostrar painel de pensamento
    utils.ThinkingPanel("Processing...")
    
    // Mostrar resposta
    start := time.Now()
    utils.ResponsePanel("# Hello! 🎉", nil, start, true)
}
```

## 📦 Estrutura

```
agno/utils/terminal/
├── styles.go      # Cores e estilos
├── renderer.go    # Renderização
├── stream.go      # Streaming
└── utils.go       # Utilitários

agno/utils/
└── panel.go       # API pública
```

## 📚 Documentação

Veja [terminal_panel_guide.md](terminal_panel_guide.md) para documentação completa.

## 🎯 Exemplos

### Demo Completo
```bash
cd cookbook/getting_started/panel_demo
go run main.go
```

### Agent Básico
```bash
cd cookbook/getting_started/01_basic_agent
go run main.go
```

## 🔄 Migração do pterm

A migração é simples! A API é compatível:

```go
// Antes
spinner := utils.ThinkingPanel(content)
utils.ResponsePanel(content, spinner, start, markdown)

// Depois
utils.ThinkingPanel(content)
utils.ResponsePanel(content, nil, start, markdown)
```

## 🎨 Visual Preview

```
╭─ 🤔 Thinking... ─────────────────────────────╮
│                                              │
│  Processing your request...                  │
│                                              │
╰──────────────────────────────────────────────╯

╭─ ✨ Response (1.2s) ─────────────────────────╮
│                                              │
│  # Breaking News! 🗽                         │
│                                              │
│  **Times Square** is buzzing!                │
│                                              │
╰──────────────────────────────────────────────╯
```

## 🏆 Benefícios

| Antes (pterm) | Depois (lipgloss) |
|---------------|-------------------|
| ❌ Quebrava com conteúdo pequeno | ✅ Sempre funciona |
| ❌ Cores básicas | ✅ Paleta moderna |
| ❌ Sem markdown | ✅ Markdown completo |
| ❌ Emojis problemáticos | ✅ Emojis nativos |

## 📝 License

MIT

---

**Feito com ❤️ usando [Lipgloss](https://github.com/charmbracelet/lipgloss) e [Glamour](https://github.com/charmbracelet/glamour)**
