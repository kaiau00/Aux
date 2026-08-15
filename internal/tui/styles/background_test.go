package styles

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// TestForceReplaceBackgroundRespectsColorProfile locks in a fix for a real
// incident on Apple's Terminal.app: the old implementation always emitted a
// raw 24-bit ("48;2;R;G;B") background sequence regardless of what color
// profile the terminal actually supports. A terminal with incomplete
// TrueColor support can silently drop or misrender that code, leaving text
// rendered against whatever background happened to be active nearby instead
// of the intended one — present and selectable, but visually unreadable.
func TestForceReplaceBackgroundRespectsColorProfile(t *testing.T) {
	orig := lipgloss.ColorProfile()
	defer lipgloss.SetColorProfile(orig)
	origDark := lipgloss.HasDarkBackground()
	defer lipgloss.SetHasDarkBackground(origDark)
	lipgloss.SetHasDarkBackground(true)

	color := lipgloss.AdaptiveColor{Dark: "#1f1a12", Light: "#fff3d6"}
	input := "\x1b[38;2;244;232;208mhi\x1b[0m"

	cases := []struct {
		name    string
		profile termenv.Profile
		want    string // substring the background sequence must contain
		absent  string // substring that must NOT appear (e.g. the old always-24-bit form)
	}{
		{"TrueColor", termenv.TrueColor, "48;2;31;26;18", ""},
		{"ANSI256", termenv.ANSI256, "48;5;", "48;2;"},
		{"Ascii", termenv.Ascii, "", "48;"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			lipgloss.SetColorProfile(c.profile)
			out := ForceReplaceBackgroundWithLipgloss(input, color)
			if c.want != "" && !strings.Contains(out, c.want) {
				t.Fatalf("profile %s: expected output to contain %q, got %q", c.name, c.want, out)
			}
			if c.absent != "" && strings.Contains(out, c.absent) {
				t.Fatalf("profile %s: expected output to NOT contain %q (forced TrueColor on an unsupported profile), got %q", c.name, c.absent, out)
			}
		})
	}
}

func TestForceReplaceBackgroundStripsExistingBackgroundCodes(t *testing.T) {
	origDark := lipgloss.HasDarkBackground()
	defer lipgloss.SetHasDarkBackground(origDark)
	lipgloss.SetHasDarkBackground(true)
	orig := lipgloss.ColorProfile()
	defer lipgloss.SetColorProfile(orig)
	lipgloss.SetColorProfile(termenv.TrueColor)

	color := lipgloss.AdaptiveColor{Dark: "#1f1a12", Light: "#fff3d6"}
	input := "\x1b[38;2;1;2;3;48;2;9;9;9mhi\x1b[48;5;200mthere\x1b[0m"

	out := ForceReplaceBackgroundWithLipgloss(input, color)
	if strings.Contains(out, "9;9;9") || strings.Contains(out, "48;5;200") {
		t.Fatalf("expected the old background codes to be stripped, got %q", out)
	}
	if strings.Count(out, "48;2;31;26;18") != 3 {
		t.Fatalf("expected the new background to be applied to all 3 escape sequences, got %q", out)
	}
}
