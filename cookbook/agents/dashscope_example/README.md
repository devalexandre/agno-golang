# Exemplo de uso do DashScope (Qwen) via endpoint compatível com OpenAI

Este exemplo mostra como usar modelos Qwen via LM Studio (API compatível com OpenAI) usando o provider `dashscope` no Agno Go.

## Sub-exemplos

- `cookbook/agents/dashscope_example/local` (LM Studio/local)
- `cookbook/agents/dashscope_example/reasoning_content` (captura `thinking`/`reasoning_content`)
- `cookbook/agents/dashscope_example/parallel_tools` (tool calls em paralelo)

## Rodando (LM Studio)

```
cd cookbook/agents/dashscope_example
LLM_STUDIO_BASE_URL="http://localhost:1234/v1" \
LLM_STUDIO_MODEL="qwen2.5-3b-instruct" \
go run main.go
```

Obs: se não setar variáveis, os exemplos assumem `http://localhost:1234/v1`.
