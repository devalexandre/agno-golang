package agent

import (
	"context"
	"fmt"

	"github.com/devalexandre/agno-golang/agno/knowledge"
	"github.com/devalexandre/agno-golang/agno/models"
)

// AgentKnowledge integrates an agent with a knowledge base for RAG functionality
type AgentKnowledge struct {
	Agent         *Agent
	KnowledgeBase knowledge.Knowledge
	NumDocuments  int
}

// NewAgentKnowledge creates a new AgentKnowledge instance
func NewAgentKnowledge(agent *Agent, knowledgeBase knowledge.Knowledge, numDocuments int) *AgentKnowledge {
	return &AgentKnowledge{
		Agent:         agent,
		KnowledgeBase: knowledgeBase,
		NumDocuments:  numDocuments,
	}
}

// Run executes the agent with automatic knowledge retrieval
func (ak *AgentKnowledge) Run(ctx context.Context, message string) (models.RunResponse, error) {
	// 1. Search for relevant documents automatically
	docs, err := ak.KnowledgeBase.Search(ctx, message, ak.NumDocuments)
	if err != nil {
		return models.RunResponse{}, fmt.Errorf("failed to search knowledge base: %w", err)
	}

	// 2. Inject context into the message
	contextualMessage := ak.buildContextualMessage(message, docs)

	// 3. Agent responds with context
	return ak.Agent.Run(contextualMessage)
}

// RunStream executes the agent with automatic knowledge retrieval and streaming response
func (ak *AgentKnowledge) RunStream(ctx context.Context, message string, fn func([]byte) error) error {
	// 1. Search for relevant documents automatically
	docs, err := ak.KnowledgeBase.Search(ctx, message, ak.NumDocuments)
	if err != nil {
		return fmt.Errorf("failed to search knowledge base: %w", err)
	}

	// 2. Inject context into the message
	contextualMessage := ak.buildContextualMessage(message, docs)

	// 3. Agent responds with context (streaming)
	return ak.Agent.RunStream(contextualMessage, fn)
}

// buildContextualMessage creates a message with injected knowledge context
func (ak *AgentKnowledge) buildContextualMessage(message string, docs []*knowledge.SearchResult) string {
	if len(docs) == 0 {
		return message
	}

	// Build context from search results
	contextStr := "Relevant information from knowledge base:\n"
	for i, doc := range docs {
		contextStr += fmt.Sprintf("%d. %s\n", i+1, doc.Document.Content)
	}

	// Inject context into message
	contextualMessage := fmt.Sprintf("%s\n\nQuestion: %s", contextStr, message)
	return contextualMessage
}

// SetNumDocuments updates the number of documents to retrieve
func (ak *AgentKnowledge) SetNumDocuments(num int) {
	ak.NumDocuments = num
}
