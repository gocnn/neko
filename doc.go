// Package neko provides a lightweight framework for building AI agents.
//
// neko is a Go implementation of the neko Python framework,
// enabling the creation of LLM-powered agents that can use tools and
// collaborate with other agents.
//
// Key Features:
//   - Simple, minimal abstractions
//   - Support for both tool-calling and code-executing agents
//   - Multi-agent hierarchies with managed agents
//   - Model-agnostic design (OpenAI, Anthropic, etc.)
//   - Safe code execution via external interpreters
//
// Basic Usage:
//
//	model := neko.NewOpenAIModel("gpt-4", apiKey)
//	agent := neko.NewToolCallingAgent(
//	    neko.WithModel(model),
//	    neko.WithToolList(tools.NewCalculatorTool()),
//	)
//	result, _ := agent.Run(ctx, "What is 2 + 2?")
//
// For more examples, see the example/ directory.
package neko
