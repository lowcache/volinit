// Package run turns a registry.Action into a command. It builds commands but
// never starts them: the caller decides whether to hand one to
// tea.ExecProcess or to start it detached.
//
// Each constructor builds what its name promises: Command constructs a
// foreground make invocation, and DetachedCommand constructs a systemd-run
// unit for detached execution. Both ignore a.Detach entirely; For is the
// dispatcher that reads it and picks between them.
package run

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	"github.com/lowcache/volinit/internal/registry"
)

// DoneMsg is delivered to the Bubble Tea model when an action finishes.
type DoneMsg struct {
	Action string
	Err    error
}

// sanitizeRepoName converts a repository path into a systemd unit-safe name
// by extracting the base name and replacing invalid characters with underscores.
// Systemd unit names restrict characters to [a-zA-Z0-9:_.\\-]; a leading dot
// is particularly problematic (.nix-config), so we replace anything else.
func sanitizeRepoName(repoPath string) string {
	base := filepath.Base(repoPath)
	// Replace any character that's not alphanumeric, hyphen, or underscore with underscore.
	return regexp.MustCompile(`[^a-zA-Z0-9_-]`).ReplaceAllString(base, "_")
}

// For picks the constructor the action asks for. Detach means the work must
// outlive the cockpit, so it goes to systemd-run; everything else runs in the
// foreground where tea.ExecProcess can hand it a real TTY.
//
// param reaches only the foreground path: carrying one into a transient unit
// needs systemd-run --setenv, and no fleet action is both detached and
// parameterised today. Revisit this, not the call sites, when one is.
func For(a registry.Action, repoPath, param string) *exec.Cmd {
	if a.Detach {
		return DetachedCommand(a, repoPath)
	}
	return Command(a, repoPath, param)
}

// Command builds a foreground `make <target>` for the repo. The caller runs
// it through tea.ExecProcess, which releases the terminal so sudo can prompt
// on a real TTY.
func Command(a registry.Action, repoPath, param string) *exec.Cmd {
	c := exec.Command("make", a.Name)
	c.Dir = repoPath
	// Inherit the full environment so foreground processes work as expected.
	c.Env = os.Environ()
	if a.ParamName != "" && param != "" {
		c.Env = append(c.Env, fmt.Sprintf("%s=%s", a.ParamName, param))
	}
	return c
}

// DetachedCommand runs the target as a transient user unit so it survives the
// cockpit exiting. `make switch` ran 4h10m on 2026-09-18; a TUI owning that
// process would reintroduce the failure `switch-detached` exists to prevent.
// The unit name includes both repo and target to avoid collisions when the
// same target name (build, deploy, clean) runs across different repositories.
func DetachedCommand(a registry.Action, repoPath string) *exec.Cmd {
	sanitized := sanitizeRepoName(repoPath)
	c := exec.Command("systemd-run",
		"--user", "--collect",
		fmt.Sprintf("--unit=volinit-%s-%s", sanitized, a.Name),
		fmt.Sprintf("--working-directory=%s", repoPath),
		"make", a.Name,
	)
	c.Dir = repoPath
	// Inherit the full environment into detached --user units deliberately:
	// this ensures that make targets see the same context (e.g., $HOME, $PATH,
	// build caches, ssh keys) as the foreground cockpit.
	c.Env = os.Environ()
	return c
}
