package run

import (
	"strings"
	"testing"

	"github.com/lowcache/volinit/internal/registry"
)

func TestCommandRunsMakeInRepo(t *testing.T) {
	c := Command(registry.Action{Name: "build"}, "/tmp/repo", "")
	if c.Dir != "/tmp/repo" {
		t.Errorf("Dir = %q", c.Dir)
	}
	got := strings.Join(c.Args, " ")
	if !strings.Contains(got, "make") || !strings.Contains(got, "build") {
		t.Errorf("args = %q", got)
	}
}

func TestCommandPassesParamAsEnv(t *testing.T) {
	a := registry.Action{Name: "anon-run", ParamName: "CMD"}
	c := Command(a, "/tmp/repo", "curl example.com")
	var found bool
	for _, e := range c.Env {
		if e == "CMD=curl example.com" {
			found = true
		}
	}
	if !found {
		t.Errorf("CMD not in env: %v", c.Env)
	}
}

func TestDetachedCommandUsesSystemdRun(t *testing.T) {
	c := DetachedCommand(registry.Action{Name: "switch"}, "/tmp/repo")
	got := strings.Join(c.Args, " ")
	for _, want := range []string{"systemd-run", "--user", "--unit=volinit-switch", "make", "switch"} {
		if !strings.Contains(got, want) {
			t.Errorf("args %q missing %q", got, want)
		}
	}
}
