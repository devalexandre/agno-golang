// Package vertexai provides integration with Google Vertex AI (Gemini models on
// GCP). It reuses the Gemini implementation with the Vertex backend, which
// authenticates via Application Default Credentials (ADC) — project + location,
// no API key.
//
// Setup: run `gcloud auth application-default login` (or set
// GOOGLE_APPLICATION_CREDENTIALS) so ADC is available.
package vertexai

import (
	"os"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/google/gemini"
)

const (
	// EnvProject is read when no project is provided.
	EnvProject = "GOOGLE_CLOUD_PROJECT"
	// EnvLocation is read when no location is provided.
	EnvLocation = "GOOGLE_CLOUD_LOCATION"
	// DefaultLocation is used when no location is configured.
	DefaultLocation = "us-central1"
	// DefaultModel is used when no model ID is provided.
	DefaultModel = "gemini-2.0-flash"
)

// Config holds the Vertex AI connection parameters.
type Config struct {
	Project  string // GCP project ID (or GOOGLE_CLOUD_PROJECT)
	Location string // GCP region (or GOOGLE_CLOUD_LOCATION; defaults to us-central1)
	Model    string // model ID (defaults to gemini-2.0-flash)
}

// NewVertexAI creates a new Vertex AI (Gemini) chat model.
//
//	chat, err := vertexai.NewVertexAI(vertexai.Config{
//	    Project:  "my-gcp-project",
//	    Location: "us-central1",
//	    Model:    "gemini-2.0-flash",
//	})
func NewVertexAI(cfg Config, options ...models.OptionClient) (models.AgnoModelInterface, error) {
	project := cfg.Project
	if project == "" {
		project = os.Getenv(EnvProject)
	}
	location := cfg.Location
	if location == "" {
		location = os.Getenv(EnvLocation)
	}
	if location == "" {
		location = DefaultLocation
	}
	model := cfg.Model
	if model == "" {
		model = DefaultModel
	}

	final := []models.OptionClient{
		models.WithID(model),
		models.WithClientParams(map[string]interface{}{
			"vertex":   true,
			"project":  project,
			"location": location,
		}),
	}
	final = append(final, options...)

	return gemini.NewGemini(final...)
}
