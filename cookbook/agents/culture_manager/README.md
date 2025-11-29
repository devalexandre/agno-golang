# Culture Manager Example

This example demonstrates how to use the **Culture Manager** to store and utilize cultural knowledge for personalized agent interactions.

## Concept

The Culture Manager allows you to:
1. Store user-specific cultural knowledge (preferences, interests, communication style)
2. Retrieve cultural context for personalized responses
3. Automatically add cultural context to agent prompts
4. Build more personalized and context-aware AI assistants

## How It Works

```
User Preferences → Culture Manager → Agent Context → Personalized Response
```

1. Store cultural knowledge (language, timezone, interests, etc.)
2. Culture Manager retrieves knowledge for the user
3. Agent receives cultural context automatically
4. Agent provides personalized, culturally-aware responses

## Running the Example

```bash
# Make sure Ollama is running
ollama serve

# Run the example
go run main.go
```

## What This Example Shows

### 1. Cultural Knowledge Storage
```go
cultureManager.UpdateCulturalKnowledge(ctx, userID, map[string]interface{}{
    "preferred_language": "Portuguese (Brazil)",
    "timezone":          "America/Sao_Paulo",
    "communication_style": "friendly and informal",
    "interests":         []string{"technology", "AI", "Go programming"},
})
```

### 2. Agent with Cultural Context
```go
agent.NewAgent(agent.AgentConfig{
    CultureManager:       cultureManager,
    EnableAgenticCulture: true,
    AddCultureToContext:  true,
})
```

### 3. Comparison
The example runs two agents side-by-side:
- **Standard Agent**: No cultural context
- **Cultural Agent**: Full cultural awareness

## Cultural Knowledge Types

You can store various types of cultural information:

- **Language Preferences**: Preferred language, dialect
- **Location**: Timezone, region, country
- **Communication Style**: Formal/informal, verbose/concise
- **Interests**: Topics of interest, hobbies
- **Previous Context**: Past conversations, topics discussed
- **Preferences**: UI preferences, notification settings
- **Professional Context**: Industry, role, expertise level

## Benefits

- **Personalization**: Responses tailored to user preferences
- **Context Continuity**: Remember previous interactions
- **Cultural Sensitivity**: Adapt to cultural norms
- **Better UX**: More relevant and engaging conversations
- **Efficiency**: Reduce repetitive questions

## Use Cases

- **Customer Support**: Remember customer preferences and history
- **Personal Assistants**: Adapt to user's communication style
- **Educational Tools**: Adjust to learning preferences
- **Multi-language Apps**: Automatic language adaptation
- **Enterprise Chatbots**: Role-based personalization

## Advanced Features

### Automatic Knowledge Updates
```go
agent.NewAgent(agent.AgentConfig{
    UpdateCulturalKnowledge: true, // Learn from conversations
})
```

### Custom Knowledge Extraction
The Culture Manager can be extended to automatically extract cultural insights from conversations using AI.

## Database Integration

The current implementation uses in-memory storage. For production:

1. Implement database storage using the provided schema:
   - `agno/db/schemas/culture.go`
2. Connect to PostgreSQL, MySQL, or other databases
3. Enable persistent cultural knowledge across sessions

## Privacy Considerations

When implementing cultural knowledge storage:
- ✅ Get user consent for data collection
- ✅ Allow users to view/edit their cultural profile
- ✅ Provide data deletion options
- ✅ Encrypt sensitive information
- ✅ Follow GDPR/privacy regulations
