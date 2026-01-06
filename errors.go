package neko

import "fmt"

// AgentError is the base error type for agent errors.
type AgentError struct {
	Message string
	Cause   error
}

func (e *AgentError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *AgentError) Unwrap() error { return e.Cause }

// Specific error types

// ErrMaxSteps indicates the agent exceeded maximum steps.
type ErrMaxSteps struct{ AgentError }

// NewErrMaxSteps creates a max steps error.
func NewErrMaxSteps(maxSteps int) *ErrMaxSteps {
	return &ErrMaxSteps{AgentError{Message: fmt.Sprintf("exceeded max steps: %d", maxSteps)}}
}

// ErrParsing indicates a parsing failure.
type ErrParsing struct{ AgentError }

// NewErrParsing creates a parsing error.
func NewErrParsing(msg string, cause error) *ErrParsing {
	return &ErrParsing{AgentError{Message: msg, Cause: cause}}
}

// ErrToolExecution indicates a tool execution failure.
type ErrToolExecution struct {
	AgentError
	ToolName string
}

// NewErrToolExecution creates a tool execution error.
func NewErrToolExecution(toolName string, cause error) *ErrToolExecution {
	return &ErrToolExecution{
		AgentError: AgentError{
			Message: fmt.Sprintf("tool '%s' execution failed", toolName),
			Cause:   cause,
		},
		ToolName: toolName,
	}
}

// ErrGeneration indicates an LLM generation failure.
type ErrGeneration struct{ AgentError }

// NewErrGeneration creates a generation error.
func NewErrGeneration(msg string, cause error) *ErrGeneration {
	return &ErrGeneration{AgentError{Message: msg, Cause: cause}}
}
