package culture

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// CultureManager manages cultural knowledge for users
type CultureManager struct {
	config CultureManagerConfig
	cache  map[string]*CulturalKnowledge
}

// NewCultureManager creates a new culture manager
func NewCultureManager(config CultureManagerConfig) *CultureManager {
	return &CultureManager{
		config: config,
		cache:  make(map[string]*CulturalKnowledge),
	}
}

// GetCulturalKnowledge retrieves cultural knowledge for a user
func (cm *CultureManager) GetCulturalKnowledge(ctx context.Context, userID string) (*CulturalKnowledge, error) {
	if !cm.config.Enabled {
		return nil, nil
	}

	// Check cache first
	if knowledge, ok := cm.cache[userID]; ok {
		return knowledge, nil
	}

	// TODO: Load from database when DB integration is ready
	// For now, return empty knowledge
	knowledge := &CulturalKnowledge{
		UserID:    userID,
		Knowledge: make(map[string]interface{}),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	cm.cache[userID] = knowledge
	return knowledge, nil
}

// UpdateCulturalKnowledge updates cultural knowledge for a user
func (cm *CultureManager) UpdateCulturalKnowledge(ctx context.Context, userID string, knowledge map[string]interface{}) error {
	if !cm.config.Enabled {
		return nil
	}

	existing, err := cm.GetCulturalKnowledge(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get existing knowledge: %w", err)
	}

	// Merge new knowledge with existing
	for key, value := range knowledge {
		existing.Knowledge[key] = value
	}
	existing.UpdatedAt = time.Now()

	// Update cache
	cm.cache[userID] = existing

	// TODO: Save to database when DB integration is ready

	return nil
}

// AddCultureToContext generates a context string from cultural knowledge
func (cm *CultureManager) AddCultureToContext(ctx context.Context, userID string) (string, error) {
	if !cm.config.Enabled {
		return "", nil
	}

	knowledge, err := cm.GetCulturalKnowledge(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("failed to get cultural knowledge: %w", err)
	}

	if len(knowledge.Knowledge) == 0 {
		return "", nil
	}

	keys := make([]string, 0, len(knowledge.Knowledge))
	for k := range knowledge.Knowledge {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var lines []string
	for _, k := range keys {
		v := knowledge.Knowledge[k]
		if v == nil {
			continue
		}
		s := fmt.Sprint(v)
		s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
		if s == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("- %s: %s", k, s))
	}
	if len(lines) == 0 {
		return "", nil
	}

	contextStr := fmt.Sprintf(`

<user_profile>
User preferences & style (lightweight profile):
%s
Instructions:
- Adapt tone/style and examples to match the profile.
- Do not treat this as factual knowledge; use it only for personalization.
</user_profile>
`, strings.Join(lines, "\n"))

	return contextStr, nil
}

// ExtractCulturalInsights extracts cultural insights from a conversation
// This can be called after agent runs to learn about user preferences
func (cm *CultureManager) ExtractCulturalInsights(ctx context.Context, userID string, conversation []string) error {
	if !cm.config.Enabled {
		return nil
	}

	// TODO: Use AI model to extract cultural insights from conversation
	// For now, this is a placeholder for future implementation

	return nil
}
