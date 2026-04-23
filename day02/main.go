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

type OutputFormat struct {
	Type   string         `json:"type"`
	Schema map[string]any `json:"schema"`
}

type OutputConfig struct {
	Format *OutputFormat `json:"format,omitempty"`
}

type Request struct {
	Model         string        `json:"model"`
	MaxTokens     int           `json:"max_tokens"`
	Messages      []Message     `json:"messages"`
	System        string        `json:"system,omitempty"`
	StopSequences []string      `json:"stop_sequences,omitempty"`
	OutputConfig  *OutputConfig `json:"output_config,omitempty"`
}

type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type Response struct {
	Content    []ContentBlock `json:"content"`
	StopReason string         `json:"stop_reason"`
	StopSeq    *string        `json:"stop_sequence"`
	Error      *struct {
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
	system := flag.String("system", "", "system prompt defining the model's role and behaviour")
	schema := flag.String("output_schema", "", "JSON schema string for structured output (output_config.format)")
	maxTokens := flag.Int("max_tokens", 1024, "maximum number of tokens to generate")
	stop := flag.String("stop_sequences", "", "stop sequence — generation halts when this string is produced")
	verbose := flag.Bool("verbose", false, "print raw API response JSON")
	flag.Parse()

	if *message == "" {
		fmt.Fprintln(os.Stderr, "usage: day02 -message <text> [-system <prompt>] [-output_schema <json-schema>] [-max_tokens <n>] [-stop_sequences <seq>]")
		os.Exit(1)
	}

	reqBody := Request{
		Model:     "claude-haiku-4-5-20251001",
		MaxTokens: *maxTokens,
		Messages:  []Message{{Role: "user", Content: *message}},
		System:    *system,
	}

	if *stop != "" {
		reqBody.StopSequences = []string{*stop}
	}

	if *schema != "" {
		var schemaObj map[string]any
		if err := json.Unmarshal([]byte(*schema), &schemaObj); err != nil {
			fmt.Fprintf(os.Stderr, "error: -output_schema must be a valid JSON schema object: %v\n", err)
			os.Exit(1)
		}
		reqBody.OutputConfig = &OutputConfig{
			Format: &OutputFormat{
				Type:   "json_schema",
				Schema: schemaObj,
			},
		}
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
	req.Header.Set("anthropic-version", "2023-06-01")

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

	if *verbose {
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, body, "", "  "); err != nil {
			fmt.Fprintf(os.Stderr, "error formatting response: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(pretty.String())
		return
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
			fmt.Print(block.Text)
		}
	}
	fmt.Println()

	if result.StopReason == "stop_sequence" && result.StopSeq != nil {
		fmt.Fprintf(os.Stderr, "[stopped at sequence: %q]\n", *result.StopSeq)
	}
}
