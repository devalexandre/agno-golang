package tools

import (
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// MathTool provides mathematical operations and calculations
type MathTool struct {
	toolkit.Toolkit
}

// MathResult represents the result of mathematical operations
type MathResult struct {
	Operation string      `json:"operation"`
	Input     interface{} `json:"input"`
	Result    interface{} `json:"result"`
	Success   bool        `json:"success"`
	Error     string      `json:"error,omitempty"`
}

// BasicMathParams represents parameters for basic math operations
type BasicMathParams struct {
	Operation string    `json:"operation" description:"Math operation: add, subtract, multiply, divide, power, sqrt, abs" required:"true"`
	Numbers   []float64 `json:"numbers" description:"Numbers to perform operation on" required:"true"`
}

// StatisticsParams represents parameters for statistical operations
type StatisticsParams struct {
	Numbers []float64 `json:"numbers" description:"Array of numbers for statistical analysis" required:"true"`
}

// TrigonometryParams represents parameters for trigonometric operations
type TrigonometryParams struct {
	Function string  `json:"function" description:"Trigonometric function: sin, cos, tan, asin, acos, atan" required:"true"`
	Angle    float64 `json:"angle" description:"Angle value" required:"true"`
	Unit     string  `json:"unit,omitempty" description:"Angle unit: degrees or radians. Default: radians"`
}

// RandomParams represents parameters for random number generation
type RandomParams struct {
	Type  string  `json:"type" description:"Type of random: integer, float, choice" required:"true"`
	Min   float64 `json:"min,omitempty" description:"Minimum value (for integer/float)"`
	Max   float64 `json:"max,omitempty" description:"Maximum value (for integer/float)"`
	Count int     `json:"count,omitempty" description:"Number of random values to generate. Default: 1"`
}

// CalculateParams represents parameters for expression evaluation
type CalculateParams struct {
	Expression string `json:"expression" description:"Mathematical expression to evaluate (basic operations only)" required:"true"`
}

// NewMathTool creates a new MathTool instance
func NewMathTool() *MathTool {
	tk := toolkit.NewToolkit()
	tk.Name = "MathTool"
	tk.Description = "A comprehensive mathematical tool for performing basic operations, statistical analysis, trigonometry, random number generation, and expression evaluation."

	mt := &MathTool{tk}

	// Register methods
	mt.Toolkit.Register("BasicMath", mt, mt.BasicMath, BasicMathParams{})
	mt.Toolkit.Register("Statistics", mt, mt.Statistics, StatisticsParams{})
	mt.Toolkit.Register("Trigonometry", mt, mt.Trigonometry, TrigonometryParams{})
	mt.Toolkit.Register("Random", mt, mt.Random, RandomParams{})
	mt.Toolkit.Register("Calculate", mt, mt.Calculate, CalculateParams{})

	return mt
}

// BasicMath performs basic mathematical operations
func (mt *MathTool) BasicMath(params BasicMathParams) (interface{}, error) {
	if params.Operation == "" {
		return nil, fmt.Errorf("operation is required")
	}

	if len(params.Numbers) == 0 {
		return nil, fmt.Errorf("at least one number is required")
	}

	operation := strings.ToLower(params.Operation)
	var result float64

	switch operation {
	case "add", "sum":
		result = 0
		for _, num := range params.Numbers {
			result += num
		}

	case "subtract", "sub":
		if len(params.Numbers) < 2 {
			return nil, fmt.Errorf("subtract requires at least 2 numbers")
		}
		result = params.Numbers[0]
		for i := 1; i < len(params.Numbers); i++ {
			result -= params.Numbers[i]
		}

	case "multiply", "mul":
		result = 1
		for _, num := range params.Numbers {
			result *= num
		}

	case "divide", "div":
		if len(params.Numbers) < 2 {
			return nil, fmt.Errorf("divide requires at least 2 numbers")
		}
		result = params.Numbers[0]
		for i := 1; i < len(params.Numbers); i++ {
			if params.Numbers[i] == 0 {
				return MathResult{
					Operation: operation,
					Input:     params.Numbers,
					Success:   false,
					Error:     "division by zero",
				}, nil
			}
			result /= params.Numbers[i]
		}

	case "power", "pow":
		if len(params.Numbers) != 2 {
			return nil, fmt.Errorf("power requires exactly 2 numbers (base, exponent)")
		}
		result = math.Pow(params.Numbers[0], params.Numbers[1])

	case "sqrt":
		if len(params.Numbers) != 1 {
			return nil, fmt.Errorf("sqrt requires exactly 1 number")
		}
		if params.Numbers[0] < 0 {
			return MathResult{
				Operation: operation,
				Input:     params.Numbers,
				Success:   false,
				Error:     "cannot calculate square root of negative number",
			}, nil
		}
		result = math.Sqrt(params.Numbers[0])

	case "abs":
		if len(params.Numbers) != 1 {
			return nil, fmt.Errorf("abs requires exactly 1 number")
		}
		result = math.Abs(params.Numbers[0])

	case "log":
		if len(params.Numbers) != 1 {
			return nil, fmt.Errorf("log requires exactly 1 number")
		}
		if params.Numbers[0] <= 0 {
			return MathResult{
				Operation: operation,
				Input:     params.Numbers,
				Success:   false,
				Error:     "logarithm undefined for non-positive numbers",
			}, nil
		}
		result = math.Log(params.Numbers[0])

	case "log10":
		if len(params.Numbers) != 1 {
			return nil, fmt.Errorf("log10 requires exactly 1 number")
		}
		if params.Numbers[0] <= 0 {
			return MathResult{
				Operation: operation,
				Input:     params.Numbers,
				Success:   false,
				Error:     "logarithm undefined for non-positive numbers",
			}, nil
		}
		result = math.Log10(params.Numbers[0])

	default:
		return nil, fmt.Errorf("unsupported operation: %s", operation)
	}

	return MathResult{
		Operation: operation,
		Input:     params.Numbers,
		Result:    result,
		Success:   true,
	}, nil
}

// Statistics performs statistical analysis on numbers
func (mt *MathTool) Statistics(params StatisticsParams) (interface{}, error) {
	if len(params.Numbers) == 0 {
		return nil, fmt.Errorf("at least one number is required")
	}

	numbers := params.Numbers
	n := float64(len(numbers))

	// Calculate sum
	sum := 0.0
	for _, num := range numbers {
		sum += num
	}

	// Calculate mean
	mean := sum / n

	// Find min and max
	min := numbers[0]
	max := numbers[0]
	for _, num := range numbers {
		if num < min {
			min = num
		}
		if num > max {
			max = num
		}
	}

	// Calculate variance and standard deviation
	variance := 0.0
	for _, num := range numbers {
		variance += math.Pow(num-mean, 2)
	}
	variance /= n
	stdDev := math.Sqrt(variance)

	// Calculate median (need to sort first)
	sorted := make([]float64, len(numbers))
	copy(sorted, numbers)

	// Simple bubble sort for median calculation
	for i := 0; i < len(sorted); i++ {
		for j := 0; j < len(sorted)-1-i; j++ {
			if sorted[j] > sorted[j+1] {
				sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
			}
		}
	}

	var median float64
	if len(sorted)%2 == 0 {
		median = (sorted[len(sorted)/2-1] + sorted[len(sorted)/2]) / 2
	} else {
		median = sorted[len(sorted)/2]
	}

	return map[string]interface{}{
		"count":              len(numbers),
		"sum":                sum,
		"mean":               mean,
		"median":             median,
		"min":                min,
		"max":                max,
		"range":              max - min,
		"variance":           variance,
		"standard_deviation": stdDev,
		"operation":          "Statistics",
	}, nil
}

// Trigonometry performs trigonometric calculations
func (mt *MathTool) Trigonometry(params TrigonometryParams) (interface{}, error) {
	if params.Function == "" {
		return nil, fmt.Errorf("trigonometric function is required")
	}

	function := strings.ToLower(params.Function)
	unit := strings.ToLower(params.Unit)
	if unit == "" {
		unit = "radians"
	}

	angle := params.Angle

	// Convert degrees to radians if necessary
	if unit == "degrees" {
		angle = angle * math.Pi / 180
	} else if unit != "radians" {
		return nil, fmt.Errorf("unit must be 'degrees' or 'radians'")
	}

	var result float64

	switch function {
	case "sin":
		result = math.Sin(angle)
	case "cos":
		result = math.Cos(angle)
	case "tan":
		result = math.Tan(angle)
	case "asin":
		if params.Angle < -1 || params.Angle > 1 {
			return MathResult{
				Operation: function,
				Input:     params.Angle,
				Success:   false,
				Error:     "asin domain error: input must be between -1 and 1",
			}, nil
		}
		result = math.Asin(params.Angle)
	case "acos":
		if params.Angle < -1 || params.Angle > 1 {
			return MathResult{
				Operation: function,
				Input:     params.Angle,
				Success:   false,
				Error:     "acos domain error: input must be between -1 and 1",
			}, nil
		}
		result = math.Acos(params.Angle)
	case "atan":
		result = math.Atan(params.Angle)
	default:
		return nil, fmt.Errorf("unsupported trigonometric function: %s", function)
	}

	// Convert result back to degrees if requested
	displayResult := result
	if unit == "degrees" && strings.HasPrefix(function, "a") {
		displayResult = result * 180 / math.Pi
	}

	return MathResult{
		Operation: function,
		Input:     params.Angle,
		Result:    displayResult,
		Success:   true,
	}, nil
}

// Random generates random numbers
func (mt *MathTool) Random(params RandomParams) (interface{}, error) {
	if params.Type == "" {
		return nil, fmt.Errorf("random type is required")
	}

	if params.Count <= 0 {
		params.Count = 1
	}

	// Seed random generator
	rand.Seed(time.Now().UnixNano())

	randomType := strings.ToLower(params.Type)
	var results []interface{}

	for i := 0; i < params.Count; i++ {
		switch randomType {
		case "integer", "int":
			min := int(params.Min)
			max := int(params.Max)
			if max <= min {
				max = min + 100 // Default range
			}
			result := rand.Intn(max-min) + min
			results = append(results, result)

		case "float":
			min := params.Min
			max := params.Max
			if max <= min {
				max = min + 1.0 // Default range
			}
			result := rand.Float64()*(max-min) + min
			results = append(results, result)

		case "boolean", "bool":
			result := rand.Intn(2) == 1
			results = append(results, result)

		default:
			return nil, fmt.Errorf("unsupported random type: %s", randomType)
		}
	}

	// Return single value if count is 1, otherwise return array
	if params.Count == 1 {
		return MathResult{
			Operation: "Random",
			Input:     params,
			Result:    results[0],
			Success:   true,
		}, nil
	}

	return MathResult{
		Operation: "Random",
		Input:     params,
		Result:    results,
		Success:   true,
	}, nil
}

// Calculate evaluates simple mathematical expressions
func (mt *MathTool) Calculate(params CalculateParams) (interface{}, error) {
	if params.Expression == "" {
		return nil, fmt.Errorf("mathematical expression is required")
	}

	// This is a simplified calculator - for production use, consider using a proper expression parser
	expression := strings.ReplaceAll(params.Expression, " ", "")

	// Handle basic operations with two operands for now
	var result float64
	var operator string
	var operands []string

	// Find the operator
	for _, op := range []string{"+", "-", "*", "/", "^"} {
		if strings.Contains(expression, op) {
			operator = op
			operands = strings.Split(expression, op)
			break
		}
	}

	if operator == "" || len(operands) != 2 {
		return MathResult{
			Operation: "Calculate",
			Input:     params.Expression,
			Success:   false,
			Error:     "invalid expression format. Supported: a+b, a-b, a*b, a/b, a^b",
		}, nil
	}

	// Parse operands
	num1, err1 := strconv.ParseFloat(operands[0], 64)
	num2, err2 := strconv.ParseFloat(operands[1], 64)

	if err1 != nil || err2 != nil {
		return MathResult{
			Operation: "Calculate",
			Input:     params.Expression,
			Success:   false,
			Error:     "invalid numbers in expression",
		}, nil
	}

	// Perform calculation
	switch operator {
	case "+":
		result = num1 + num2
	case "-":
		result = num1 - num2
	case "*":
		result = num1 * num2
	case "/":
		if num2 == 0 {
			return MathResult{
				Operation: "Calculate",
				Input:     params.Expression,
				Success:   false,
				Error:     "division by zero",
			}, nil
		}
		result = num1 / num2
	case "^":
		result = math.Pow(num1, num2)
	}

	return MathResult{
		Operation: "Calculate",
		Input:     params.Expression,
		Result:    result,
		Success:   true,
	}, nil
}
