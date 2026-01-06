package neko

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Agent is the core interface for all agent types.
type Agent interface {
	Run(ctx context.Context, task string, opts ...RunOption) (*RunResult, error)
	Name() string
	Description() string
}

// RunOptions holds options for agent execution.
type RunOptions struct {
	MaxSteps  int
	Reset     bool
	Images    [][]byte
	ExtraArgs map[string]any
}

// RunOption is a functional option for Run.
type RunOption func(*RunOptions)

// WithMaxSteps sets the maximum steps.
func WithMaxSteps(n int) RunOption {
	return func(o *RunOptions) { o.MaxSteps = n }
}

// WithReset controls memory reset.
func WithReset(reset bool) RunOption {
	return func(o *RunOptions) { o.Reset = reset }
}

// BaseAgent provides common agent functionality.
type BaseAgent struct {
	name          string
	description   string
	model         Model
	tools         *ToolRegistry
	memory        *Memory
	managedAgents map[string]Agent
	callbacks     *CallbackRegistry
	maxSteps      int
	systemPrompt  string
	mu            sync.Mutex
}

// AgentOption configures a BaseAgent.
type AgentOption func(*BaseAgent)

// WithName sets the agent name.
func WithName(name string) AgentOption {
	return func(a *BaseAgent) { a.name = name }
}

// WithDescription sets the agent description.
func WithDescription(desc string) AgentOption {
	return func(a *BaseAgent) { a.description = desc }
}

// WithModel sets the LLM model.
func WithModel(m Model) AgentOption {
	return func(a *BaseAgent) { a.model = m }
}

// WithAgentMaxSteps sets maximum execution steps.
func WithAgentMaxSteps(n int) AgentOption {
	return func(a *BaseAgent) { a.maxSteps = n }
}

// WithManagedAgents adds sub-agents.
func WithManagedAgents(agents ...Agent) AgentOption {
	return func(a *BaseAgent) {
		for _, agent := range agents {
			a.managedAgents[agent.Name()] = agent
		}
	}
}

// WithToolList adds tools to the agent.
func WithToolList(tools ...Tool) AgentOption {
	return func(a *BaseAgent) {
		for _, tool := range tools {
			a.tools.Register(tool)
		}
	}
}

func (a *BaseAgent) Name() string        { return a.name }
func (a *BaseAgent) Description() string { return a.description }

// ToolCallingAgent uses JSON tool calls.
type ToolCallingAgent struct {
	BaseAgent
}

// NewToolCallingAgent creates a tool-calling agent.
func NewToolCallingAgent(opts ...AgentOption) *ToolCallingAgent {
	a := &ToolCallingAgent{
		BaseAgent: BaseAgent{
			tools:         NewToolRegistry(),
			managedAgents: make(map[string]Agent),
			callbacks:     NewCallbackRegistry(),
			maxSteps:      20,
		},
	}
	a.tools.Register(NewFinalAnswerTool())

	for _, opt := range opts {
		opt(&a.BaseAgent)
	}

	if a.systemPrompt == "" {
		a.systemPrompt = defaultToolCallingPrompt(a.tools)
	}
	a.memory = NewMemory(a.systemPrompt)

	return a
}

// Run executes the agent on a task.
func (a *ToolCallingAgent) Run(ctx context.Context, task string, opts ...RunOption) (*RunResult, error) {
	options := &RunOptions{MaxSteps: a.maxSteps, Reset: true}
	for _, opt := range opts {
		opt(options)
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	startTime := time.Now()
	if options.Reset {
		a.memory.Reset()
	}
	a.memory.AddStep(&TaskStep{Task: task, Images: options.Images})

	var finalOutput any
	state := "success"

	for step := 1; step <= options.MaxSteps; step++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		actionStep := &ActionStep{StepNumber: step, Timing: Timing{StartTime: time.Now()}}
		msgs := a.memory.ToMessages()
		toolList := a.allTools()

		resp, err := a.model.Generate(ctx, msgs, WithTools(toolList...))
		if err != nil {
			actionStep.Error = err
			actionStep.Timing = NewTiming(actionStep.Timing.StartTime)
			a.memory.AddStep(actionStep)
			continue
		}

		actionStep.ModelOutput = resp.Content
		actionStep.TokenUsage = resp.TokenUsage

		if len(resp.ToolCalls) > 0 {
			actionStep.ToolCalls = resp.ToolCalls
			var observations []string

			for _, tc := range resp.ToolCalls {
				result, err := a.executeTool(tc)
				if err != nil {
					observations = append(observations, fmt.Sprintf("Error executing %s: %v", tc.Name, err))
				} else {
					observations = append(observations, fmt.Sprintf("%v", result))
					if tc.Name == "final_answer" {
						actionStep.IsFinal = true
						finalOutput = result
					}
				}
			}
			actionStep.Observations = strings.Join(observations, "\n")
		}

		actionStep.Timing = NewTiming(actionStep.Timing.StartTime)
		a.memory.AddStep(actionStep)
		a.callbacks.Trigger(actionStep)

		if actionStep.IsFinal {
			a.memory.AddStep(&FinalAnswerStep{Output: finalOutput})
			break
		}
	}

	if finalOutput == nil {
		state = "max_steps_error"
	}

	tokens := a.memory.TotalTokens()
	return &RunResult{
		Output:     finalOutput,
		State:      state,
		Steps:      a.memory.Steps,
		TokenUsage: &tokens,
		Timing:     NewTiming(startTime),
	}, nil
}

func (a *BaseAgent) allTools() []Tool {
	tools := make([]Tool, 0)
	for _, t := range a.tools.All() {
		tools = append(tools, t)
	}
	for name, agent := range a.managedAgents {
		tools = append(tools, &agentTool{name: name, agent: agent})
	}
	return tools
}

func (a *BaseAgent) executeTool(tc ToolCall) (any, error) {
	if agent, ok := a.managedAgents[tc.Name]; ok {
		taskArg, _ := tc.Arguments["task"].(string)
		result, err := agent.Run(context.Background(), taskArg)
		if err != nil {
			return nil, err
		}
		return result.Output, nil
	}

	tool, ok := a.tools.Get(tc.Name)
	if !ok {
		return nil, fmt.Errorf("unknown tool: %s", tc.Name)
	}
	return tool.Execute(tc.Arguments)
}

type agentTool struct {
	name  string
	agent Agent
}

func (t *agentTool) Name() string        { return t.name }
func (t *agentTool) Description() string { return t.agent.Description() }
func (t *agentTool) OutputType() string  { return "string" }
func (t *agentTool) Inputs() map[string]ToolInput {
	return map[string]ToolInput{
		"task": {Type: "string", Description: "Task for this agent", Required: true},
	}
}
func (t *agentTool) Execute(args map[string]any) (any, error) {
	task, _ := args["task"].(string)
	result, err := t.agent.Run(context.Background(), task)
	if err != nil {
		return nil, err
	}
	return result.Output, nil
}

// CodeExecutor runs code and returns results.
type CodeExecutor interface {
	Execute(code string, state map[string]any) (output any, logs string, err error)
}

// CodeAgent executes actions as code.
type CodeAgent struct {
	BaseAgent
	executor  CodeExecutor
	execState map[string]any
}

// NewCodeAgent creates a code-executing agent.
func NewCodeAgent(executor CodeExecutor, opts ...AgentOption) *CodeAgent {
	a := &CodeAgent{
		BaseAgent: BaseAgent{
			tools:         NewToolRegistry(),
			managedAgents: make(map[string]Agent),
			callbacks:     NewCallbackRegistry(),
			maxSteps:      20,
		},
		executor:  executor,
		execState: make(map[string]any),
	}
	a.tools.Register(NewFinalAnswerTool())

	for _, opt := range opts {
		opt(&a.BaseAgent)
	}

	if a.systemPrompt == "" {
		a.systemPrompt = defaultCodeAgentPrompt(a.tools)
	}
	a.memory = NewMemory(a.systemPrompt)

	return a
}

// Run executes the code agent.
func (a *CodeAgent) Run(ctx context.Context, task string, opts ...RunOption) (*RunResult, error) {
	options := &RunOptions{MaxSteps: a.maxSteps, Reset: true}
	for _, opt := range opts {
		opt(options)
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	startTime := time.Now()
	if options.Reset {
		a.memory.Reset()
		a.execState = make(map[string]any)
	}
	a.memory.AddStep(&TaskStep{Task: task})

	var finalOutput any
	state := "success"

	for step := 1; step <= options.MaxSteps; step++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		actionStep := &ActionStep{StepNumber: step, Timing: Timing{StartTime: time.Now()}}
		msgs := a.memory.ToMessages()

		resp, err := a.model.Generate(ctx, msgs, WithStopSequences("Observation:", "</code>"))
		if err != nil {
			actionStep.Error = err
			a.memory.AddStep(actionStep)
			continue
		}

		actionStep.ModelOutput = resp.Content
		actionStep.TokenUsage = resp.TokenUsage

		code := parseCodeBlock(resp.Content)
		if code == "" {
			actionStep.Error = fmt.Errorf("no code block found")
			a.memory.AddStep(actionStep)
			continue
		}
		actionStep.CodeAction = code

		output, logs, err := a.executor.Execute(code, a.execState)
		if err != nil {
			actionStep.Error = err
			actionStep.Observations = logs
		} else {
			actionStep.Observations = logs
			if isFinalAnswer(code) {
				actionStep.IsFinal = true
				finalOutput = output
			}
		}

		actionStep.Timing = NewTiming(actionStep.Timing.StartTime)
		a.memory.AddStep(actionStep)
		a.callbacks.Trigger(actionStep)

		if actionStep.IsFinal {
			a.memory.AddStep(&FinalAnswerStep{Output: finalOutput})
			break
		}
	}

	if finalOutput == nil {
		state = "max_steps_error"
	}

	tokens := a.memory.TotalTokens()
	return &RunResult{
		Output:     finalOutput,
		State:      state,
		Steps:      a.memory.Steps,
		TokenUsage: &tokens,
		Timing:     NewTiming(startTime),
	}, nil
}

func parseCodeBlock(text string) string {
	// Try <code>...</code> pattern (may be cut off by stop sequence)
	re := regexp.MustCompile(`(?s)<code>(.*?)(?:</code>|$)`)
	matches := re.FindStringSubmatch(text)
	if len(matches) > 1 && strings.TrimSpace(matches[1]) != "" {
		return strings.TrimSpace(matches[1])
	}
	// Try markdown code blocks
	re = regexp.MustCompile("(?s)```(?:python|py)?\\n?(.*?)(?:```|$)")
	matches = re.FindStringSubmatch(text)
	if len(matches) > 1 && strings.TrimSpace(matches[1]) != "" {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

func isFinalAnswer(code string) bool {
	return strings.Contains(code, "final_answer(")
}

func defaultToolCallingPrompt(tools *ToolRegistry) string {
	return fmt.Sprintf(`You are an expert assistant. Use tools to solve tasks.

Available tools:
%s

Always use tools when needed. Call final_answer when done.`, tools.ToCodePrompt())
}

func defaultCodeAgentPrompt(tools *ToolRegistry) string {
	return fmt.Sprintf(`You are an expert assistant who solves tasks using code.

Write Python code in <code></code> blocks. Use print() for intermediate results.
Call final_answer(result) when done.

Available tools as functions:
%s

Example:
Thought: I need to search for information.
<code>
result = web_search("query")
print(result)
</code>`, tools.ToCodePrompt())
}
