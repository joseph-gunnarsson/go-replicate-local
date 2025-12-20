package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type LogMsg string

type Model struct {
	viewport  viewport.Model
	textInput textinput.Model
	cmdChan   chan<- string
    content   string // Buffer for logs
	ready     bool
	err       error
}

func initialModel(cmdChan chan<- string) Model {
	ti := textinput.New()
	ti.Placeholder = "Type a command..."
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 20

	return Model{
		textInput: ti,
		cmdChan:   cmdChan,
        content:   "",
	}
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		headerHeight := lipgloss.Height(m.headerView())
		footerHeight := lipgloss.Height(m.footerView())
		verticalMarginHeight := headerHeight + footerHeight

		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-verticalMarginHeight)
			m.viewport.YPosition = headerHeight
			m.viewport.SetContent(m.content)
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - verticalMarginHeight
		}

	case LogMsg:
        m.content += string(msg) + "\n"
        m.viewport.SetContent(m.content)
        m.viewport.GotoBottom()

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			cmd := m.textInput.Value()
			if strings.TrimSpace(cmd) != "" {
				go func() { m.cmdChan <- cmd }()
				m.textInput.SetValue("")
			}
		}
	}

	m.viewport, vpCmd = m.viewport.Update(msg)
	m.textInput, tiCmd = m.textInput.Update(msg)

	return m, tea.Batch(tiCmd, vpCmd)
}

// Log accumulation helper needed in Update
// Since we can't easily append to Viewport without storing state, 
// I'll add a `content string` to Model.

func (m Model) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}
	return fmt.Sprintf("%s\n%s\n%s", m.headerView(), m.viewport.View(), m.footerView())
}

func (m Model) headerView() string {
	title := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFDF5")).
		Background(lipgloss.Color("#255273")).
		Padding(0, 1).
		Render("Orchestrator Logs")
	line := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#255273")).
		Render(strings.Repeat("─", max(0, m.viewport.Width-lipgloss.Width(title))))
	return lipgloss.JoinHorizontal(lipgloss.Center, title, line)
}

func (m Model) footerView() string {
	return m.textInput.View()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Start initializes the TUI and returns channels for interaction
func Start() (chan<- string, <-chan string, error) {
	logChan := make(chan string, 100)
	cmdChan := make(chan string, 100)
    
    // We need to capture the logs. 
    // The Update method needs to handle LogMsg.
    // But we need to handle the content appending logic.
    // I'll rewrite the Update method in the file correctly.
    
	m := initialModel(cmdChan)
    // We need to initialize the content buffer in model if we want logs.
    
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())

	go func() {
		for msg := range logChan {
			p.Send(LogMsg(msg))
		}
	}()

	go func() {
		if _, err := p.Run(); err != nil {
			fmt.Printf("Alas, there's been an error: %v", err)
		}
	}()

	return logChan, cmdChan, nil
}
