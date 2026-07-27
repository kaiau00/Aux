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
	for _, p := range []string{"/css/../server.go", "/etc/passwd", "/assets/index.html", "/css/missing.css"} {
		req := httptest.NewRequest(http.MethodGet, p, nil)
		rec := httptest.NewRecorder()
		server.handleStaticAsset(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("path %q: status = %d, want 404", p, rec.Code)
		}
	}
}

func TestIndexReferencesSplitTokens(t *testing.T) {
	// The split is only real if index.html actually loads the tokens stylesheet.
	data, err := assets.ReadFile("assets/index.html")
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	if !strings.Contains(string(data), `href="css/tokens.css"`) {
		t.Fatal("index.html must reference the split css/tokens.css")
	}
}
