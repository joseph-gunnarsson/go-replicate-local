package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type LogMsg string
type StatusMsg string
type SetFilterMsg string

type Program = tea.Program

var (
	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFDF5")).
			Background(lipgloss.Color("#6C5CE7")).
			Padding(0, 1).
			Bold(true)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A0A0A0")).
			Padding(0, 1)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6B6B")).
			Bold(true)

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#2ECC71")).
			Bold(true)

	lineStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6C5CE7"))
)

type Model struct {
	viewport       viewport.Model
	textInput      textinput.Model
	cmdChan        chan<- string
	content        string
	statusMessage  string
	isolatedFilter string
	ready          bool
	width          int
	height         int
}

func NewModel(cmdChan chan<- string) Model {
	ti := textinput.New()
	ti.Placeholder = "Type a command (help for list)..."
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 50

	return Model{
		textInput: ti,
		cmdChan:   cmdChan,
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
		m.width = msg.Width
		m.height = msg.Height

		headerHeight := lipgloss.Height(m.headerView())
		footerHeight := lipgloss.Height(m.footerView())
		verticalMarginHeight := headerHeight + footerHeight + 1

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

	case StatusMsg:
		m.statusMessage = string(msg)

	case SetFilterMsg:
		m.isolatedFilter = string(msg)

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			go func() { m.cmdChan <- "quit" }()
			return m, tea.Quit
		case tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			cmd := strings.TrimSpace(m.textInput.Value())
			if cmd != "" {
				go func() { m.cmdChan <- cmd }()
				m.textInput.SetValue("")
			}
		}
	}

	m.viewport, vpCmd = m.viewport.Update(msg)
	m.textInput, tiCmd = m.textInput.Update(msg)

	return m, tea.Batch(tiCmd, vpCmd)
}

func (m Model) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}
	return fmt.Sprintf("%s\n%s\n%s", m.headerView(), m.viewport.View(), m.footerView())
}

func (m Model) headerView() string {
	title := titleStyle.Render("🚀 Go-Sim Orchestrator")

	var status string
	if m.isolatedFilter != "" {
		status = statusStyle.Render(fmt.Sprintf("Isolated: %s", m.isolatedFilter))
	} else {
		status = statusStyle.Render("Showing: all")
	}

	titleWidth := lipgloss.Width(title)
	statusWidth := lipgloss.Width(status)
	lineWidth := max(0, m.width-titleWidth-statusWidth)
	line := lineStyle.Render(strings.Repeat("─", lineWidth))

	return lipgloss.JoinHorizontal(lipgloss.Center, title, line, status)
}

func (m Model) footerView() string {
	var statusLine string
	if m.statusMessage != "" {
		statusLine = m.statusMessage + "\n"
	}

	inputLine := m.textInput.View()
	hint := helpStyle.Render(" (Ctrl+C to quit)")

	return statusLine + inputLine + hint
}

func (m *Model) SetIsolatedFilter(name string) {
	m.isolatedFilter = name
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func HelpText() string {
	return `
┌─────────────────────────────────────────────────┐
│                 Available Commands               │
├─────────────────────────────────────────────────┤
│  help              Show this help message        │
│  list              List all running replicas     │
│  isolate <name>    Show logs from one replica    │
│  showall           Show logs from all replicas   │
│  kill <name>       Stop a specific replica       │
│  quit              Shutdown and exit             │
└─────────────────────────────────────────────────┘`
}

func FormatReplicaList(replicas []string) string {
	if len(replicas) == 0 {
		return "No replicas running."
	}

	sort.Strings(replicas)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Running replicas (%d):\n", len(replicas)))
	for _, r := range replicas {
		sb.WriteString(fmt.Sprintf("  • %s\n", r))
	}
	return sb.String()
}

func FormatError(msg string) string {
	return errorStyle.Render("✗ " + msg)
}

func FormatSuccess(msg string) string {
	return successStyle.Render("✓ " + msg)
}

func Setup() (logChan chan<- string, cmdChan <-chan string, program *tea.Program) {
	logs := make(chan string, 100)
	cmds := make(chan string, 100)

	m := NewModel(cmds)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())

	go func() {
		for msg := range logs {
			p.Send(LogMsg(msg))
		}
	}()
	
	return logs, cmds, p
}

func SendStatus(p *tea.Program, msg string) {
	p.Send(StatusMsg(msg))
}

func SendLog(p *tea.Program, msg string) {
	p.Send(LogMsg(msg))
}
