package tools

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"strconv"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// CalculatorTool provides simple mathematical expression evaluation.
type CalculatorTool struct {
	toolkit.Toolkit
}

// CalculatorParams defines the parameters for the Calculate method.
type CalculatorParams struct {
	Expression string `json:"expression" description:"A mathematical expression to evaluate (e.g., '2 + 3 * 4', '(10 - 2) / 4')." required:"true"`
}

// NewCalculatorTool creates a new Calculator tool.
func NewCalculatorTool() *CalculatorTool {
	t := &CalculatorTool{}

	tk := toolkit.NewToolkit()
	tk.Name = "CalculatorTool"
	tk.Description = "Evaluate mathematical expressions. Supports +, -, *, /, parentheses."

	t.Toolkit = tk
	t.Toolkit.Register("Calculate", "Evaluate a mathematical expression and return the result.", t, t.Calculate, CalculatorParams{})

	return t
}

// Calculate evaluates a mathematical expression.
func (t *CalculatorTool) Calculate(params CalculatorParams) (interface{}, error) {
	if params.Expression == "" {
		return nil, fmt.Errorf("expression is required")
	}

	result, err := evalExpr(params.Expression)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate expression '%s': %w", params.Expression, err)
	}

	return map[string]interface{}{
		"expression": params.Expression,
		"result":     result,
	}, nil
}

// evalExpr evaluates a simple mathematical expression using Go's AST parser.
func evalExpr(expr string) (float64, error) {
	node, err := parser.ParseExpr(expr)
	if err != nil {
		return 0, fmt.Errorf("invalid expression: %w", err)
	}
	return evalNode(node)
}

func evalNode(node ast.Expr) (float64, error) {
	switch n := node.(type) {
	case *ast.BasicLit:
		if n.Kind == token.INT || n.Kind == token.FLOAT {
			return strconv.ParseFloat(n.Value, 64)
		}
		return 0, fmt.Errorf("unsupported literal: %s", n.Value)

	case *ast.BinaryExpr:
		left, err := evalNode(n.X)
		if err != nil {
			return 0, err
		}
		right, err := evalNode(n.Y)
		if err != nil {
			return 0, err
		}

		switch n.Op {
		case token.ADD:
			return left + right, nil
		case token.SUB:
			return left - right, nil
		case token.MUL:
			return left * right, nil
		case token.QUO:
			if right == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			return left / right, nil
		case token.REM:
			if right == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			return math.Mod(left, right), nil
		default:
			return 0, fmt.Errorf("unsupported operator: %s", n.Op)
		}

	case *ast.ParenExpr:
		return evalNode(n.X)

	case *ast.UnaryExpr:
		val, err := evalNode(n.X)
		if err != nil {
			return 0, err
		}
		if n.Op == token.SUB {
			return -val, nil
		}
		return val, nil

	default:
		return 0, fmt.Errorf("unsupported expression type: %T", node)
	}
}

// Execute implements the toolkit.Tool interface.
func (t *CalculatorTool) Execute(methodName string, input json.RawMessage) (interface{}, error) {
	return t.Toolkit.Execute(methodName, input)
}
