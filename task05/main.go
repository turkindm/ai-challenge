package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
)

const defaultGatewayURL = "https://api.anthropic.com"

const defaultTask = `В prod упал сервис: p99 latency выросла с 50ms до 8s, CPU в норме, память растёт, БД нагрузка в норме.
Опиши план диагностики: ровно 5 шагов, каждый — одно предложение. Без кода, без подзаголовков.`

// Актуальные цены Anthropic ($/1M токенов)
// https://www.anthropic.com/pricing
var models = []Model{
	{
		ID:               "claude-opus-4-6",
		Label:            "Сильная (Opus 4.6)",
		InputPricePer1M:  15.00,
		OutputPricePer1M: 75.00,
	},
	{
		ID:               "claude-sonnet-4-6",
		Label:            "Средняя (Sonnet 4.6)",
		InputPricePer1M:  3.00,
		OutputPricePer1M: 15.00,
	},
	{
		ID:               "claude-haiku-4-5-20251001",
		Label:            "Слабая (Haiku 4.5)",
		InputPricePer1M:  0.80,
		OutputPricePer1M: 4.00,
	},
}

type Model struct {
	ID               string
	Label            string
	InputPricePer1M  float64
	OutputPricePer1M float64
}

func (m Model) Cost(inputTokens, outputTokens int) float64 {
	return float64(inputTokens)/1_000_000*m.InputPricePer1M +
		float64(outputTokens)/1_000_000*m.OutputPricePer1M
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Request struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	Messages  []Message `json:"messages"`
	System    string    `json:"system,omitempty"`
}

type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type Response struct {
	Content []ContentBlock `json:"content"`
	Usage   Usage          `json:"usage"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type Result struct {
	Text         string
	InputTokens  int
	OutputTokens int
	Elapsed      time.Duration
}

func ask(apiKey, gatewayURL string, model Model, system, userMessage string, maxTokens int) (Result, error) {
	reqBody := Request{
		Model:     model.ID,
		MaxTokens: maxTokens,
		Messages:  []Message{{Role: "user", Content: userMessage}},
		System:    system,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return Result{}, fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequest("POST", gatewayURL+"/v1/messages", bytes.NewReader(data))
	if err != nil {
		return Result{}, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	elapsed := time.Since(start)
	if err != nil {
		return Result{}, fmt.Errorf("read body: %w", err)
	}

	var result Response
	if err := json.Unmarshal(body, &result); err != nil {
		return Result{}, fmt.Errorf("unmarshal: %w", err)
	}
	if result.Error != nil {
		return Result{}, fmt.Errorf("API error: %s", result.Error.Message)
	}

	var text string
	for _, block := range result.Content {
		if block.Type == "text" {
			text += block.Text
		}
	}

	return Result{
		Text:         text,
		InputTokens:  result.Usage.InputTokens,
		OutputTokens: result.Usage.OutputTokens,
		Elapsed:      elapsed,
	}, nil
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

	message := flag.String("message", defaultTask, "prompt to send to all models")
	system := flag.String("system", "Ты полезный ассистент.", "system prompt")
	maxTokens := flag.Int("max_tokens", 1024, "maximum number of tokens to generate")
	flag.Parse()

	fmt.Printf("Запрос: %s\n\n", *message)

	type summary struct {
		label        string
		inputTokens  int
		outputTokens int
		cost         float64
		elapsed      time.Duration
	}
	var summaries []summary

	for _, model := range models {
		fmt.Printf("=== %s ===\n", model.Label)

		res, err := ask(apiKey, gatewayURL, model, *system, *message, *maxTokens)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ошибка: %v\n\n", err)
			continue
		}

		cost := model.Cost(res.InputTokens, res.OutputTokens)
		fmt.Println(res.Text)
		fmt.Printf("  Время:   %v\n", res.Elapsed.Round(time.Millisecond))
		fmt.Printf("  Токены:  вход=%d  выход=%d  всего=%d\n",
			res.InputTokens, res.OutputTokens, res.InputTokens+res.OutputTokens)
		fmt.Printf("  Цена:    $%.6f\n", cost)
		fmt.Println()

		summaries = append(summaries, summary{
			label:        model.Label,
			inputTokens:  res.InputTokens,
			outputTokens: res.OutputTokens,
			cost:         cost,
			elapsed:      res.Elapsed,
		})
	}

	if len(summaries) < 2 {
		return
	}

	fmt.Println("=== Сводка ===")
	fmt.Printf("%-26s  %9s  %8s  %8s  %10s\n", "Модель", "Время", "Вх.тк.", "Вых.тк.", "Цена ($)")
	fmt.Printf("%-26s  %9s  %8s  %8s  %10s\n",
		"──────────────────────────", "─────────", "──────", "───────", "─────────")
	for _, s := range summaries {
		fmt.Printf("%-26s  %9v  %8d  %8d  %10.6f\n",
			s.label, s.elapsed.Round(time.Millisecond),
			s.inputTokens, s.outputTokens, s.cost)
	}
}
