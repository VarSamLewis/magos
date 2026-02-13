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

// GetBashTool returns the bash tool definition for Claude to use.
func GetBashTool() anthropic.ToolUnionParam {
	return anthropic.ToolUnionParam{
		OfTool: &anthropic.ToolParam{
			Name:        "bash",
			Description: anthropic.String("Execute a bash command in the isolated VM worktree. Use this to read files, modify code, run tests, check git status, or any other shell operations. The working directory is set to the git worktree root."),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: map[string]interface{}{
					"command": map[string]interface{}{
						"type":        "string",
						"description": "The bash command to execute. Can be a single command or multiple commands separated by && or ;",
					},
					"explanation": map[string]interface{}{
						"type":        "string",
						"description": "Brief explanation of what this command does and why you're running it",
					},
				},
				Required: []string{"command", "explanation"},
			},
		},
	}
}

// SendMessage sends a conversation history to Claude and returns the response.
// The history should be a proper sequence of alternating user/assistant messages.
func (c *Client) SendMessage(ctx context.Context, history []anthropic.MessageParam) (*anthropic.Message, error) {
	c.logger.Debug("sending message to Claude", "turns", len(history))

	systemPrompt := `You are Magos, an AI coding assistant with access to bash commands in an isolated VM.

The VM has access to:
- A git worktree with the user's code (current working directory)
- Standard development tools (git, compilers, linters, etc.)
- Language-specific validators

Use the bash tool to:
- Read files (cat, head, tail, grep)
- List directory contents (ls, find, tree)
- Modify code (sed, text editors, write files with cat > file <<EOF)
- Run tests and linters
- Check git status and diffs
- Execute any shell command

All changes will be validated before merging to the main branch.
Be concise and helpful. Explain what you're doing before running commands.`

	message, err := c.anthropic.Messages.New(ctx, anthropic.MessageNewParams{
		MaxTokens: 4096,
		System: []anthropic.TextBlockParam{
			{Text: systemPrompt},
		},
		Messages: history,
		Model:    anthropic.ModelClaudeSonnet4_5_20250929,
		Tools:    []anthropic.ToolUnionParam{GetBashTool()},
	})
	if err != nil {
		c.logger.Error("failed to send message to Claude", "error", err)
		return nil, err
	}

	c.logger.Debug("received response from Claude",
		"content_blocks", len(message.Content),
		"stop_reason", message.StopReason)

	return message, nil
}
