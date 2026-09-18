package registry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func boolp(b bool) *bool { return &b }

func TestApplySidecarAnnotatesMatchingActions(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".volinit"), 0o755); err != nil {
		t.Fatal(err)
	}
	toml := `
[switch]
detach = true

[deploy]
confirm = "Rescue-path deploy. Continue?"

[anon-run]
param = { name = "CMD", prompt = "Command to jail" }
`
	if err := os.WriteFile(filepath.Join(dir, ".volinit", "actions.toml"), []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}

	r := Repo{Path: dir, Actions: []Action{
		{Name: "switch"}, {Name: "deploy"}, {Name: "anon-run"}, {Name: "build"},
	}}
	if err := ApplySidecar(&r); err != nil {
		t.Fatalf("ApplySidecar: %v", err)
	}
	if !r.Actions[0].Detach {
		t.Error("switch should be detached")
	}
	if r.Actions[1].Confirm == "" {
		t.Error("deploy should carry a confirm")
	}
	if r.Actions[2].ParamName != "CMD" {
		t.Errorf("ParamName = %q, want CMD", r.Actions[2].ParamName)
	}
	if r.Actions[2].ParamPrompt != "Command to jail" {
		t.Errorf("ParamPrompt = %q, want 'Command to jail'", r.Actions[2].ParamPrompt)
	}
	if r.Actions[3].Detach || r.Actions[3].Confirm != "" {
		t.Error("build has no entry and must stay plain")
	}
}

func TestApplySidecarMissingFileIsNotAnError(t *testing.T) {
	r := Repo{Path: t.TempDir(), Actions: []Action{{Name: "build"}}}
	if err := ApplySidecar(&r); err != nil {
		t.Fatalf("missing sidecar must not error, got %v", err)
	}
}

func TestGatedTableDriven(t *testing.T) {
	tests := []struct {
		name   string
		action Action
		want   bool
	}{
		{"build/nil/\"\"", Action{Name: "build"}, false},
		{"switch/nil/\"\"", Action{Name: "switch"}, true},
		{"switch/Gate=false", Action{Name: "switch", Gate: boolp(false)}, false},
		{"build/Gate=true", Action{Name: "build", Gate: boolp(true)}, true},
		{"build/nil/Confirm=\"sure?\"", Action{Name: "build", Confirm: "sure?"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.action.Gated(); got != tt.want {
				t.Errorf("Gated() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestApplySidecarDecodesGate(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".volinit"), 0o755); err != nil {
		t.Fatal(err)
	}
	toml := `
[build]
gate = false

[deploy]
gate = true
`
	if err := os.WriteFile(filepath.Join(dir, ".volinit", "actions.toml"), []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}

	r := Repo{Path: dir, Actions: []Action{
		{Name: "build"}, {Name: "deploy"}, {Name: "switch"},
	}}
	if err := ApplySidecar(&r); err != nil {
		t.Fatalf("ApplySidecar: %v", err)
	}

	// gate = false must decode to a NON-NIL pointer to false
	if r.Actions[0].Gate == nil {
		t.Fatal("build Gate should not be nil after sidecar")
	}
	if *r.Actions[0].Gate != false {
		t.Errorf("build Gate = %v, want false", *r.Actions[0].Gate)
	}

	// gate = true must decode to a NON-NIL pointer to true
	if r.Actions[1].Gate == nil {
		t.Fatal("deploy Gate should not be nil after sidecar")
	}
	if *r.Actions[1].Gate != true {
		t.Errorf("deploy Gate = %v, want true", *r.Actions[1].Gate)
	}

	// No sidecar entry => Gate stays nil
	if r.Actions[2].Gate != nil {
		t.Errorf("switch Gate = %v, want nil (no sidecar entry)", r.Actions[2].Gate)
	}
}

func TestDoctorInvariantsSynthetic(t *testing.T) {
	// Five cases across two repos:
	//   build/nil/""  → not gated (safe name)
	//   switch/nil/"" → GatedByHeuristic
	//   switch/Gate=false → not gated (exempted)
	//   build/Gate=true  → not GatedByHeuristic (explicit gate)
	//   build/nil/Confirm="sure?" → not GatedByHeuristic (confirm implies gate)
	repos := []Repo{
		{Name: "repoA", Actions: []Action{
			{Name: "build"},
			{Name: "switch"},
			{Name: "switch", Gate: boolp(false)},
		}},
		{Name: "repoB", Actions: []Action{
			{Name: "build", Gate: boolp(true)},
			{Name: "build", Confirm: "sure?"},
		}},
	}

	warns := Doctor(repos)

	// Count actions where GatedByHeuristic is true
	heuristicCount := 0
	for _, r := range repos {
		for _, a := range r.Actions {
			if a.GatedByHeuristic() {
				heuristicCount++
			}
		}
	}

	if len(warns) != heuristicCount {
		t.Fatalf("Doctor returned %d warnings, want %d (GatedByHeuristic count)", len(warns), heuristicCount)
	}

	// Every Doctor line must name an action whose Gated() is true
	for _, w := range warns {
		if !strings.Contains(w, "gated by name heuristic only") {
			t.Errorf("unexpected Doctor format: %q", w)
		}
	}

	// Exactly one hit: switch in repoA (nil Gate, no Confirm, "switch" matches dangerous)
	if len(warns) != 1 {
		t.Fatalf("expected exactly 1 warning, got %d: %v", len(warns), warns)
	}
	if !strings.Contains(warns[0], "switch") {
		t.Errorf("warning should name switch, got %q", warns[0])
	}
	if !strings.Contains(warns[0], "repoA") {
		t.Errorf("warning should name repoA, got %q", warns[0])
	}
}

func TestDoctorInvariantsRealFleet(t *testing.T) {
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

	repos := Discover(roots)
	for i := range repos {
		if err := ApplySidecar(&repos[i]); err != nil {
			t.Fatalf("ApplySidecar: %v", err)
		}
	}

	warns := Doctor(repos)

	// Count actions where GatedByHeuristic is true
	heuristicCount := 0
	for _, r := range repos {
		for _, a := range r.Actions {
			if a.GatedByHeuristic() {
				heuristicCount++
			}
		}
	}

	if len(warns) != heuristicCount {
		t.Fatalf("Doctor returned %d warnings, want %d (GatedByHeuristic count)", len(warns), heuristicCount)
	}

	// Every Doctor line must name an action whose Gated() is true
	for _, w := range warns {
		// Extract the action name from the Doctor line: `repo: "action" is gated...`
		start := strings.Index(w, "\"")
		end := strings.Index(w[start+1:], "\"")
		if start < 0 || end < 0 {
			t.Errorf("cannot parse action name from Doctor line: %q", w)
			continue
		}
		actionName := w[start+1 : start+1+end]

		// Find the action and check Gated
		found := false
		for _, r := range repos {
			for _, a := range r.Actions {
				if a.Name == actionName && a.Gated() {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("Doctor flagged %q but Gated() is false", actionName)
		}
	}
}

func TestDoctorFlagsDangerousActionsWithoutConfirm(t *testing.T) {
	repos := []Repo{{Name: "cfg", Actions: []Action{
		{Name: "switch"},
		{Name: "deploy", Confirm: "ok?"},
		{Name: "build"},
	}}}
	warns := Doctor(repos)
	if len(warns) != 1 {
		t.Fatalf("got %d warnings, want 1: %v", len(warns), warns)
	}
	if !strings.Contains(warns[0], "switch") {
		t.Errorf("warning should name switch, got %q", warns[0])
	}
}
