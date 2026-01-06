package neko

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

// Model is the interface for LLM backends.
type Model interface {
	Generate(ctx context.Context, messages []Message, opts ...GenerateOption) (*Message, error)
	ModelID() string
}

// GenerateOptions holds generation parameters.
type GenerateOptions struct {
	StopSequences []string
	Tools         []Tool
	Temperature   float64
	MaxTokens     int64
}

// GenerateOption is a functional option for Generate.
type GenerateOption func(*GenerateOptions)

// WithStopSequences sets stop sequences.
func WithStopSequences(seqs ...string) GenerateOption {
	return func(o *GenerateOptions) { o.StopSequences = seqs }
}

// WithTools provides tools for function calling.
func WithTools(tools ...Tool) GenerateOption {
	return func(o *GenerateOptions) { o.Tools = tools }
}

// WithTemperature sets generation temperature.
func WithTemperature(t float64) GenerateOption {
	return func(o *GenerateOptions) { o.Temperature = t }
}

// WithMaxTokens sets max output tokens.
func WithMaxTokens(n int64) GenerateOption {
	return func(o *GenerateOptions) { o.MaxTokens = n }
}

// OpenAIModel implements Model using official OpenAI Go SDK.
type OpenAIModel struct {
	client      openai.Client
	modelID     string
	temperature float64
	maxTokens   int64
}

// OpenAIOption configures OpenAIModel.
type OpenAIOption func(*OpenAIModel)

// WithOpenAITemperature sets default temperature.
func WithOpenAITemperature(t float64) OpenAIOption {
	return func(m *OpenAIModel) { m.temperature = t }
}

// WithOpenAIMaxTokens sets default max tokens.
func WithOpenAIMaxTokens(n int64) OpenAIOption {
	return func(m *OpenAIModel) { m.maxTokens = n }
}

// NewOpenAIModel creates an OpenAI model using the official SDK.
func NewOpenAIModel(modelID, apiKey string, opts ...OpenAIOption) *OpenAIModel {
	client := openai.NewClient(option.WithAPIKey(apiKey))
	m := &OpenAIModel{
		client:      client,
		modelID:     modelID,
		temperature: 0.7,
		maxTokens:   4096,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// NewOpenAIModelWithBaseURL creates an OpenAI-compatible model with custom base URL.
func NewOpenAIModelWithBaseURL(modelID, apiKey, baseURL string, opts ...OpenAIOption) *OpenAIModel {
	client := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL(baseURL),
	)
	m := &OpenAIModel{
		client:      client,
		modelID:     modelID,
		temperature: 0.7,
		maxTokens:   4096,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

func (m *OpenAIModel) ModelID() string { return m.modelID }

// Generate sends messages to OpenAI and returns response.
func (m *OpenAIModel) Generate(ctx context.Context, messages []Message, opts ...GenerateOption) (*Message, error) {
	options := &GenerateOptions{
		Temperature: m.temperature,
		MaxTokens:   m.maxTokens,
	}
	for _, opt := range opts {
		opt(options)
	}

	// Convert messages to OpenAI format
	oaiMsgs := m.convertMessages(messages)

	// Build request params
	params := openai.ChatCompletionNewParams{
		Model:       m.modelID,
		Messages:    oaiMsgs,
		Temperature: openai.Float(options.Temperature),
		MaxTokens:   openai.Int(options.MaxTokens),
	}

	// Add stop sequences if provided
	if len(options.StopSequences) > 0 {
		params.Stop = openai.ChatCompletionNewParamsStopUnion{
			OfStringArray: options.StopSequences,
		}
	}

	// Add tools if provided
	if len(options.Tools) > 0 {
		params.Tools = m.convertTools(options.Tools)
	}

	// Make the API call
	resp, err := m.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("openai completion failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response choices returned")
	}

	choice := resp.Choices[0]
	result := &Message{
		Role:    RoleAssistant,
		Content: choice.Message.Content,
		TokenUsage: &TokenUsage{
			InputTokens:  int(resp.Usage.PromptTokens),
			OutputTokens: int(resp.Usage.CompletionTokens),
		},
	}

	// Parse tool calls if present
	if len(choice.Message.ToolCalls) > 0 {
		for _, tc := range choice.Message.ToolCalls {
			var args map[string]any
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				args = map[string]any{"raw": tc.Function.Arguments}
			}
			result.ToolCalls = append(result.ToolCalls, ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: args,
			})
		}
	}

	return result, nil
}

func (m *OpenAIModel) convertMessages(messages []Message) []openai.ChatCompletionMessageParamUnion {
	result := make([]openai.ChatCompletionMessageParamUnion, 0, len(messages))
	for _, msg := range messages {
		switch msg.Role {
		case RoleSystem:
			result = append(result, openai.SystemMessage(msg.Content))
		case RoleUser:
			result = append(result, openai.UserMessage(msg.Content))
		case RoleAssistant:
			result = append(result, openai.AssistantMessage(msg.Content))
		case RoleTool:
			// Tool messages converted to user messages (like Python version)
			result = append(result, openai.UserMessage(msg.Content))
		}
	}
	return result
}

func (m *OpenAIModel) convertTools(tools []Tool) []openai.ChatCompletionToolUnionParam {
	result := make([]openai.ChatCompletionToolUnionParam, 0, len(tools))
	for _, tool := range tools {
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

		result = append(result, openai.ChatCompletionToolUnionParam{
			OfFunction: &openai.ChatCompletionFunctionToolParam{
				Function: openai.FunctionDefinitionParam{
					Name:        tool.Name(),
					Description: openai.String(tool.Description()),
					Parameters: openai.FunctionParameters{
						"type":       "object",
						"properties": props,
						"required":   required,
					},
				},
			},
		})
	}
	return result
}

// StreamingModel extends Model with streaming support.
type StreamingModel interface {
	Model
	GenerateStream(ctx context.Context, messages []Message, opts ...GenerateOption) (<-chan StreamDelta, error)
}

// StreamDelta represents a streaming chunk.
type StreamDelta struct {
	Content   string     `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	Done      bool       `json:"done"`
	Error     error      `json:"error,omitempty"`
}

// GenerateStream implements streaming generation using official SDK.
func (m *OpenAIModel) GenerateStream(ctx context.Context, messages []Message, opts ...GenerateOption) (<-chan StreamDelta, error) {
	options := &GenerateOptions{
		Temperature: m.temperature,
		MaxTokens:   m.maxTokens,
	}
	for _, opt := range opts {
		opt(options)
	}

	oaiMsgs := m.convertMessages(messages)

	params := openai.ChatCompletionNewParams{
		Model:       m.modelID,
		Messages:    oaiMsgs,
		Temperature: openai.Float(options.Temperature),
		MaxTokens:   openai.Int(options.MaxTokens),
	}

	if len(options.StopSequences) > 0 {
		params.Stop = openai.ChatCompletionNewParamsStopUnion{
			OfStringArray: options.StopSequences,
		}
	}

	stream := m.client.Chat.Completions.NewStreaming(ctx, params)

	ch := make(chan StreamDelta)
	go func() {
		defer close(ch)

		acc := openai.ChatCompletionAccumulator{}
		for stream.Next() {
			chunk := stream.Current()
			acc.AddChunk(chunk)

			// Send content delta
			if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
				ch <- StreamDelta{Content: chunk.Choices[0].Delta.Content}
			}

			// Check for finished tool calls
			if tool, ok := acc.JustFinishedToolCall(); ok {
				var args map[string]any
				json.Unmarshal([]byte(tool.Arguments), &args)
				ch <- StreamDelta{
					ToolCalls: []ToolCall{{
						ID:        tool.ID,
						Name:      tool.Name,
						Arguments: args,
					}},
				}
			}
		}

		if stream.Err() != nil {
			ch <- StreamDelta{Error: stream.Err(), Done: true}
			return
		}

		ch <- StreamDelta{Done: true}
	}()

	return ch, nil
}
