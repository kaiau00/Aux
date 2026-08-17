package dashboard

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStaticAssetServesTokens(t *testing.T) {
	server := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/css/tokens.css", nil)
	rec := httptest.NewRecorder()
	server.handleStaticAsset(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/css") {
		t.Fatalf("content-type = %q, want text/css", ct)
	}
	if !strings.Contains(rec.Body.String(), "--bg") {
		t.Fatalf("tokens.css should define design tokens, got: %s", rec.Body.String())
	}
}

func TestStaticAssetRejectsTraversalAndUnknownTrees(t *testing.T) {
	server := &Server{}
	for _, p := range []string{"/css/../server.go", "/etc/passwd", "/assets/tasks.html", "/css/missing.css"} {
		req := httptest.NewRequest(http.MethodGet, p, nil)
		rec := httptest.NewRecorder()
		server.handleStaticAsset(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("path %q: status = %d, want 404", p, rec.Code)
		}
	}
}

func TestEveryPageReferencesSplitTokens(t *testing.T) {
	// The split is only real if every top-level page actually loads the
	// tokens stylesheet (and the shared base/components stylesheets).
	for _, page := range []string{"tasks.html", "sessions.html"} {
		data, err := assets.ReadFile("assets/" + page)
		if err != nil {
			t.Fatalf("read %s: %v", page, err)
		}
		content := string(data)
		if !strings.Contains(content, `href="css/tokens.css"`) {
			t.Fatalf("%s must reference the split css/tokens.css", page)
		}
		if !strings.Contains(content, `href="css/base.css"`) {
			t.Fatalf("%s must reference the shared css/base.css", page)
		}
		if !strings.Contains(content, `src="js/nav.js"`) {
			t.Fatalf("%s must load the shared nav.js", page)
		}
	}
}
