package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"syscall"

	"golang.org/x/term"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	listWidth = 30

	selectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("229")).
			Background(lipgloss.Color("57"))

	normalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("99")).
			PaddingLeft(1)

	borderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("99"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	emptyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true).
			PaddingLeft(1)

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	clientBadgeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("72"))

	paneStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62"))
)

type session struct {
	name      string
	clients   string
	startedIn string
	cmd       string
}

type historyMsg struct {
	name    string
	content string
}

type model struct {
	sessions []session
	cursor   int
	viewport viewport.Model
	width    int
	height   int
	preview  string
	loading  bool
	selected string
}

func getSessions() []session {
	out, err := exec.Command("zmx", "l").Output()
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var sessions []session
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		s := session{}
		for _, field := range strings.Split(line, "\t") {
			k, v, ok := strings.Cut(field, "=")
			if !ok {
				continue
			}
			switch k {
			case "session_name":
				s.name = v
			case "clients":
				s.clients = v
			case "started_in":
				s.startedIn = v
			case "cmd":
				s.cmd = v
			}
		}
		if s.name != "" {
			sessions = append(sessions, s)
		}
	}
	return sessions
}

// Strip non-SGR escape sequences (cursor movement, mode switches, OSC, etc.)
// while preserving SGR sequences (CSI...m) for colors/styles.
var (
	csiSeq = regexp.MustCompile(`\x1b\[([0-9;]*)([A-Za-z])`)
	oscSeq = regexp.MustCompile(`\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)`)
	pmSeq  = regexp.MustCompile(`\x1b\[\?[0-9;]*[A-Za-z]`)
)

func cleanVT(s string) string {
	s = pmSeq.ReplaceAllString(s, "")
	s = oscSeq.ReplaceAllString(s, "")
	s = csiSeq.ReplaceAllStringFunc(s, func(match string) string {
		sub := csiSeq.FindStringSubmatch(match)
		if len(sub) == 3 && sub[2] == "m" {
			return match // keep SGR (color) sequences
		}
		return ""
	})
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.TrimLeft(s, "\n")
	return s
}

func fetchHistory(name string) tea.Cmd {
	return func() tea.Msg {
		out, err := exec.Command("zmx", "history", name, "--vt").Output()
		if err != nil {
			return historyMsg{name: name, content: "(no scrollback available)"}
		}
		content := cleanVT(string(out))
		if strings.TrimSpace(content) == "" {
			return historyMsg{name: name, content: "(empty scrollback)"}
		}
		return historyMsg{name: name, content: content}
	}
}

func initialModel() model {
	sessions := getSessions()
	vp := viewport.New(0, 0)
	m := model{
		sessions: sessions,
		viewport: vp,
	}
	return m
}

func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{tea.WindowSize()}
	if len(m.sessions) > 0 {
		cmds = append(cmds, fetchHistory(m.sessions[0].name))
		m.loading = true
	}
	return tea.Batch(cmds...)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.recalcViewport()
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				m.loading = true
				return m, fetchHistory(m.sessions[m.cursor].name)
			}

		case "down", "j":
			if m.cursor < len(m.sessions)-1 {
				m.cursor++
				m.loading = true
				return m, fetchHistory(m.sessions[m.cursor].name)
			}

		case "enter":
			if len(m.sessions) > 0 {
				m.selected = m.sessions[m.cursor].name
				return m, tea.Quit
			}
		}

	case historyMsg:
		if len(m.sessions) > 0 && msg.name == m.sessions[m.cursor].name {
			m.preview = msg.content
			m.viewport.SetContent(msg.content)
			m.viewport.GotoBottom()
			m.loading = false
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m *model) recalcViewport() {
	_, previewW, innerH := m.paneDimensions()
	vpH := innerH - 2
	if vpH < 1 {
		vpH = 1
	}
	m.viewport.Width = previewW
	m.viewport.Height = vpH
	m.viewport.SetContent(m.preview)
}

func (m model) paneDimensions() (listW, previewW, innerH int) {
	// paneStyle has rounded border = 1 cell on each side
	// innerH = total height - pane top/bottom border (2) - help line (1) - 1 blank
	innerH = m.height - 4
	if innerH < 4 {
		innerH = 4
	}
	listW = listWidth
	// total width = listPane(listW + 2 border) + gap(1) + previewPane(previewW + 2 border)
	previewW = m.width - (listW + 2) - 1 - 2
	if previewW < 10 {
		previewW = 10
	}
	return
}

func padToHeight(s string, height int) string {
	lines := strings.Split(s, "\n")
	for len(lines) < height {
		lines = append(lines, "")
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	return strings.Join(lines, "\n")
}

func (m model) View() string {
	if m.width == 0 {
		return "loading..."
	}

	if len(m.sessions) == 0 {
		return emptyStyle.Render("No zmx sessions found. Start one with: zmx a <name>")
	}

	listW, previewW, innerH := m.paneDimensions()

	// === List pane content ===
	title := titleStyle.MaxWidth(listW).Render("sessions")
	sep := borderStyle.Render(strings.Repeat("─", listW))

	var listItems strings.Builder
	for i, s := range m.sessions {
		prefix := "  "
		style := normalStyle
		if i == m.cursor {
			prefix = "▸ "
			style = selectedStyle
		}
		badge := ""
		if s.clients != "" && s.clients != "0" {
			badge = " " + clientBadgeStyle.Render("("+s.clients+")")
		}
		listItems.WriteString(prefix + style.Render(s.name) + badge + "\n")
	}

	// title=1 line, sep=1 line, remaining=list items
	listItemsHeight := innerH - 2
	listBody := padToHeight(strings.TrimRight(listItems.String(), "\n"), listItemsHeight)

	listInner := title + "\n" + sep + "\n" + listBody
	listPane := paneStyle.Width(listW).Render(listInner)

	// === Preview pane content ===
	sess := m.sessions[m.cursor]
	previewLabel := sess.name
	if sess.startedIn != "" {
		dir := sess.startedIn
		home, _ := os.UserHomeDir()
		if home != "" && strings.HasPrefix(dir, home) {
			dir = "~" + dir[len(home):]
		}
		previewLabel += " " + dimStyle.Render(dir)
	}
	previewTitle := titleStyle.MaxWidth(previewW).Render(previewLabel)
	previewSep := borderStyle.Render(strings.Repeat("─", previewW))

	vpHeight := innerH - 2
	var previewContent string
	if m.loading {
		previewContent = padToHeight(helpStyle.Render("loading..."), vpHeight)
	} else {
		previewContent = padToHeight(m.viewport.View(), vpHeight)
	}

	previewInner := previewTitle + "\n" + previewSep + "\n" + previewContent
	previewPane := paneStyle.Width(previewW).Render(previewInner)

	main := lipgloss.JoinHorizontal(lipgloss.Top, listPane, " ", previewPane)
	help := helpStyle.Render(" j/k: navigate  enter: attach  q: quit")

	return main + "\n" + help
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	m, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	final := m.(model)
	if final.selected != "" {
		// Ensure terminal is in sane cooked mode before handing off to zmx
		fd := int(os.Stdin.Fd())
		if oldState, err := term.GetState(fd); err == nil {
			term.Restore(fd, oldState)
		}

		// Drain any buffered input from bubbletea's raw mode
		buf := make([]byte, 256)
		syscall.SetNonblock(fd, true)
		for {
			n, _ := os.Stdin.Read(buf)
			if n == 0 {
				break
			}
		}
		syscall.SetNonblock(fd, false)

		zmxPath, err := exec.LookPath("zmx")
		if err != nil {
			fmt.Fprintf(os.Stderr, "zmx not found: %v\n", err)
			os.Exit(1)
		}
		syscall.Exec(zmxPath, []string{"zmx", "a", final.selected}, os.Environ())
	}
}
