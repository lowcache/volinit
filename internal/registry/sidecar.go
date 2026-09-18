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

// dangerous names targets that look hazardous. Doctor is an authoring aid to help
// the user identify which targets may need a confirmation gate or other special
// handling; it prioritizes breadth (catching targets across diverse repos and
// hazard classes) over precision (avoiding false positives). Make targets are not
// a stable API and sidecar entries rot; better to flag too much and let the user
// decide than to miss a destructive target because it used unfamiliar vocabulary.
var dangerous = []string{
	"switch",   // system rebuild
	"deploy",   // push to production or remote
	"rekey",    // change encryption keys
	"force",    // skip safety checks
	"arm",      // enable something dangerous
	"disarm",   // disable safety
	"clean",    // destructive cleanup
	"rm",       // remove
	"push",     // irreversible sends or publishes
	"send",     // irreversible communication
	"boot",     // bootloader or boot-time changes
	"restart",  // service restart, can sever session
	"rebuild",  // full rebuild
	"update",   // bulk updates (can break things)
	"trash",    // destructive removal
	"split",    // split operations (e.g. history splits)
}

// Doctor reports actions that lack a confirmation gate. It is an authoring aid
// answering "which targets should I write a sidecar entry for?" — not a runtime
// gate, and not a claim that each hit is inherently dangerous. Only a human
// reading the output can decide whether a specific target needs protection.
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
						"%s: %q has no confirm in .volinit/actions.toml — review whether it needs one", r.Name, a.Name))
					break
				}
			}
		}
	}
	return warns
}
