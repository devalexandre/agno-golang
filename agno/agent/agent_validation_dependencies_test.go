package agent

import (
	"testing"
)

// TestInputValidatorRequired tests the required field validation
func TestInputValidatorRequired(t *testing.T) {
	type TestInput struct {
		Name  string `required:"true"`
		Email string `required:"true"`
		Age   int
	}

	validator := NewInputValidator(TestInput{})

	// Test with missing required field
	input := TestInput{
		Name:  "",
		Email: "test@example.com",
	}

	err := validator.ValidateInput(input)
	if err == nil {
		t.Fatal("expected validation error for empty Name")
	}

	// Test with valid input
	input = TestInput{
		Name:  "John",
		Email: "test@example.com",
		Age:   30,
	}

	err = validator.ValidateInput(input)
	if err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

// TestInputValidatorMinMax tests min/max value validation
func TestInputValidatorMinMax(t *testing.T) {
	type TestInput struct {
		Age   int     `min:"0" max:"150"`
		Score float64 `min:"0" max:"100"`
	}

	validator := NewInputValidator(TestInput{})

	// Test with value below min
	input := TestInput{
		Age:   -5,
		Score: 50,
	}

	err := validator.ValidateInput(input)
	if err == nil {
		t.Fatal("expected validation error for Age < 0")
	}

	// Test with value above max
	input = TestInput{
		Age:   200,
		Score: 50,
	}

	err = validator.ValidateInput(input)
	if err == nil {
		t.Fatal("expected validation error for Age > 150")
	}

	// Test with valid values
	input = TestInput{
		Age:   30,
		Score: 85.5,
	}

	err = validator.ValidateInput(input)
	if err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

// TestInputValidatorLength tests minlen/maxlen validation
func TestInputValidatorLength(t *testing.T) {
	type TestInput struct {
		Username string `minlen:"3" maxlen:"20"`
		Bio      string `minlen:"0" maxlen:"500"`
	}

	validator := NewInputValidator(TestInput{})

	// Test with string too short
	input := TestInput{
		Username: "ab",
		Bio:      "Test bio",
	}

	err := validator.ValidateInput(input)
	if err == nil {
		t.Fatal("expected validation error for Username too short")
	}

	// Test with string too long
	input = TestInput{
		Username: "thisusernameiswaytoolongformyvalidation",
		Bio:      "Test bio",
	}

	err = validator.ValidateInput(input)
	if err == nil {
		t.Fatal("expected validation error for Username too long")
	}

	// Test with valid values
	input = TestInput{
		Username: "john_doe",
		Bio:      "This is my bio",
	}

	err = validator.ValidateInput(input)
	if err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

// TestInputValidatorOneOf tests oneof validation
func TestInputValidatorOneOf(t *testing.T) {
	type TestInput struct {
		Status string `oneof:"active,inactive,pending"`
	}

	validator := NewInputValidator(TestInput{})

	// Test with invalid value
	input := TestInput{
		Status: "invalid",
	}

	err := validator.ValidateInput(input)
	if err == nil {
		t.Fatal("expected validation error for invalid Status")
	}

	// Test with valid value
	input = TestInput{
		Status: "active",
	}

	err = validator.ValidateInput(input)
	if err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

// TestInputValidatorNilSchema tests validation with nil schema
func TestInputValidatorNilSchema(t *testing.T) {
	validator := NewInputValidator(nil)

	err := validator.ValidateInput("any input")
	if err != nil {
		t.Fatalf("expected no error for nil schema: %v", err)
	}
}

// TestInputValidatorNilInput tests validation with nil input
func TestInputValidatorNilInput(t *testing.T) {
	type TestInput struct {
		Name string `required:"true"`
	}

	validator := NewInputValidator(TestInput{})

	err := validator.ValidateInput(nil)
	if err == nil {
		t.Fatal("expected validation error for nil input")
	}
}

// TestDependencyManagerBasic tests basic dependency management
func TestDependencyManagerBasic(t *testing.T) {
	dm := NewDependencyManager()

	// Set a dependency
	err := dm.SetDependency("db", "database_connection")
	if err != nil {
		t.Fatalf("failed to set dependency: %v", err)
	}

	// Get the dependency
	value, err := dm.GetDependency("db")
	if err != nil {
		t.Fatalf("failed to get dependency: %v", err)
	}

	if value != "database_connection" {
		t.Fatalf("expected 'database_connection' but got %v", value)
	}
}

// TestDependencyManagerResolver tests dependency resolver
func TestDependencyManagerResolver(t *testing.T) {
	dm := NewDependencyManager()

	// Register a resolver
	counter := 0
	err := dm.RegisterResolver("counter", func() (interface{}, error) {
		counter++
		return counter, nil
	})
	if err != nil {
		t.Fatalf("failed to register resolver: %v", err)
	}

	// Get the dependency (should resolve)
	value1, err := dm.GetDependency("counter")
	if err != nil {
		t.Fatalf("failed to get dependency: %v", err)
	}

	if value1 != 1 {
		t.Fatalf("expected 1 but got %v", value1)
	}

	// Get again (should be cached)
	value2, err := dm.GetDependency("counter")
	if err != nil {
		t.Fatalf("failed to get dependency: %v", err)
	}

	if value2 != 1 {
		t.Fatalf("expected cached value 1 but got %v", value2)
	}

	// Clear cache and get again
	dm.ClearCache()
	value3, err := dm.GetDependency("counter")
	if err != nil {
		t.Fatalf("failed to get dependency: %v", err)
	}

	if value3 != 2 {
		t.Fatalf("expected 2 but got %v", value3)
	}
}

// TestDependencyManagerDelete tests dependency deletion
func TestDependencyManagerDelete(t *testing.T) {
	dm := NewDependencyManager()

	// Set a dependency
	dm.SetDependency("temp", "temporary_value")

	// Verify it exists
	if !dm.HasDependency("temp") {
		t.Fatal("dependency should exist after SetDependency")
	}

	// Delete it
	err := dm.DeleteDependency("temp")
	if err != nil {
		t.Fatalf("failed to delete dependency: %v", err)
	}

	// Verify it's gone
	if dm.HasDependency("temp") {
		t.Fatal("dependency should not exist after DeleteDependency")
	}

	// Try to get it (should fail)
	_, err = dm.GetDependency("temp")
	if err == nil {
		t.Fatal("expected error when getting deleted dependency")
	}
}

// TestDependencyManagerResolveDependencies tests template resolution
func TestDependencyManagerResolveDependencies(t *testing.T) {
	dm := NewDependencyManager()

	// Set some dependencies
	dm.SetDependency("app_name", "MyApp")
	dm.SetDependency("version", "1.0.0")
	dm.SetDependency("port", 8080)

	// Test template resolution
	template := "App: {app_name}, Version: {version}, Port: {port}"
	result, err := dm.ResolveDependencies(template)
	if err != nil {
		t.Fatalf("failed to resolve dependencies: %v", err)
	}

	expected := "App: MyApp, Version: 1.0.0, Port: 8080"
	if result != expected {
		t.Fatalf("expected '%s' but got '%s'", expected, result)
	}

	// Test with missing dependency
	template = "App: {app_name}, Unknown: {unknown_dep}"
	_, err = dm.ResolveDependencies(template)
	if err == nil {
		t.Fatal("expected error when resolving unknown dependency")
	}
}

// TestDependencyManagerInjectDependencies tests dependency injection
func TestDependencyManagerInjectDependencies(t *testing.T) {
	dm := NewDependencyManager()

	// Set dependencies
	dm.SetDependency("db_connection", "postgresql://localhost:5432")
	dm.SetDependency("cache_host", "localhost:6379")

	// Create a target struct
	type Config struct {
		Database string `inject:"db_connection"`
		Cache    string `inject:"cache_host"`
	}

	target := &Config{}

	// Inject dependencies
	err := dm.InjectDependencies(target)
	if err != nil {
		t.Fatalf("failed to inject dependencies: %v", err)
	}

	// Verify injection
	if target.Database != "postgresql://localhost:5432" {
		t.Fatalf("expected 'postgresql://localhost:5432' but got '%s'", target.Database)
	}

	if target.Cache != "localhost:6379" {
		t.Fatalf("expected 'localhost:6379' but got '%s'", target.Cache)
	}
}

// TestDependencyManagerMergeDependencies tests merging
func TestDependencyManagerMergeDependencies(t *testing.T) {
	dm1 := NewDependencyManager()
	dm1.SetDependency("db", "db_value")
	dm1.SetDependency("cache", "cache_value")

	dm2 := NewDependencyManager()
	dm2.SetDependency("logger", "logger_value")
	dm2.SetDependency("db", "new_db_value") // Override

	// Merge dm2 into dm1
	err := dm1.MergeDependencies(dm2)
	if err != nil {
		t.Fatalf("failed to merge dependencies: %v", err)
	}

	// Verify merge
	db, _ := dm1.GetDependency("db")
	if db != "new_db_value" {
		t.Fatalf("expected 'new_db_value' but got '%s'", db)
	}

	logger, _ := dm1.GetDependency("logger")
	if logger != "logger_value" {
		t.Fatalf("expected 'logger_value' but got '%s'", logger)
	}
}

// TestDependencyManagerToMap tests conversion to map
func TestDependencyManagerToMap(t *testing.T) {
	dm := NewDependencyManager()

	// Set some dependencies
	dm.SetDependency("string_dep", "value")
	dm.SetDependency("int_dep", 42)
	dm.SetDependency("bool_dep", true)

	// Convert to map
	depMap := dm.ToMap()

	if depMap["string_dep"] != "value" {
		t.Fatalf("expected 'value' but got %v", depMap["string_dep"])
	}

	if depMap["int_dep"] != 42 {
		t.Fatalf("expected 42 but got %v", depMap["int_dep"])
	}

	if depMap["bool_dep"] != true {
		t.Fatalf("expected true but got %v", depMap["bool_dep"])
	}
}

// TestDependencyManagerErrors tests error handling
func TestDependencyManagerErrors(t *testing.T) {
	dm := NewDependencyManager()

	// Test empty name errors
	_, err := dm.GetDependency("")
	if err == nil {
		t.Fatal("expected error for empty dependency name")
	}

	err = dm.SetDependency("", "value")
	if err == nil {
		t.Fatal("expected error for empty dependency name")
	}

	// Test nil value
	err = dm.SetDependency("test", nil)
	if err == nil {
		t.Fatal("expected error for nil dependency value")
	}

	// Test non-existent dependency
	_, err = dm.GetDependency("non_existent")
	if err == nil {
		t.Fatal("expected error for non-existent dependency")
	}
}
