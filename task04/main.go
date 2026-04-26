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

const defaultTask = `Придумай 5 названий для внутреннего CLI-инструмента, 
который помогает разработчикам управлять локальным окружением (запуск сервисов, миграции, сиды БД).`

var temperatures = []float64{0, 0.7, 1.0}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Request struct {
	Model       string    `json:"model"`
	MaxTokens   int       `json:"max_tokens"`
	Messages    []Message `json:"messages"`
	System      string    `json:"system,omitempty"`
	Temperature *float64  `json:"temperature,omitempty"`
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

func ask(apiKey, gatewayURL, system, userMessage string, temperature *float64) (string, error) {
	reqBody := Request{
		Model:       "claude-haiku-4-5-20251001",
		MaxTokens:   1024,
		Messages:    []Message{{Role: "user", Content: userMessage}},
		System:      system,
		Temperature: temperature,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequest("POST", gatewayURL+"/v1/messages", bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}

	var result Response
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("unmarshal: %w", err)
	}

	if result.Error != nil {
		return "", fmt.Errorf("API error: %s", result.Error.Message)
	}

	var text string
	for _, block := range result.Content {
		if block.Type == "text" {
			text += block.Text
		}
	}
	return text, nil
}

func temperatureLabel(t float64) string {
	switch {
	case t == 0:
		return "детерминированный"
	case t < 1.0:
		return "сбалансированный"
	default:
		return "максимально случайный"
	}
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

	message := flag.String("message", defaultTask, "prompt to send with different temperatures")
	system := flag.String("system", "Ты полезный ассистент.", "system prompt")
	flag.Parse()

	task := *message
	fmt.Printf("Запрос: %s\n\n", task)

	for _, temp := range temperatures {
		t := temp // capture for pointer
		fmt.Printf("=== temperature=%.1f (%s) ===\n", t, temperatureLabel(t))

		result, err := ask(apiKey, gatewayURL, *system, task, &t)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ошибка: %v\n", err)
		} else {
			fmt.Println(result)
		}
		fmt.Println()
	}
}
