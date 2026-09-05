// Package guardrails provides reusable validation and policy rules that can be
// applied to the input of a run, the output of a run, or to tool calls.
//
// A guardrail is any type implementing the Guardrail interface. Guardrails are
// data agnostic: they receive the value being validated and decide whether it is
// allowed. Guardrails that need the textual representation of the value use
// ExtractText, which understands plain strings, Input/Output envelopes, and
// arbitrary structs exposing a textual field.
package guardrails

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
)

// Guardrail represents a reusable validation/policy rule.
type Guardrail interface {
	// Check validates the data according to the guardrail policy.
	// It returns an error when validation fails.
	Check(ctx context.Context, data interface{}) error
	// GetName returns the guardrail name for identification.
	GetName() string
	// GetDescription returns a human-readable description.
	GetDescription() string
}

// InputChecker is implemented by guardrails that can validate (and possibly
// rewrite) the input of a run.
type InputChecker interface {
	CheckInput(ctx context.Context, input *Input) error
}

// OutputChecker is implemented by guardrails that can validate (and possibly
// rewrite) the output of a run.
type OutputChecker interface {
	CheckOutput(ctx context.Context, output *Output) error
}

// TextProvider is implemented by values that expose the text a guardrail should
// inspect.
type TextProvider interface {
	GuardrailText() string
}

// Input is the mutable envelope carrying the content of a run input.
// Guardrails running in redact/mask mode rewrite Content in place.
type Input struct {
	// Content is the textual content of the input.
	Content string
	// Metadata carries arbitrary information about the input.
	Metadata map[string]interface{}
}

// NewInput creates an Input from a string.
func NewInput(content string) *Input {
	return &Input{Content: content}
}

// GuardrailText returns the content inspected by guardrails.
func (i *Input) GuardrailText() string {
	if i == nil {
		return ""
	}
	return i.Content
}

// SetText replaces the content of the input.
func (i *Input) SetText(content string) {
	i.Content = content
}

// Output is the mutable envelope carrying the content of a run output.
type Output struct {
	// Content is the textual content of the output.
	Content string
	// Metadata carries arbitrary information about the output.
	Metadata map[string]interface{}
}

// NewOutput creates an Output from a string.
func NewOutput(content string) *Output {
	return &Output{Content: content}
}

// GuardrailText returns the content inspected by guardrails.
func (o *Output) GuardrailText() string {
	if o == nil {
		return ""
	}
	return o.Content
}

// SetText replaces the content of the output.
func (o *Output) SetText(content string) {
	o.Content = content
}

// ===== CHECK ERRORS =====

// CheckTrigger identifies why a guardrail rejected the data.
type CheckTrigger string

const (
	// TriggerOffTopic is used when the content is outside the allowed domain.
	TriggerOffTopic CheckTrigger = "off_topic"
	// TriggerInputNotAllowed is the default trigger for input checks.
	TriggerInputNotAllowed CheckTrigger = "input_not_allowed"
	// TriggerOutputNotAllowed is the default trigger for output checks.
	TriggerOutputNotAllowed CheckTrigger = "output_not_allowed"
	// TriggerValidationFailed is used when a generic validation fails.
	TriggerValidationFailed CheckTrigger = "validation_failed"
	// TriggerPromptInjection is used when a prompt injection attempt is detected.
	TriggerPromptInjection CheckTrigger = "prompt_injection"
	// TriggerPIIDetected is used when personally identifiable information is detected.
	TriggerPIIDetected CheckTrigger = "pii_detected"
	// TriggerModerationViolation is used when a moderation provider flags the content.
	TriggerModerationViolation CheckTrigger = "moderation_violation"
	// TriggerNSFW is used when not-safe-for-work content is detected.
	TriggerNSFW CheckTrigger = "nsfw_detected"
	// TriggerRateLimited is used when a rate limit is exceeded.
	TriggerRateLimited CheckTrigger = "rate_limited"
	// TriggerLoopDetected is used when a run exceeds its iteration budget.
	TriggerLoopDetected CheckTrigger = "loop_detected"
)

// Stage identifies which side of the run a check error belongs to.
type Stage string

const (
	// StageInput marks errors raised while checking input.
	StageInput Stage = "input_check_error"
	// StageOutput marks errors raised while checking output.
	StageOutput Stage = "output_check_error"
)

// CheckError is returned when a guardrail rejects the data it inspected.
type CheckError struct {
	// Stage tells whether the failure happened on input or output.
	Stage Stage
	// Trigger identifies the reason for the failure.
	Trigger CheckTrigger
	// Message is the human-readable reason of the failure.
	Message string
	// Guardrail is the name of the guardrail that raised the error.
	Guardrail string
	// AdditionalData carries extra details about the failure.
	AdditionalData map[string]interface{}
}

// NewInputCheckError creates a CheckError for an input check failure.
func NewInputCheckError(message string, trigger CheckTrigger, additionalData map[string]interface{}) *CheckError {
	if trigger == "" {
		trigger = TriggerInputNotAllowed
	}
	return &CheckError{Stage: StageInput, Trigger: trigger, Message: message, AdditionalData: additionalData}
}

// NewOutputCheckError creates a CheckError for an output check failure.
func NewOutputCheckError(message string, trigger CheckTrigger, additionalData map[string]interface{}) *CheckError {
	if trigger == "" {
		trigger = TriggerOutputNotAllowed
	}
	return &CheckError{Stage: StageOutput, Trigger: trigger, Message: message, AdditionalData: additionalData}
}

// Error implements the error interface.
func (e *CheckError) Error() string {
	if e.Guardrail != "" {
		return fmt.Sprintf("%s: %s", e.Guardrail, e.Message)
	}
	return e.Message
}

// ErrorID returns the identifier of the trigger that caused the failure.
func (e *CheckError) ErrorID() string {
	return string(e.Trigger)
}

// WithGuardrail returns a copy of the error attributed to the given guardrail.
func (e *CheckError) WithGuardrail(name string) *CheckError {
	clone := *e
	clone.Guardrail = name
	return &clone
}

// AsCheckError extracts a *CheckError from an error chain.
func AsCheckError(err error) (*CheckError, bool) {
	var checkErr *CheckError
	if errors.As(err, &checkErr) {
		return checkErr, true
	}
	return nil, false
}

// IsTrigger reports whether the error chain contains a CheckError with the given trigger.
func IsTrigger(err error, trigger CheckTrigger) bool {
	checkErr, ok := AsCheckError(err)
	return ok && checkErr.Trigger == trigger
}

// ===== COMPOSITION =====

// Func is a simple function-based guardrail implementation.
type Func struct {
	Name        string
	Description string
	CheckFunc   func(ctx context.Context, data interface{}) error
}

// Check runs the wrapped function.
func (g *Func) Check(ctx context.Context, data interface{}) error {
	if g.CheckFunc == nil {
		return nil
	}
	return g.CheckFunc(ctx, data)
}

// GetName returns the guardrail name.
func (g *Func) GetName() string { return g.Name }

// GetDescription returns the guardrail description.
func (g *Func) GetDescription() string { return g.Description }

// NewFunc creates a function-based guardrail.
func NewFunc(name, description string, check func(ctx context.Context, data interface{}) error) *Func {
	return &Func{Name: name, Description: description, CheckFunc: check}
}

// Chain executes multiple guardrails in sequence, stopping at the first failure.
type Chain struct {
	Name       string
	Guardrails []Guardrail
}

// NewChain creates a chain of guardrails.
func NewChain(name string, gs ...Guardrail) *Chain {
	return &Chain{Name: name, Guardrails: gs}
}

// Add appends guardrails to the chain.
func (c *Chain) Add(gs ...Guardrail) *Chain {
	c.Guardrails = append(c.Guardrails, gs...)
	return c
}

// Check runs every guardrail in the chain.
func (c *Chain) Check(ctx context.Context, data interface{}) error {
	for _, g := range c.Guardrails {
		if err := g.Check(ctx, data); err != nil {
			return decorate(g.GetName(), err)
		}
	}
	return nil
}

// CheckInput runs every guardrail in the chain against the input, giving
// guardrails that implement InputChecker the chance to rewrite the content.
func (c *Chain) CheckInput(ctx context.Context, input *Input) error {
	return RunInput(ctx, c.Guardrails, input)
}

// CheckOutput runs every guardrail in the chain against the output.
func (c *Chain) CheckOutput(ctx context.Context, output *Output) error {
	return RunOutput(ctx, c.Guardrails, output)
}

// GetName returns the chain name.
func (c *Chain) GetName() string { return c.Name }

// GetDescription returns a description of the chain.
func (c *Chain) GetDescription() string {
	return fmt.Sprintf("Chain of %d guardrails", len(c.Guardrails))
}

// decorate attributes an error to the guardrail that raised it. A CheckError
// already carries its stage, trigger and details, so it is returned as is, with
// the guardrail name filled in when it is missing. Any other error is wrapped
// with the name of the guardrail that produced it.
func decorate(name string, err error) error {
	if err == nil {
		return nil
	}
	if checkErr, ok := AsCheckError(err); ok {
		if checkErr.Guardrail == "" {
			return checkErr.WithGuardrail(name)
		}
		return err
	}
	return fmt.Errorf("guardrail '%s' failed: %w", name, err)
}

// Run executes a list of guardrails against arbitrary data.
func Run(ctx context.Context, gs []Guardrail, data interface{}) error {
	for _, g := range gs {
		if err := g.Check(ctx, data); err != nil {
			return decorate(g.GetName(), err)
		}
	}
	return nil
}

// RunInput executes a list of guardrails against an input. Guardrails
// implementing InputChecker are given the mutable input so they can redact or
// mask its content.
func RunInput(ctx context.Context, gs []Guardrail, input *Input) error {
	if input == nil {
		return nil
	}
	for _, g := range gs {
		var err error
		if checker, ok := g.(InputChecker); ok {
			err = checker.CheckInput(ctx, input)
		} else {
			err = g.Check(ctx, input)
		}
		if err != nil {
			return decorate(g.GetName(), err)
		}
	}
	return nil
}

// RunOutput executes a list of guardrails against an output. Guardrails
// implementing OutputChecker are given the mutable output so they can redact or
// mask its content.
func RunOutput(ctx context.Context, gs []Guardrail, output *Output) error {
	if output == nil {
		return nil
	}
	for _, g := range gs {
		var err error
		if checker, ok := g.(OutputChecker); ok {
			err = checker.CheckOutput(ctx, output)
		} else {
			err = g.Check(ctx, output)
		}
		if err != nil {
			return decorate(g.GetName(), err)
		}
	}
	return nil
}

// ===== TEXT EXTRACTION =====

// textFieldNames are the struct fields and map keys inspected when extracting
// text from an arbitrary value.
var textFieldNames = []string{"Content", "Text", "Message", "Output", "Input", "Response"}

// ExtractText returns the textual representation a guardrail should inspect.
// The second return value reports whether text could be extracted at all.
func ExtractText(data interface{}) (string, bool) {
	switch v := data.(type) {
	case nil:
		return "", false
	case string:
		return v, true
	case *string:
		if v == nil {
			return "", false
		}
		return *v, true
	case []byte:
		return string(v), true
	case TextProvider:
		return v.GuardrailText(), true
	case error:
		return v.Error(), true
	case fmt.Stringer:
		return v.String(), true
	case map[string]interface{}:
		return textFromMap(v)
	case map[string]string:
		generic := make(map[string]interface{}, len(v))
		for key, value := range v {
			generic[key] = value
		}
		return textFromMap(generic)
	}
	return textFromValue(reflect.ValueOf(data), 0)
}

func textFromMap(m map[string]interface{}) (string, bool) {
	for _, name := range textFieldNames {
		for _, key := range []string{name, strings.ToLower(name)} {
			if raw, ok := m[key]; ok {
				if text, ok := ExtractText(raw); ok && text != "" {
					return text, true
				}
			}
		}
	}
	return "", false
}

// textFromValue walks pointers, interfaces and structs looking for a textual field.
func textFromValue(v reflect.Value, depth int) (string, bool) {
	if depth > 3 || !v.IsValid() {
		return "", false
	}

	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if v.IsNil() {
			return "", false
		}
		return textFromValue(v.Elem(), depth+1)
	case reflect.String:
		return v.String(), true
	case reflect.Slice, reflect.Array:
		var parts []string
		for i := 0; i < v.Len(); i++ {
			if text, ok := textFromValue(v.Index(i), depth+1); ok && text != "" {
				parts = append(parts, text)
			}
		}
		if len(parts) == 0 {
			return "", false
		}
		return strings.Join(parts, "\n"), true
	case reflect.Struct:
		for _, name := range textFieldNames {
			field := v.FieldByName(name)
			if !field.IsValid() || !field.CanInterface() {
				continue
			}
			if text, ok := textFromValue(field, depth+1); ok && text != "" {
				return text, true
			}
		}
	}
	return "", false
}
