package embedder

import (
	"errors"
	"os"
)

// ErrLangDBProjectMissing is returned when no LangDB project ID is configured.
var ErrLangDBProjectMissing = errors.New("LANGDB_PROJECT_ID not set")

// NewLangDBEmbedder creates an embedder for the LangDB AI gateway, which
// exposes an OpenAI-compatible /embeddings endpoint per project, mirroring
// Agno Python's LangDBEmbedder.
//
// apiKey falls back to LANGDB_API_KEY and projectID to LANGDB_PROJECT_ID.
// baseURL defaults to https://api.us-east-1.langdb.ai/{projectID}/v1. The
// default model is text-embedding-ada-002 (1536 dimensions).
func NewLangDBEmbedder(apiKey, projectID, model string, dimensions int, baseURL string) (*OpenAIEmbedder, error) {
	apiKey = firstNonEmpty(apiKey, os.Getenv("LANGDB_API_KEY"))
	projectID = firstNonEmpty(projectID, os.Getenv("LANGDB_PROJECT_ID"))
	if projectID == "" && baseURL == "" {
		return nil, ErrLangDBProjectMissing
	}
	if model == "" {
		model = "text-embedding-ada-002"
	}
	if dimensions <= 0 {
		dimensions = 1536
	}
	if baseURL == "" {
		baseURL = "https://api.us-east-1.langdb.ai/" + projectID + "/v1"
	}
	return NewOpenAILikeEmbedder(baseURL, model, apiKey, dimensions), nil
}
