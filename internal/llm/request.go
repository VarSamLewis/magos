package llm

import (
	"context"
	"errors"
	"log/slog"
	"os"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

var (
	ErrMissingAPIKey = errors.New("MAGOS_ANTHROPIC_API_KEY environment variable not set")
)

// Client wraps the Anthropic API client with logging capabilities.
type Client struct {
	anthropic *anthropic.Client
	logger    *slog.Logger
}

// NewClient creates a new LLM client using the MAGOS_ANTHROPIC_API_KEY environment variable.
func NewClient(logger *slog.Logger) (*Client, error) {
	apiKey := os.Getenv("MAGOS_ANTHROPIC_API_KEY")
	if apiKey == "" {
		logger.Error("API key not found in environment")
		return nil, ErrMissingAPIKey
	}

	client := anthropic.NewClient(
		option.WithAPIKey(apiKey),
	)

	logger.Info("LLM client initialized successfully")

	return &Client{
		anthropic: &client,
		logger:    logger,
	}, nil
}

// SendMessage sends a conversation history to Claude and returns the response.
// The history should be a proper sequence of alternating user/assistant messages.
func (c *Client) SendMessage(ctx context.Context, history []anthropic.MessageParam) (*anthropic.Message, error) {
	c.logger.Debug("sending message to Claude", "turns", len(history))

	fileTree := []string{"main.go", "cmd/root.go", "internal/tui/model.go"}
	systemPrompt := BuildSystemPrompt("/tmp", fileTree)

	message, err := c.anthropic.Messages.New(ctx, anthropic.MessageNewParams{
		MaxTokens: 4096,
		System: []anthropic.TextBlockParam{
			{Text: systemPrompt},
		},
		Messages: history,
		Model:    anthropic.ModelClaudeSonnet4_5_20250929,
	})
	if err != nil {
		c.logger.Error("failed to send message to Claude", "error", err)
		return nil, err
	}

	c.logger.Debug("received response from Claude", "content_blocks", len(message.Content))

	return message, nil
}
