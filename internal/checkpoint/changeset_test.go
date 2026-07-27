package checkpoint

import "testing"

func TestChangesFromClassifiesOperations(t *testing.T) {
	before := map[string]string{
		"mod.go":  "old",
		"del.go":  "gone-soon",
		"same.go": "x",
	}
	latest := []FileVersion{
		{Path: "mod.go", Content: "new"},   // modify
		{Path: "add.go", Content: "fresh"}, // add (no before)
		{Path: "del.go", Content: ""},      // delete (empty after)
		{Path: "same.go", Content: "x"},    // unchanged -> skipped
	}
	changes := ChangesFrom(before, latest)
	got := map[string]Operation{}
	for _, c := range changes {
		got[c.Path] = c.Operation
	}
	if len(changes) != 3 {
		t.Fatalf("expected 3 changes (same.go skipped), got %d: %+v", len(changes), got)
	}
	if got["mod.go"] != OpModify || got["add.go"] != OpAdd || got["del.go"] != OpDelete {
		t.Fatalf("wrong operations: %+v", got)
	}
	if _, ok := got["same.go"]; ok {
		t.Fatal("net-unchanged file must be skipped")
	}
}
