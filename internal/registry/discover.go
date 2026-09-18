package registry

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Branch names the three top-level groupings the cockpit presents.
const (
	BranchSystem  = "system"
	BranchWriting = "writing"
	BranchCode    = "code"
)

// Repo is one discovered repository and the actions it exposes.
type Repo struct {
	Name    string
	Path    string
	Branch  string
	Actions []Action
}

// maxDepth bounds the walk. The blogs live at sites/blogs/<name>, three
// levels below ~/CodeRepo, so anything shallower misses them.
const maxDepth = 4

// Discover walks each root for Makefiles and parses whichever dialect each
// uses. Nothing is executed: both parsers are static readers.
func Discover(roots []string) []Repo {
	var out []Repo
	for _, root := range roots {
		base := strings.Count(filepath.Clean(root), string(os.PathSeparator))
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil // unreadable dirs are skipped, never fatal
			}
			if d.IsDir() {
				if strings.Count(path, string(os.PathSeparator))-base >= maxDepth {
					return fs.SkipDir
				}
				name := d.Name()
				if name == ".git" || name == "node_modules" || name == "vendor" {
					return fs.SkipDir
				}
				return nil
			}
			if d.Name() != "Makefile" {
				return nil
			}
			src, readErr := os.ReadFile(path)
			if readErr != nil {
				return nil
			}
			// Documented dialects first; a Makefile using neither still
			// contributes its plain targets rather than vanishing from the
			// cockpit. Only a Makefile with no targets at all is dropped.
			actions := ParseHashHash(string(src))
			if len(actions) == 0 {
				actions = ParseHelpTarget(string(src))
			}
			if len(actions) == 0 {
				actions = ParseBareTargets(string(src))
			}
			if len(actions) == 0 {
				return nil
			}
			dir := filepath.Dir(path)
			out = append(out, Repo{
				Name:    filepath.Base(dir),
				Path:    dir,
				Branch:  classify(dir),
				Actions: actions,
			})
			return nil
		})
	}
	return out
}

// classify buckets a repo by path. nix-config is the system; anything under
// a blogs/ or sites/ path is writing; everything else is code.
func classify(dir string) string {
	switch {
	case strings.Contains(dir, ".nix-config"):
		return BranchSystem
	case strings.Contains(dir, "/blogs/"), strings.Contains(dir, "/sites/"):
		return BranchWriting
	default:
		return BranchCode
	}
}
