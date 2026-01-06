package neko

import (
	"fmt"
	"strings"
)

// Memory stores the agent's conversation history and steps.
type Memory struct {
	SystemPrompt string
	Steps        []Step
}

// NewMemory creates a new memory instance.
func NewMemory(systemPrompt string) *Memory {
	return &Memory{
		SystemPrompt: systemPrompt,
		Steps:        make([]Step, 0),
	}
}

// Reset clears all steps while keeping system prompt.
func (m *Memory) Reset() {
	m.Steps = make([]Step, 0)
}

// AddStep appends a step to memory.
func (m *Memory) AddStep(step Step) {
	m.Steps = append(m.Steps, step)
}

// LastStep returns the most recent step, or nil.
func (m *Memory) LastStep() Step {
	if len(m.Steps) == 0 {
		return nil
	}
	return m.Steps[len(m.Steps)-1]
}

// ToMessages converts memory to a message list for LLM.
func (m *Memory) ToMessages() []Message {
	msgs := []Message{{Role: RoleSystem, Content: m.SystemPrompt}}
	for _, step := range m.Steps {
		msgs = append(msgs, step.ToMessages()...)
	}
	return msgs
}

// TotalTokens returns cumulative token usage.
func (m *Memory) TotalTokens() TokenUsage {
	var total TokenUsage
	for _, step := range m.Steps {
		switch s := step.(type) {
		case *ActionStep:
			if s.TokenUsage != nil {
				total.InputTokens += s.TokenUsage.InputTokens
				total.OutputTokens += s.TokenUsage.OutputTokens
			}
		case *PlanningStep:
			if s.TokenUsage != nil {
				total.InputTokens += s.TokenUsage.InputTokens
				total.OutputTokens += s.TokenUsage.OutputTokens
			}
		}
	}
	return total
}

// ActionSteps returns only action steps.
func (m *Memory) ActionSteps() []*ActionStep {
	var steps []*ActionStep
	for _, s := range m.Steps {
		if as, ok := s.(*ActionStep); ok {
			steps = append(steps, as)
		}
	}
	return steps
}

// Summary returns a brief summary of the memory state.
func (m *Memory) Summary() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Memory: %d steps\n", len(m.Steps)))

	for i, step := range m.Steps {
		switch s := step.(type) {
		case *TaskStep:
			sb.WriteString(fmt.Sprintf("  [%d] Task: %s...\n", i, truncate(s.Task, 50)))
		case *ActionStep:
			status := "pending"
			if s.IsFinal {
				status = "final"
			} else if s.Error != nil {
				status = "error"
			} else if s.Observations != "" {
				status = "done"
			}
			sb.WriteString(fmt.Sprintf("  [%d] Action #%d: %s\n", i, s.StepNumber, status))
		case *PlanningStep:
			sb.WriteString(fmt.Sprintf("  [%d] Planning: %s...\n", i, truncate(s.Plan, 50)))
		case *FinalAnswerStep:
			sb.WriteString(fmt.Sprintf("  [%d] Final Answer\n", i))
		}
	}

	tokens := m.TotalTokens()
	sb.WriteString(fmt.Sprintf("Total tokens: %d (in: %d, out: %d)\n",
		tokens.Total(), tokens.InputTokens, tokens.OutputTokens))

	return sb.String()
}

func truncate(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// CallbackRegistry manages step callbacks.
type CallbackRegistry struct {
	callbacks map[string][]func(Step)
}

// NewCallbackRegistry creates a callback registry.
func NewCallbackRegistry() *CallbackRegistry {
	return &CallbackRegistry{callbacks: make(map[string][]func(Step))}
}

// Register adds a callback for a step type.
func (r *CallbackRegistry) Register(stepType string, fn func(Step)) {
	r.callbacks[stepType] = append(r.callbacks[stepType], fn)
}

// Trigger fires callbacks for a step.
func (r *CallbackRegistry) Trigger(step Step) {
	for _, fn := range r.callbacks[step.StepType()] {
		fn(step)
	}
	// Also trigger "all" callbacks
	for _, fn := range r.callbacks["all"] {
		fn(step)
	}
}
