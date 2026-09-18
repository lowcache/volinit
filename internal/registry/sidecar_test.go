package registry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
