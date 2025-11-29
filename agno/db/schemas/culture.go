package schemas

// CulturalKnowledgeSchema defines the database schema for cultural knowledge
// This can be used with SQL databases or document stores
const CulturalKnowledgeSchema = `
CREATE TABLE IF NOT EXISTS cultural_knowledge (
    id SERIAL PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    knowledge_key VARCHAR(255) NOT NULL,
    knowledge_value TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, knowledge_key)
);

CREATE INDEX IF NOT EXISTS idx_cultural_knowledge_user_id ON cultural_knowledge(user_id);
`

// CulturalKnowledgeTable represents the table name
const CulturalKnowledgeTable = "cultural_knowledge"
