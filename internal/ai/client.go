// Package ai implements the seller AI assistant: an LLM client abstraction
// plus the safety pipeline required by docs/security.md before any draft
// reaches a human for approval. Nothing in this package can send a message
// to Fiverr — there is no such API (docs/fiverr-api-capabilities.md).
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type ChatMessage struct {
	Role    string // "user" or "assistant"
	Content string
}

type GenerateRequest struct {
	System    string
	Messages  []ChatMessage
	MaxTokens int
}

type GenerateResponse struct {
	Text         string
	InputTokens  int
	OutputTokens int
}

// LLMClient is deliberately narrow — just enough to drive the safety
// pipeline in pipeline.go — so swapping models or providers never touches
// business logic.
type LLMClient interface {
	Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error)
	ModelName() string
}

// AnthropicClient calls the official Anthropic Messages API directly over
// HTTPS. No unofficial or third-party SDK is used.
type AnthropicClient struct {
	apiKey string
	model  string
	http   *http.Client
}

func NewAnthropicClient(apiKey, model string) *AnthropicClient {
	return &AnthropicClient{apiKey: apiKey, model: model, http: &http.Client{Timeout: 60 * time.Second}}
}

func (a *AnthropicClient) ModelName() string { return a.model }

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
	MaxTokens int                `json:"max_tokens"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

func (a *AnthropicClient) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	if a.apiKey == "" {
		return nil, fmt.Errorf("anthropic: ANTHROPIC_API_KEY is not configured")
	}

	msgs := make([]anthropicMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		msgs = append(msgs, anthropicMessage{Role: m.Role, Content: m.Content})
	}
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1024
	}

	body, err := json.Marshal(anthropicRequest{Model: a.model, System: req.System, Messages: msgs, MaxTokens: maxTokens})
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", a.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := a.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic: request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var parsed anthropicResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("anthropic: malformed response (status %d): %w", resp.StatusCode, err)
	}
	if resp.StatusCode != http.StatusOK {
		if parsed.Error != nil {
			return nil, fmt.Errorf("anthropic: %s: %s", parsed.Error.Type, parsed.Error.Message)
		}
		return nil, fmt.Errorf("anthropic: request failed with status %d", resp.StatusCode)
	}

	var text string
	for _, c := range parsed.Content {
		if c.Type == "text" {
			text += c.Text
		}
	}

	return &GenerateResponse{
		Text:         text,
		InputTokens:  parsed.Usage.InputTokens,
		OutputTokens: parsed.Usage.OutputTokens,
	}, nil
}

// MockClient is used in tests and whenever ANTHROPIC_API_KEY is unset, so the
// rest of the application (including the frontend) keeps working in local
// dev without a real API key. It never fabricates a business promise — it
// echoes back a clearly-labeled placeholder.
type MockClient struct{}

func (MockClient) ModelName() string { return "mock-llm" }

func (MockClient) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	var last string
	if len(req.Messages) > 0 {
		last = req.Messages[len(req.Messages)-1].Content
	}
	text := fmt.Sprintf("[MOCK AI RESPONSE — no ANTHROPIC_API_KEY configured]\n\nBased on: %.200s", last)
	return &GenerateResponse{Text: text, InputTokens: len(last) / 4, OutputTokens: len(text) / 4}, nil
}
