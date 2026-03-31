package cmd

import (
	"flag"
	"fmt"
	"log/slog"
	"magos/internal/db"
	"magos/internal/filemanager"
	"magos/internal/llm"
	"magos/internal/tui"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

var (
	sandboxMode   bool
	promptFile    string
	sandboxPrompt string
)

func init() {
	flag.BoolVar(&sandboxMode, "sandbox-mode", false, "Run in sandbox mode (inside VM)")
	flag.StringVar(&promptFile, "prompt-file", "", "Path to file containing the prompt (sandbox mode)")
	flag.StringVar(&sandboxPrompt, "prompt", "", "Prompt to execute (sandbox mode)")
}

func Execute() {
	flag.Parse()

	if sandboxMode {
		executeSandboxMode()
		return
	}

	executeTUIMode()
}

func executeTUIMode() {
	gitManager, err := filemanager.NewManager()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize git manager: %v\n", err)
		fmt.Fprintf(os.Stderr, "Make sure you're running Magos in a git repository\n")
		os.Exit(1)
	}

	logDB, err := db.NewLogDB(gitManager.ProjectRoot())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize log database: %v\n", err)
		os.Exit(1)
	}
	defer logDB.Close()

	sqliteHandler := db.NewSQLiteHandler(logDB, slog.LevelDebug)
	logger := slog.New(sqliteHandler)

	llmClient, err := llm.NewClient(logger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize LLM client: %v\n", err)
		fmt.Fprintf(os.Stderr, "Please set MAGOS_ANTHROPIC_API_KEY environment variable\n")
		os.Exit(1)
	}

	p := tea.NewProgram(
		tui.InitialModel(logger, llmClient, gitManager),
		tea.WithAltScreen(),
	)
	if _, err := p.Run(); err != nil {
		logger.Error("application error", "error", err)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func executeSandboxMode() {
	apiKey := os.Getenv("MAGOS_ANTHROPIC_API_KEY")
	if apiKey == "" {
		fmt.Fprintf(os.Stderr, "MAGOS_ANTHROPIC_API_KEY not set\n")
		os.Exit(1)
	}

	prompt := sandboxPrompt
	if promptFile != "" {
		data, err := os.ReadFile(promptFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to read prompt file: %v\n", err)
			os.Exit(1)
		}
		prompt = string(data)
	}

	if prompt == "" {
		fmt.Fprintf(os.Stderr, "No prompt provided\n")
		os.Exit(1)
	}

	gitManager, err := filemanager.NewManager()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize git manager: %v\n", err)
		os.Exit(1)
	}

	logDB, err := db.NewLogDB(gitManager.ProjectRoot())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize log database: %v\n", err)
		os.Exit(1)
	}
	defer logDB.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	llmClient, err := llm.NewClient(logger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize LLM client: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(
		tui.InitialModelSandbox(logger, llmClient, gitManager, "/mnt/workspace", prompt),
	)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
