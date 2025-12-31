# Exemplo de uso do vLLM via API compatível com OpenAI (likeopenai)

Este exemplo mostra como usar o agente Go para acessar um endpoint vLLM compatível com a API do OpenAI, reproduzindo a chamada cURL abaixo:

```
curl -X POST "https://z2bg1juojbhurv-8000.proxy.runpod.net/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-IrR7Bwxtin0haWagUnPrBgq5PurnUz86" \
  --data '{
    "model": "EssentialAI/rnj-1-instruct",
    "messages": [
      {
        "role": "user",
        "content": "What is the capital of France?"
      }
    ]
  }'
```

## Como rodar

1. Instale as dependências do projeto principal.
2. Execute:

```
cd cookbook/agents/vllm_example
GO111MODULE=on go run main.go
```

## Código principal
Veja `main.go` para detalhes de implementação.
