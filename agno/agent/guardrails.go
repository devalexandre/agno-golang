package agent

import (
	"context"
	"fmt"
)

// Guardrail represents a reusable validation/policy rule
type Guardrail interface {
	// Check validates the input according to the guardrail policy
	// Returns error if validation fails
	Check(ctx context.Context, data interface{}) error
	// GetName returns the guardrail name for identification
	GetName() string
	// GetDescription returns a human-readable description
	GetDescription() string
}

// GuardrailFunc is a simple function-based guardrail implementation
type GuardrailFunc struct {
	Name        string
	Description string
	CheckFunc   func(ctx context.Context, data interface{}) error
}

func (g *GuardrailFunc) Check(ctx context.Context, data interface{}) error {
	return g.CheckFunc(ctx, data)
}

func (g *GuardrailFunc) GetName() string {
	return g.Name
}

func (g *GuardrailFunc) GetDescription() string {
	return g.Description
}

// GuardrailChain executes multiple guardrails in sequence
type GuardrailChain struct {
	Name       string
	Guardrails []Guardrail
}

func (gc *GuardrailChain) Check(ctx context.Context, data interface{}) error {
	for _, guardrail := range gc.Guardrails {
		if err := guardrail.Check(ctx, data); err != nil {
			return fmt.Errorf("%s failed: %w", guardrail.GetName(), err)
		}
	}
	return nil
}

func (gc *GuardrailChain) GetName() string {
	return gc.Name
}

func (gc *GuardrailChain) GetDescription() string {
	return fmt.Sprintf("Chain of %d guardrails", len(gc.Guardrails))
}

// RunGuardrails executes a list of guardrails on data
func RunGuardrails(ctx context.Context, guardrails []Guardrail, data interface{}) error {
	for _, gr := range guardrails {
		if err := gr.Check(ctx, data); err != nil {
			return fmt.Errorf("guardrail '%s' failed: %w", gr.GetName(), err)
		}
	}
	return nil
}
