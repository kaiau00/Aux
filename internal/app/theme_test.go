package app

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// TestBackgroundOverride locks in the fix for a real incident: markdown
// message text (internal/tui/styles/markdown.go) resolves its colors via
// lipgloss.HasDarkBackground(), which is a single terminal query (OSC 11)
// cached for the whole process. If that query is misread or times out in a
// given terminal/multiplexer, every AdaptiveColor — including the high-
// contrast text/background pair used for message content — can resolve to
// the wrong (inverted) variant for the entire session, making every message
// unreadable while the agent keeps working normally. Pinning an explicit
// default removes the dependency on that query for the common case.
func TestBackgroundOverride(t *testing.T) {
	cases := []struct {
		pref     string
		wantDark bool
		wantOK   bool
	}{
		{"", true, true}, // unset -> default to dark, matches every built-in theme
		{"dark", true, true},
		{"light", false, true},
		{"auto", false, false}, // leaves live detection in charge
		{"bogus", true, true},  // unrecognized -> safe default, not "auto"
	}
	for _, c := range cases {
		dark, ok := backgroundOverride(c.pref)
		if dark != c.wantDark || ok != c.wantOK {
			t.Fatalf("backgroundOverride(%q) = (%v, %v), want (%v, %v)", c.pref, dark, ok, c.wantDark, c.wantOK)
		}
	}
}

// TestPinnedBackgroundSurvivesEarlierMisdetection proves the fix actually
// neutralizes a bad prior reading, not just a hypothetical one:
// bubbletea itself calls lipgloss.HasDarkBackground() in a package init()
// (tea_init.go) before any of aux's own code runs, to avoid hanging later —
// so by the time initTheme's logic runs, lipgloss's one-shot terminal query
// has *already* fired and cached whatever it detected. If that early
// detection was wrong (as can happen with cloud-sync/multiplexed terminals),
// this override must still win.
func TestPinnedBackgroundSurvivesEarlierMisdetection(t *testing.T) {
	// Simulate a bad early detection, exactly as bubbletea's init() could
	// leave behind on a terminal that answers the OSC 11 query incorrectly.
	lipgloss.SetHasDarkBackground(false)

	dark, ok := backgroundOverride("dark")
	if !ok {
		t.Fatal("backgroundOverride(\"dark\") should always resolve to an explicit override")
	}
	lipgloss.SetHasDarkBackground(dark)

	if !lipgloss.HasDarkBackground() {
		t.Fatal("expected the pinned dark background to override the earlier bad detection")
	}
}
