package models

import "testing"

func TestInferCatalogProvider(t *testing.T) {
	tests := []struct {
		endpoint string
		want     string
	}{
		{"https://api.minimax.io/v1", "minimax"},
		{"http://localhost:1234/v1", ""},
		{"https://api.openai.com/v1", "openai"},
	}

	for _, tt := range tests {
		if got := inferCatalogProvider(tt.endpoint); got != tt.want {
			t.Errorf("inferCatalogProvider(%q) = %q, want %q", tt.endpoint, got, tt.want)
		}
	}
}

func TestResolveLocalModelLimitsUsesCatalog(t *testing.T) {
	resetModelsCatalogForTest()
	catalogByID = map[string]catalogLimits{
		"MiniMax-M3": {Context: 1_000_000, Output: 128_000},
	}
	catalogByProvider = map[string]map[string]catalogLimits{
		"minimax": {
			"MiniMax-M3": {Context: 1_000_000, Output: 128_000},
		},
	}
	catalogLoaded = true

	contextWindow, defaultMaxTokens := resolveLocalModelLimits(localModel{ID: "MiniMax-M3"}, "https://api.minimax.io/v1")
	if contextWindow != 1_000_000 {
		t.Fatalf("contextWindow = %d, want 1000000", contextWindow)
	}
	if defaultMaxTokens != 128_000 {
		t.Fatalf("defaultMaxTokens = %d, want 128000", defaultMaxTokens)
	}
}

func TestResolveLocalModelLimitsPrefersLMStudio(t *testing.T) {
	resetModelsCatalogForTest()
	catalogByID = map[string]catalogLimits{
		"qwen3": {Context: 1_000_000, Output: 128_000},
	}
	catalogLoaded = true

	contextWindow, defaultMaxTokens := resolveLocalModelLimits(localModel{
		ID:                  "qwen3",
		LoadedContextLength: 65_536,
		MaxContextLength:    131_072,
	}, "http://localhost:1234/v1")

	if contextWindow != 65_536 {
		t.Fatalf("contextWindow = %d, want 65536", contextWindow)
	}
	if defaultMaxTokens != 65_536 {
		t.Fatalf("defaultMaxTokens = %d, want 65536", defaultMaxTokens)
	}
}
