# Phase 1 Examples - Input Validation & Dependencies Manager

Este diretório contém exemplos práticos de como usar as novas funcionalidades implementadas na Phase 1 do Agno Agent.

## 📚 Exemplos Disponíveis

### 1. Input Validation (`02_input_validation`)
Demonstra como usar Input Validation para validar dados de entrada antes de processar com o agent.

**Features:**
- Validação de campos obrigatórios (required)
- Validação de min/max length
- Validação de ranges numéricos
- Mensagens de erro detalhadas
- Integração com Agent

**Como executar:**
```bash
cd 02_input_validation
go run main.go
```

**Casos de Uso:**
- ✅ Validar entrada de usuário contra schema
- ✅ Enforçar regras de negócio
- ✅ Proteger o agent de dados inválidos
- ✅ Prover mensagens de erro úteis

---

### 2. Dependencies Manager (`03_dependencies_manager`)
Demonstra como usar o Dependency Manager para gerenciar e resolver dependências de aplicação.

**Features:**
- Definir e recuperar dependências simples
- Registrar resolvers dinâmicos
- Resolução com cache automático
- Merge de dependency managers
- Template processing com placeholders
- Injeção de dependências em structs

**Como executar:**
```bash
cd 03_dependencies_manager
go run main.go
```

**Casos de Uso:**
- ✅ Gerenciar conexões de banco de dados
- ✅ Compartilhar configurações entre componentes
- ✅ Resolver valores dinâmicos (timestamps, roles, etc)
- ✅ Injetar dependências em structs
- ✅ Processar templates com variáveis

---

## 🚀 Executar Todos os Exemplos

```bash
# Compilar todos
go build ./cookbook/getting_started/02_input_validation
go build ./cookbook/getting_started/03_dependencies_manager

# Ou executar com go run
go run ./cookbook/getting_started/02_input_validation/main.go
go run ./cookbook/getting_started/03_dependencies_manager/main.go
```

## 🔗 Requisitos

- Go 1.21+
- Ollama rodando localmente em `http://localhost:11434`
- Modelo `llama3.2:latest` instalado no Ollama

## 📖 Documentação

Para mais detalhes sobre Input Validation e Dependencies Manager, consulte:
- `agno/agent/input_validation.go` - Implementação completa
- `agno/agent/dependencies.go` - Implementação completa
- `docs/AGENT_PYTHON_VS_GO_IMPLEMENTATION_PLAN.md` - Planejamento da Phase 1

## ✅ Testes

Ambos os componentes têm testes unitários completos:

```bash
# Executar todos os testes
go test ./agno/agent -v

# Executar apenas testes específicos
go test ./agno/agent -v -run "TestInputValidator"
go test ./agno/agent -v -run "TestDependencyManager"
```

## 🎓 Próximas Fases

- **Phase 2**: Parser Model + Output Formatting (semana 2-3)
- **Phase 3**: Context Builders avançados (semana 3-4)
- **Phase 4**: Media Handling (semana 4-5)

---

Desenvolvido como parte da Phase 1 de implementação do Agno Agent Python → Go
