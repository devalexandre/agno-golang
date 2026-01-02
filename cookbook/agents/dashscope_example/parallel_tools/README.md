# Exemplo: Qwen com tool calls em paralelo

Este exemplo configura duas tools simples e pede ao modelo para chamá-las no mesmo turno (*parallel tool calls*).

## LM Studio

```
cd cookbook/agents/dashscope_example/parallel_tools
LLM_STUDIO_BASE_URL=http://localhost:1234/v1 \
LLM_STUDIO_MODEL='qwen2.5-3b-instruct' \
go run main.go
```
