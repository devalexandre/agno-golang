# Parser Model Example

This example demonstrates how to use a **Parser Model** with an agent to parse and structure responses from the main model.

## Concept

The Parser Model feature allows you to:
1. Use an expensive, creative model (e.g., GPT-4) for generating content
2. Use a cheaper, focused model (e.g., GPT-4-mini) for parsing and structuring that content

This is cost-effective when you need creative output but also want structured data.

## How It Works

```
User Input → Main Model (Creative) → Parser Model (Structure) → Final Output
```

1. **Main Model** generates verbose, creative content
2. **Parser Model** parses and structures that content
3. You get both the original creative output AND the structured version

## Running the Example

```bash
export OPENAI_API_KEY=your_api_key_here
go run main.go
```

## Configuration

```go
agent.NewAgent(agent.AgentConfig{
    Model: mainModel,              // Expensive, creative model
    ParserModel: parserModel,      // Cheap parsing model
    ParserModelPrompt: "...",      // Custom parsing instructions
})
```

## Benefits

- **Cost Optimization**: Use expensive models only for creativity
- **Structured Output**: Get parsed, structured data automatically
- **Flexibility**: Customize parsing logic with prompts
- **Fallback**: If parser fails, original response is used

## Use Cases

- Extract structured data from creative text
- Summarize long-form content
- Convert prose to bullet points
- Parse technical details from explanations
