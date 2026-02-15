# Using agno-golang with Ollama Cloud

This guide explains how to use agno-golang with Ollama Cloud's remote models.

## Prerequisites

1. **Ollama Cloud Account**: Sign up at [ollama.com](https://ollama.com)
2. **API Key**: Get your API key from https://ollama.com/settings/api
3. **SSH Key**: Required for Ollama Cloud authentication

## Setup

### 1. Generate SSH Key

The Ollama SDK requires an SSH key for cloud authentication. Generate it with:

```bash
mkdir -p ~/.ollama
ssh-keygen -t ed25519 -f ~/.ollama/id_ed25519 -N "" -C "ollama-cloud-key"
```

This creates two files:
- `~/.ollama/id_ed25519` - Private key (keep secret)
- `~/.ollama/id_ed25519.pub` - Public key (register with Ollama)

### 2. Register Public Key

1. Copy your public key:
   ```bash
   cat ~/.ollama/id_ed25519.pub
   ```

2. Go to https://ollama.com/settings/keys

3. Click "Add SSH Key"

4. Paste your public key and save

### 3. Set API Key

Export your Ollama Cloud API key:

```bash
export OLLAMA_API_KEY="your-api-key-here"
```

Add to `~/.bashrc` or `~/.zshrc` for persistence:

```bash
echo 'export OLLAMA_API_KEY="your-api-key-here"' >> ~/.bashrc
source ~/.bashrc
```

## Usage

### Basic Example

```go
package main

import (
    "context"
    "log"
    "os"

    "github.com/devalexandre/agno-golang/agno/agent"
    "github.com/devalexandre/agno-golang/agno/models"
    "github.com/devalexandre/agno-golang/agno/models/ollama"
)

func main() {
    // Create Ollama Cloud model
    model, err := ollama.NewOllamaChat(
        models.WithID("deepseek-v3.1:671b-cloud"),  // Cloud model
        models.WithBaseURL("https://ollama.com"),    // Ollama Cloud URL
        models.WithAPIKey(os.Getenv("OLLAMA_API_KEY")), // API key from env
    )
    if err != nil {
        log.Fatalf("Failed to create model: %v", err)
    }

    // Create agent
    agent, err := agent.NewAgent(agent.AgentConfig{
        Model:        model,
        Context:      context.Background(),
        Name:         "Assistant",
        Instructions: "You are a helpful assistant.",
        Stream:       true,
    })
    if err != nil {
        log.Fatalf("Failed to create agent: %v", err)
    }

    // Run agent
    response, err := agent.Run("What is Go programming language?")
    if err != nil {
        log.Fatalf("Failed to run agent: %v", err)
    }

    log.Printf("Response: %s", response)
}
```

## Available Cloud Models

Check available cloud models at https://ollama.com/library

Popular cloud models (note the `-cloud` suffix):
- `deepseek-v3.1:671b-cloud` - DeepSeek v3.1 (671B parameters)
- `llama3.3:70b-cloud` - Llama 3.3 70B
- `qwen2.5:72b-cloud` - Qwen 2.5 72B
- `gemma2:27b-cloud` - Gemma 2 27B

## Testing Connection

Test your setup with curl:

```bash
curl https://ollama.com/api/chat \
  -H "Authorization: Bearer $OLLAMA_API_KEY" \
  -d '{
    "model":"deepseek-v3.1:671b-cloud",
    "messages": [{
      "role": "user",
      "content": "Hello!"
    }],
    "stream": false
  }'
```

If this works but Go code fails with "401 Unauthorized", ensure:
1. SSH key is registered at https://ollama.com/settings/keys
2. `OLLAMA_API_KEY` environment variable is set
3. You're using a cloud model (with `-cloud` suffix)

## Troubleshooting

### Error: "no such file or directory: /home/user/.ollama/id_ed25519"

**Solution**: Generate the SSH key (see step 1 above)

### Error: "401 Unauthorized"

**Possible causes**:
1. Public SSH key not registered → Register at https://ollama.com/settings/keys
2. Invalid API key → Check at https://ollama.com/settings/api
3. Wrong model name → Use cloud models (with `-cloud` suffix)

### Error: "Failed to load private key"

**Solution**: Check file permissions:
```bash
chmod 600 ~/.ollama/id_ed25519
chmod 644 ~/.ollama/id_ed25519.pub
```

## Why SSH Key AND API Key?

Ollama Cloud uses **dual authentication**:
- **SSH Key**: Authenticates your machine/identity
- **API Key**: Authorizes API access and billing

Both are required for cloud model access.

## Local vs Cloud

**Local Ollama** (localhost:11434):
- ✅ No SSH key needed
- ✅ No API key needed
- ✅ Free usage
- ❌ Limited to your hardware

**Ollama Cloud** (ollama.com):
- ❌ Requires SSH key registration
- ❌ Requires API key
- ❌ Paid usage (credits)
- ✅ Access to large models (70B+)
- ✅ Fast inference on cloud GPUs

## Example: Cookbook

See complete example at `cookbook/agents/ollama-cloud/main.go`

Run with:
```bash
export OLLAMA_API_KEY="your-key"
go run cookbook/agents/ollama-cloud/main.go
```

## Security Notes

1. **Never commit** `~/.ollama/id_ed25519` (private key)
2. **Never commit** `OLLAMA_API_KEY` to git
3. Use environment variables for sensitive data
4. Rotate keys periodically at https://ollama.com/settings

## Resources

- Ollama Cloud Docs: https://ollama.com/docs
- API Reference: https://github.com/ollama/ollama/blob/main/docs/api.md
- Model Library: https://ollama.com/library
