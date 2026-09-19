package registry

import "sort"

// Door is one top-level branch of the cockpit's menu.
type Door struct {
	Name   string // System, Writing, Code
	Noun   string // what its subsystems are called, plural
	Groups []Group
}

// Group is one subsystem: a Makefile section of a system repo, or a whole
// repo behind the other doors.
type Group struct {
	Name    string
	Repo    string
	Path    string
	Actions []Action
}

var doors = []struct{ branch, name, noun string }{
	{BranchSystem, "System", "subsystems"},
	{BranchWriting, "Writing", "sites"},
	{BranchCode, "Code", "projects"},
}

// Menu arranges repos into doors → subsystems → tasks. Doors come in fixed
// order and are left out when empty.
func Menu(repos []Repo) []Door {
	var out []Door
	for _, d := range doors {
		var in []Repo
		for _, r := range repos {
			if branchOf(r) == d.branch {
				in = append(in, r)
			}
		}
		sort.SliceStable(in, func(i, j int) bool { return in[i].Name < in[j].Name })
		door := Door{Name: d.name, Noun: d.noun}
		for _, r := range in {
			if d.branch == BranchSystem {
				door.Groups = append(door.Groups, sections(r)...)
			} else if acts := runnable(r.Actions); len(acts) > 0 {
				door.Groups = append(door.Groups, Group{Name: r.Name, Repo: r.Name, Path: r.Path, Actions: acts})
			}
		}
		if len(door.Groups) > 0 {
			out = append(out, door)
		}
	}
	return out
}

// branchOf files anything not classified system or writing under code,
// which is how the spec defines the code door.
func branchOf(r Repo) string {
	if r.Branch == BranchSystem || r.Branch == BranchWriting {
		return r.Branch
	}
	return BranchCode
}

// sections splits a system repo by Makefile section in order of first
// appearance; unsectioned targets group under the repo's own name.
func sections(r Repo) []Group {
	var out []Group
	at := map[string]int{}
	for _, a := range runnable(r.Actions) {
		name := a.Section
		if name == "" {
			name = r.Name
		}
		i, ok := at[name]
		if !ok {
			i = len(out)
			at[name] = i
			out = append(out, Group{Name: name, Repo: r.Name, Path: r.Path})
		}
		out[i].Actions = append(out[i].Actions, a)
	}
	return out
}

// runnable drops `help`: it prints the Makefile's own target list, which
// the menu replaces.
func runnable(as []Action) []Action {
	var out []Action
	for _, a := range as {
		if a.Name != "help" {
			out = append(out, a)
		}
	}
	return out
}
