package page

import (
	"strings"
	"testing"

	"github.com/aux-ai/aux-cli/internal/app"
	"github.com/aux-ai/aux-cli/internal/tui/components/chat"
	"github.com/aux-ai/aux-cli/internal/tui/layout"
	tea "github.com/charmbracelet/bubbletea"
)

// newTestChatPage builds a minimal chatPage without the heavier services
// NewChatPage's messages/editor components need, just enough to exercise the
// context-drawer overlay logic in View()/Update().
func newTestChatPage(t *testing.T) *chatPage {
	t.Helper()
	left := layout.NewContainer(chat.NewContextPaneCmp(&app.App{}), layout.WithPadding(0, 0, 0, 0))
	contextPane := chat.NewContextPaneCmp(&app.App{})
	contextPaneContainer := layout.NewContainer(contextPane, layout.WithPadding(1, 1, 1, 1))
	p := &chatPage{
		contextPane:          contextPane,
		contextPaneContainer: contextPaneContainer,
		layout: layout.NewSplitPane(
			layout.WithLeftPanel(left),
			layout.WithRightPanel(contextPaneContainer),
			layout.WithCollapseRightBelow(narrowWidthThreshold),
		),
	}
	p.SetSize(60, 24)
	p.width, p.height = 60, 24
	return p
}

func TestContextDrawerTogglesViaKeybinding(t *testing.T) {
	p := newTestChatPage(t)
	before := p.View()

	_, _ = p.Update(tea.KeyMsg{Type: tea.KeyCtrlG})
	if !p.showContextDrawer {
		t.Fatal("expected ctrl+g to open the context drawer")
	}
	after := p.View()
	if after == before {
		t.Fatal("expected the drawer overlay to change the rendered view")
	}

	_, _ = p.Update(tea.KeyMsg{Type: tea.KeyCtrlG})
	if p.showContextDrawer {
		t.Fatal("expected a second ctrl+g to close the drawer")
	}
}

func TestEscClosesOpenContextDrawerBeforeAnythingElse(t *testing.T) {
	p := newTestChatPage(t)
	p.showContextDrawer = true

	_, _ = p.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if p.showContextDrawer {
		t.Fatal("expected esc to close an open context drawer")
	}
}

func TestContextDrawerReachableWhenNarrow(t *testing.T) {
	p := newTestChatPage(t)
	p.SetSize(60, 24) // below narrowWidthThreshold: split layout drops the right panel
	p.width, p.height = 60, 24

	collapsed := p.View()
	if strings.TrimSpace(collapsed) == "" {
		t.Skip("collapsed layout rendered nothing to compare against")
	}

	p.showContextDrawer = true
	drawer := p.View()
	if drawer == collapsed {
		t.Fatal("expected the drawer to make the collapsed context panel visible again")
	}
}
