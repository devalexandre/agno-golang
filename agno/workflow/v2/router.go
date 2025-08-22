package v2

import (
	"context"
	"fmt"
	"strings"
)

// RouterFunc represents a function that determines which route to take
type RouterFunc func(*StepInput) string

// Router represents a routing construct that directs execution flow
type Router struct {
	Name        string
	Description string

	// The routing function that returns the route name
	RouteFunc RouterFunc

	// Map of route names to steps
	Routes map[string][]interface{}

	// Default route if no match is found (optional)
	DefaultRoute []interface{}

	// Configuration
	CaseSensitive     bool // Whether route names are case-sensitive
	AllowPartialMatch bool // Allow partial string matching for route names

	// Internal state
	selectedRoute string
	executedRoute string
}

// NewRouter creates a new Router instance
func NewRouter(options ...RouterOption) *Router {
	r := &Router{
		Routes:        make(map[string][]interface{}),
		CaseSensitive: true,
	}

	for _, opt := range options {
		opt(r)
	}

	return r
}

// RouterOption is a functional option for configuring a Router
type RouterOption func(*Router)

// WithRouterName sets the router name
func WithRouterName(name string) RouterOption {
	return func(r *Router) {
		r.Name = name
	}
}

// WithRouterDescription sets the router description
func WithRouterDescription(desc string) RouterOption {
	return func(r *Router) {
		r.Description = desc
	}
}

// WithRouteFunc sets the routing function
func WithRouteFunc(fn RouterFunc) RouterOption {
	return func(r *Router) {
		r.RouteFunc = fn
	}
}

// WithRoute adds a route with its associated steps
func WithRoute(routeName string, steps ...interface{}) RouterOption {
	return func(r *Router) {
		r.Routes[routeName] = steps
	}
}

// WithRoutes sets multiple routes at once
func WithRoutes(routes map[string][]interface{}) RouterOption {
	return func(r *Router) {
		for name, steps := range routes {
			r.Routes[name] = steps
		}
	}
}

// WithDefaultRoute sets the default route steps
func WithDefaultRoute(steps ...interface{}) RouterOption {
	return func(r *Router) {
		r.DefaultRoute = steps
	}
}

// WithCaseSensitive sets whether route matching is case-sensitive
func WithCaseSensitive(caseSensitive bool) RouterOption {
	return func(r *Router) {
		r.CaseSensitive = caseSensitive
	}
}

// WithPartialMatch enables partial route name matching
func WithPartialMatch(allow bool) RouterOption {
	return func(r *Router) {
		r.AllowPartialMatch = allow
	}
}

// Execute evaluates the routing function and executes the selected route
func (r *Router) Execute(ctx context.Context, input *StepInput) (*StepOutput, error) {
	if r.RouteFunc == nil {
		return nil, fmt.Errorf("router '%s' has no routing function", r.Name)
	}

	// Determine the route
	r.selectedRoute = r.RouteFunc(input)

	// Find matching route
	stepsToExecute, routeName := r.findRoute(r.selectedRoute)

	if stepsToExecute == nil {
		// Use default route if available
		if r.DefaultRoute != nil && len(r.DefaultRoute) > 0 {
			stepsToExecute = r.DefaultRoute
			routeName = "default"
		} else {
			return &StepOutput{
				StepName:     r.Name,
				ExecutorType: "router",
				Event:        string(RouterExecutionCompletedEvent),
				Metadata: map[string]interface{}{
					"selected_route": r.selectedRoute,
					"message":        fmt.Sprintf("no route found for '%s' and no default route defined", r.selectedRoute),
				},
			}, nil
		}
	}

	r.executedRoute = routeName

	// Execute the selected route
	var lastOutput *StepOutput
	stepInput := &StepInput{
		Message:             input.Message,
		PreviousStepContent: input.PreviousStepContent,
		AdditionalData:      input.AdditionalData,
		Images:              input.Images,
		Videos:              input.Videos,
		Audio:               input.Audio,
		PreviousStepOutputs: make(map[string]*StepOutput),
	}

	// Copy previous step outputs
	if input.PreviousStepOutputs != nil {
		for k, v := range input.PreviousStepOutputs {
			stepInput.PreviousStepOutputs[k] = v
		}
	}

	for i, item := range stepsToExecute {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Update input with previous step output
		if lastOutput != nil {
			stepInput.PreviousStepContent = lastOutput.Content
		}

		var output *StepOutput
		var err error

		switch v := item.(type) {
		case *Step:
			output, err = v.Execute(ctx, stepInput)
		case ExecutorFunc:
			output, err = v(stepInput)
		case func(*StepInput) (*StepOutput, error):
			output, err = v(stepInput)
		case *Loop:
			output, err = v.Execute(ctx, stepInput)
		case *Parallel:
			output, err = v.Execute(ctx, stepInput)
		case *Condition:
			output, err = v.Execute(ctx, stepInput)
		case *Router:
			output, err = v.Execute(ctx, stepInput)
		default:
			return nil, fmt.Errorf("unsupported step type at index %d in router '%s' route '%s': %T", i, r.Name, routeName, v)
		}

		if err != nil {
			return nil, fmt.Errorf("router '%s' route '%s' step %d failed: %w", r.Name, routeName, i, err)
		}

		if output != nil {
			stepName := fmt.Sprintf("%s_%s_step_%d", r.Name, routeName, i)
			if output.StepName != "" {
				stepName = fmt.Sprintf("%s_%s", output.StepName, routeName)
			}

			stepInput.PreviousStepOutputs[stepName] = output
			lastOutput = output
		}
	}

	// Create final output
	finalOutput := &StepOutput{
		StepName:     r.Name,
		ExecutorType: "router",
		Event:        string(RouterExecutionCompletedEvent),
		NextStep:     r.executedRoute,
		Metadata: map[string]interface{}{
			"selected_route": r.selectedRoute,
			"executed_route": r.executedRoute,
			"steps_executed": len(stepsToExecute),
		},
	}

	if lastOutput != nil {
		finalOutput.Content = lastOutput.Content
	}

	return finalOutput, nil
}

// findRoute finds the matching route based on the route name
func (r *Router) findRoute(routeName string) ([]interface{}, string) {
	// Direct match
	if steps, exists := r.Routes[routeName]; exists {
		return steps, routeName
	}

	// Case-insensitive match if configured
	if !r.CaseSensitive {
		lowerRouteName := strings.ToLower(routeName)
		for name, steps := range r.Routes {
			if strings.ToLower(name) == lowerRouteName {
				return steps, name
			}
		}
	}

	// Partial match if configured
	if r.AllowPartialMatch {
		for name, steps := range r.Routes {
			if r.CaseSensitive {
				if strings.Contains(name, routeName) || strings.Contains(routeName, name) {
					return steps, name
				}
			} else {
				lowerName := strings.ToLower(name)
				lowerRouteName := strings.ToLower(routeName)
				if strings.Contains(lowerName, lowerRouteName) || strings.Contains(lowerRouteName, lowerName) {
					return steps, name
				}
			}
		}
	}

	return nil, ""
}

// AddRoute adds a new route to the router
func (r *Router) AddRoute(name string, steps ...interface{}) {
	r.Routes[name] = steps
}

// Common routing functions

// RouteByContent creates a routing function based on content
func RouteByContent() RouterFunc {
	return func(input *StepInput) string {
		if input.PreviousStepContent == nil {
			return "no_content"
		}

		contentStr, ok := input.PreviousStepContent.(string)
		if !ok {
			return "non_string_content"
		}

		// Simple content-based routing logic
		lowerContent := strings.ToLower(contentStr)

		if strings.Contains(lowerContent, "error") {
			return "error"
		} else if strings.Contains(lowerContent, "success") {
			return "success"
		} else if strings.Contains(lowerContent, "warning") {
			return "warning"
		}

		return "default"
	}
}

// RouteByMetadata creates a routing function based on metadata
func RouteByMetadata(key string) RouterFunc {
	return func(input *StepInput) string {
		if input.AdditionalData == nil {
			return "no_metadata"
		}

		value, exists := input.AdditionalData[key]
		if !exists {
			return "key_not_found"
		}

		// Convert value to string for routing
		return fmt.Sprintf("%v", value)
	}
}

// RouteByStepOutput creates a routing function based on a specific step's output
func RouteByStepOutput(stepName string, field string) RouterFunc {
	return func(input *StepInput) string {
		output := input.GetStepOutput(stepName)
		if output == nil {
			return "step_not_found"
		}

		if output.Metadata != nil {
			if value, exists := output.Metadata[field]; exists {
				return fmt.Sprintf("%v", value)
			}
		}

		return "field_not_found"
	}
}

// RouteByFunction creates a custom routing function
func RouteByFunction(fn func(*StepInput) string) RouterFunc {
	return fn
}

// RouteByContentType creates a routing function based on content type
func RouteByContentType() RouterFunc {
	return func(input *StepInput) string {
		if input.PreviousStepContent == nil {
			return "nil"
		}

		switch input.PreviousStepContent.(type) {
		case string:
			return "string"
		case int, int32, int64, float32, float64:
			return "number"
		case bool:
			return "boolean"
		case map[string]interface{}:
			return "object"
		case []interface{}:
			return "array"
		default:
			return "unknown"
		}
	}
}

// RouteByMessageType creates a routing function based on message type
func RouteByMessageType() RouterFunc {
	return func(input *StepInput) string {
		if input.Message == nil {
			return "no_message"
		}

		switch input.Message.(type) {
		case string:
			return "text"
		case map[string]interface{}:
			return "structured"
		case []interface{}:
			return "list"
		default:
			return "unknown"
		}
	}
}

// SimpleRouter creates a simple router with predefined routes
func SimpleRouter(routeFunc RouterFunc, routes map[string][]interface{}) *Router {
	return NewRouter(
		WithRouteFunc(routeFunc),
		WithRoutes(routes),
	)
}

// SwitchRouter creates a switch-like router with a default case
func SwitchRouter(routeFunc RouterFunc, routes map[string][]interface{}, defaultSteps ...interface{}) *Router {
	return NewRouter(
		WithRouteFunc(routeFunc),
		WithRoutes(routes),
		WithDefaultRoute(defaultSteps...),
	)
}
