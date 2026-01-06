package neko

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Tool is the interface that all tools must implement.
type Tool interface {
	Name() string
	Description() string
	Inputs() map[string]ToolInput
	OutputType() string
	Execute(args map[string]any) (any, error)
}

// BaseTool provides common tool functionality.
type BaseTool struct {
	name        string
	description string
	inputs      map[string]ToolInput
	outputType  string
}

func (t *BaseTool) Name() string                 { return t.name }
func (t *BaseTool) Description() string          { return t.description }
func (t *BaseTool) Inputs() map[string]ToolInput { return t.inputs }
func (t *BaseTool) OutputType() string           { return t.outputType }

// Schema returns the tool's JSON schema for LLM consumption.
func (t *BaseTool) Schema() ToolSchema {
	return ToolSchema{
		Name:        t.name,
		Description: t.description,
		Inputs:      t.inputs,
		OutputType:  t.outputType,
	}
}

// ToolRegistry manages available tools.
type ToolRegistry struct {
	tools map[string]Tool
}

// NewToolRegistry creates a new registry.
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{tools: make(map[string]Tool)}
}

// Register adds a tool to the registry.
func (r *ToolRegistry) Register(tool Tool) {
	r.tools[tool.Name()] = tool
}

// Get retrieves a tool by name.
func (r *ToolRegistry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// All returns all registered tools.
func (r *ToolRegistry) All() map[string]Tool {
	return r.tools
}

// Names returns all tool names.
func (r *ToolRegistry) Names() []string {
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// ToJSONSchema converts tools to OpenAI function calling format.
func (r *ToolRegistry) ToJSONSchema() []map[string]any {
	schemas := make([]map[string]any, 0, len(r.tools))
	for _, tool := range r.tools {
		props := make(map[string]any)
		required := []string{}

		for name, input := range tool.Inputs() {
			props[name] = map[string]any{
				"type":        input.Type,
				"description": input.Description,
			}
			if input.Required {
				required = append(required, name)
			}
		}

		schema := map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        tool.Name(),
				"description": tool.Description(),
				"parameters": map[string]any{
					"type":       "object",
					"properties": props,
					"required":   required,
				},
			},
		}
		schemas = append(schemas, schema)
	}
	return schemas
}

// ToCodePrompt generates Python-style function signatures for CodeAgent.
func (r *ToolRegistry) ToCodePrompt() string {
	var sb strings.Builder
	for _, tool := range r.tools {
		sb.WriteString(fmt.Sprintf("def %s(", tool.Name()))

		params := []string{}
		for name, input := range tool.Inputs() {
			params = append(params, fmt.Sprintf("%s: %s", name, goTypeToPython(input.Type)))
		}
		sb.WriteString(strings.Join(params, ", "))
		sb.WriteString(fmt.Sprintf(") -> %s:\n", goTypeToPython(tool.OutputType())))
		sb.WriteString(fmt.Sprintf("    \"\"\"%s\"\"\"\n\n", tool.Description()))
	}
	return sb.String()
}

func goTypeToPython(t string) string {
	switch t {
	case "string":
		return "str"
	case "integer":
		return "int"
	case "number":
		return "float"
	case "boolean":
		return "bool"
	case "array":
		return "list"
	case "object":
		return "dict"
	default:
		return "Any"
	}
}

// FinalAnswerTool is the built-in tool for returning final answers.
type FinalAnswerTool struct {
	BaseTool
}

// NewFinalAnswerTool creates the final_answer tool.
func NewFinalAnswerTool() *FinalAnswerTool {
	return &FinalAnswerTool{
		BaseTool: BaseTool{
			name:        "final_answer",
			description: "Provides a final answer to the given problem.",
			inputs: map[string]ToolInput{
				"answer": {Type: "string", Description: "The final answer to the problem", Required: true},
			},
			outputType: "string",
		},
	}
}

func (t *FinalAnswerTool) Execute(args map[string]any) (any, error) {
	answer, ok := args["answer"]
	if !ok {
		return nil, fmt.Errorf("missing required argument: answer")
	}
	return answer, nil
}

// FuncTool wraps a Go function as a Tool.
type FuncTool struct {
	BaseTool
	fn func(map[string]any) (any, error)
}

// NewFuncTool creates a tool from a function.
func NewFuncTool(name, description string, inputs map[string]ToolInput, outputType string, fn func(map[string]any) (any, error)) *FuncTool {
	return &FuncTool{
		BaseTool: BaseTool{
			name:        name,
			description: description,
			inputs:      inputs,
			outputType:  outputType,
		},
		fn: fn,
	}
}

func (t *FuncTool) Execute(args map[string]any) (any, error) {
	return t.fn(args)
}

// ValidateToolArgs validates arguments against tool schema.
func ValidateToolArgs(tool Tool, args map[string]any) error {
	for name, input := range tool.Inputs() {
		if input.Required {
			if _, ok := args[name]; !ok {
				return fmt.Errorf("missing required argument: %s", name)
			}
		}
	}
	return nil
}

// ParseToolCallJSON parses a JSON string into a ToolCall.
func ParseToolCallJSON(data string) (*ToolCall, error) {
	var tc ToolCall
	if err := json.Unmarshal([]byte(data), &tc); err != nil {
		return nil, fmt.Errorf("failed to parse tool call: %w", err)
	}
	return &tc, nil
}
