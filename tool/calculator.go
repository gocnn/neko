package tool

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"

	"github.com/gocnn/neko"
)

// CalculatorTool evaluates mathematical expressions.
type CalculatorTool struct {
	neko.BaseTool
}

// NewCalculatorTool creates a calculator tool.
func NewCalculatorTool() *CalculatorTool {
	return &CalculatorTool{}
}

func (t *CalculatorTool) Name() string        { return "calculator" }
func (t *CalculatorTool) Description() string { return "Evaluates mathematical expressions." }
func (t *CalculatorTool) OutputType() string  { return "number" }

func (t *CalculatorTool) Inputs() map[string]neko.ToolInput {
	return map[string]neko.ToolInput{
		"expression": {Type: "string", Description: "Math expression to evaluate", Required: true},
	}
}

func (t *CalculatorTool) Execute(args map[string]any) (any, error) {
	expr, ok := args["expression"].(string)
	if !ok || expr == "" {
		return nil, fmt.Errorf("expression is required")
	}

	result, err := evalExpr(expr)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate: %w", err)
	}
	return result, nil
}

// evalExpr safely evaluates a mathematical expression.
func evalExpr(expr string) (float64, error) {
	node, err := parser.ParseExpr(expr)
	if err != nil {
		return 0, err
	}
	return eval(node)
}

func eval(node ast.Expr) (float64, error) {
	switch n := node.(type) {
	case *ast.BasicLit:
		if n.Kind == token.INT || n.Kind == token.FLOAT {
			return strconv.ParseFloat(n.Value, 64)
		}
	case *ast.BinaryExpr:
		left, err := eval(n.X)
		if err != nil {
			return 0, err
		}
		right, err := eval(n.Y)
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
		}
	case *ast.ParenExpr:
		return eval(n.X)
	case *ast.UnaryExpr:
		val, err := eval(n.X)
		if err != nil {
			return 0, err
		}
		if n.Op == token.SUB {
			return -val, nil
		}
		return val, nil
	}
	return 0, fmt.Errorf("unsupported expression")
}
