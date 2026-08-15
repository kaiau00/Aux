package theme

import (
	"testing"
)

func TestContrastRatioKnownValues(t *testing.T) {
	// Black on white is the maximum possible ratio, 21:1.
	ratio, err := ContrastRatio("#000000", "#ffffff")
	if err != nil {
		t.Fatalf("ContrastRatio: %v", err)
	}
	if ratio < 20.9 || ratio > 21.1 {
		t.Fatalf("black/white ratio = %.2f, want ~21", ratio)
	}
	// Identical colors have no contrast: ratio 1.
	ratio, err = ContrastRatio("#336699", "#336699")
	if err != nil {
		t.Fatalf("ContrastRatio: %v", err)
	}
	if ratio < 0.99 || ratio > 1.01 {
		t.Fatalf("identical-color ratio = %.2f, want 1", ratio)
	}
	// Order must not matter.
	a, _ := ContrastRatio("#111111", "#eeeeee")
	b, _ := ContrastRatio("#eeeeee", "#111111")
	if a != b {
		t.Fatalf("ContrastRatio should be symmetric: %.4f vs %.4f", a, b)
	}
}

func TestContrastRatioRejectsInvalidHex(t *testing.T) {
	if _, err := ContrastRatio("not-a-color", "#ffffff"); err == nil {
		t.Fatal("expected an error for an invalid hex color")
	}
}

// wcagAANormal is WCAG 2.1's minimum contrast ratio for normal text (SC
// 1.4.3). Every registered theme already clears this without any color
// changes, so it's used as the real floor rather than settling for the
// lower large-text minimum.
const wcagAANormal = 4.5

// TestThemeTextMeetsMinimumContrast checks every registered theme's primary
// text-on-background contrast, in both light and dark variants, against the
// WCAG AA-large floor (roadmapplan.md §13.18). This is a deterministic,
// offline substitute for the browser-based contrast tooling this environment
// doesn't have (docs/live-gates-and-remainders.md).
func TestThemeTextMeetsMinimumContrast(t *testing.T) {
	for _, name := range AvailableThemes() {
		th := GetTheme(name)
		if th == nil {
			t.Fatalf("theme %q registered but not retrievable", name)
		}
		cases := []struct {
			label string
			fg    string
			bg    string
		}{
			{"dark: text/background", th.Text().Dark, th.Background().Dark},
			{"light: text/background", th.Text().Light, th.Background().Light},
		}
		for _, c := range cases {
			ratio, err := ContrastRatio(c.fg, c.bg)
			if err != nil {
				t.Fatalf("theme %q %s: %v", name, c.label, err)
			}
			if ratio < wcagAANormal {
				t.Errorf("theme %q %s contrast = %.2f, want >= %.1f (fg=%s bg=%s)",
					name, c.label, ratio, wcagAANormal, c.fg, c.bg)
			}
		}
	}
}
