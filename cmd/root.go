package cmd

import (
	"fmt"
	"log/slog"
	"magos/internal/filemanager"
	"magos/internal/llm"
	"magos/internal/tui"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

// Execute initializes the application services and launches the TUI.
func Execute() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	llmClient, err := llm.NewClient(logger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize LLM client: %v\n", err)
		fmt.Fprintf(os.Stderr, "Please set MAGOS_ANTHROPIC_API_KEY environment variable\n")
		os.Exit(1)
	}

	gitManager, err := filemanager.NewManager()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize git manager: %v\n", err)
		fmt.Fprintf(os.Stderr, "Make sure you're running Magos in a git repository\n")
		os.Exit(1)
	}

	p := tea.NewProgram(tui.InitialModel(logger, llmClient, gitManager))
	if _, err := p.Run(); err != nil {
		logger.Error("application error", "error", err)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
