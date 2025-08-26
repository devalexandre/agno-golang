package memory

import (
	"github.com/devalexandre/agno-golang/agno/models"
)

// MessagePair represents a pair of user and assistant messages
type MessagePair struct {
	UserMessage      models.Message
	AssistantMessage models.Message
	ModelMessage     models.Message
}
