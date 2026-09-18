package registry

import (
	"regexp"
	"strings"
)

// Action is one runnable entry discovered in a repository.
type Action struct {
	Name        string
	Description string
	Section     string
}

// targetLine matches `## :name: ....: description` — the dots are decorative
// and variable in length, so they are consumed rather than counted.
var targetLine = regexp.MustCompile(`^##\s*:([A-Za-z0-9_-]+):\s*\.*\s*:\s*(.+?)\s*$`)

// sectionLine matches `## Section Name` but not a target line, which is why
// this is applied only after targetLine fails.
var sectionLine = regexp.MustCompile(`^##\s+([A-Z][^:]*?)\s*$`)

// ParseHashHash reads the nix-config dialect: `## Section` headers grouping
// `## :target: ....: description` entries. Nothing is executed.
func ParseHashHash(src string) []Action {
	var out []Action
	section := ""
	for _, line := range strings.Split(src, "\n") {
		if m := targetLine.FindStringSubmatch(line); m != nil {
			out = append(out, Action{Name: m[1], Description: m[2], Section: section})
			continue
		}
		if m := sectionLine.FindStringSubmatch(line); m != nil {
			section = m[1]
		}
	}
	return out
}
