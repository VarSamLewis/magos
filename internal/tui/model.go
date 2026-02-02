package tui

import (
	"context"
	"fmt"
	"log/slog"
	"magos/internal/llm"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type model struct {
	textarea   textarea.Model
	viewport   viewport.Model
	termWidth  int
	termHeight int
	ready      bool
	logger     *slog.Logger
	llmClient  *llm.Client
}

type llmResponseMsg struct {
	response string
	err      error
}

// InitialModel creates and returns the initial TUI model state.
func InitialModel(logger *slog.Logger, llmClient *llm.Client) model {
	ta := textarea.New()
	ta.Placeholder = "Ask Magos something..."
	ta.Focus()
	ta.SetHeight(1)
	ta.ShowLineNumbers = false

	vp := viewport.New(80, 20)
	vp.SetContent("Waiting for input...")

	logger.Info("initialized TUI model")

	return model{
		textarea:  ta,
		viewport:  vp,
		ready:     false,
		logger:    logger,
		llmClient: llmClient,
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

// sendLLMRequest sends a user message to Claude and returns a tea.Cmd that delivers the response.
func sendLLMRequest(client *llm.Client, userMessage string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		message, err := client.SendMessage(ctx, userMessage)
		if err != nil {
			return llmResponseMsg{err: err}
		}

		var responseText string
		for _, block := range message.Content {
			responseText += block.Text
		}

		return llmResponseMsg{response: responseText}
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

		reservedHeight := 17

		m.viewport.Width = msg.Width - 4
		m.viewport.Height = msg.Height - reservedHeight

		m.textarea.SetWidth(msg.Width - 8)

		m.ready = true
		return m, nil

	case llmResponseMsg:
		if msg.err != nil {
			m.logger.Error("LLM request failed", "error", msg.err)
			errorMsg := fmt.Sprintf("Error: %s", msg.err.Error())
			m.appendMessage(errorMsg)
		} else {
			m.logger.Debug("received LLM response", "length", len(msg.response))
			responseMsg := fmt.Sprintf("Magos: %s", msg.response)
			m.appendMessage(responseMsg)
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

				m.textarea.Reset()

				return m, sendLLMRequest(m.llmClient, userInput)
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

	asciiArt := `
      ╔═══════════════════════╗
      ║   MAGOS TUI v1.0      ║
      ╚═══════════════════════╝
      `

	helpText := "ENTER to send • PgUp/PgDn or Ctrl+U/D to scroll • Ctrl+C quit"

	return fmt.Sprintf(
		"%s\n\n%s\n\n%s\n%s",
		asciiArt,
		m.viewport.View(),
		style.Render(m.textarea.View()),
		helpText,
	)
}

