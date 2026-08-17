package profile

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Scanner reads specific project inputs and returns typed profile entries plus a
// fingerprint of the raw inputs it read. Scanners must be deterministic.
type Scanner interface {
	Name() string
	Scan(ctx context.Context, root string) (ScanResult, error)
}

// DefaultScanners returns the built-in scanner set for PR 5.
func DefaultScanners() []Scanner {
	return []Scanner{
		GoModScanner{},
		PackageJSONScanner{},
		MakefileScanner{},
		InstructionScanner{},
	}
}

func readFile(root, name string) (string, bool) {
	b, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		return "", false
	}
	return string(b), true
}

func fingerprint(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// GoModScanner detects Go modules and workspaces.
type GoModScanner struct{}

func (GoModScanner) Name() string { return "go" }

func (GoModScanner) Scan(_ context.Context, root string) (ScanResult, error) {
	goMod, ok := readFile(root, "go.mod")
	if !ok {
		return ScanResult{}, nil
	}
	var entries []EntryDraft
	entries = append(entries, EntryDraft{
		Type: EntryLanguage, Key: "go", Value: map[string]string{"module": goModulePath(goMod)},
		SourceType: "file", SourceRef: "go.mod", Confidence: 0.99, TokenEstimate: estimateTokens("go module"),
	})
	entries = append(entries, EntryDraft{
		Type: EntryValidationCommand, Key: "go.test",
		Value:      map[string]string{"command": "go test ./...", "scope": "repository"},
		SourceType: "go.mod", SourceRef: "go.mod", Confidence: 0.9, TokenEstimate: estimateTokens("go test ./..."),
	})
	entries = append(entries, EntryDraft{
		Type: EntryBuildCommand, Key: "go.build",
		Value:      map[string]string{"command": "go build ./...", "scope": "repository"},
		SourceType: "go.mod", SourceRef: "go.mod", Confidence: 0.9, TokenEstimate: estimateTokens("go build ./..."),
	})
	goWork, hasWork := readFile(root, "go.work")
	if hasWork {
		entries = append(entries, EntryDraft{
			Type: EntryWorkspace, Key: "go.work", Value: map[string]any{"kind": "go-workspace"},
			SourceType: "file", SourceRef: "go.work", Confidence: 0.95, TokenEstimate: estimateTokens("go workspace"),
		})
	}
	return ScanResult{Entries: entries, Fingerprint: fingerprint("go", goMod, goWork)}, nil
}

func goModulePath(goMod string) string {
	for _, line := range strings.Split(goMod, "\n") {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, "module "); ok {
			return strings.TrimSpace(after)
		}
	}
	return ""
}

// PackageJSONScanner detects Node/JS/TS projects and their scripts.
type PackageJSONScanner struct{}

func (PackageJSONScanner) Name() string { return "node" }

func (PackageJSONScanner) Scan(_ context.Context, root string) (ScanResult, error) {
	raw, ok := readFile(root, "package.json")
	if !ok {
		return ScanResult{}, nil
	}
	var pkg struct {
		Name    string            `json:"name"`
		Scripts map[string]string `json:"scripts"`
	}
	_ = json.Unmarshal([]byte(raw), &pkg)

	lang := "javascript"
	_, hasTS := readFile(root, "tsconfig.json")
	if hasTS {
		lang = "typescript"
	}
	entries := []EntryDraft{{
		Type: EntryLanguage, Key: lang, Value: map[string]string{"package": pkg.Name},
		SourceType: "file", SourceRef: "package.json", Confidence: 0.95, TokenEstimate: estimateTokens("node package"),
	}}

	// Map common scripts to validation commands.
	scriptToIntent := map[string]string{"test": "node.test", "lint": "node.lint", "typecheck": "node.typecheck", "build": "node.build"}
	keys := make([]string, 0, len(pkg.Scripts))
	for k := range pkg.Scripts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, script := range keys {
		key, mapped := scriptToIntent[script]
		if !mapped {
			continue
		}
		entryType := EntryValidationCommand
		if script == "build" {
			entryType = EntryBuildCommand
		}
		cmd := "npm run " + script
		entries = append(entries, EntryDraft{
			Type: entryType, Key: key,
			Value:      map[string]string{"command": cmd, "scope": "package"},
			SourceType: "package.json:scripts", SourceRef: script, Confidence: 0.85, TokenEstimate: estimateTokens(cmd),
		})
	}
	return ScanResult{Entries: entries, Fingerprint: fingerprint("node", raw, boolStr(hasTS))}, nil
}

func boolStr(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// MakefileScanner surfaces common Makefile targets as commands.
type MakefileScanner struct{}

func (MakefileScanner) Name() string { return "make" }

func (MakefileScanner) Scan(_ context.Context, root string) (ScanResult, error) {
	raw, ok := readFile(root, "Makefile")
	if !ok {
		return ScanResult{}, nil
	}
	targets := makeTargets(raw)
	var entries []EntryDraft
	interesting := map[string]string{"test": "make.test", "build": "make.build", "lint": "make.lint", "check": "make.check"}
	names := make([]string, 0, len(targets))
	for t := range targets {
		names = append(names, t)
	}
	sort.Strings(names)
	for _, t := range names {
		key, ok := interesting[t]
		if !ok {
			continue
		}
		entryType := EntryValidationCommand
		if t == "build" {
			entryType = EntryBuildCommand
		}
		cmd := "make " + t
		entries = append(entries, EntryDraft{
			Type: entryType, Key: key, Value: map[string]string{"command": cmd, "scope": "repository"},
			SourceType: "Makefile", SourceRef: t, Confidence: 0.8, TokenEstimate: estimateTokens(cmd),
		})
	}
	return ScanResult{Entries: entries, Fingerprint: fingerprint("make", raw)}, nil
}

func makeTargets(raw string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, line := range strings.Split(raw, "\n") {
		if strings.HasPrefix(line, "\t") || strings.HasPrefix(line, " ") {
			continue
		}
		if idx := strings.Index(line, ":"); idx > 0 {
			name := strings.TrimSpace(line[:idx])
			if name != "" && !strings.ContainsAny(name, " =") {
				out[name] = struct{}{}
			}
		}
	}
	return out
}

// InstructionScanner imports compatible agent-instruction files as bounded excerpts.
type InstructionScanner struct{}

func (InstructionScanner) Name() string { return "instructions" }

const maxInstructionChars = 4000

func (InstructionScanner) Scan(_ context.Context, root string) (ScanResult, error) {
	files := []string{"AGENTS.md", "CLAUDE.md", ".cursorrules", "AUX.md"}
	var entries []EntryDraft
	var fpParts []string
	for _, name := range files {
		content, ok := readFile(root, name)
		if !ok {
			continue
		}
		fpParts = append(fpParts, name, content)
		excerpt := content
		if len(excerpt) > maxInstructionChars {
			excerpt = excerpt[:maxInstructionChars]
		}
		entries = append(entries, EntryDraft{
			Type: EntryInstruction, Key: name, Value: map[string]string{"excerpt": excerpt},
			SourceType: "file", SourceRef: name, Confidence: 1.0, TokenEstimate: estimateTokens(excerpt),
		})
	}
	if len(entries) == 0 {
		return ScanResult{}, nil
	}
	return ScanResult{Entries: entries, Fingerprint: fingerprint(append([]string{"instructions"}, fpParts...)...)}, nil
}
