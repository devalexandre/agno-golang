# Agno Flow (Fluent API)

O pacote `flow` fornece uma API fluida (Fluent/Fluid API) para a construção de workflows no Agno. Ele simplifica a criação de sequências de passos, condições, loops e execuções paralelas de forma legível e declarativa.

## Visão Geral

Em vez de instanciar manualmente objetos `v2.Step`, `v2.Condition`, `v2.Loop`, etc., o `FlowBuilder` permite encadear chamadas de métodos para definir a estrutura do seu workflow.

### Benefícios

- **Legibilidade:** O código reflete claramente o fluxo lógico.
- **Simplicidade:** Reduz a verbosidade na configuração de passos e condições.
- **Integração:** Funciona perfeitamente com `Agent`, `Team` e funções personalizadas (`ExecutorFunc`).

---

## Como Começar

Importe o pacote `flow` e o pacote de workflow `v2`:

```go
import (
    "github.com/devalexandre/agno-golang/agno/flow"
    v2 "github.com/devalexandre/agno-golang/agno/workflow/v2"
)
```

### Exemplo Básico

Um workflow simples que transforma texto e aplica uma condição:

```go
func main() {
    workflow := flow.New("Basic Workflow").
        Description("Um exemplo simples de workflow").
        Step("uppercase", func(input *v2.StepInput) (*v2.StepOutput, error) {
            msg := input.GetMessageAsString()
            return &v2.StepOutput{
                Content: strings.ToUpper(msg),
            }, nil
        }).
        If(v2.IfContentContains("HELLO"),
            func(input *v2.StepInput) (*v2.StepOutput, error) {
                return &v2.StepOutput{
                    Content: "Saudação detectada: " + input.GetLastStepContent().(string),
                }, nil
            },
        ).
        Else(
            func(input *v2.StepInput) (*v2.StepOutput, error) {
                return &v2.StepOutput{
                    Content: "Mensagem normal: " + input.GetLastStepContent().(string),
                }, nil
            },
        ).
        Build()

    workflow.PrintResponse("hello world", false)
}
```

---

## Integração com Agentes

O `FlowBuilder` facilita o uso de Agentes como passos do workflow.

```go
func main() {
    researcher, _ := agent.NewAgent(agent.AgentConfig{
        Name: "Researcher",
        Model: model,
        Instructions: "Busque fatos sobre o tópico.",
    })

    writer, _ := agent.NewAgent(agent.AgentConfig{
        Name: "Writer",
        Model: model,
        Instructions: "Escreva um email profissional baseado na pesquisa.",
    })

    workflow := flow.New("AI Content Flow").
        Step("research", researcher).
        If(flow.IfSuccess(),
            v2.NewStep(v2.WithName("writer"), v2.WithAgent(writer)),
        ).
        Build()

    workflow.PrintResponse("Impacto da IA em 2026", true)
}
```

---

## Referência da API

### `flow.New(name string) *FlowBuilder`
Inicia a construção de um novo workflow com o nome fornecido.

### `Description(desc string) *FlowBuilder`
Define a descrição do workflow.

### `Debug(debug bool) *FlowBuilder`
Ativa ou desativa o modo de depuração.

### `Step(name string, executor any, options ...v2.StepOption) *FlowBuilder`
Adiciona um passo ao workflow. O `executor` pode ser:
- `Agent`
- `Team`
- `ExecutorFunc`
- `func(*v2.StepInput) (*v2.StepOutput, error)`

### `If(condition v2.ConditionFunc, thenSteps ...any) *ConditionBuilder`
Adiciona uma estrutura condicional. Retorna um `ConditionBuilder` que permite encadear um `.Else()`.

### `Else(elseSteps ...any) *FlowBuilder`
Define os passos a serem executados caso a condição do `If` seja falsa. Retorna ao `FlowBuilder`.

### `Loop(condition v2.LoopCondition, steps ...any) *FlowBuilder`
Adiciona um laço de repetição baseado em uma condição.

### `Parallel(steps ...any) *FlowBuilder`
Adiciona passos que serão executados em paralelo.

### `Build() *v2.Workflow`
Finaliza a construção e retorna o objeto `v2.Workflow` pronto para execução.

---

## Funções Auxiliares

### `flow.IfSuccess()`
Uma função de conveniência que retorna uma `ConditionFunc` que verifica se o passo anterior retornou um resultado válido (não nulo).
