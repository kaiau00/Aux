package chat

import (
	"strings"
	"testing"
)

func TestComposerPlaceholder(t *testing.T) {
	cases := []struct {
		followUp, slash bool
		want            string
	}{
		{false, false, "Describe a task for Aux…"},
		{true, false, "Reply or refine the task…"},
		{false, true, "Run a command…"},
		{true, true, "Run a command…"}, // slash wins
	}
	for _, c := range cases {
		if got := composerPlaceholder(c.followUp, c.slash); got != c.want {
			t.Fatalf("composerPlaceholder(%v,%v)=%q want %q", c.followUp, c.slash, got, c.want)
		}
	}
}

func TestComposerHintReflectsState(t *testing.T) {
	// Busy shows a clear disabled reason and cancel affordance.
	if got := composerHint(true, true, false); !strings.Contains(got, "working") || !strings.Contains(got, "cancel") {
		t.Fatalf("busy hint should state working + cancel: %q", got)
	}
	// Unfocused invites focus.
	if got := composerHint(false, false, false); !strings.Contains(got, "focus") {
		t.Fatalf("unfocused hint should invite focus: %q", got)
	}
	// Focused surfaces send + the multiline affordance (discoverable, not hidden).
	focused := composerHint(true, false, false)
	for _, want := range []string{"enter send", "newline", "editor"} {
		if !strings.Contains(focused, want) {
			t.Fatalf("focused hint missing %q: %q", want, focused)
		}
	}
	// Attachment delete affordance only appears when there are attachments.
	if strings.Contains(focused, "delete") {
		t.Fatalf("no-attachment hint should not mention delete: %q", focused)
	}
	if got := composerHint(true, false, true); !strings.Contains(got, "delete") {
		t.Fatalf("attachment hint should mention delete: %q", got)
	}
}

func TestIsSlashCommand(t *testing.T) {
	cases := map[string]bool{
		"/init":        true,
		"  /compact":   true,
		"/":            true,
		"/run args":    false, // has whitespace -> composing args, not the command token
		"hello":        false,
		"":             false,
		"a/b":          false,
		"/multi\nline": false,
	}
	for in, want := range cases {
		if got := isSlashCommand(in); got != want {
			t.Fatalf("isSlashCommand(%q)=%v want %v", in, got, want)
		}
	}
}
