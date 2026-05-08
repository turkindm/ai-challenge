package main

import (
	"fmt"
	"os"

	"ai-challenge/task06/agent"
	"ai-challenge/common/ui/chat"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load("../.env")

	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "error: ANTHROPIC_API_KEY is not set")
		os.Exit(1)
	}

	gatewayURL := os.Getenv("ANTHROPIC_GATEWAY_URL")

	opts := []agent.Option{
		agent.WithModel("claude-haiku-4-5-20251001"),
		agent.WithSystem("Ты полезный ассистент. Отвечай по-русски, кратко и по делу."),
	}
	if gatewayURL != "" {
		opts = append(opts, agent.WithGatewayURL(gatewayURL))
	}

	ag := agent.New(apiKey, opts...)

	if err := chat.Run(ag); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
