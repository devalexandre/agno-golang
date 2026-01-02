# Exemplo: Qwen `reasoning_content` / `thinking`

Este exemplo mostra como capturar o conteúdo de raciocínio retornado por alguns modelos Qwen (ex.: modelos *thinking*) via API compatível com OpenAI.

## LM Studio

```
cd cookbook/agents/dashscope_example/reasoning_content
LLM_STUDIO_BASE_URL=http://localhost:1234/v1 \
LLM_STUDIO_MODEL='qwen3-4b-thinking-2507' \
go run main.go
```

Opcional: se seu backend suportar um parâmetro estilo DashScope para habilitar thinking, rode com:

```
LLM_STUDIO_ENABLE_THINKING=1 go run main.go
```
