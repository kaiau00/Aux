package prompt

import (
	"strings"
	"testing"

	"github.com/aux-ai/aux-cli/internal/config"
)

// When the Ponytail config is disabled the protocol must NOT appear in any
// agent prompt, even for agents that normally receive it (Coder, Task).
func TestPonytailProtocolRespectsConfigFlag(t *testing.T) {
	tmpDir := t.TempDir()
	cfg, err := config.Load(tmpDir, false)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Default is disabled.
	if cfg.Ponytail.Enabled {
		t.Fatalf("expected Ponytail.Enabled default to be false")
	}

	for _, agent := range []config.AgentName{config.AgentCoder, config.AgentTask} {
		t.Run(string(agent)+"/disabled", func(t *testing.T) {
			if got := GetAgentPrompt(agent, "" /* unused */); strings.Contains(got, "PONYTAIL PROTOCOL") {
				t.Fatalf("ponytail leaked into %s prompt with flag disabled", agent)
			}
		})
	}

	// Opt-in: enable and verify the protocol appears.
	cfg.Ponytail.Enabled = true
	for _, agent := range []config.AgentName{config.AgentCoder, config.AgentTask} {
		t.Run(string(agent)+"/enabled", func(t *testing.T) {
			if got := GetAgentPrompt(agent, ""); !strings.Contains(got, "PONYTAIL PROTOCOL") {
				t.Fatalf("expected ponytail in %s prompt with flag enabled", agent)
			}
		})
	}
}