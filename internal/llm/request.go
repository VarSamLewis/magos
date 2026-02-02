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

// SendMessage sends a user message to Claude and returns the response.
func (c *Client) SendMessage(ctx context.Context, userMessage string) (*anthropic.Message, error) {
	c.logger.Debug("sending message to Claude", "message_length", len(userMessage))

	message, err := c.anthropic.Messages.New(ctx, anthropic.MessageNewParams{
		MaxTokens: 1024,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(userMessage)),
		},
		Model: anthropic.ModelClaudeSonnet4_5_20250929,
	})
	if err != nil {
		c.logger.Error("failed to send message to Claude", "error", err)
		return nil, err
	}

	c.logger.Debug("received response from Claude", "content_blocks", len(message.Content))

	return message, nil
}
