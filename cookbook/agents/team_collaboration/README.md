# Team Collaboration Example

This example demonstrates how to create and use multi-agent teams for collaborative task completion using the Agno Team system.

## Features Demonstrated

- **Multi-Agent Teams**: Create teams with specialized agents
- **Agent Collaboration**: Coordinate multiple agents to complete complex tasks
- **Task Delegation**: Distribute work among team members based on expertise
- **Response Synthesis**: Combine outputs from multiple agents into cohesive results
- **Team Modes**: Demonstrate different collaboration patterns (Coordinate mode)

## What is Team Collaboration?

Team collaboration in Agno allows you to:
- Create teams of specialized agents with different roles
- Delegate tasks to the most appropriate team members
- Synthesize responses from multiple perspectives
- Scale complex workflows across multiple agents
- Improve output quality through diverse expertise

## Prerequisites

- Go 1.21 or higher
- Ollama Cloud API key

## Setup

1. Set your Ollama Cloud API key:
```bash
export OLLAMA_API_KEY=your_api_key_here
```

2. Run the example:
```bash
go run main.go
```

## How It Works

### 1. Create Specialized Agents

The example creates three specialized agents:

**Research Specialist**
```go
researchAgent, err := agent.NewAgent(agent.AgentConfig{
    Name:        "Research Specialist",
    Role:        "researcher",
    Description: "Expert in gathering and analyzing information",
    Instructions: "Gather relevant information, analyze data, provide factual responses...",
})
```

**Content Writer**
```go
writerAgent, err := agent.NewAgent(agent.AgentConfig{
    Name:        "Content Writer",
    Role:        "writer",
    Description: "Expert in creating engaging content",
    Instructions: "Transform research into engaging content...",
})
```

**Editor**
```go
editorAgent, err := agent.NewAgent(agent.AgentConfig{
    Name:        "Editor",
    Role:        "editor",
    Description: "Expert in reviewing and improving content",
    Instructions: "Review content for clarity and accuracy...",
})
```

### 2. Create Agent Wrappers

Wrap agents to implement the TeamMember interface:
```go
researchMember := &AgentWrapper{agent: researchAgent}
writerMember := &AgentWrapper{agent: writerAgent}
editorMember := &AgentWrapper{agent: editorAgent}
```

### 3. Create the Team

```go
contentTeam := team.NewTeam(team.TeamConfig{
    Context:     ctx,
    Name:        "Content Creation Team",
    Description: "A team specialized in creating high-quality content",
    Model:       ollamaModel,
    Members:     []team.TeamMember{researchMember, writerMember, editorMember},
    Mode:        team.CoordinateMode,
})
```

### 4. Individual Agent Workflow

The example demonstrates a complete content creation workflow:

1. **Research Phase**: Research Specialist gathers information
2. **Writing Phase**: Content Writer creates article based on research
3. **Editing Phase**: Editor reviews and improves the content

### 5. Team-Level Operation

The team can also work collectively on tasks:
```go
teamResponse, err := contentTeam.Run(teamTask)
```

The team leader coordinates members and synthesizes their responses.

## Team Modes

Agno supports three collaboration modes:

### 1. Route Mode
- Team leader routes requests to the most appropriate member
- Best for: Specialized tasks requiring specific expertise

### 2. Coordinate Mode (Used in this example)
- Team leader delegates tasks and synthesizes responses
- Best for: Complex tasks requiring multiple perspectives

### 3. Collaborate Mode
- All members work on the same task
- Best for: Tasks benefiting from diverse viewpoints

## Benefits

- **Specialized Expertise**: Each agent focuses on their domain
- **Better Quality**: Multiple perspectives improve output
- **Efficient Delegation**: Tasks go to the right expert
- **Scalable Workflows**: Easy to add more team members
- **Improved Collaboration**: Agents complement each other's strengths

## Output

The example demonstrates:
1. Creating three specialized agents
2. Individual agent workflow (research → write → edit)
3. Team-level collaborative operation
4. Team statistics and member information

## Use Cases

- **Content Creation**: Research, writing, editing teams
- **Software Development**: Design, coding, testing teams
- **Customer Support**: Triage, resolution, follow-up teams
- **Data Analysis**: Collection, analysis, reporting teams
- **Project Management**: Planning, execution, review teams

## Team Configuration Options

```go
type TeamConfig struct {
    Context      context.Context
    Name         string
    Description  string
    Model        models.AgnoModelInterface  // Team leader model
    Members      []TeamMember
    Mode         TeamMode                   // Route, Coordinate, or Collaborate
    Debug        bool
    Markdown     bool
    Async        bool                       // Execute members concurrently
}
```

## Advanced Features

- **Async Execution**: Set `Async: true` for concurrent member execution
- **Memory Integration**: Add memory for team context persistence
- **Tool Integration**: Equip team with shared tools
- **Session Management**: Track team interactions over time

## Related Examples

- `memory_example/` - Memory management for teams
- `session_management/` - Session state for teams
- `workflow_prompt/` - Workflow orchestration
- `agentic_state/` - State management across agents
