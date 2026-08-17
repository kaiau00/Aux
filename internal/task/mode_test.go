package task

import "testing"

func TestInferMode(t *testing.T) {
	cases := []struct {
		objective string
		want      Mode
	}{
		{"Fix the bug causing a panic on startup", ModeBugDiagnosis},
		{"Add a new endpoint for user profiles", ModeImplementation},
		{"Refactor the payment module to extract a service", ModeRefactor},
		{"Write unit tests for the parser", ModeTestAuthoring},
		{"Review the code in the auth package", ModeCodeReview},
		{"Explain how does the retrieval gate work", ModeResearch},
		{"Upgrade the dependency to v2 and migrate to the new API", ModeMaintenance},
		{"Just do something vague", ModeImplementation}, // default
	}
	for _, c := range cases {
		if got := InferMode(c.objective); got != c.want {
			t.Errorf("InferMode(%q) = %q, want %q", c.objective, got, c.want)
		}
	}
}
