package neko

import (
	"encoding/json"
	"time"
)

// MessageRole represents the role of a message in conversation.
type MessageRole string

const (
	RoleSystem    MessageRole = "system"
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleTool      MessageRole = "tool"
)

// Message represents a chat message.
type Message struct {
	Role       MessageRole `json:"role"`
	Content    string      `json:"content"`
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
	TokenUsage *TokenUsage `json:"token_usage,omitempty"`
	Images     [][]byte    `json:"images,omitempty"`
}

// ToolCall represents a tool invocation.
type ToolCall struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// TokenUsage tracks token consumption.
type TokenUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// Total returns total tokens used.
func (t TokenUsage) Total() int {
	return t.InputTokens + t.OutputTokens
}

// Timing captures execution timing.
type Timing struct {
	StartTime time.Time     `json:"start_time"`
	EndTime   time.Time     `json:"end_time"`
	Duration  time.Duration `json:"duration"`
}

// NewTiming creates timing from start time.
func NewTiming(start time.Time) Timing {
	end := time.Now()
	return Timing{
		StartTime: start,
		EndTime:   end,
		Duration:  end.Sub(start),
	}
}

// RunResult holds the result of an agent run.
type RunResult struct {
	Output     any         `json:"output"`
	State      string      `json:"state"` // "success" or "max_steps_error"
	Steps      []Step      `json:"steps"`
	TokenUsage *TokenUsage `json:"token_usage,omitempty"`
	Timing     Timing      `json:"timing"`
}

// Step is the interface for all step types.
type Step interface {
	StepType() string
	ToMessages() []Message
}

// ActionStep represents one action taken by the agent.
type ActionStep struct {
	StepNumber   int         `json:"step_number"`
	Timing       Timing      `json:"timing"`
	ModelOutput  string      `json:"model_output,omitempty"`
	CodeAction   string      `json:"code_action,omitempty"`
	ToolCalls    []ToolCall  `json:"tool_calls,omitempty"`
	Observations string      `json:"observations,omitempty"`
	Error        error       `json:"error,omitempty"`
	TokenUsage   *TokenUsage `json:"token_usage,omitempty"`
	IsFinal      bool        `json:"is_final_answer"`
}

func (s *ActionStep) StepType() string { return "action" }

func (s *ActionStep) ToMessages() []Message {
	var msgs []Message
	// Assistant output (model's response text)
	if s.ModelOutput != "" {
		msgs = append(msgs, Message{Role: RoleAssistant, Content: s.ModelOutput})
	}
	// Tool calls as text (converted to assistant message)
	if len(s.ToolCalls) > 0 {
		msgs = append(msgs, Message{Role: RoleAssistant, Content: formatToolCalls(s.ToolCalls)})
	}
	// Observations as user message
	if s.Observations != "" {
		msgs = append(msgs, Message{Role: RoleUser, Content: "Observation:\n" + s.Observations})
	}
	// Errors as user message
	if s.Error != nil {
		errorMsg := "Error:\n" + s.Error.Error() + "\nPlease try again or use another approach."
		msgs = append(msgs, Message{Role: RoleUser, Content: errorMsg})
	}
	return msgs
}

// formatToolCalls converts tool calls to text representation for message history.
func formatToolCalls(toolCalls []ToolCall) string {
	calls := make([]map[string]any, 0, len(toolCalls))
	for _, tc := range toolCalls {
		calls = append(calls, map[string]any{
			"id":   tc.ID,
			"type": "function",
			"function": map[string]any{
				"name":      tc.Name,
				"arguments": tc.Arguments,
			},
		})
	}
	data, _ := json.Marshal(calls)
	return "Calling tools:\n" + string(data)
}

// TaskStep represents the initial task.
type TaskStep struct {
	Task   string   `json:"task"`
	Images [][]byte `json:"images,omitempty"`
}

func (s *TaskStep) StepType() string { return "task" }

func (s *TaskStep) ToMessages() []Message {
	return []Message{{Role: RoleUser, Content: "Task:\n" + s.Task}}
}

// PlanningStep represents a planning phase.
type PlanningStep struct {
	Plan       string      `json:"plan"`
	Timing     Timing      `json:"timing"`
	TokenUsage *TokenUsage `json:"token_usage,omitempty"`
}

func (s *PlanningStep) StepType() string { return "planning" }

func (s *PlanningStep) ToMessages() []Message {
	return []Message{
		{Role: RoleAssistant, Content: s.Plan},
		{Role: RoleUser, Content: "Now proceed and carry out this plan."},
	}
}

// FinalAnswerStep marks the final answer.
type FinalAnswerStep struct {
	Output any `json:"output"`
}

func (s *FinalAnswerStep) StepType() string { return "final_answer" }

func (s *FinalAnswerStep) ToMessages() []Message {
	return nil
}

// ToolInput describes a tool parameter.
type ToolInput struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Required    bool   `json:"required,omitempty"`
}

// ToolSchema describes a tool's interface.
type ToolSchema struct {
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Inputs      map[string]ToolInput `json:"inputs"`
	OutputType  string               `json:"output_type"`
}
