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

// defaultTask — задача по умолчанию, если пользователь не передал -message.
const defaultTask = `Необходимо реализовать систему автоматического управления шторой. Какой микроконтроллер использовать для реализации?`

// promptSimple — вариант 1: обычный запрос без дополнительных инструкций.
const promptSimple = "Ты полезный ассистент. Отвечай чётко и по делу."

// promptStepByStep — вариант 2: инструкция решать пошагово.
const promptStepByStep = "Ты полезный ассистент. Реши задачу пошагово"

// promptMetaPrefix — вариант 3 (шаг 1): просим LLM составить промпт для решения задачи.
const promptMetaPrefix = "Ты эксперт по составлению промптов. Получив задачу, сформулируй оптимальный system prompt для её решения. Верни только текст промпта без пояснений."

// promptExpertTemplate — шаблон системного промпта для варианта 4.
// %s заменяется на название роли эксперта.
const promptExpertTemplate = "Ты %s. Реши предложенную задачу."

// experts — список ролей для варианта 4.
var experts = []struct {
	icon string
	name string
}{
	{"🔍", "Аналитик"},
	{"⚙️", "Инженер"},
	{"🧐", "Критик"},
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

func ask(apiKey, gatewayURL, system, userMessage string) (string, error) {
	reqBody := Request{
		Model:     "claude-haiku-4-5-20251001",
		MaxTokens: 1024,
		Messages:  []Message{{Role: "user", Content: userMessage}},
		System:    system,
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

// solveWithGeneratedPrompt — вариант 3: сначала просим LLM составить промпт,
// затем используем этот промпт как system prompt для решения задачи.
func solveWithGeneratedPrompt(apiKey, gatewayURL, task string) (string, error) {
	generatedPrompt, err := ask(apiKey, gatewayURL, promptMetaPrefix, task)
	if err != nil {
		return "", fmt.Errorf("генерация промпта: %w", err)
	}

	fmt.Printf("  [сгенерированный промпт]: %s\n\n", generatedPrompt)

	return ask(apiKey, gatewayURL, generatedPrompt, task)
}

func printVariant(n int, title string) {
	fmt.Printf("=== Вариант %d: %s ===\n", n, title)
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

	message := flag.String("message", defaultTask, "task to solve (uses built-in default if omitted)")
	flag.Parse()

	if *message == "" {
		fmt.Fprintln(os.Stderr, "error: no task provided and defaultTask is empty")
		os.Exit(1)
	}

	task := *message

	// Вариант 1: обычный запрос
	printVariant(1, "обычный")
	if result, err := ask(apiKey, gatewayURL, promptSimple, task); err != nil {
		fmt.Fprintf(os.Stderr, "ошибка: %v\n", err)
	} else {
		fmt.Println(result)
	}
	fmt.Println()

	// Вариант 2: пошаговое решение
	printVariant(2, "пошаговое решение")
	if result, err := ask(apiKey, gatewayURL, promptStepByStep, task); err != nil {
		fmt.Fprintf(os.Stderr, "ошибка: %v\n", err)
	} else {
		fmt.Println(result)
	}
	fmt.Println()

	// Вариант 3: LLM генерирует промпт, затем решает задачу им
	printVariant(3, "LLM-сгенерированный промпт")
	if result, err := solveWithGeneratedPrompt(apiKey, gatewayURL, task); err != nil {
		fmt.Fprintf(os.Stderr, "ошибка: %v\n", err)
	} else {
		fmt.Println(result)
	}
	fmt.Println()

	// Вариант 4: группа экспертов — каждый решает одно и то же задание
	printVariant(4, "группа экспертов")
	for _, expert := range experts {
		fmt.Printf("--- %s %s ---\n", expert.icon, expert.name)
		if result, err := ask(apiKey, gatewayURL, fmt.Sprintf(promptExpertTemplate, expert.name), task); err != nil {
			fmt.Fprintf(os.Stderr, "ошибка: %v\n", err)
		} else {
			fmt.Println(result)
		}
		fmt.Println()
	}
}
