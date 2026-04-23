package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/joho/godotenv"
)

const defaultGatewayURL = "https://api.anthropic.com"

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Request struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	Messages  []Message `json:"messages"`
}

type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type Response struct {
	Content []ContentBlock `json:"content"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func main() {
	_ = godotenv.Load("../.env")

	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "error: ANTHROPIC_API_KEY is not set")
		os.Exit(1)
	}

	gatewayURL := os.Getenv("ANTHROPIC_GATEWAY_URL")
	if gatewayURL == "" {
		gatewayURL = defaultGatewayURL
	}

	message := flag.String("message", "", "message to send to the model")
	flag.Parse()

	if *message == "" {
		fmt.Fprintln(os.Stderr, "usage: task01 -message <text>")
		os.Exit(1)
	}
	userMessage := *message

	reqBody := Request{
		Model:     "claude-haiku-4-5-20251001",
		MaxTokens: 1024,
		Messages: []Message{
			{Role: "user", Content: userMessage},
		},
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error marshaling request: %v\n", err)
		os.Exit(1)
	}

	req, err := http.NewRequest("POST", gatewayURL+"/v1/messages", bytes.NewReader(data))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating request: %v\n", err)
		os.Exit(1)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error sending request: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading response: %v\n", err)
		os.Exit(1)
	}

	var result Response
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Fprintf(os.Stderr, "error parsing response: %v\n", err)
		os.Exit(1)
	}

	if result.Error != nil {
		fmt.Fprintf(os.Stderr, "API error: %s\n", result.Error.Message)
		os.Exit(1)
	}

	for _, block := range result.Content {
		if block.Type == "text" {
			fmt.Println(block.Text)
		}
	}
}
