package relatedproject

import "strings"

// ParseGoModDeps extracts the module path and required module paths from a raw
// go.mod file's contents. It handles both the single-line form
// (`require example.com/x v1.2.3`) and the block form
// (`require (\n\texample.com/x v1.2.3\n)`). Indirect requirements are included —
// callers decide whether to treat them differently. Malformed or missing input
// yields an empty module path and nil deps rather than an error, since this
// feeds best-effort background derivation (see app.deriveRelatedProjects).
func ParseGoModDeps(goMod string) (modulePath string, deps []string) {
	inRequireBlock := false
	for _, raw := range strings.Split(goMod, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		if after, ok := strings.CutPrefix(line, "module "); ok {
			modulePath = strings.TrimSpace(after)
			continue
		}
		if inRequireBlock {
			if line == ")" {
				inRequireBlock = false
				continue
			}
			if dep := requirePath(line); dep != "" {
				deps = append(deps, dep)
			}
			continue
		}
		if line == "require (" {
			inRequireBlock = true
			continue
		}
		if after, ok := strings.CutPrefix(line, "require "); ok {
			if dep := requirePath(after); dep != "" {
				deps = append(deps, dep)
			}
		}
	}
	return modulePath, deps
}

// requirePath extracts the module path from one require-line body, e.g.
// "example.com/x v1.2.3" or "example.com/x v1.2.3 // indirect".
func requirePath(body string) string {
	body = strings.TrimSpace(body)
	if i := strings.Index(body, "//"); i >= 0 {
		body = strings.TrimSpace(body[:i])
	}
	fields := strings.Fields(body)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}
