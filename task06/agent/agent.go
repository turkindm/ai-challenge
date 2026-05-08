package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	defaultGatewayURL = "https://api.anthropic.com"
	defaultModel      = "claude-haiku-4-5-20251001"
	anthropicVersion  = "2023-06-01"
)

// Message — одно сообщение в диалоге.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Agent инкапсулирует логику общения с Anthropic API.
// Каждый запрос независим — агент не хранит историю диалога.
type Agent struct {
	apiKey       string
	gatewayURL   string
	modelID      string
	systemPrompt string
	client       *http.Client
}

// Option позволяет конфигурировать агента при создании.
type Option func(*Agent)

// WithModel задаёт модель Anthropic.
func WithModel(modelID string) Option {
	return func(a *Agent) { a.modelID = modelID }
}

// WithSystem задаёт системный промпт.
func WithSystem(system string) Option {
	return func(a *Agent) { a.systemPrompt = system }
}

// WithGatewayURL переопределяет URL шлюза.
func WithGatewayURL(url string) Option {
	return func(a *Agent) { a.gatewayURL = url }
}

// New создаёт нового агента.
func New(apiKey string, opts ...Option) *Agent {
	a := &Agent{
		apiKey:       apiKey,
		gatewayURL:   defaultGatewayURL,
		modelID:      defaultModel,
		systemPrompt: "Ты полезный ассистент. Отвечай кратко и по делу.",
		client:       &http.Client{Timeout: 60 * time.Second},
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// Send отправляет сообщение пользователя агенту и возвращает ответ модели.
// Каждый запрос независим — история не сохраняется.
func (a *Agent) Send(ctx context.Context, userMessage string) (string, error) {
	reqBody := struct {
		Model     string    `json:"model"`
		MaxTokens int       `json:"max_tokens"`
		System    string    `json:"system,omitempty"`
		Messages  []Message `json:"messages"`
	}{
		Model:     a.modelID,
		MaxTokens: 2048,
		System:    a.systemPrompt,
		Messages:  []Message{{Role: "user", Content: userMessage}},
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		a.gatewayURL+"/v1/messages", bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)

	resp, err := a.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
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

// ModelID возвращает ID текущей модели.
func (a *Agent) ModelID() string {
	return a.modelID
}
