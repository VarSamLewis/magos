package tui

import (
	"context"
	"fmt"
	"log/slog"
	"magos/internal/filemanager"
	"magos/internal/llm"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
)

type model struct {
	textarea        textarea.Model
	viewport        viewport.Model
	debugView       viewport.Model
	termWidth       int
	termHeight      int
	ready           bool
	logger          *slog.Logger
	llmClient       *llm.Client
	gitManager      *filemanager.Manager
	sessionID       string
	maxPromptChains int
	worktreeCreated bool
	worktreePath    string
}

type llmResponseMsg struct {
	response     string
	err          error
	chainCount   int
	conversation []anthropic.MessageParam
}

// InitialModel creates and returns the initial TUI model state.
func InitialModel(logger *slog.Logger, llmClient *llm.Client, gitManager *filemanager.Manager) model {
	ta := textarea.New()
	ta.Placeholder = "Ask Magos something..."
	ta.Focus()
	ta.SetHeight(1)
	ta.ShowLineNumbers = false

	vp := viewport.New(80, 20)
	vp.SetContent("Waiting for input...")

	debugVp := viewport.New(80, 3)
	debugVp.SetContent("")

	sessionID := uuid.New().String()[:8]

	logger.Info("initialized TUI model", "session_id", sessionID)

	return model{
		textarea:        ta,
		viewport:        vp,
		debugView:       debugVp,
		ready:           false,
		logger:          logger,
		llmClient:       llmClient,
		gitManager:      gitManager,
		sessionID:       sessionID,
		maxPromptChains: 3,
		worktreeCreated: false,
		worktreePath:    "",
	}
}

// Init initializes the TUI and returns the initial command.
func (m model) Init() tea.Cmd {
	return textarea.Blink
}

// appendMessage adds a new message to the viewport and scrolls to bottom.
func (m *model) appendMessage(message string) {
	wrappedMsg := message
	if m.viewport.Width > 0 {
		wrappedMsg = lipgloss.NewStyle().Width(m.viewport.Width).Render(message)
	}

	currentContent := m.viewport.View()
	if currentContent == "Waiting for input..." {
		currentContent = ""
	}

	if currentContent != "" {
		currentContent += "\n---\n"
	}
	currentContent += wrappedMsg

	m.viewport.SetContent(lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Render(currentContent))
	m.viewport.GotoBottom()
}

// executeTool executes a single tool call and returns the result.
func (m *model) executeTool(tool llm.ToolCall) (string, error) {
	switch tool.Tool {
	case "read_files", "read_file":
		path, _ := tool.GetStringArg("path")
		return llm.ExecuteReadFiles(m.gitManager.ProjectRoot(), path)

	case "modify_files":
		if !m.worktreeCreated {
			m.appendDebug(fmt.Sprintf("[Git] Creating worktree (session: %s)", m.sessionID))
			wtPath, err := m.gitManager.CreateWorktree(m.sessionID)
			if err != nil {
				return "", fmt.Errorf("failed to create worktree: %w", err)
			}
			m.worktreePath = wtPath
			m.worktreeCreated = true
			m.appendDebug(fmt.Sprintf("[Git] Created worktree at: %s", wtPath))
		}

		modifications, err := tool.GetFileModifications()
		if err != nil {
			return "", err
		}

		return llm.ExecuteModifyFiles(m.worktreePath, modifications)

	default:
		return "", fmt.Errorf("unknown tool: %s", tool.Tool)
	}
}

// appendDebug adds a debug message to the debug viewport.
func (m *model) appendDebug(message string) {
	wrappedMsg := message
	if m.debugView.Width > 0 {
		wrappedMsg = lipgloss.NewStyle().Width(m.debugView.Width).Render(message)
	}

	currentContent := m.debugView.View()
	if currentContent != "" {
		currentContent += "\n"
	}
	currentContent += wrappedMsg

	m.debugView.SetContent(lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(currentContent))
	m.debugView.GotoBottom()
}

// sendLLMRequest sends a user message to Claude and returns a tea.Cmd that delivers the response.
func sendLLMRequest(client *llm.Client, userMessage string, maxChains int) tea.Cmd {
	history := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(userMessage)),
	}
	return sendLLMRequestWithChain(client, history, maxChains, 0)
}

// sendLLMRequestWithChain handles prompt chaining by sending the accumulated
// message history and returning the response for the TUI to process.
func sendLLMRequestWithChain(client *llm.Client, history []anthropic.MessageParam, maxChains, currentChain int) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		message, err := client.SendMessage(ctx, history)
		if err != nil {
			return llmResponseMsg{
				err:          err,
				chainCount:   currentChain,
				conversation: history,
			}
		}

		var responseText string
		for _, block := range message.Content {
			responseText += block.Text
		}

		// Append the assistant's response to history.
		history = append(history, anthropic.NewAssistantMessage(anthropic.NewTextBlock(responseText)))

		return llmResponseMsg{
			response:     responseText,
			chainCount:   currentChain,
			conversation: history,
		}
	}
}

// Update handles incoming messages and updates the model state.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var tiCmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termWidth = msg.Width
		m.termHeight = msg.Height

		m.logger.Debug("window resized", "width", msg.Width, "height", msg.Height)

		reservedHeight := 21

		m.viewport.Width = msg.Width - 4
		m.viewport.Height = msg.Height - reservedHeight

		m.debugView.Width = msg.Width - 4
		m.debugView.Height = 3

		m.textarea.SetWidth(msg.Width - 8)

		m.ready = true
		return m, nil

	case llmResponseMsg:
		if msg.err != nil {
			m.logger.Error("LLM request failed", "error", msg.err)
			m.appendDebug(fmt.Sprintf("[LLM] Error: %s", msg.err))
			errorMsg := fmt.Sprintf("Error: %s", msg.err.Error())
			m.appendMessage(errorMsg)

			m.appendDebug(fmt.Sprintf("[Git] Cleaning up worktree (session: %s)", m.sessionID))
			if err := m.gitManager.CleanupWorktree(m.sessionID); err != nil {
				m.appendDebug(fmt.Sprintf("[Git] Cleanup error: %s", err))
			}
			return m, nil
		}

		m.logger.Debug("received LLM response", "length", len(msg.response), "chain", msg.chainCount)
		m.appendDebug(fmt.Sprintf("[LLM] Chain %d - Response (%d chars)", msg.chainCount+1, len(msg.response)))

		toolCalls := llm.ParseToolCalls(msg.response)

		if len(toolCalls) > 0 {
			if msg.chainCount >= m.maxPromptChains {
				m.appendDebug(fmt.Sprintf("[LLM] Max chains (%d) reached, stopping despite tool calls", m.maxPromptChains))
			} else {
				m.appendDebug(fmt.Sprintf("[LLM] Found %d tool call(s), continuing chain...", len(toolCalls)))

				toolResults := ""
				for _, tool := range toolCalls {
					m.appendDebug(fmt.Sprintf("[Tool] Executing: %s", tool.Tool))

					result, err := m.executeTool(tool)
					if err != nil {
						m.appendDebug(fmt.Sprintf("[Tool] Error: %s", err))
						toolResults += fmt.Sprintf("\nTool: %s\nError: %s\n", tool.Tool, err)
					} else {
						m.appendDebug(fmt.Sprintf("[Tool] Success"))
						toolResults += fmt.Sprintf("\nTool: %s\nResult: %s\n", tool.Tool, result)
					}
				}

				history := append(msg.conversation, anthropic.NewUserMessage(anthropic.NewTextBlock(toolResults)))
			return m, sendLLMRequestWithChain(m.llmClient, history, m.maxPromptChains, msg.chainCount+1)
			}
		} else {
			m.appendDebug("[LLM] No tool calls found, chain complete")
		}

		responseMsg := fmt.Sprintf("Magos: %s", msg.response)
		m.appendMessage(responseMsg)

		if m.worktreeCreated {
			m.appendDebug(fmt.Sprintf("[Git] Merging worktree (session: %s)", m.sessionID))
			if err := m.gitManager.MergeWorktree(m.sessionID); err != nil {
				m.appendDebug(fmt.Sprintf("[Git] Merge error: %s", err))
				m.appendMessage(fmt.Sprintf("Git Merge Error: %s", err))
			} else {
				m.appendDebug("[Git] Merge successful")
			}

			m.appendDebug(fmt.Sprintf("[Git] Cleaning up worktree (session: %s)", m.sessionID))
			if err := m.gitManager.CleanupWorktree(m.sessionID); err != nil {
				m.appendDebug(fmt.Sprintf("[Git] Cleanup error: %s", err))
			} else {
				m.appendDebug("[Git] Cleanup successful")
			}

			m.worktreeCreated = false
			m.worktreePath = ""
		} else {
			m.appendDebug("[Git] No worktree created, skipping merge/cleanup")
		}

		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.logger.Info("user quit application")
			return m, tea.Quit
		case "enter":
			userInput := m.textarea.Value()
			if userInput != "" {
				m.logger.Info("user sent message", "length", len(userInput))

				userMsg := fmt.Sprintf("You: %s", userInput)
				m.appendMessage(userMsg)

				m.appendDebug(fmt.Sprintf("[Git] Creating worktree (session: %s)", m.sessionID))
				wtPath, err := m.gitManager.CreateWorktree(m.sessionID)
				if err != nil {
					m.appendDebug(fmt.Sprintf("[Git] Error: %s", err))
					m.appendMessage(fmt.Sprintf("Git Error: %s", err))
					m.textarea.Reset()
					return m, nil
				}
				m.appendDebug(fmt.Sprintf("[Git] Created worktree at: %s", wtPath))

				m.textarea.Reset()

				return m, sendLLMRequest(m.llmClient, userInput, m.maxPromptChains)
			}

			m.textarea, tiCmd = m.textarea.Update(msg)
			return m, tiCmd
		case "pgup":
			m.viewport.ScrollUp(3)
			return m, nil
		case "pgdown":
			m.viewport.ScrollDown(3)
			return m, nil
		case "ctrl+u":
			m.viewport.HalfPageUp()
			return m, nil
		case "ctrl+d":
			m.viewport.HalfViewDown()
			return m, nil
		}
	}

	m.textarea, tiCmd = m.textarea.Update(msg)

	return m, tiCmd
}

// View renders the TUI layout.
func (m model) View() string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1).
		Margin(1)

	debugStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(0, 1)

	asciiArt := `
      ╔═══════════════════════╗
      ║   MAGOS TUI v1.0      ║
      ╚═══════════════════════╝
      `

	helpText := "ENTER to send • PgUp/PgDn or Ctrl+U/D to scroll • Ctrl+C quit"

	return fmt.Sprintf(
		"%s\n\n%s\n\n%s\n%s\n%s",
		asciiArt,
		m.viewport.View(),
		style.Render(m.textarea.View()),
		debugStyle.Render(m.debugView.View()),
		helpText,
	)
}

