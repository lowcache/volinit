package registry

import (
	"os"
	"path/filepath"
	"testing"
)

func doorNames(ds []Door) []string {
	var out []string
	for _, d := range ds {
		out = append(out, d.Name)
	}
	return out
}

func TestMenuOrdersDoorsAndSkipsEmptyOnes(t *testing.T) {
	got := Menu([]Repo{
		{Name: "app", Branch: BranchCode, Actions: []Action{{Name: "build"}}},
		{Name: "cfg", Branch: BranchSystem, Actions: []Action{{Name: "switch"}}},
	})
	if len(got) != 2 || got[0].Name != "System" || got[1].Name != "Code" {
		t.Fatalf("doors = %v, want [System Code]", doorNames(got))
	}
	if got[0].Noun != "subsystems" || got[1].Noun != "projects" {
		t.Errorf("nouns = %q, %q", got[0].Noun, got[1].Noun)
	}
}

func TestSystemRepoSplitsBySection(t *testing.T) {
	gs := Menu([]Repo{{Name: "cfg", Path: "/c", Branch: BranchSystem, Actions: []Action{
		{Name: "switch", Section: "System Operations"},
		{Name: "sops-edit", Section: "Secret Management"},
		{Name: "build", Section: "System Operations"},
		{Name: "loose"},
	}}})[0].Groups
	want := []struct {
		name string
		n    int
	}{{"System Operations", 2}, {"Secret Management", 1}, {"cfg", 1}}
	if len(gs) != len(want) {
		t.Fatalf("got %d groups, want %d", len(gs), len(want))
	}
	for i, w := range want {
		if gs[i].Name != w.name || len(gs[i].Actions) != w.n {
			t.Errorf("group %d = %q with %d actions, want %q with %d", i, gs[i].Name, len(gs[i].Actions), w.name, w.n)
		}
		if gs[i].Repo != "cfg" || gs[i].Path != "/c" {
			t.Errorf("group %q lost its repo: %q %q", gs[i].Name, gs[i].Repo, gs[i].Path)
		}
	}
}

func TestOtherDoorsGroupByRepoInNameOrder(t *testing.T) {
	gs := Menu([]Repo{
		{Name: "wiki", Branch: BranchWriting, Actions: []Action{{Name: "deploy"}}},
		{Name: "blog", Branch: BranchWriting, Actions: []Action{{Name: "serve", Section: "Dev"}, {Name: "deploy"}}},
	})[0].Groups
	if len(gs) != 2 || gs[0].Name != "blog" || gs[1].Name != "wiki" {
		t.Fatalf("writing groups wrong: %+v", gs)
	}
	if len(gs[0].Actions) != 2 {
		t.Error("outside the system door a repo is one subsystem, sections or not")
	}
}

func TestHelpIsDropped(t *testing.T) {
	got := Menu([]Repo{
		{Name: "site", Branch: BranchWriting, Actions: []Action{{Name: "help"}, {Name: "build"}}},
		{Name: "bare", Branch: BranchCode, Actions: []Action{{Name: "help"}}},
	})
	if len(got) != 1 {
		t.Fatalf("a door holding only help targets must vanish, got %v", doorNames(got))
	}
	if as := got[0].Groups[0].Actions; len(as) != 1 || as[0].Name != "build" {
		t.Errorf("help survived: %+v", as)
	}
}

func TestUnclassifiedRepoIsCode(t *testing.T) {
	got := Menu([]Repo{{Name: "x", Actions: []Action{{Name: "build"}}}})
	if len(got) != 1 || got[0].Name != "Code" {
		t.Fatalf("doors = %v, want [Code]", doorNames(got))
	}
}

// The menu may regroup the fleet but must never lose a runnable target.
func TestMenuKeepsEveryRealTargetButHelp(t *testing.T) {
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
	want := 0
	for _, r := range repos {
		for _, a := range r.Actions {
			if a.Name != "help" {
				want++
			}
		}
	}
	got := 0
	for _, d := range Menu(repos) {
		for _, g := range d.Groups {
			got += len(g.Actions)
		}
	}
	if got != want {
		t.Fatalf("menu holds %d targets, fleet has %d runnable", got, want)
	}
}
