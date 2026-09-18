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

// echoedHelp matches `@echo "make <target>   <description>"` inside a help
// recipe. Quotes may be single or double; spacing between the two is
// decorative alignment.
var echoedHelp = regexp.MustCompile(`@echo\s+["']make\s+([A-Za-z0-9_-]+)\s+(.+?)["']\s*$`)

// ParseHelpTarget reads the blog dialect: a `help:` target whose recipe
// echoes its own documentation. The recipe is read statically — `make help`
// is never run, which keeps discovery cheap and safe at shell-init time.
func ParseHelpTarget(src string) []Action {
	var out []Action
	inHelp := false
	for _, line := range strings.Split(src, "\n") {
		if strings.HasPrefix(line, "help:") {
			inHelp = true
			continue
		}
		if !inHelp {
			continue
		}
		// A recipe line starts with a tab. Anything else ends the recipe.
		if !strings.HasPrefix(line, "\t") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			break
		}
		if m := echoedHelp.FindStringSubmatch(line); m != nil {
			out = append(out, Action{Name: m[1], Description: strings.TrimSpace(m[2])})
		}
	}
	return out
}
