package registry

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The discovery walk runs on every interactive shell. This fails the build
// if it drifts past the budget, which is what stops latency creeping in one
// feature at a time.
func TestDiscoveryStaysWithinBudget(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir")
	}
	roots := []string{filepath.Join(home, ".nix-config"), filepath.Join(home, "CodeRepo")}
	for _, r := range roots {
		if _, err := os.Stat(r); err != nil {
			t.Skip("fleet not present on this machine")
		}
	}
	const budget = 20 * time.Millisecond
	start := time.Now()
	Discover(roots)
	if elapsed := time.Since(start); elapsed > budget {
		t.Fatalf("discovery took %v, budget is %v", elapsed, budget)
	}
}
