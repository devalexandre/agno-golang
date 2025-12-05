package agent

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
)

// DependencyResolver defines a function that can resolve a dependency value
type DependencyResolver func() (interface{}, error)

// DependencyManager manages application dependencies and their resolution
type DependencyManager struct {
	dependencies map[string]interface{}
	resolvers    map[string]DependencyResolver
	cache        map[string]interface{}
	mu           sync.RWMutex
}

// NewDependencyManager creates a new dependency manager
func NewDependencyManager() *DependencyManager {
	return &DependencyManager{
		dependencies: make(map[string]interface{}),
		resolvers:    make(map[string]DependencyResolver),
		cache:        make(map[string]interface{}),
	}
}

// SetDependency sets a dependency value directly
// This is used for concrete values like database connections, config objects, etc.
func (dm *DependencyManager) SetDependency(name string, value interface{}) error {
	if name == "" {
		return fmt.Errorf("dependency name cannot be empty")
	}

	if value == nil {
		return fmt.Errorf("dependency value cannot be nil")
	}

	dm.mu.Lock()
	defer dm.mu.Unlock()

	dm.dependencies[name] = value
	// Clear cache for this dependency when setting new value
	delete(dm.cache, name)

	return nil
}

// GetDependency retrieves a dependency value by name
// It first checks if the value is cached, then checks direct dependencies,
// then tries to resolve using a resolver if available
func (dm *DependencyManager) GetDependency(name string) (interface{}, error) {
	if name == "" {
		return nil, fmt.Errorf("dependency name cannot be empty")
	}

	dm.mu.RLock()
	// Check cache first
	if cached, ok := dm.cache[name]; ok {
		dm.mu.RUnlock()
		return cached, nil
	}

	// Check direct dependencies
	if dep, ok := dm.dependencies[name]; ok {
		dm.mu.RUnlock()
		return dep, nil
	}

	// Check if resolver exists
	resolver, ok := dm.resolvers[name]
	dm.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("dependency '%s' not found", name)
	}

	// Resolve using resolver function
	value, err := resolver()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve dependency '%s': %w", name, err)
	}

	// Cache the resolved value
	dm.mu.Lock()
	dm.cache[name] = value
	dm.mu.Unlock()

	return value, nil
}

// RegisterResolver registers a resolver function for a dependency
// The resolver function will be called when the dependency is requested
// and it's not in the cache or direct dependencies
func (dm *DependencyManager) RegisterResolver(name string, resolver DependencyResolver) error {
	if name == "" {
		return fmt.Errorf("dependency name cannot be empty")
	}

	if resolver == nil {
		return fmt.Errorf("resolver function cannot be nil")
	}

	dm.mu.Lock()
	defer dm.mu.Unlock()

	dm.resolvers[name] = resolver
	// Clear cache for this dependency when registering new resolver
	delete(dm.cache, name)

	return nil
}

// DeleteDependency removes a dependency by name
func (dm *DependencyManager) DeleteDependency(name string) error {
	if name == "" {
		return fmt.Errorf("dependency name cannot be empty")
	}

	dm.mu.Lock()
	defer dm.mu.Unlock()

	delete(dm.dependencies, name)
	delete(dm.resolvers, name)
	delete(dm.cache, name)

	return nil
}

// ClearDependencies removes all dependencies
func (dm *DependencyManager) ClearDependencies() {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	dm.dependencies = make(map[string]interface{})
	dm.resolvers = make(map[string]DependencyResolver)
	dm.cache = make(map[string]interface{})
}

// ClearCache clears the cached dependency values (but keeps the dependencies themselves)
func (dm *DependencyManager) ClearCache() {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	dm.cache = make(map[string]interface{})
}

// HasDependency checks if a dependency exists
func (dm *DependencyManager) HasDependency(name string) bool {
	if name == "" {
		return false
	}

	dm.mu.RLock()
	defer dm.mu.RUnlock()

	_, inDeps := dm.dependencies[name]
	_, inResolvers := dm.resolvers[name]

	return inDeps || inResolvers
}

// GetAllDependencies returns a map of all dependencies
func (dm *DependencyManager) GetAllDependencies() map[string]interface{} {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	// Create a copy
	result := make(map[string]interface{})
	for k, v := range dm.dependencies {
		result[k] = v
	}

	return result
}

// ResolveDependencies processes a template string and resolves any dependencies
// Template format: "The database is {db_connection} and cache is {cache}"
// It will replace {dependency_name} with the resolved value
func (dm *DependencyManager) ResolveDependencies(template string) (string, error) {
	if template == "" {
		return "", nil
	}

	result := template
	// Find all {name} patterns
	for i := 0; i < len(result); i++ {
		if result[i] == '{' {
			// Find closing bracket
			j := strings.Index(result[i:], "}")
			if j == -1 {
				break
			}
			j = i + j

			// Extract dependency name
			depName := result[i+1 : j]

			// Try to resolve
			value, err := dm.GetDependency(depName)
			if err != nil {
				return "", err
			}

			// Convert to string
			valueStr := fmt.Sprintf("%v", value)

			// Replace in template
			result = result[:i] + valueStr + result[j+1:]
			i += len(valueStr) - 1
		}
	}

	return result, nil
}

// InjectDependencies injects dependencies into a struct's fields
// It looks for struct tags "inject" and injects matching dependencies
// Example:
//
//	type MyService struct {
//	    DB    *sql.DB `inject:"db_connection"`
//	    Cache *redis.Client `inject:"cache"`
//	}
func (dm *DependencyManager) InjectDependencies(target interface{}) error {
	if target == nil {
		return fmt.Errorf("target cannot be nil")
	}

	targetValue := reflect.ValueOf(target)
	if targetValue.Kind() != reflect.Ptr {
		return fmt.Errorf("target must be a pointer")
	}

	targetValue = targetValue.Elem()
	if targetValue.Kind() != reflect.Struct {
		return fmt.Errorf("target must be a struct")
	}

	targetType := targetValue.Type()

	for i := 0; i < targetType.NumField(); i++ {
		field := targetType.Field(i)
		fieldValue := targetValue.Field(i)

		// Check for "inject" tag
		if tag, ok := field.Tag.Lookup("inject"); ok {
			// Get the dependency
			dep, err := dm.GetDependency(tag)
			if err != nil {
				return fmt.Errorf("failed to inject field %s: %w", field.Name, err)
			}

			// Check if field is settable
			if !fieldValue.CanSet() {
				return fmt.Errorf("field %s is not settable", field.Name)
			}

			// Try to set the dependency
			depValue := reflect.ValueOf(dep)
			if !depValue.Type().AssignableTo(fieldValue.Type()) {
				return fmt.Errorf("dependency %s is not assignable to field %s (expected %s, got %s)",
					tag, field.Name, fieldValue.Type(), depValue.Type())
			}

			fieldValue.Set(depValue)
		}
	}

	return nil
}

// MergeDependencies merges another dependency manager's dependencies into this one
// Dependencies from the other manager will be added/overwritten
func (dm *DependencyManager) MergeDependencies(other *DependencyManager) error {
	if other == nil {
		return fmt.Errorf("other dependency manager cannot be nil")
	}

	other.mu.RLock()
	otherDeps := make(map[string]interface{})
	for k, v := range other.dependencies {
		otherDeps[k] = v
	}
	other.mu.RUnlock()

	dm.mu.Lock()
	defer dm.mu.Unlock()

	for k, v := range otherDeps {
		dm.dependencies[k] = v
		// Clear cache for this dependency
		delete(dm.cache, k)
	}

	return nil
}

// ToMap converts the dependency manager to a map for serialization/context building
// It resolves all dependencies that are simple types (not interfaces or pointers to unknown types)
func (dm *DependencyManager) ToMap() map[string]interface{} {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	result := make(map[string]interface{})

	for k, v := range dm.dependencies {
		// Try to safely convert to string representation
		result[k] = dm.safeToString(v)
	}

	return result
}

// safeToString safely converts a value to a string representation
func (dm *DependencyManager) safeToString(v interface{}) interface{} {
	if v == nil {
		return nil
	}

	switch val := v.(type) {
	case string:
		return val
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return val
	case bool:
		return val
	default:
		// For complex types, return string representation
		return fmt.Sprintf("%v", val)
	}
}
