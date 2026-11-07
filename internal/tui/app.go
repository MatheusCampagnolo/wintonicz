package tui

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type keyMap struct {
	quit key.Binding
	help key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.help, k.quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.help, k.quit}}
}

type model struct {
	keys  keyMap
	help  help.Model
	width int
}

func newModel() model {
	return model{
		keys: keyMap{
			quit: key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
			help: key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		},
		help: help.New(),
	}
}

// Run starts the initial interactive shell.
func Run() error {
	program := tea.NewProgram(newModel())
	_, err := program.Run()
	return err
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		m.width = message.Width
		m.help.SetWidth(message.Width)
	case tea.KeyPressMsg:
		switch message.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "?":
			m.help.ShowAll = !m.help.ShowAll
		}
	}

	return m, nil
}

func (m model) View() tea.View {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7DD3FC")).Render("Wintonicz")
	subtitle := lipgloss.NewStyle().Foreground(lipgloss.Color("#A1A1AA")).Render("Windows support toolbox")
	body := "The TUI shell is running. Diagnostic pages are not available yet."

	if m.width > 0 && m.width < 64 {
		body = "Diagnostic pages are coming in the next milestone."
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		title,
		subtitle,
		"",
		body,
		"",
		m.help.View(m.keys),
	)

	view := tea.NewView(content)
	view.AltScreen = true
	return view
}
