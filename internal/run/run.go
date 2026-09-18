// Package run turns a registry.Action into a command. It builds commands but
// never starts them: the caller decides whether to hand one to
// tea.ExecProcess or to start it detached.
package run

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/lowcache/volinit/internal/registry"
)

// DoneMsg is delivered to the Bubble Tea model when an action finishes.
type DoneMsg struct {
	Action string
	Err    error
}

// Command builds a foreground `make <target>` for the repo. The caller runs
// it through tea.ExecProcess, which releases the terminal so sudo can prompt
// on a real TTY.
func Command(a registry.Action, repoPath, param string) *exec.Cmd {
	c := exec.Command("make", a.Name)
	c.Dir = repoPath
	c.Env = os.Environ()
	if a.ParamName != "" && param != "" {
		c.Env = append(c.Env, fmt.Sprintf("%s=%s", a.ParamName, param))
	}
	return c
}

// DetachedCommand runs the target as a transient user unit so it survives the
// cockpit exiting. `make switch` ran 4h10m on 2026-09-18; a TUI owning that
// process would reintroduce the failure `switch-detached` exists to prevent.
func DetachedCommand(a registry.Action, repoPath string) *exec.Cmd {
	c := exec.Command("systemd-run",
		"--user", "--collect",
		fmt.Sprintf("--unit=volinit-%s", a.Name),
		fmt.Sprintf("--working-directory=%s", repoPath),
		"make", a.Name,
	)
	c.Dir = repoPath
	c.Env = os.Environ()
	return c
}
