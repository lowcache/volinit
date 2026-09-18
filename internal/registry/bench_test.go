package registry

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The discovery walk runs on every interactive shell. This asserts against
// our own walk cost, not the operator's filesystem cache state.
//
// A cold run (first touch of these directories since boot, or since they
// were last evicted from the page cache) pays real directory-entry I/O that
// has nothing to do with Discover's logic — on this machine that's ~50-60ms
// against a ~1.4G, 1500+ directory ~/CodeRepo tree. That cost is real and
// worth knowing about, but it's paid once per boot, not once per shell, so
// asserting on it here would make the test flaky for a reason we don't
// control and can't fix by improving Discover. So: run once to warm the
// cache (result discarded, cold time logged for visibility), then time and
// assert a second run, which reflects steady-state interactive-shell cost.
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

	coldStart := time.Now()
	Discover(roots) // warm the page cache; result discarded
	t.Logf("cold walk: %v (not asserted; paid once per boot)", time.Since(coldStart))

	warmStart := time.Now()
	Discover(roots)
	if elapsed := time.Since(warmStart); elapsed > budget {
		t.Fatalf("discovery took %v, budget is %v", elapsed, budget)
	}
}
