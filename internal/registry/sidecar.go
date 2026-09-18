package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// sidecarEntry mirrors one table in .volinit/actions.toml.
type sidecarEntry struct {
	Confirm string `toml:"confirm"`
	Sudo    bool   `toml:"sudo"`
	Detach  bool   `toml:"detach"`
	Stream  bool   `toml:"stream"`
	When    string `toml:"when"`
	Param   struct {
		Name   string `toml:"name"`
		Prompt string `toml:"prompt"`
	} `toml:"param"`
}

// ApplySidecar overlays <repo>/.volinit/actions.toml onto already-discovered
// actions. Absent or unreadable, everything still runs plainly — the sidecar
// is never required.
func ApplySidecar(r *Repo) error {
	path := filepath.Join(r.Path, ".volinit", "actions.toml")
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var entries map[string]sidecarEntry
	if _, err := toml.Decode(string(raw), &entries); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	for i := range r.Actions {
		e, ok := entries[r.Actions[i].Name]
		if !ok {
			continue
		}
		r.Actions[i].Confirm = e.Confirm
		r.Actions[i].Sudo = e.Sudo
		r.Actions[i].Detach = e.Detach
		r.Actions[i].Stream = e.Stream
		r.Actions[i].When = e.When
		r.Actions[i].ParamName = e.Param.Name
		r.Actions[i].ParamPrompt = e.Param.Prompt
	}
	return nil
}

// dangerous names targets that change system or published state. Make
// targets are not a stable API and the sidecar rots; this is the cheap
// check that catches a destructive target sitting one keypress deep.
var dangerous = []string{"switch", "deploy", "rekey", "force", "arm", "disarm", "clean", "rm"}

// Doctor reports actions that look destructive but carry no confirmation.
func Doctor(repos []Repo) []string {
	var warns []string
	for _, r := range repos {
		for _, a := range r.Actions {
			if a.Confirm != "" {
				continue
			}
			for _, d := range dangerous {
				if strings.Contains(a.Name, d) {
					warns = append(warns, fmt.Sprintf(
						"%s: %q looks destructive but has no confirm in .volinit/actions.toml", r.Name, a.Name))
					break
				}
			}
		}
	}
	return warns
}
