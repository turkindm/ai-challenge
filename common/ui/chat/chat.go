// Package chat предоставляет переиспользуемый TUI-чат на Bubble Tea v2.
// Достаточно реализовать интерфейс Agent и вызвать Run.
package chat

import (
	"context"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
)

// Agent — интерфейс, который должен реализовать агент любой задачи.
type Agent interface {
	Send(ctx context.Context, message string) (string, error)
}

// Run запускает TUI-чат с переданным агентом. Блокируется до выхода.
func Run(ag Agent) error {
	p := tea.NewProgram(initialModel(ag))
	_, err := p.Run()
	return err
}

// ── стили ─────────────────────────────────────────────────────────────────

var (
	styleBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63"))

	styleHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("63")).
			Padding(0, 1)

	styleUserLabel = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212"))

	styleAssistantLabel = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("86"))

	styleErrorLabel = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("196"))

	styleInputPrompt = lipgloss.NewStyle().
				Foreground(lipgloss.Color("63")).
				Bold(true)

	styleHelp = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	styleSpinner = lipgloss.NewStyle().
			Foreground(lipgloss.Color("220"))
)

// ── внутренние типы ────────────────────────────────────────────────────────

type responseMsg struct{ text string }
type errMsg struct{ err error }
type spinTickMsg struct{}

type chatEntry struct {
	role    string // "user" | "assistant" | "error"
	content string
}

type model struct {
	ag       Agent
	entries  []chatEntry
	input    string
	loading  bool
	width    int
	height   int
	scroll   int
	spinStep int
}

var spinnerFrames = []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}

func initialModel(ag Agent) model {
	return model{ag: ag, width: 80, height: 24}
}

func spinTick() tea.Cmd {
	return func() tea.Msg { return spinTickMsg{} }
}

// ── Bubble Tea interface ───────────────────────────────────────────────────

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyPressMsg:
		if m.loading {
			return m, nil
		}
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit

		case "enter":
			text := strings.TrimSpace(m.input)
			if text == "" {
				return m, nil
			}
			m.entries = append(m.entries, chatEntry{role: "user", content: text})
			m.input = ""
			m.loading = true
			m.scroll = 0
			return m, tea.Batch(sendCmd(m.ag, text), spinTick())

		case "backspace":
			if runes := []rune(m.input); len(runes) > 0 {
				m.input = string(runes[:len(runes)-1])
			}

		case "ctrl+u":
			m.input = ""

		case "space":
			m.input += " "

		case "up":
			m.scroll++

		case "down":
			if m.scroll > 0 {
				m.scroll--
			}

		default:
			if utf8.RuneCountInString(msg.String()) == 1 {
				m.input += msg.String()
			}
		}

	case spinTickMsg:
		if m.loading {
			m.spinStep = (m.spinStep + 1) % len(spinnerFrames)
			return m, spinTick()
		}

	case responseMsg:
		m.loading = false
		m.entries = append(m.entries, chatEntry{role: "assistant", content: msg.text})
		m.scroll = 0

	case errMsg:
		m.loading = false
		m.entries = append(m.entries, chatEntry{role: "error", content: msg.err.Error()})
		m.scroll = 0
	}

	return m, nil
}

func (m model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	return v
}

// ── рендер ────────────────────────────────────────────────────────────────

func (m model) render() string {
	innerW := m.width - 2

	header := styleHeader.Render("AI Agent")
	helpLine := styleHelp.Render("Enter — отправить  ·  Ctrl+U — очистить  ·  ↑↓ — прокрутка  ·  Esc — выход")

	cursorBlink := "█"
	if m.loading {
		cursorBlink = ""
	}
	inputLine := styleInputPrompt.Render("›") + " " + m.input + cursorBlink

	// высота области чата: вся высота минус header, рамки, поле ввода, подсказка
	chatH := m.height - 7
	if chatH < 3 {
		chatH = 3
	}
	chatW := innerW - 2

	var lines []string
	for _, e := range m.entries {
		lines = append(lines, renderEntry(e, chatW)...)
		lines = append(lines, "")
	}
	if m.loading {
		lines = append(lines, styleSpinner.Render(spinnerFrames[m.spinStep])+" думаю…", "")
	}

	// прокрутка
	end := len(lines) - m.scroll
	if end < 0 {
		end = 0
	}
	start := end - chatH
	if start < 0 {
		start = 0
	}
	visible := lines[start:end]
	for len(visible) < chatH {
		visible = append(visible, "")
	}

	chatBox := styleBorder.Width(innerW).Height(chatH).Render(strings.Join(visible, "\n"))
	inputBox := styleBorder.Width(innerW).Render(inputLine)

	return lipgloss.JoinVertical(lipgloss.Left, header, chatBox, inputBox, helpLine)
}

func renderEntry(e chatEntry, width int) []string {
	var label string
	switch e.role {
	case "user":
		label = styleUserLabel.Render("Вы:")
	case "assistant":
		label = styleAssistantLabel.Render("Agent:")
	default:
		label = styleErrorLabel.Render("Ошибка:")
	}
	return append([]string{label}, wordWrap(e.content, width)...)
}

func wordWrap(text string, maxW int) []string {
	if maxW <= 0 {
		return []string{text}
	}
	var result []string
	for _, rawLine := range strings.Split(text, "\n") {
		words := strings.Fields(rawLine)
		if len(words) == 0 {
			result = append(result, "")
			continue
		}
		cur := ""
		for _, w := range words {
			if cur == "" {
				cur = w
			} else if len(cur)+1+len(w) <= maxW {
				cur += " " + w
			} else {
				result = append(result, cur)
				cur = w
			}
		}
		if cur != "" {
			result = append(result, cur)
		}
	}
	return result
}

// ── команда отправки ───────────────────────────────────────────────────────

func sendCmd(ag Agent, input string) tea.Cmd {
	return func() tea.Msg {
		text, err := ag.Send(context.Background(), input)
		if err != nil {
			return errMsg{err}
		}
		return responseMsg{text}
	}
}
