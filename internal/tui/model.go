package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"magos/internal/executor"
	"magos/internal/filemanager"
	"magos/internal/llm"
	"magos/internal/validation"

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

		// Build response text and content blocks for history
		var responseText string
		var contentBlocks []anthropic.ContentBlockParamUnion

		for _, block := range message.Content {
			// Handle text blocks
			if block.Text != "" {
				responseText += block.Text
				contentBlocks = append(contentBlocks, anthropic.ContentBlockParamUnion{
					OfText: &anthropic.TextBlockParam{
						Text: block.Text,
					},
				})
			}

			// Handle tool use blocks (check the Type field)
			if block.Type == "tool_use" {
				contentBlocks = append(contentBlocks, anthropic.ContentBlockParamUnion{
					OfToolUse: &anthropic.ToolUseBlockParam{
						ID:    block.ID,
						Name:  block.Name,
						Input: block.Input,
					},
				})
			}
		}

		// Append the full assistant message to history
		history = append(history, anthropic.NewAssistantMessage(contentBlocks...))

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

		m.logger.Debug("received LLM response",
			"chain", msg.chainCount,
			"response_len", len(msg.response))

		// Get the last assistant message from conversation
		lastMsg := msg.conversation[len(msg.conversation)-1]

		// Check if the response contains tool use
		hasToolUse := false
		var textResponse string
		var toolCalls []executor.ToolCall

		// Parse the content blocks from the assistant message
		if lastMsg.Role == anthropic.MessageParamRoleAssistant {
			for _, contentBlock := range lastMsg.Content {
				// Handle text blocks
				if contentBlock.OfText != nil {
					textResponse += contentBlock.OfText.Text
				}

				// Handle tool use blocks
				if contentBlock.OfToolUse != nil {
					hasToolUse = true

					// Extract command and explanation from tool input
					// Input is 'any' type - try to assert it to map first
					var input map[string]interface{}

					// Try direct map assertion first
					if inputMap, ok := contentBlock.OfToolUse.Input.(map[string]interface{}); ok {
						input = inputMap
					} else if rawMsg, ok := contentBlock.OfToolUse.Input.(json.RawMessage); ok {
						// If it's json.RawMessage, unmarshal it
						if err := json.Unmarshal(rawMsg, &input); err != nil {
							m.appendDebug(fmt.Sprintf("[LLM] Failed to parse tool input: %s", err))
							continue
						}
					} else {
						m.appendDebug(fmt.Sprintf("[LLM] Unexpected tool input type: %T", contentBlock.OfToolUse.Input))
						continue
					}

					command, ok := input["command"].(string)
					if !ok {
						m.appendDebug("[LLM] Tool input missing 'command' field")
						continue
					}

					explanation := ""
					if exp, ok := input["explanation"].(string); ok {
						explanation = exp
					}

					toolCalls = append(toolCalls, executor.ToolCall{
						ID:          contentBlock.OfToolUse.ID,
						Name:        contentBlock.OfToolUse.Name,
						Command:     command,
						Explanation: explanation,
					})

					m.appendDebug(fmt.Sprintf("[LLM] Tool call: %s", explanation))
				}
			}
		}

		// Display text response if any
		if textResponse != "" {
			m.appendMessage(fmt.Sprintf("Magos: %s", textResponse))
		}

		// Execute tool calls if present
		if hasToolUse && len(toolCalls) > 0 {
			m.appendDebug(fmt.Sprintf("[Executor] Executing %d tool call(s)", len(toolCalls)))

			exec := executor.NewExecutor(m.logger, m.worktreePath)
			ctx := context.Background()

			// Build tool results for next LLM turn
			var toolResultBlocks []anthropic.ContentBlockParamUnion

			for i, toolCall := range toolCalls {
				// Execute the command
				result := exec.Execute(ctx, toolCall)

				m.appendDebug(fmt.Sprintf("[Executor] Tool %d: exit=%d, duration=%dms",
					i+1, result.ExitCode, result.Duration.Milliseconds()))

				// Display result to user
				m.appendMessage(executor.FormatResultForDisplay(result))

				// Build tool_result for LLM
				toolResultBlocks = append(toolResultBlocks, anthropic.ContentBlockParamUnion{
					OfToolResult: &anthropic.ToolResultBlockParam{
						ToolUseID: result.ToolCallID,
						Content: []anthropic.ToolResultBlockParamContentUnion{
							{
								OfText: &anthropic.TextBlockParam{
									Text: executor.FormatResultForToolResponse(result),
								},
							},
						},
					},
				})
			}

			// Continue conversation with tool results if we haven't exceeded max chains
			if msg.chainCount < m.maxPromptChains {
				m.appendDebug(fmt.Sprintf("[LLM] Sending tool results back (chain %d/%d)",
					msg.chainCount+2, m.maxPromptChains))

				return m, sendLLMRequestWithChain(
					m.llmClient,
					append(msg.conversation, anthropic.NewUserMessage(toolResultBlocks...)),
					m.maxPromptChains,
					msg.chainCount+1,
				)
			} else {
				m.appendDebug("[LLM] Max prompt chains reached, stopping")
			}
		} else if msg.response != "" {
			m.appendMessage(fmt.Sprintf("Magos: %s", msg.response))
		}

		if m.worktreeCreated {
			// Run validation before merging
			m.appendDebug("[Validation] Running validators on worktree...")
			runner, err := validation.NewRunner(m.worktreePath)
			if err != nil {
				m.appendDebug(fmt.Sprintf("[Validation] Runner creation failed: %s", err))
				m.appendMessage(fmt.Sprintf("Validation setup error: %s", err))
			} else {
				result, err := runner.Run()
				if err != nil {
					m.appendDebug(fmt.Sprintf("[Validation] Execution failed: %s", err))
					m.appendMessage(fmt.Sprintf("Validation error: %s", err))
				} else if !result.Passed {
					m.appendDebug(fmt.Sprintf("[Validation] FAILED - found issues"))
					validationMsg := fmt.Sprintf("Validation Failed:\n%s", result.FormatForLLM())
					m.appendMessage(validationMsg)

					// Don't merge if validation fails
					m.appendDebug("[Git] Skipping merge due to validation failure")
					m.appendDebug(fmt.Sprintf("[Git] Cleaning up worktree (session: %s)", m.sessionID))
					if err := m.gitManager.CleanupWorktree(m.sessionID); err != nil {
						m.appendDebug(fmt.Sprintf("[Git] Cleanup error: %s", err))
					}
					m.worktreeCreated = false
					m.worktreePath = ""
					return m, nil
				} else {
					m.appendDebug("[Validation] PASSED - all checks successful")
				}
			}

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
				m.worktreeCreated = true
				m.worktreePath = wtPath

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

