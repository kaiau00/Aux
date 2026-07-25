package ids

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewUnique(t *testing.T) {
	seen := make(map[string]struct{}, 1000)
	for range 1000 {
		id := New()
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate id generated: %s", id)
		}
		seen[id] = struct{}{}
		if _, err := uuid.Parse(id); err != nil {
			t.Fatalf("id %q is not a valid uuid: %v", id, err)
		}
	}
}

func TestNewIsTimeSortable(t *testing.T) {
	first := New()
	time.Sleep(2 * time.Millisecond)
	second := New()
	if first >= second {
		t.Fatalf("expected UUIDv7 ids to sort by creation time: %q !< %q", first, second)
	}
}
