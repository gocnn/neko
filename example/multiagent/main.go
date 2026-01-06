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

	model := neko.NewOpenAIModelWithBaseURL(modelID, apiKey, baseURL)

	// Create specialized sub-agents
	searchAgent := neko.NewToolCallingAgent(
		neko.WithModel(model),
		neko.WithName("web_researcher"),
		neko.WithDescription("Expert at finding information on the web"),
		neko.WithToolList(tool.NewWebSearchTool(10)),
	)

	mathAgent := neko.NewToolCallingAgent(
		neko.WithModel(model),
		neko.WithName("mathematician"),
		neko.WithDescription("Expert at mathematical calculations"),
		neko.WithToolList(tool.NewCalculatorTool()),
	)

	// Create orchestrator agent with managed agents
	orchestrator := neko.NewToolCallingAgent(
		neko.WithModel(model),
		neko.WithManagedAgents(searchAgent, mathAgent),
		neko.WithAgentMaxSteps(15),
	)

	// Run complex task
	ctx := context.Background()
	result, err := orchestrator.Run(ctx,
		"Find the population of Tokyo and calculate what percentage it is of Japan's total population.")
	if err != nil {
		log.Fatalf("Agent error: %v", err)
	}

	fmt.Printf("Result: %v\n", result.Output)
	fmt.Printf("State: %s\n", result.State)
	fmt.Printf("Duration: %v\n", result.Timing.Duration)
}
