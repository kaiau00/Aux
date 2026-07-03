package context

import "testing"

func TestRankFilesUsesPromptPersonalization(t *testing.T) {
	graph := Graph{
		Nodes: []GraphNode{
			{ID: "auth", Name: "Authenticate", Path: "internal/auth.go"},
			{ID: "db", Name: "Database", Path: "internal/db.go"},
			{ID: "ui", Name: "View", Path: "internal/ui.go"},
		},
		Edges: []GraphEdge{
			{From: "auth", To: "db"},
			{From: "ui", To: "db"},
		},
	}

	ranked := RankFiles(graph, "fix authenticate token flow", PageRankOptions{})
	if len(ranked) == 0 {
		t.Fatal("expected ranked files")
	}
	if ranked[0].Path != "internal/auth.go" {
		t.Fatalf("expected auth file first, got %s", ranked[0].Path)
	}
}

func TestRankFilesHandlesDanglingNodes(t *testing.T) {
	graph := Graph{
		Nodes: []GraphNode{
			{ID: "target", Name: "Target", Path: "target.go"},
			{ID: "dangling", Name: "Dangling", Path: "dangling.go"},
		},
	}

	ranked := RankFiles(graph, "target", PageRankOptions{MaxIterations: 10})
	if len(ranked) != 2 {
		t.Fatalf("expected two ranked files, got %d", len(ranked))
	}
	if ranked[0].Path != "target.go" {
		t.Fatalf("expected personalized dangling graph to favor target.go, got %s", ranked[0].Path)
	}
}
