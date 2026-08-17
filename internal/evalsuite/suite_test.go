package evalsuite

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validTask(id string) Task {
	return Task{
		ID:           id,
		Repo:         "/tmp/repo",
		BaseRevision: "abc123",
		Prompt:       "do the thing",
		Success:      []string{"go test ./..."},
	}
}

func TestValidSuitePasses(t *testing.T) {
	s := Suite{Name: "ok", Tasks: []Task{validTask("a"), validTask("b")}}
	if err := s.Validate(); err != nil {
		t.Fatalf("expected a valid suite, got %v", err)
	}
}

// A task with no success command always passes, which silently inflates the
// success rate the whole gate depends on.
func TestTaskWithoutSuccessCommandIsRejected(t *testing.T) {
	bad := validTask("a")
	bad.Success = nil
	err := Suite{Name: "s", Tasks: []Task{bad}}.Validate()
	if err == nil {
		t.Fatal("a task that cannot fail must be rejected")
	}
	if !strings.Contains(err.Error(), "never fail") {
		t.Fatalf("the error should explain why this matters, got %v", err)
	}
}

// Without a pinned revision the task measures a moving target.
func TestTaskWithoutBaseRevisionIsRejected(t *testing.T) {
	bad := validTask("a")
	bad.BaseRevision = ""
	err := Suite{Name: "s", Tasks: []Task{bad}}.Validate()
	if err == nil {
		t.Fatal("an unpinned task must be rejected")
	}
	if !strings.Contains(err.Error(), "comparable") {
		t.Fatalf("the error should explain why this matters, got %v", err)
	}
}

// Duplicate ids would silently merge two tasks' histories.
func TestDuplicateTaskIDsAreRejected(t *testing.T) {
	err := Suite{Name: "s", Tasks: []Task{validTask("dup"), validTask("dup")}}.Validate()
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("expected a duplicate-id error, got %v", err)
	}
}

// All problems at once: fixing a suite one error per run is miserable.
func TestValidateReportsEveryProblemAtOnce(t *testing.T) {
	err := Suite{Tasks: []Task{{}}}.Validate()
	if err == nil {
		t.Fatal("expected errors")
	}
	msg := err.Error()
	for _, want := range []string{"no name", "missing id", "missing repo", "missing baseRevision", "missing prompt", "never fail"} {
		if !strings.Contains(msg, want) {
			t.Errorf("expected %q in the report, got:\n%s", want, msg)
		}
	}
}

func TestEmptySuiteIsRejected(t *testing.T) {
	if err := (Suite{Name: "empty"}).Validate(); err == nil {
		t.Fatal("a suite with no tasks proves nothing and must be rejected")
	}
}

func TestCorrectedCount(t *testing.T) {
	a, b, c := validTask("a"), validTask("b"), validTask("c")
	b.Corrected, c.Corrected = true, true

	if got := (Suite{Name: "s", Tasks: []Task{a, b, c}}).CorrectedCount(); got != 2 {
		t.Fatalf("got %d corrected tasks, want 2", got)
	}
}

func TestLoadSuiteRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "suite.json")
	content := `{
  "name": "smoke",
  "tasks": [
    {
      "id": "adds-a-test",
      "repo": "/tmp/example",
      "baseRevision": "abc123",
      "prompt": "add a test for Foo",
      "success": ["go test ./..."],
      "corrected": true
    }
  ]
}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	s, err := LoadSuite(path)
	if err != nil {
		t.Fatalf("LoadSuite: %v", err)
	}
	if s.Name != "smoke" || len(s.Tasks) != 1 || s.Tasks[0].ID != "adds-a-test" {
		t.Fatalf("unexpected suite: %+v", s)
	}
	if s.CorrectedCount() != 1 {
		t.Fatal("the corrected flag should survive loading")
	}
}

// Loading must validate: an invalid suite discovered mid-run has already cost
// money.
func TestLoadSuiteValidates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	os.WriteFile(path, []byte(`{"name":"bad","tasks":[{"id":"x"}]}`), 0o600)

	if _, err := LoadSuite(path); err == nil {
		t.Fatal("LoadSuite must validate, not just parse")
	}
}

func TestSuiteRunSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "baseline.json")
	original := SuiteRun{
		Suite: "smoke",
		Label: "baseline",
		Runs: []Run{
			{TaskID: "a", Succeeded: true, InputTokens: 100, OutputTokens: 50, Turns: 3, Cost: 0.01},
			{TaskID: "b", Succeeded: false, FailureReason: "tests failed", CostUnknown: true},
		},
	}
	if err := original.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := LoadRun(path)
	if err != nil {
		t.Fatalf("LoadRun: %v", err)
	}
	if loaded.Summarize() != original.Summarize() {
		t.Fatalf("a saved baseline must survive the round trip:\n got %+v\nwant %+v",
			loaded.Summarize(), original.Summarize())
	}
	if loaded.Runs[1].FailureReason != "tests failed" {
		t.Fatal("failure reasons must persist, or a red baseline is unreadable later")
	}
}
