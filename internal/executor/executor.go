package executor

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"
)

// ToolCall represents a structured tool call from the LLM.
type ToolCall struct {
	ID          string // Unique ID for this tool call
	Name        string // Tool name (e.g., "bash")
	Command     string // The bash command to execute
	Explanation string // Why the LLM is running this command
}

// CommandResult contains the outcome of executing a command.
type CommandResult struct {
	ToolCallID  string        // ID of the tool call that triggered this
	Command     string        // The command that was executed
	Explanation string        // Why this command was run
	Stdout      string        // Standard output
	Stderr      string        // Standard error
	ExitCode    int           // Exit code (0 = success)
	Duration    time.Duration // How long it took
	Error       error         // Any error encountered
}

// Executor handles command execution from LLM tool calls.
type Executor struct {
	logger     *slog.Logger
	workingDir string
	timeout    time.Duration
}

// NewExecutor creates a new command executor.
func NewExecutor(logger *slog.Logger, workingDir string) *Executor {
	return &Executor{
		logger:     logger,
		workingDir: workingDir,
		timeout:    30 * time.Second,
	}
}

// SetTimeout updates the command execution timeout.
func (e *Executor) SetTimeout(timeout time.Duration) {
	e.timeout = timeout
}

// Execute runs a single tool call and returns the result.
func (e *Executor) Execute(ctx context.Context, toolCall ToolCall) *CommandResult {
	e.logger.Info("executing tool call",
		"tool", toolCall.Name,
		"command", toolCall.Command,
		"explanation", toolCall.Explanation,
		"workdir", e.workingDir)

	start := time.Now()

	// Create context with timeout
	execCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	// Create the command
	execCmd := exec.CommandContext(execCtx, "bash", "-c", toolCall.Command)
	execCmd.Dir = e.workingDir

	// Capture stdout and stderr
	var stdout, stderr strings.Builder
	execCmd.Stdout = &stdout
	execCmd.Stderr = &stderr

	// Run the command
	err := execCmd.Run()
	duration := time.Since(start)

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			// Other error (context timeout, command not found, etc.)
			exitCode = -1
		}
	}

	result := &CommandResult{
		ToolCallID:  toolCall.ID,
		Command:     toolCall.Command,
		Explanation: toolCall.Explanation,
		Stdout:      stdout.String(),
		Stderr:      stderr.String(),
		ExitCode:    exitCode,
		Duration:    duration,
		Error:       err,
	}

	e.logger.Info("command completed",
		"exit_code", exitCode,
		"duration_ms", duration.Milliseconds(),
		"stdout_len", len(result.Stdout),
		"stderr_len", len(result.Stderr),
	)

	return result
}

// FormatResultForToolResponse formats a command result for returning to the LLM as a tool_result.
func FormatResultForToolResponse(result *CommandResult) string {
	var builder strings.Builder

	// Include exit code
	builder.WriteString(fmt.Sprintf("Exit code: %d\n", result.ExitCode))

	// Include stdout if present
	if result.Stdout != "" {
		builder.WriteString(fmt.Sprintf("\nStdout:\n%s", result.Stdout))
	}

	// Include stderr if present
	if result.Stderr != "" {
		builder.WriteString(fmt.Sprintf("\nStderr:\n%s", result.Stderr))
	}

	// Include error details if command failed
	if result.Error != nil && result.ExitCode == -1 {
		builder.WriteString(fmt.Sprintf("\nError: %s", result.Error))
	}

	return strings.TrimSpace(builder.String())
}

// FormatResultForDisplay formats a command result for displaying to the user.
func FormatResultForDisplay(result *CommandResult) string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("$ %s\n", result.Command))
	if result.Explanation != "" {
		builder.WriteString(fmt.Sprintf("  (%s)\n", result.Explanation))
	}
	builder.WriteString(fmt.Sprintf("  Exit: %d | Duration: %dms\n",
		result.ExitCode, result.Duration.Milliseconds()))

	if result.Stdout != "" {
		builder.WriteString(fmt.Sprintf("\n%s\n", result.Stdout))
	}

	if result.Stderr != "" {
		builder.WriteString(fmt.Sprintf("\nStderr:\n%s\n", result.Stderr))
	}

	return builder.String()
}
