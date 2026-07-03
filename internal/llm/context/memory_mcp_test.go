package context

import "testing"

func TestParseGraphResponse(t *testing.T) {
	graph, err := ParseGraphResponse(`{
		"nodes": [
			{"id": "a", "name": "A", "path": "a.go"},
			{"id": "b", "name": "B", "path": "b.go"}
		],
		"edges": [
			{"from": "a", "to": "b", "type": "imports"}
		]
	}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(graph.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(graph.Nodes))
	}
	if len(graph.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(graph.Edges))
	}
}

func TestParseGraphResponseSupportsAlternateKeys(t *testing.T) {
	graph, err := ParseGraphResponse(`{
		"symbols": [
			{"symbol_id": "handler", "symbol": "Handler", "file_path": "handler.go"}
		],
		"relationships": []
	}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(graph.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(graph.Nodes))
	}
	if graph.Nodes[0].Path != "handler.go" {
		t.Fatalf("expected handler.go, got %s", graph.Nodes[0].Path)
	}
}
