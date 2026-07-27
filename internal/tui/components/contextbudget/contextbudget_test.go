package contextbudget

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/aux-ai/aux-cli/internal/viewmodel"
)

func sampleBudget() viewmodel.ContextBudgetVM {
	return viewmodel.ContextBudgetVM{
		TotalTokens: 18200,
		LimitTokens: 64000,
		Categories: []viewmodel.ContextCategoryVM{
			{Label: "Task and plan", Tokens: 3100},
			{Label: "Active code", Tokens: 8400},
			{Label: "Tool results", Tokens: 2200},
		},
		ResidentPages: 6,
		PinnedPages:   1,
		SavedTokens:   3100,
	}
}

func TestFormatTokens(t *testing.T) {
	cases := map[int64]string{
		0:         "0",
		512:       "512",
		18200:     "18.2k",
		64000:     "64k",
		2_500_000: "2.5M",
	}
	for n, want := range cases {
		if got := FormatTokens(n); got != want {
			t.Fatalf("FormatTokens(%d)=%q want %q", n, got, want)
		}
	}
}

func TestHeaderText(t *testing.T) {
	got := HeaderText(sampleBudget())
	for _, want := range []string{"18.2k", "64k", "28%"} {
		if !strings.Contains(got, want) {
			t.Fatalf("header %q missing %q", got, want)
		}
	}
	// Unknown limit omits the ratio rather than dividing by zero.
	noLimit := HeaderText(viewmodel.ContextBudgetVM{TotalTokens: 1000})
	if strings.Contains(noLimit, "/") {
		t.Fatalf("unknown limit should not show a ratio: %q", noLimit)
	}
}

func TestSignalsText(t *testing.T) {
	got := SignalsText(sampleBudget())
	for _, want := range []string{"6 resident", "1 pinned", "saved 3.1k"} {
		if !strings.Contains(got, want) {
			t.Fatalf("signals %q missing %q", got, want)
		}
	}
	if SignalsText(viewmodel.ContextBudgetVM{}) != "" {
		t.Fatal("empty budget should have no signals")
	}
}

func TestRenderEmptyHidden(t *testing.T) {
	if got := Render(viewmodel.ContextBudgetVM{}, 40); got != "" {
		t.Fatalf("empty budget should render nothing, got %q", got)
	}
	if got := Render(sampleBudget(), 0); got != "" {
		t.Fatalf("zero width should render nothing, got %q", got)
	}
}

func TestRenderLargestCategoryFirst(t *testing.T) {
	out := Render(sampleBudget(), 40)
	// "Active code" (8.4k) must appear before "Task and plan" (3.1k).
	active := strings.Index(out, "Active code")
	task := strings.Index(out, "Task and plan")
	if active < 0 || task < 0 || active > task {
		t.Fatalf("categories should be largest-first; active=%d task=%d\n%s", active, task, out)
	}
}

func TestRenderFitsWidth(t *testing.T) {
	const width = 28
	out := Render(sampleBudget(), width)
	for _, line := range strings.Split(out, "\n") {
		if w := lipgloss.Width(line); w > width {
			t.Fatalf("line %q width %d exceeds %d", line, w, width)
		}
	}
}

func TestRenderReconcilesTotal(t *testing.T) {
	// The header total must equal the projected total (the display reconciles
	// with the prompt manifest, §13.11).
	vm := sampleBudget()
	out := Render(vm, 60)
	if !strings.Contains(out, FormatTokens(vm.TotalTokens)) {
		t.Fatalf("rendered budget missing projected total %s:\n%s", FormatTokens(vm.TotalTokens), out)
	}
}
