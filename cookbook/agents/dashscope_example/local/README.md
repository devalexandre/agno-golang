# Exemplo: Qwen local (LM Studio) usando provider `dashscope`

O provider `dashscope` reutiliza a implementação OpenAI-compatible, então você pode apontar para o LM Studio (ou vLLM) e usar os mesmos modelos locais.

## Rodar

```
cd cookbook/agents/dashscope_example/local
LLM_STUDIO_BASE_URL=http://localhost:1234/v1 \
LLM_STUDIO_MODEL='qwen3-vl-2b-instruct' \
go run main.go
```
