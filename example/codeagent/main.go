package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/gocnn/neko"
	"github.com/gocnn/neko/exec"
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

	// Create Python executor
	pyExecutor := exec.NewPythonExecutor(
		exec.WithPythonPath("python"),
	)

	// Create CodeAgent (neko' innovation - code as actions)
	agent := neko.NewCodeAgent(
		pyExecutor,
		neko.WithModel(neko.NewOpenAIModelWithBaseURL(modelID, apiKey, baseURL)),
		neko.WithToolList(
			tool.NewWebSearchTool(5),
			tool.NewCalculatorTool(),
		),
		neko.WithAgentMaxSteps(10),
	)

	// Run task - agent will write Python code to solve it
	ctx := context.Background()
	result, err := agent.Run(ctx,
		"Calculate the factorial of 10, then find its square root.")
	if err != nil {
		log.Fatalf("Agent error: %v", err)
	}

	fmt.Printf("Result: %v\n", result.Output)
	fmt.Printf("Steps taken: %d\n", len(result.Steps))

	// Print generated code and observations from each step
	for _, step := range result.Steps {
		if actionStep, ok := step.(*neko.ActionStep); ok {
			if actionStep.CodeAction != "" {
				fmt.Printf("\n--- Step %d Code ---\n%s\n",
					actionStep.StepNumber, actionStep.CodeAction)
			}
			if actionStep.Observations != "" {
				fmt.Printf("--- Observations ---\n%s\n", actionStep.Observations)
			}
			if actionStep.Error != nil {
				fmt.Printf("--- Error ---\n%v\n", actionStep.Error)
			}
			if actionStep.IsFinal {
				fmt.Printf("--- FINAL STEP ---\n")
			}
		}
	}
}
