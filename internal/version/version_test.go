package version

import (
	"strings"
	"testing"
)

func TestInfo(t *testing.T) {
	info := Info()

	// Should contain version info
	if !strings.Contains(info, "WireRift") {
		t.Error("Info should contain 'WireRift'")
	}
	if !strings.Contains(info, "commit:") {
		t.Error("Info should contain 'commit:'")
	}
	if !strings.Contains(info, "built:") {
		t.Error("Info should contain 'built:'")
	}
	if !strings.Contains(info, "go:") {
		t.Error("Info should contain 'go:'")
	}
}

func TestShort(t *testing.T) {
	short := Short()

	// Default is "dev"
	if short != "dev" {
		t.Errorf("Short() = %q, want %q", short, "dev")
	}
}

func TestVersionDefaults(t *testing.T) {
	// These are the default values before ldflags are applied
	if Version != "dev" {
		t.Errorf("Version = %q, want %q", Version, "dev")
	}
	if Commit != "none" {
		t.Errorf("Commit = %q, want %q", Commit, "none")
	}
	if BuildDate != "unknown" {
		t.Errorf("BuildDate = %q, want %q", BuildDate, "unknown")
	}
}
