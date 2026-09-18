package registry

import (
	"os"
	"path/filepath"
	"testing"
)

func writeMakefile(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Makefile"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoverFindsBothDialects(t *testing.T) {
	root := t.TempDir()
	writeMakefile(t, filepath.Join(root, "sys"),
		"## System Operations\n## :switch: ....: Rebuild\nswitch:\n\techo hi\n")
	writeMakefile(t, filepath.Join(root, "sites", "blogs", "wiki"),
		"help:\n\t@echo \"make serve   Live preview\"\n")

	repos := Discover([]string{root})
	if len(repos) != 2 {
		t.Fatalf("got %d repos, want 2", len(repos))
	}
	byName := map[string]Repo{}
	for _, r := range repos {
		byName[r.Name] = r
	}
	if got := byName["sys"].Actions; len(got) != 1 || got[0].Name != "switch" {
		t.Errorf("sys actions = %+v", got)
	}
	if got := byName["wiki"].Actions; len(got) != 1 || got[0].Name != "serve" {
		t.Errorf("wiki actions = %+v", got)
	}
}

func TestDiscoverClassifiesBranches(t *testing.T) {
	root := t.TempDir()
	writeMakefile(t, filepath.Join(root, "sites", "blogs", "wiki"),
		"help:\n\t@echo \"make serve   x\"\n")
	writeMakefile(t, filepath.Join(root, "tether"),
		"help:\n\t@echo \"make build   x\"\n")

	byName := map[string]Repo{}
	for _, r := range Discover([]string{root}) {
		byName[r.Name] = r
	}
	if byName["wiki"].Branch != BranchWriting {
		t.Errorf("wiki branch = %q, want %q", byName["wiki"].Branch, BranchWriting)
	}
	if byName["tether"].Branch != BranchCode {
		t.Errorf("tether branch = %q, want %q", byName["tether"].Branch, BranchCode)
	}
}

// The spec requires bare target enumeration for Makefiles using neither
// documented dialect; four fleet repos have no annotations at all and would
// otherwise be invisible.
func TestDiscoverFallsBackToBareTargets(t *testing.T) {
	root := t.TempDir()
	writeMakefile(t, filepath.Join(root, "bare"), "VER := 1\n.PHONY: all\nall: test\n\techo hi\ntest:\n\techo t\n")
	repos := Discover([]string{root})
	if len(repos) != 1 {
		t.Fatalf("got %d repos, want 1 — an undocumented Makefile still has runnable targets", len(repos))
	}
	var names []string
	for _, a := range repos[0].Actions {
		names = append(names, a.Name)
	}
	if len(names) != 2 || names[0] != "all" || names[1] != "test" {
		t.Errorf("bare actions = %v, want [all test] — VER := and .PHONY are not targets", names)
	}
}

func TestDiscoverSkipsMakefileWithNoTargets(t *testing.T) {
	root := t.TempDir()
	writeMakefile(t, filepath.Join(root, "empty"), "VER := 1\nexport VER\n")
	if repos := Discover([]string{root}); len(repos) != 0 {
		t.Fatalf("got %d repos, want 0 — a Makefile with no targets has nothing to run", len(repos))
	}
}
