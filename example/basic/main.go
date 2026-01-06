package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/gocnn/neko"
	"github.com/gocnn/neko/tool"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found")
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	baseURL := os.Getenv("OPENAI_BASE_URL")
	modelID := os.Getenv("OPENAI_MODEL")

	// Create agent with tools
	agent := neko.NewToolCallingAgent(
		neko.WithModel(neko.NewOpenAIModelWithBaseURL(modelID, apiKey, baseURL)),
		neko.WithToolList(
			tool.NewWebSearchTool(5),
			tool.NewCalculatorTool(),
		),
		neko.WithAgentMaxSteps(10),
	)

	// Run task
	ctx := context.Background()
	result, err := agent.Run(ctx, "What is 15 * 23 + 100?")
	if err != nil {
		log.Fatalf("Agent error: %v", err)
	}

	fmt.Printf("Result: %v\n", result.Output)
	fmt.Printf("Steps: %d\n", len(result.Steps))
	fmt.Printf("Tokens: %d\n", result.TokenUsage.Total())
}
