package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/aux-ai/aux-cli/internal/app"
	"github.com/aux-ai/aux-cli/internal/llm/tools"
	"github.com/aux-ai/aux-cli/internal/message"
	"github.com/aux-ai/aux-cli/internal/pubsub"
	"github.com/aux-ai/aux-cli/internal/session"
	"github.com/aux-ai/aux-cli/internal/tui/styles"
	"github.com/aux-ai/aux-cli/internal/tui/theme"
)

// ContextEntry represents a single file currently loaded into the agent's
// context window. The path is the display path (relative to the working
// directory); the line count is the number of lines the agent actually
// received, not the file's total length.
type ContextEntry struct {
	Path        string
	AbsPath     string
	Lines       int
	Score       float64
	Reason      string
	CrossedOff  bool
	ReadAt      time.Time
	MessageID   string
	ToolCallID  string
}

type contextPaneCmp struct {
	app    *app.App
	width  int
	height int

	mu         sync.Mutex
	sessionID  string
	entries    []ContextEntry
	selected   int
	offset     int

	keys ContextPaneKeys
}

type ContextPaneKeys struct {
	Up       key.Binding
	Down     key.Binding
	CrossOff key.Binding
	Uncross  key.Binding
	Clear    key.Binding
}

var defaultContextPaneKeys = ContextPaneKeys{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "context up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "context down"),
	),
	CrossOff: key.NewBinding(
		key.WithKeys("x"),
		key.WithHelp("x", "cross off"),
	),
	Uncross: key.NewBinding(
		key.WithKeys("u"),
		key.WithHelp("u", "un-cross"),
	),
	Clear: key.NewBinding(
		key.WithKeys("c"),
		key.WithHelp("c", "clear crossed"),
	),
}

func (m *contextPaneCmp) Init() tea.Cmd {
	if m.app == nil || m.app.Messages == nil {
		return nil
	}
	ctx := context.Background()
	ch := m.app.Messages.Subscribe(ctx)
	return func() tea.Msg {
		return <-ch
	}
}

func (m *contextPaneCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case SessionSelectedMsg:
		if msg.ID != m.sessionID {
			m.sessionID = msg.ID
			m.mu.Lock()
			m.entries = nil
			m.selected = 0
			m.offset = 0
			m.mu.Unlock()
		}
	case SessionClearedMsg:
		m.sessionID = ""
		m.mu.Lock()
		m.entries = nil
		m.selected = 0
		m.offset = 0
		m.mu.Unlock()
	case pubsub.Event[message.Message]:
		if m.sessionID == "" || msg.Payload.SessionID != m.sessionID {
			return m, nil
		}
		if msg.Type == pubsub.CreatedEvent || msg.Type == pubsub.UpdatedEvent {
			m.absorbMessage(msg.Payload)
		}
		return m, func() tea.Msg {
			ctx := context.Background()
			ch := m.app.Messages.Subscribe(ctx)
			return <-ch
		}
	case pubsub.Event[session.Session]:
		if m.sessionID == "" && msg.Type == pubsub.CreatedEvent {
			m.sessionID = msg.Payload.ID
		}
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Up):
			m.moveSelection(-1)
		case key.Matches(msg, m.keys.Down):
			m.moveSelection(1)
		case key.Matches(msg, m.keys.CrossOff):
			m.toggleCross(true)
		case key.Matches(msg, m.keys.Uncross):
			m.toggleCross(false)
		case key.Matches(msg, m.keys.Clear):
			m.clearCrossed()
		}
	}
	return m, nil
}

func (m *contextPaneCmp) BindingKeys() []key.Binding {
	return []key.Binding{m.keys.Up, m.keys.Down, m.keys.CrossOff, m.keys.Uncross, m.keys.Clear}
}

func (m *contextPaneCmp) SetSize(width, height int) tea.Cmd {
	m.width = width
	m.height = height
	return nil
}

func (m *contextPaneCmp) GetSize() (int, int) {
	return m.width, m.height
}

func (m *contextPaneCmp) moveSelection(delta int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.entries) == 0 {
		m.selected = 0
		return
	}
	m.selected += delta
	if m.selected < 0 {
		m.selected = 0
	}
	if m.selected >= len(m.entries) {
		m.selected = len(m.entries) - 1
	}
}

func (m *contextPaneCmp) toggleCross(crossed bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.selected < 0 || m.selected >= len(m.entries) {
		return
	}
	m.entries[m.selected].CrossedOff = crossed
}

func (m *contextPaneCmp) clearCrossed() {
	m.mu.Lock()
	defer m.mu.Unlock()
	kept := m.entries[:0]
	for _, e := range m.entries {
		if !e.CrossedOff {
			kept = append(kept, e)
		}
	}
	m.entries = kept
	if m.selected >= len(m.entries) {
		m.selected = max(0, len(m.entries)-1)
	}
}

// absorbMessage scans a message for view tool results and appends one
// ContextEntry per successful read. Re-reads of the same path replace the
// prior entry so the pane reflects what is currently in the agent's context,
// not a historical list.
func (m *contextPaneCmp) absorbMessage(msg message.Message) {
	results := msg.ToolResults()
	if len(results) == 0 {
		return
	}

	var newEntries []ContextEntry
	for _, result := range results {
		if result.IsError {
			continue
		}
		var meta tools.ViewResponseMetadata
		if err := json.Unmarshal([]byte(result.Metadata), &meta); err != nil {
			continue
		}
		if meta.FilePath == "" {
			continue
		}
		entry := ContextEntry{
			AbsPath:    meta.FilePath,
			Path:       removeWorkingDirPrefix(meta.FilePath),
			Lines:      strings.Count(meta.Content, "\n") + 1,
			Score:      meta.DecisionScore,
			Reason:     meta.DecisionReason,
			ReadAt:     time.Now(),
			MessageID:  msg.ID,
			ToolCallID: result.ToolCallID,
		}
		if !meta.DecisionAllowed {
			// Surface rejected reads too, so the user can see what the
			// gate blocked. They render with a distinct style.
			entry.Reason = "rejected: " + entry.Reason
		}
		newEntries = append(newEntries, entry)
	}

	if len(newEntries) == 0 {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Replace prior entries for the same path; keep crossed-off state if the
	// user already marked them.
	for _, ne := range newEntries {
		replaced := false
		for i, existing := range m.entries {
			if existing.AbsPath == ne.AbsPath {
				if existing.CrossedOff {
					ne.CrossedOff = true
				}
				m.entries[i] = ne
				replaced = true
				break
			}
		}
		if !replaced {
			m.entries = append(m.entries, ne)
		}
	}
	sort.SliceStable(m.entries, func(i, j int) bool {
		return m.entries[i].ReadAt.Before(m.entries[j].ReadAt)
	})
}

func (m *contextPaneCmp) View() string {
	t := theme.CurrentTheme()
	baseStyle := styles.BaseStyle()

	m.mu.Lock()
	entries := append([]ContextEntry(nil), m.entries...)
	selected := m.selected
	m.mu.Unlock()

	header := baseStyle.
		Width(m.width).
		Foreground(t.Primary()).
		Bold(true).
		Render(fmt.Sprintf(" Context (%d)", len(entries)))

	footer := baseStyle.
		Width(m.width).
		Foreground(t.TextMuted()).
		Render(" ↑/↓ move · x off · u on · c clear")

	if len(entries) == 0 {
		empty := baseStyle.
			Width(m.width).
			Foreground(t.TextMuted()).
			Italic(true).
			Render(" no files loaded yet")
		return baseStyle.
			Width(m.width).
			Height(m.height).
			Render(
				lipgloss.JoinVertical(
					lipgloss.Left,
					header,
					" ",
					empty,
					" ",
					footer,
				),
			)
	}

	bodyHeight := m.height - 4
	if bodyHeight < 1 {
		bodyHeight = 1
	}
	start := selected - bodyHeight/2
	if start < 0 {
		start = 0
	}
	if start > len(entries)-bodyHeight {
		start = len(entries) - bodyHeight
	}
	if start < 0 {
		start = 0
	}
	end := start + bodyHeight
	if end > len(entries) {
		end = len(entries)
	}

	var rows []string
	for i := start; i < end; i++ {
		rows = append(rows, m.renderRow(entries[i], i == selected, m.width))
	}

	return baseStyle.
		Width(m.width).
		Height(m.height).
		Render(
			lipgloss.JoinVertical(
				lipgloss.Left,
				header,
				lipgloss.JoinVertical(lipgloss.Left, rows...),
				" ",
				footer,
			),
		)
}

func (m *contextPaneCmp) renderRow(e ContextEntry, selected bool, width int) string {
	t := theme.CurrentTheme()
	baseStyle := styles.BaseStyle()

	marker := "  "
	if selected {
		marker = "▸ "
	}

	displayPath := e.Path
	if displayPath == "" {
		displayPath = filepath.Base(e.AbsPath)
	}

	lineInfo := fmt.Sprintf("%d ln", e.Lines)
	scoreInfo := ""
	if e.Score > 0 {
		scoreInfo = fmt.Sprintf(" · %.3f", e.Score)
	}
	tail := " " + lineInfo + scoreInfo

	pathWidth := width - lipgloss.Width(marker) - lipgloss.Width(tail) - 2
	if pathWidth < 4 {
		pathWidth = 4
	}
	if lipgloss.Width(displayPath) > pathWidth {
		displayPath = ansiTruncate(displayPath, pathWidth-1, "…")
	}

	style := baseStyle.Width(width)
	switch {
	case e.CrossedOff:
		style = style.Foreground(t.TextMuted()).Strikethrough(true)
	case strings.HasPrefix(e.Reason, "rejected:"):
		style = style.Foreground(t.Error())
	case e.Reason != "":
		style = style.Foreground(t.Text())
	default:
		style = style.Foreground(t.Text())
	}
	if selected {
		style = style.Foreground(t.Primary()).Bold(true)
	}

	return style.Render(marker + displayPath + tail)
}

// ansiTruncate trims a string to roughly maxWidth display cells.
func ansiTruncate(s string, maxWidth int, ellipsis string) string {
	if maxWidth <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= maxWidth {
		return s
	}
	// Simple rune-based truncation; ignores ANSI since we operate on raw paths.
	runes := []rune(s)
	if len(runes) <= maxWidth {
		return s
	}
	if len(ellipsis) > maxWidth {
		ellipsis = ellipsis[:maxWidth]
	}
	return string(runes[:maxWidth-len(ellipsis)]) + ellipsis
}

func NewContextPaneCmp(app *app.App) tea.Model {
	return &contextPaneCmp{
		app:  app,
		keys: defaultContextPaneKeys,
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}