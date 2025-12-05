package agent

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// InputSchema defines the interface for input validation
type InputSchema interface {
	Validate() error
}

// ValidationError represents an error that occurred during validation
type ValidationError struct {
	Field   string
	Message string
	Value   interface{}
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error on field '%s': %s (got %v)", e.Field, e.Message, e.Value)
}

// ValidationErrors is a collection of validation errors
type ValidationErrors []ValidationError

func (es ValidationErrors) Error() string {
	if len(es) == 0 {
		return "no validation errors"
	}

	var messages []string
	for _, e := range es {
		messages = append(messages, e.Error())
	}
	return fmt.Sprintf("%d validation error(s):\n  - %s", len(es), strings.Join(messages, "\n  - "))
}

// InputValidator handles validation of agent input
type InputValidator struct {
	schema interface{}
}

// NewInputValidator creates a new input validator
func NewInputValidator(schema interface{}) *InputValidator {
	return &InputValidator{
		schema: schema,
	}
}

// ValidateInput validates input against the schema
func (v *InputValidator) ValidateInput(input interface{}) error {
	if v.schema == nil {
		return nil
	}

	// If input is nil, it's invalid
	if input == nil {
		return ValidationError{
			Field:   "input",
			Message: "input is required",
			Value:   nil,
		}
	}

	// Get the type of the schema
	schemaType := reflect.TypeOf(v.schema)
	inputType := reflect.TypeOf(input)

	// Check if input matches schema type
	if schemaType.Kind() == reflect.Ptr {
		schemaType = schemaType.Elem()
	}
	if inputType.Kind() == reflect.Ptr {
		inputType = inputType.Elem()
	}

	// If types don't match, try to check field compatibility
	if schemaType != inputType {
		if schemaType.Kind() == reflect.Struct && inputType.Kind() == reflect.Struct {
			return v.validateStructFields(input)
		}
		return ValidationError{
			Field:   "input",
			Message: fmt.Sprintf("expected %s but got %s", schemaType.Name(), inputType.Name()),
			Value:   input,
		}
	}

	// If the input implements InputSchema interface, call its Validate method
	if validator, ok := input.(InputSchema); ok {
		return validator.Validate()
	}

	// Validate struct fields if input is a struct
	if inputType.Kind() == reflect.Struct {
		return v.validateStructFields(input)
	}

	return nil
}

// validateStructFields validates the fields of a struct
func (v *InputValidator) validateStructFields(input interface{}) error {
	inputValue := reflect.ValueOf(input)
	if inputValue.Kind() == reflect.Ptr {
		inputValue = inputValue.Elem()
	}

	inputType := inputValue.Type()
	var errors ValidationErrors

	// Iterate through struct fields
	for i := 0; i < inputType.NumField(); i++ {
		field := inputType.Field(i)
		fieldValue := inputValue.Field(i)

		// Check for "required" tag
		if tag, ok := field.Tag.Lookup("required"); ok && tag == "true" {
			if isZeroValue(fieldValue) {
				errors = append(errors, ValidationError{
					Field:   field.Name,
					Message: "field is required",
					Value:   fieldValue.Interface(),
				})
			}
		}

		// Check for "min" tag (for numeric types)
		if tag, ok := field.Tag.Lookup("min"); ok {
			if err := v.validateMinValue(field.Name, fieldValue, tag); err != nil {
				errors = append(errors, *err)
			}
		}

		// Check for "max" tag (for numeric types)
		if tag, ok := field.Tag.Lookup("max"); ok {
			if err := v.validateMaxValue(field.Name, fieldValue, tag); err != nil {
				errors = append(errors, *err)
			}
		}

		// Check for "minlen" tag (for strings and slices)
		if tag, ok := field.Tag.Lookup("minlen"); ok {
			if err := v.validateMinLength(field.Name, fieldValue, tag); err != nil {
				errors = append(errors, *err)
			}
		}

		// Check for "maxlen" tag (for strings and slices)
		if tag, ok := field.Tag.Lookup("maxlen"); ok {
			if err := v.validateMaxLength(field.Name, fieldValue, tag); err != nil {
				errors = append(errors, *err)
			}
		}

		// Check for "pattern" tag (for strings, regex)
		if tag, ok := field.Tag.Lookup("pattern"); ok {
			if err := v.validatePattern(field.Name, fieldValue, tag); err != nil {
				errors = append(errors, *err)
			}
		}

		// Check for "oneof" tag (for enum-like validation)
		if tag, ok := field.Tag.Lookup("oneof"); ok {
			if err := v.validateOneOf(field.Name, fieldValue, tag); err != nil {
				errors = append(errors, *err)
			}
		}
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}

// isZeroValue checks if a value is the zero value for its type
func isZeroValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:
		return v.String() == ""
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0.0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Slice, reflect.Array, reflect.Map:
		return v.Len() == 0
	case reflect.Ptr, reflect.Interface:
		return v.IsNil()
	default:
		return false
	}
}

// validateMinValue validates minimum value for numeric types
func (v *InputValidator) validateMinValue(fieldName string, value reflect.Value, minStr string) *ValidationError {
	min, err := strconv.ParseFloat(minStr, 64)
	if err != nil {
		return &ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("invalid min tag: %s", err),
		}
	}

	var actual float64
	switch value.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		actual = float64(value.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		actual = float64(value.Uint())
	case reflect.Float32, reflect.Float64:
		actual = value.Float()
	default:
		return nil
	}

	if actual < min {
		return &ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("value must be >= %v", min),
			Value:   actual,
		}
	}

	return nil
}

// validateMaxValue validates maximum value for numeric types
func (v *InputValidator) validateMaxValue(fieldName string, value reflect.Value, maxStr string) *ValidationError {
	max, err := strconv.ParseFloat(maxStr, 64)
	if err != nil {
		return &ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("invalid max tag: %s", err),
		}
	}

	var actual float64
	switch value.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		actual = float64(value.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		actual = float64(value.Uint())
	case reflect.Float32, reflect.Float64:
		actual = value.Float()
	default:
		return nil
	}

	if actual > max {
		return &ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("value must be <= %v", max),
			Value:   actual,
		}
	}

	return nil
}

// validateMinLength validates minimum length for strings and slices
func (v *InputValidator) validateMinLength(fieldName string, value reflect.Value, minStr string) *ValidationError {
	minLen, err := strconv.Atoi(minStr)
	if err != nil {
		return &ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("invalid minlen tag: %s", err),
		}
	}

	var actual int
	switch value.Kind() {
	case reflect.String:
		actual = len(value.String())
	case reflect.Slice, reflect.Array, reflect.Map:
		actual = value.Len()
	default:
		return nil
	}

	if actual < minLen {
		return &ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("length must be >= %d", minLen),
			Value:   actual,
		}
	}

	return nil
}

// validateMaxLength validates maximum length for strings and slices
func (v *InputValidator) validateMaxLength(fieldName string, value reflect.Value, maxStr string) *ValidationError {
	maxLen, err := strconv.Atoi(maxStr)
	if err != nil {
		return &ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("invalid maxlen tag: %s", err),
		}
	}

	var actual int
	switch value.Kind() {
	case reflect.String:
		actual = len(value.String())
	case reflect.Slice, reflect.Array, reflect.Map:
		actual = value.Len()
	default:
		return nil
	}

	if actual > maxLen {
		return &ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("length must be <= %d", maxLen),
			Value:   actual,
		}
	}

	return nil
}

// validatePattern validates string against a regex pattern
func (v *InputValidator) validatePattern(fieldName string, value reflect.Value, pattern string) *ValidationError {
	if value.Kind() != reflect.String {
		return nil
	}

	// For now, we'll just check if a pattern is provided
	// Full regex validation would require "regexp" package
	// This is a placeholder for future implementation
	return nil
}

// validateOneOf validates that value is one of allowed values
func (v *InputValidator) validateOneOf(fieldName string, value reflect.Value, allowed string) *ValidationError {
	allowedValues := strings.Split(allowed, ",")
	actual := fmt.Sprintf("%v", value.Interface())

	for _, av := range allowedValues {
		if strings.TrimSpace(av) == actual {
			return nil
		}
	}

	return &ValidationError{
		Field:   fieldName,
		Message: fmt.Sprintf("value must be one of %v", allowedValues),
		Value:   value.Interface(),
	}
}

// ValidateInputSchema validates that the input matches the provided schema
// This is a convenience function for direct validation
func ValidateInputSchema(schema interface{}, input interface{}) error {
	if schema == nil {
		return nil
	}

	validator := NewInputValidator(schema)
	return validator.ValidateInput(input)
}
