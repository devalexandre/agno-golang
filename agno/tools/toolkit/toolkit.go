package toolkit

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
)

// NewToolkit initializes a new empty Toolkit.
func NewToolkit() Toolkit {
	return Toolkit{
		methods: make(map[string]Method),
		cache:   &sync.Map{},
	}
}

// GetName returns the toolkit name.
func (tk *Toolkit) GetName() string {
	return tk.Name
}

// GetDescription returns the toolkit description.
func (tk *Toolkit) GetDescription() string {
	return tk.Description
}

// --- Hook Management ---

// AddPreHook adds a pre-execution hook to the toolkit.
// PreHooks run before every method execution. Returning an error aborts the call.
func (tk *Toolkit) AddPreHook(hook HookFunc) {
	tk.preHooks = append(tk.preHooks, hook)
}

// AddPostHook adds a post-execution hook to the toolkit.
// PostHooks run after every method execution with the result and error.
func (tk *Toolkit) AddPostHook(hook PostHookFunc) {
	tk.postHooks = append(tk.postHooks, hook)
}

// --- Tool Filtering ---

// IncludeTools restricts GetMethods to only return the named methods.
// Method names should be without the toolkit prefix (e.g., "Search" not "MyTool_Search").
func (tk *Toolkit) IncludeTools(names ...string) {
	tk.includedTools = make(map[string]bool, len(names))
	for _, n := range names {
		tk.includedTools[tk.Name+"_"+n] = true
	}
}

// ExcludeTools hides the named methods from GetMethods.
// Method names should be without the toolkit prefix.
func (tk *Toolkit) ExcludeTools(names ...string) {
	tk.excludedTools = make(map[string]bool, len(names))
	for _, n := range names {
		tk.excludedTools[tk.Name+"_"+n] = true
	}
}

// --- Registration ---

// Register registers a method in the toolkit.
// methodName = Function name
// fn = Execution function
// paramExample = Example struct that represents the parameters for schema generation
func (tk *Toolkit) Register(methodName, description string, receiver interface{}, fn interface{}, paramExample interface{}) {
	if _, ok := tk.methods[methodName]; ok {
		panic(fmt.Sprintf("Register: method %s already registered", methodName))
	}

	if methodName == "" {
		panic("Register: methodName cannot be empty")
	}

	if description == "" {
		panic("Register: description cannot be empty")
	}

	if receiver == nil {
		panic("Register: receiver cannot be nil")
	}

	if fn == nil {
		panic("Register: fn cannot be nil")
	}

	funcValue := reflect.ValueOf(fn)
	funcType := funcValue.Type()

	if funcType.Kind() != reflect.Func {
		panic("Register expects a function")
	}

	// Generate schema based on the provided struct
	paramType := reflect.TypeOf(paramExample)
	if paramType.Kind() == reflect.Ptr {
		paramType = paramType.Elem()
	}
	if paramType.Kind() != reflect.Struct {
		panic(fmt.Sprintf("Register: paramExample must be a struct, got %v", paramType.Kind()))
	}

	schema := GenerateSchemaFromType(paramType)

	fullMethodName := tk.Name + "_" + methodName

	tk.methods[fullMethodName] = Method{
		Receiver:    receiver,
		Description: description,
		Function:    fn,
		Schema:      schema,
		ParamType:   paramType,
	}
}

// MethodOption configures optional properties on a registered method.
type MethodOption func(*Method)

// WithConfirmation marks the method as requiring user confirmation before execution.
func WithConfirmation() MethodOption {
	return func(m *Method) {
		m.RequiresConfirmation = true
	}
}

// WithStopAfterCall tells the agent to stop the tool-calling loop after this method.
func WithStopAfterCall() MethodOption {
	return func(m *Method) {
		m.StopAfterCall = true
	}
}

// WithMethodPreHook adds a pre-execution hook specific to this method.
func WithMethodPreHook(hook HookFunc) MethodOption {
	return func(m *Method) {
		m.PreHooks = append(m.PreHooks, hook)
	}
}

// WithMethodPostHook adds a post-execution hook specific to this method.
func WithMethodPostHook(hook PostHookFunc) MethodOption {
	return func(m *Method) {
		m.PostHooks = append(m.PostHooks, hook)
	}
}

// RegisterWithOptions registers a method with additional options (hooks, confirmation, etc.).
func (tk *Toolkit) RegisterWithOptions(methodName, description string, receiver interface{}, fn interface{}, paramExample interface{}, opts ...MethodOption) {
	tk.Register(methodName, description, receiver, fn, paramExample)

	fullMethodName := tk.Name + "_" + methodName
	method := tk.methods[fullMethodName]
	for _, opt := range opts {
		opt(&method)
	}
	tk.methods[fullMethodName] = method
}

// --- Querying ---

// GetMethods returns all methods registered in the toolkit, respecting include/exclude filters.
func (tk *Toolkit) GetMethods() map[string]Method {
	if tk.includedTools == nil && tk.excludedTools == nil {
		return tk.methods
	}

	filtered := make(map[string]Method, len(tk.methods))
	for name, method := range tk.methods {
		if tk.includedTools != nil && !tk.includedTools[name] {
			continue
		}
		if tk.excludedTools != nil && tk.excludedTools[name] {
			continue
		}
		filtered[name] = method
	}
	return filtered
}

// GetDescriptionOfMethod returns the description of a specific registered method.
func (tk *Toolkit) GetDescriptionOfMethod(methodName string) string {
	method, ok := tk.methods[methodName]
	if !ok {
		panic(fmt.Sprintf("GetDescriptionOfMethod: method %s not found", methodName))
	}
	return method.Description
}

// GetFunction returns the execution function associated with a registered method.
func (tk *Toolkit) GetFunction(methodName string) interface{} {
	method, ok := tk.methods[tk.Name+"_"+methodName]
	if !ok {
		panic(fmt.Sprintf("GetFunction: method %s not found", methodName))
	}
	return method.Function
}

// GetParameterStruct returns the JSON schema of parameters for the registered method.
func (tk *Toolkit) GetParameterStruct(methodName string) map[string]interface{} {
	method, ok := tk.methods[methodName]
	if !ok {
		panic(fmt.Sprintf("GetParameterStruct: method %s not found", methodName))
	}
	return method.Schema
}

// --- Caching helpers ---

func (tk *Toolkit) cacheKey(methodName string, input json.RawMessage) string {
	h := md5.Sum(append([]byte(methodName), input...))
	return fmt.Sprintf("%x", h)
}

func (tk *Toolkit) ensureCache() {
	if tk.cache == nil {
		tk.cache = &sync.Map{}
	}
}

func (tk *Toolkit) getCached(key string) (interface{}, error, bool) {
	tk.ensureCache()
	val, ok := tk.cache.Load(key)
	if !ok {
		return nil, nil, false
	}
	entry := val.(cacheEntry)
	if time.Now().After(entry.expiresAt) {
		tk.cache.Delete(key)
		return nil, nil, false
	}
	return entry.result, entry.err, true
}

func (tk *Toolkit) setCache(key string, result interface{}, err error) {
	tk.ensureCache()
	tk.cache.Store(key, cacheEntry{
		result:    result,
		err:       err,
		expiresAt: time.Now().Add(tk.Cache.TTL),
	})
}

// ClearCache removes all cached results.
func (tk *Toolkit) ClearCache() {
	tk.ensureCache()
	tk.cache.Range(func(key, _ interface{}) bool {
		tk.cache.Delete(key)
		return true
	})
}

// --- Execution ---

// Execute runs the function associated with a method, passing the JSON input.
// It runs pre-hooks, checks cache, executes the function, updates cache, and runs post-hooks.
func (tk *Toolkit) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	method, ok := tk.methods[methodName]
	if !ok {
		return nil, fmt.Errorf("Execute: method %s not found", methodName)
	}

	// Run toolkit-level pre-hooks
	for _, hook := range tk.preHooks {
		if err := hook(methodName, input); err != nil {
			return nil, fmt.Errorf("Execute: pre-hook error: %w", err)
		}
	}

	// Run method-level pre-hooks
	for _, hook := range method.PreHooks {
		if err := hook(methodName, input); err != nil {
			return nil, fmt.Errorf("Execute: method pre-hook error: %w", err)
		}
	}

	// Check cache
	if tk.Cache.Enabled {
		key := tk.cacheKey(methodName, input)
		if result, err, found := tk.getCached(key); found {
			return result, err
		}
	}

	// Parse JSON to intermediate map
	var argsMap map[string]interface{}
	if err := json.Unmarshal(input, &argsMap); err != nil {
		return nil, fmt.Errorf("Execute: failed to parse input JSON: %w", err)
	}

	// Fix common types that come as strings
	for i := 0; i < method.ParamType.NumField(); i++ {
		field := method.ParamType.Field(i)
		jsonName := field.Tag.Get("json")
		if jsonName == "" {
			jsonName = strings.ToLower(field.Name)
		} else {
			jsonName = strings.Split(jsonName, ",")[0]
		}

		val, exists := argsMap[jsonName]
		if !exists {
			continue
		}

		switch field.Type.Kind() {
		case reflect.Float64:
			if strVal, ok := val.(string); ok {
				if f, err := strconv.ParseFloat(strVal, 64); err == nil {
					argsMap[jsonName] = f
				}
			}
		case reflect.Bool:
			if strVal, ok := val.(string); ok {
				if b, err := strconv.ParseBool(strVal); err == nil {
					argsMap[jsonName] = b
				}
			}
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if strVal, ok := val.(string); ok {
				if i, err := strconv.ParseInt(strVal, 10, 64); err == nil {
					argsMap[jsonName] = i
				}
			}
		}
	}

	// Re-marshal corrected data
	cleanJSON, err := json.Marshal(argsMap)
	if err != nil {
		return nil, fmt.Errorf("Execute: failed to marshal corrected input: %w", err)
	}

	paramInstance := reflect.New(method.ParamType).Interface()
	if err := json.Unmarshal(cleanJSON, paramInstance); err != nil {
		return nil, fmt.Errorf("Execute: failed to unmarshal corrected input: %w", err)
	}

	args := []reflect.Value{
		reflect.ValueOf(paramInstance).Elem(),
	}
	resultValues := reflect.ValueOf(method.Function).Call(args)

	result := resultValues[0].Interface()
	var errResult error
	if !resultValues[1].IsNil() {
		errResult = resultValues[1].Interface().(error)
	}

	// Store in cache
	if tk.Cache.Enabled {
		key := tk.cacheKey(methodName, input)
		tk.setCache(key, result, errResult)
	}

	// Run method-level post-hooks
	for _, hook := range method.PostHooks {
		hook(methodName, input, result, errResult)
	}

	// Run toolkit-level post-hooks
	for _, hook := range tk.postHooks {
		hook(methodName, input, result, errResult)
	}

	return result, errResult
}

// --- Schema Generation ---

// GenerateSchemaFromType generates a JSON Schema based on the provided type.
func GenerateSchemaFromType(paramType reflect.Type) map[string]interface{} {
	schema := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
		"required":   []string{},
	}

	properties := schema["properties"].(map[string]interface{})
	requiredFields := schema["required"].([]string)

	for i := 0; i < paramType.NumField(); i++ {
		field := paramType.Field(i)

		// Get field name from json tag
		fieldName := field.Tag.Get("json")
		if fieldName == "" || fieldName == "-" {
			fieldName = strings.ToLower(field.Name)
		} else {
			fieldName = strings.Split(fieldName, ",")[0] // remove ,omitempty
		}

		if fieldName == "" {
			continue
		}

		// Map to JSON Schema type
		typeStr := mapGoTypeToJSONType(field.Type.Kind())

		// Tag description
		description := field.Tag.Get("description")

		prop := map[string]interface{}{
			"type":        typeStr,
			"description": description,
		}
		// If it's an array or slice, define items automatically
		if field.Type.Kind() == reflect.Slice || field.Type.Kind() == reflect.Array {
			elemType := field.Type.Elem().Kind()
			prop["items"] = map[string]interface{}{
				"type": mapGoTypeToJSONType(elemType),
			}
		}

		properties[fieldName] = prop

		// If the tag is required, add it
		if field.Tag.Get("required") == "true" {
			requiredFields = append(requiredFields, fieldName)
		}
	}

	// Update required fields
	schema["required"] = requiredFields

	return schema
}

// mapGoTypeToJSONType converts Go types to JSON Schema types.
func mapGoTypeToJSONType(kind reflect.Kind) string {
	switch kind {
	case reflect.String:
		return "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Bool:
		return "boolean"
	case reflect.Slice, reflect.Array:
		return "array"
	case reflect.Map, reflect.Struct:
		return "object"
	default:
		return "string"
	}
}
