package dashboard

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTasksPageServedWithToken(t *testing.T) {
	server := &Server{token: "secret"}
	req := httptest.NewRequest(http.MethodGet, "/tasks?token=secret", nil)
	rec := httptest.NewRecorder()
	server.handleTasksPage(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("content-type = %q, want text/html", ct)
	}
	if !strings.Contains(rec.Body.String(), "js/tasks.js") {
		t.Fatalf("tasks page must load its script")
	}
}

func TestTasksPageRequiresToken(t *testing.T) {
	server := &Server{token: "secret"}
	req := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	rec := httptest.NewRecorder()
	server.handleTasksPage(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", rec.Code)
	}
}

func TestTasksAssetsEmbeddedAndWired(t *testing.T) {
	// The workspace JS must consume the read-only /api/v1 projections.
	js, err := assets.ReadFile("assets/js/tasks.js")
	if err != nil {
		t.Fatalf("read tasks.js: %v", err)
	}
	for _, want := range []string{"/api/v1/tasks", "/api/v1/tasks/"} {
		if !strings.Contains(string(js), want) {
			t.Fatalf("tasks.js must call %q", want)
		}
	}
	// It must escape untrusted content (task objectives, file paths).
	if !strings.Contains(string(js), "function esc(") {
		t.Fatal("tasks.js must HTML-escape untrusted content")
	}
	if _, err := assets.ReadFile("assets/css/tasks.css"); err != nil {
		t.Fatalf("tasks.css must be embedded: %v", err)
	}
}
