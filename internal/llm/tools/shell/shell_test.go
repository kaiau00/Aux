package shell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func writeTemp(t *testing.T, content []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "out")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

func TestReadFileOrEmptyReturnsSmallOutputVerbatim(t *testing.T) {
	want := "hello\nworld\n"
	got := readFileOrEmpty(writeTemp(t, []byte(want)))
	if got != want {
		t.Fatalf("output under the cap must be returned unchanged, got %q", got)
	}
}

func TestReadFileOrEmptyOnMissingFile(t *testing.T) {
	if got := readFileOrEmpty(filepath.Join(t.TempDir(), "nope")); got != "" {
		t.Fatalf("expected empty string for a missing file, got %q", got)
	}
}

// The bug this guards: a command producing hundreds of megabytes was read fully
// into memory before the tool layer truncated it to ~30KB.
func TestReadFileOrEmptyBoundsOversizedOutput(t *testing.T) {
	oversized := MaxCaptureBytes * 4
	content := make([]byte, oversized)
	for i := range content {
		content[i] = byte('a' + i%26)
	}
	// Distinctive markers at both ends so we can prove which parts survived.
	copy(content[:10], []byte("HEADMARKER"))
	copy(content[oversized-10:], []byte("TAILMARKER"))

	got := readFileOrEmpty(writeTemp(t, content))

	if len(got) > MaxCaptureBytes+512 {
		t.Fatalf("captured %d bytes, which exceeds the %d cap plus marker", len(got), MaxCaptureBytes)
	}
	if !strings.HasPrefix(got, "HEADMARKER") {
		t.Fatal("the head of the output must be preserved")
	}
	if !strings.HasSuffix(got, "TAILMARKER") {
		t.Fatal("the tail of the output must be preserved")
	}
	if !strings.Contains(got, "bytes of output dropped") {
		t.Fatal("truncated output must say so, or the model reads a gap as the real output")
	}
}

// Cutting at a fixed byte offset lands mid-rune for any non-ASCII output, which
// would otherwise emit replacement characters into the transcript.
func TestReadFileOrEmptyKeepsValidUTF8WhenTruncating(t *testing.T) {
	// A 3-byte rune repeated never aligns with the power-of-two cut points.
	unit := []byte("日")
	content := make([]byte, 0, MaxCaptureBytes*3)
	for len(content) < MaxCaptureBytes*3 {
		content = append(content, unit...)
	}

	got := readFileOrEmpty(writeTemp(t, content))

	if !utf8.ValidString(got) {
		t.Fatal("truncation must not split a multi-byte rune")
	}
	if !strings.Contains(got, "bytes of output dropped") {
		t.Fatal("expected the truncation marker")
	}
}

func TestTrimPartialRuneHelpers(t *testing.T) {
	full := []byte("日本語")

	// Cut one byte short at the end: the trailing partial rune must go.
	if got := trimPartialRuneSuffix(full[:len(full)-1]); got != "日本" {
		t.Fatalf("suffix trim: got %q, want %q", got, "日本")
	}
	// Cut one byte into the start: the leading partial rune must go.
	if got := trimPartialRunePrefix(full[1:]); got != "本語" {
		t.Fatalf("prefix trim: got %q, want %q", got, "本語")
	}
	// Already-valid input is untouched.
	if got := trimPartialRuneSuffix(full); got != "日本語" {
		t.Fatalf("valid input must pass through, got %q", got)
	}
}
