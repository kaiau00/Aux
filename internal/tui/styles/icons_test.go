package styles

import "testing"

func TestSupportsUnicodeDefaultsTrueWithNoLocale(t *testing.T) {
	for _, k := range []string{"AUX_ASCII_ICONS", "LC_ALL", "LC_CTYPE", "LANG"} {
		t.Setenv(k, "")
	}
	if !SupportsUnicode() {
		t.Fatal("expected Unicode support to default true when no locale is configured")
	}
}

func TestSupportsUnicodeFalseForCLocale(t *testing.T) {
	for _, k := range []string{"AUX_ASCII_ICONS", "LC_ALL", "LC_CTYPE"} {
		t.Setenv(k, "")
	}
	t.Setenv("LANG", "C")
	if SupportsUnicode() {
		t.Fatal("expected the C locale to be treated as ASCII-only")
	}
}

func TestSupportsUnicodeTrueForUTF8Locale(t *testing.T) {
	for _, k := range []string{"AUX_ASCII_ICONS", "LC_ALL", "LC_CTYPE"} {
		t.Setenv(k, "")
	}
	t.Setenv("LANG", "en_US.UTF-8")
	if !SupportsUnicode() {
		t.Fatal("expected a UTF-8 locale to support Unicode")
	}
}

func TestSupportsUnicodeExplicitOverride(t *testing.T) {
	for _, k := range []string{"LC_ALL", "LC_CTYPE"} {
		t.Setenv(k, "")
	}
	t.Setenv("LANG", "en_US.UTF-8")
	t.Setenv("AUX_ASCII_ICONS", "1")
	if SupportsUnicode() {
		t.Fatal("expected AUX_ASCII_ICONS=1 to force ASCII regardless of locale")
	}
}

func TestPickIconRespectsUnicodeSupport(t *testing.T) {
	for _, k := range []string{"LC_ALL", "LC_CTYPE"} {
		t.Setenv(k, "")
	}
	t.Setenv("LANG", "en_US.UTF-8")
	t.Setenv("AUX_ASCII_ICONS", "")
	if got := pickIcon("✓", "v"); got != "✓" {
		t.Fatalf("expected the unicode glyph, got %q", got)
	}

	t.Setenv("AUX_ASCII_ICONS", "1")
	if got := pickIcon("✓", "v"); got != "v" {
		t.Fatalf("expected the ascii fallback, got %q", got)
	}
}
