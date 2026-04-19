// Package advisor is the Claude-powered strategic advisor for the Council.
// It talks to the Anthropic Messages API, builds situational context from
// trading + finance state, and drives multi-turn threads grounded in the
// user's real numbers (not generic advice).
package advisor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"
)

// DefaultModel is the Claude model the advisor uses.
// Opus 4.7 balances reasoning quality with cost for multi-turn strategic work.
const DefaultModel = "claude-opus-4-7"

// MaxOutputTokens caps a single advisor reply.
const MaxOutputTokens = 4096

// AnthropicAPIBase is the Messages API endpoint.
const AnthropicAPIBase = "https://api.anthropic.com/v1/messages"

// AnthropicAPIVersion is the expected header value.
const AnthropicAPIVersion = "2023-06-01"

// Client wraps the Anthropic Messages API with the advisor's conventions.
type Client struct {
	apiKey string
	model  string
	http   *http.Client
	logger *slog.Logger
}

// NewClient constructs a Claude API client from CLAUDE_API_KEY in env.
// Returns nil (with no error) if the key isn't set — callers must handle that.
func NewClient(logger *slog.Logger) *Client {
	key := os.Getenv("CLAUDE_API_KEY")
	if key == "" {
		if logger != nil {
			logger.Warn("CLAUDE_API_KEY not set — advisor will be disabled")
		}
		return nil
	}
	return &Client{
		apiKey: key,
		model:  DefaultModel,
		http:   &http.Client{Timeout: 120 * time.Second},
		logger: logger,
	}
}

// Message is an advisor-facing message for the Messages API.
// Only user/assistant roles are sent; system prompt goes in the system param.
type Message struct {
	Role    string `json:"role"` // "user" | "assistant"
	Content string `json:"content"`
}

type messagesRequest struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	System    string    `json:"system,omitempty"`
	Messages  []Message `json:"messages"`
}

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type messagesResponse struct {
	ID      string         `json:"id"`
	Type    string         `json:"type"`
	Role    string         `json:"role"`
	Model   string         `json:"model"`
	Content []contentBlock `json:"content"`
	Usage   usage          `json:"usage"`
	Error   *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Reply is the assistant's response after a turn.
type Reply struct {
	Content   string
	Model     string
	TokensIn  int
	TokensOut int
}

// Complete sends a system prompt + message history to Claude and returns the reply.
func (c *Client) Complete(ctx context.Context, system string, history []Message) (*Reply, error) {
	if c == nil {
		return nil, fmt.Errorf("advisor not configured (CLAUDE_API_KEY missing)")
	}
	reqBody := messagesRequest{
		Model:     c.model,
		MaxTokens: MaxOutputTokens,
		System:    system,
		Messages:  history,
	}
	raw, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", AnthropicAPIBase, bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", AnthropicAPIVersion)

	res, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http call: %w", err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)

	var parsed messagesResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("decode response (status %d): %w — body: %s", res.StatusCode, err, truncate(string(body), 500))
	}
	if res.StatusCode >= 400 || parsed.Error != nil {
		msg := ""
		if parsed.Error != nil {
			msg = parsed.Error.Message
		}
		return nil, fmt.Errorf("anthropic API error (status %d): %s", res.StatusCode, msg)
	}

	// Concat all text blocks — advisor doesn't use tool use yet.
	var text string
	for _, b := range parsed.Content {
		if b.Type == "text" {
			text += b.Text
		}
	}
	return &Reply{
		Content:   text,
		Model:     parsed.Model,
		TokensIn:  parsed.Usage.InputTokens,
		TokensOut: parsed.Usage.OutputTokens,
	}, nil
}

// Available reports whether the client is usable.
func (c *Client) Available() bool { return c != nil && c.apiKey != "" }

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
