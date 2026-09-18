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

func TestDiscoverSkipsRepoWithNoActions(t *testing.T) {
	root := t.TempDir()
	writeMakefile(t, filepath.Join(root, "bare"), "all:\n\techo hi\n")
	if repos := Discover([]string{root}); len(repos) != 0 {
		t.Fatalf("got %d repos, want 0 — a Makefile with no documented targets is not worth a menu entry", len(repos))
	}
}
