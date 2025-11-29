package culture

import (
	"time"
)

// CulturalKnowledge represents cultural knowledge for a specific user
type CulturalKnowledge struct {
	UserID    string                 `json:"user_id"`
	Knowledge map[string]interface{} `json:"knowledge"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
}

// CultureManagerConfig configures the culture manager
type CultureManagerConfig struct {
	// Model is the AI model used for cultural knowledge processing
	Model interface{}
	// DB is the database for storing cultural knowledge
	DB interface{}
	// Enabled determines if culture management is active
	Enabled bool
}
