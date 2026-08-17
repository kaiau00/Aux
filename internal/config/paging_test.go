package config

import "testing"

func TestSetContextPagingAcceptsOnlyValidModes(t *testing.T) {
	prev := cfg
	t.Cleanup(func() { cfg = prev })
	cfg = &Config{}

	for _, mode := range []string{"on", "off"} {
		if err := SetContextPaging(mode); err != nil {
			t.Fatalf("SetContextPaging(%q): %v", mode, err)
		}
		if cfg.Context.Paging != mode {
			t.Fatalf("paging = %q, want %q", cfg.Context.Paging, mode)
		}
	}

	// A typo must be rejected rather than silently leaving paging off while
	// the user believes they enabled it.
	if err := SetContextPaging("yes"); err == nil {
		t.Fatal("expected an error for an unrecognized paging mode")
	}
	if cfg.Context.Paging != "off" {
		t.Fatalf("a rejected value must not change the setting, got %q", cfg.Context.Paging)
	}
}

func TestSetContextPagingWithoutConfigErrors(t *testing.T) {
	prev := cfg
	t.Cleanup(func() { cfg = prev })
	cfg = nil
	if err := SetContextPaging("on"); err == nil {
		t.Fatal("expected an error when no config is loaded")
	}
}
