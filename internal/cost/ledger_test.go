package cost

import (
	"testing"

	"github.com/aux-ai/aux-cli/internal/llm/models"
	"github.com/aux-ai/aux-cli/internal/llm/provider"
)

func TestComputeCost(t *testing.T) {
	priced := models.Model{
		Provider:           models.ProviderAnthropic,
		CostPer1MIn:        3.0,
		CostPer1MOut:       15.0,
		CostPer1MInCached:  3.75,
		CostPer1MOutCached: 0.30,
	}
	tests := []struct {
		name      string
		model     models.Model
		usage     provider.TokenUsage
		wantCost  float64
		wantState CostState
	}{
		{
			name:  "no cache metrics",
			model: priced,
			usage: provider.TokenUsage{InputTokens: 1_000_000, OutputTokens: 1_000_000},
			// 3.0 + 15.0
			wantCost:  18.0,
			wantState: CostKnown,
		},
		{
			name:  "with cache metrics kept separate",
			model: priced,
			usage: provider.TokenUsage{
				InputTokens:         1_000_000,
				OutputTokens:        1_000_000,
				CacheCreationTokens: 1_000_000,
				CacheReadTokens:     1_000_000,
			},
			// 3.75 (cache create) + 0.30 (cache read) + 3.0 (in) + 15.0 (out)
			wantCost:  22.05,
			wantState: CostKnown,
		},
		{
			name:      "local model zero rates is known-free",
			model:     models.Model{Provider: models.ProviderLocal},
			usage:     provider.TokenUsage{InputTokens: 5_000, OutputTokens: 5_000},
			wantCost:  0,
			wantState: CostKnown,
		},
		{
			name:      "unknown pricing for non-local zero-rate model",
			model:     models.Model{Provider: models.ProviderOpenAI},
			usage:     provider.TokenUsage{InputTokens: 5_000, OutputTokens: 5_000},
			wantCost:  0,
			wantState: CostUnknown,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCost, gotState := ComputeCost(tt.model, tt.usage)
			if !almostEqual(gotCost, tt.wantCost) {
				t.Errorf("cost = %v, want %v", gotCost, tt.wantCost)
			}
			if gotState != tt.wantState {
				t.Errorf("state = %v, want %v", gotState, tt.wantState)
			}
		})
	}
}

func almostEqual(a, b float64) bool {
	const eps = 1e-9
	d := a - b
	return d < eps && d > -eps
}
