package agent

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type ChainToolCondition interface {
	Evaluate(ctx context.Context, toolName string, result interface{}) bool
}

type ResultContainsCondition struct {
	Pattern string
}

func (c *ResultContainsCondition) Evaluate(ctx context.Context, toolName string, result interface{}) bool {
	resultStr := fmt.Sprintf("%v", result)
	return strings.Contains(resultStr, c.Pattern)
}

type ResultMatchesRegexCondition struct {
	Pattern string
	regex   *regexp.Regexp
}

func (c *ResultMatchesRegexCondition) Evaluate(ctx context.Context, toolName string, result interface{}) bool {
	if c.regex == nil {
		var err error
		c.regex, err = regexp.Compile(c.Pattern)
		if err != nil {
			return false
		}
	}
	resultStr := fmt.Sprintf("%v", result)
	return c.regex.MatchString(resultStr)
}

type ResultLengthCondition struct {
	Operator string
	Value    int
}

func (c *ResultLengthCondition) Evaluate(ctx context.Context, toolName string, result interface{}) bool {
	resultStr := fmt.Sprintf("%v", result)
	length := len(resultStr)

	switch c.Operator {
	case ">":
		return length > c.Value
	case "<":
		return length < c.Value
	case ">=":
		return length >= c.Value
	case "<=":
		return length <= c.Value
	case "==":
		return length == c.Value
	case "!=":
		return length != c.Value
	}
	return false
}

type ResultNumericCondition struct {
	Operator string
	Value    float64
}

func (c *ResultNumericCondition) Evaluate(ctx context.Context, toolName string, result interface{}) bool {
	var numValue float64

	switch v := result.(type) {
	case float64:
		numValue = v
	case int:
		numValue = float64(v)
	case string:
		parsed, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return false
		}
		numValue = parsed
	default:
		return false
	}

	switch c.Operator {
	case ">":
		return numValue > c.Value
	case "<":
		return numValue < c.Value
	case ">=":
		return numValue >= c.Value
	case "<=":
		return numValue <= c.Value
	case "==":
		return numValue == c.Value
	case "!=":
		return numValue != c.Value
	}
	return false
}

type ChainToolStep struct {
	ToolName  string
	Condition ChainToolCondition
	Execute   bool
}

type ChainToolConfig struct {
	Steps []ChainToolStep
}

func NewResultContainsCondition(pattern string) *ResultContainsCondition {
	return &ResultContainsCondition{Pattern: pattern}
}

func NewResultLengthCondition(operator string, value int) *ResultLengthCondition {
	return &ResultLengthCondition{Operator: operator, Value: value}
}

func NewResultNumericCondition(operator string, value float64) *ResultNumericCondition {
	return &ResultNumericCondition{Operator: operator, Value: value}
}
