package dashboard

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleIndexRequiresToken(t *testing.T) {
	server := &Server{token: "secret"}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	server.handleIndex(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized without token, got %d", rec.Code)
	}
}

func TestHandleIndexAcceptsToken(t *testing.T) {
	server := &Server{token: "secret"}
	req := httptest.NewRequest(http.MethodGet, "/?token=secret", nil)
	rec := httptest.NewRecorder()

	server.handleIndex(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected ok with token, got %d", rec.Code)
	}
	if rec.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Fatalf("expected html content type, got %q", rec.Header().Get("Content-Type"))
	}
}

func TestSnapshotRejectsMutationMethods(t *testing.T) {
	server := &Server{token: "secret"}
	req := httptest.NewRequest(http.MethodPost, "/api/snapshot?token=secret", nil)
	rec := httptest.NewRecorder()

	server.handleSnapshot(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected method not allowed, got %d", rec.Code)
	}
}
