package context

import (
	"math"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

type GraphNode struct {
	ID   string
	Name string
	Path string
	Type string
}

type GraphEdge struct {
	From string
	To   string
	Type string
}

type Graph struct {
	Nodes []GraphNode
	Edges []GraphEdge
}

type RankedFile struct {
	Path  string
	Score float64
}

type PageRankOptions struct {
	Damping       float64
	MaxIterations int
	Epsilon       float64
}

func RankFiles(graph Graph, prompt string, options PageRankOptions) []RankedFile {
	if len(graph.Nodes) == 0 {
		return nil
	}
	if options.Damping <= 0 || options.Damping >= 1 {
		options.Damping = 0.85
	}
	if options.MaxIterations <= 0 {
		options.MaxIterations = 50
	}
	if options.Epsilon <= 0 {
		options.Epsilon = 1e-6
	}

	nodeIndex := make(map[string]int, len(graph.Nodes))
	for i, node := range graph.Nodes {
		if node.ID == "" {
			graph.Nodes[i].ID = nodeIdentity(node)
		}
		nodeIndex[graph.Nodes[i].ID] = i
	}

	adjacency := make([]map[int]struct{}, len(graph.Nodes))
	for _, edge := range graph.Edges {
		from, fromOK := nodeIndex[edge.From]
		to, toOK := nodeIndex[edge.To]
		if !fromOK || !toOK || from == to {
			continue
		}
		if adjacency[from] == nil {
			adjacency[from] = make(map[int]struct{})
		}
		if adjacency[to] == nil {
			adjacency[to] = make(map[int]struct{})
		}
		adjacency[from][to] = struct{}{}
		adjacency[to][from] = struct{}{}
	}

	personalization := buildPersonalization(graph.Nodes, prompt)
	ranks := make([]float64, len(graph.Nodes))
	copy(ranks, personalization)

	for i := 0; i < options.MaxIterations; i++ {
		next := make([]float64, len(graph.Nodes))
		for j, weight := range personalization {
			next[j] = (1 - options.Damping) * weight
		}

		var dangling float64
		for from, neighbors := range adjacency {
			if len(neighbors) == 0 {
				dangling += ranks[from]
				continue
			}
			share := options.Damping * ranks[from] / float64(len(neighbors))
			for to := range neighbors {
				next[to] += share
			}
		}
		if dangling > 0 {
			for j, weight := range personalization {
				next[j] += options.Damping * dangling * weight
			}
		}

		if rankDelta(ranks, next) < options.Epsilon {
			ranks = next
			break
		}
		ranks = next
	}

	fileScores := make(map[string]float64)
	for i, node := range graph.Nodes {
		path := normalizeGraphPath(node.Path)
		if path == "" {
			continue
		}
		fileScores[path] += ranks[i] + personalization[i]
	}

	files := make([]RankedFile, 0, len(fileScores))
	for path, score := range fileScores {
		files = append(files, RankedFile{Path: path, Score: score})
	}
	sort.Slice(files, func(i, j int) bool {
		if math.Abs(files[i].Score-files[j].Score) < 1e-12 {
			return files[i].Path < files[j].Path
		}
		return files[i].Score > files[j].Score
	})
	return files
}

func buildPersonalization(nodes []GraphNode, prompt string) []float64 {
	terms := promptTerms(prompt)
	weights := make([]float64, len(nodes))
	var total float64
	for i, node := range nodes {
		text := strings.ToLower(strings.Join([]string{node.ID, node.Name, node.Path, filepath.Base(node.Path)}, " "))
		for _, term := range terms {
			if strings.Contains(text, term) {
				weights[i]++
			}
		}
		total += weights[i]
	}
	if total == 0 {
		for i := range weights {
			weights[i] = 1 / float64(len(weights))
		}
		return weights
	}
	for i := range weights {
		weights[i] /= total
	}
	return weights
}

func promptTerms(prompt string) []string {
	seen := make(map[string]struct{})
	var terms []string
	for _, raw := range strings.FieldsFunc(prompt, func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' || r == '.' || r == '/')
	}) {
		term := strings.ToLower(strings.Trim(raw, "`'\""))
		if len(term) < 2 {
			continue
		}
		if _, ok := seen[term]; ok {
			continue
		}
		seen[term] = struct{}{}
		terms = append(terms, term)
	}
	return terms
}

func nodeIdentity(node GraphNode) string {
	for _, value := range []string{node.ID, node.Path, node.Name} {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return "node"
}

func rankDelta(previous, next []float64) float64 {
	var delta float64
	for i := range previous {
		delta += math.Abs(previous[i] - next[i])
	}
	return delta
}

func normalizeGraphPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	return filepath.Clean(path)
}
