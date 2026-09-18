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

func TestCommandIgnoresParamWhenValueEmpty(t *testing.T) {
	a := registry.Action{Name: "build", ParamName: "CMD"}
	c := Command(a, "/tmp/repo", "")
	for _, e := range c.Env {
		if strings.HasPrefix(e, "CMD=") {
			t.Errorf("CMD should not be in env when value is empty, got: %v", c.Env)
		}
	}
}

func TestDetachedCommandUsesSystemdRun(t *testing.T) {
	c := DetachedCommand(registry.Action{Name: "switch"}, "/tmp/repo")
	got := strings.Join(c.Args, " ")
	for _, want := range []string{"systemd-run", "--user", "--unit=volinit-repo-switch", "make", "switch"} {
		if !strings.Contains(got, want) {
			t.Errorf("args %q missing %q", got, want)
		}
	}
}

func TestDetachedCommandUnitNameIncludesRepo(t *testing.T) {
	// Same target name in different repos should produce different unit names
	c1 := DetachedCommand(registry.Action{Name: "build"}, "/home/alice/nix-config")
	c2 := DetachedCommand(registry.Action{Name: "build"}, "/home/bob/myrepo")

	unit1 := ""
	unit2 := ""
	for _, arg := range c1.Args {
		if strings.HasPrefix(arg, "--unit=") {
			unit1 = arg
		}
	}
	for _, arg := range c2.Args {
		if strings.HasPrefix(arg, "--unit=") {
			unit2 = arg
		}
	}

	if unit1 == unit2 {
		t.Errorf("different repos with same target should have different unit names, both got %q", unit1)
	}
	if !strings.Contains(unit1, "nix-config") {
		t.Errorf("unit1 %q should contain repo name nix-config", unit1)
	}
	if !strings.Contains(unit2, "myrepo") {
		t.Errorf("unit2 %q should contain repo name myrepo", unit2)
	}
}

func TestDetachedCommandSanitizesLeadingDot(t *testing.T) {
	c := DetachedCommand(registry.Action{Name: "switch"}, "/home/user/.nix-config")
	got := strings.Join(c.Args, " ")
	// Leading dot should be converted to underscore
	if !strings.Contains(got, "--unit=volinit-_nix-config-switch") {
		t.Errorf("args %q should have sanitized unit name with _nix-config", got)
	}
}

func TestBothConstructorsIgnoreDetach(t *testing.T) {
	a := registry.Action{Name: "target", Detach: true}

	c := Command(a, "/tmp/repo", "")
	cArgs := strings.Join(c.Args, " ")
	// Command should produce plain `make` argv, ignoring a.Detach
	if !strings.Contains(cArgs, "make") || !strings.Contains(cArgs, "target") {
		t.Errorf("Command args %q should be plain make invocation, got %v", cArgs, c.Args)
	}
	if strings.Contains(cArgs, "systemd-run") {
		t.Errorf("Command should not use systemd-run, got %q", cArgs)
	}

	d := DetachedCommand(a, "/tmp/repo")
	dArgs := strings.Join(d.Args, " ")
	// DetachedCommand should use systemd-run, regardless of a.Detach value
	if !strings.Contains(dArgs, "systemd-run") {
		t.Errorf("DetachedCommand args %q should use systemd-run", dArgs)
	}
	if !strings.Contains(dArgs, "make") || !strings.Contains(dArgs, "target") {
		t.Errorf("DetachedCommand args %q should include make target", dArgs)
	}
}

func TestForDispatchesOnDetach(t *testing.T) {
	fg := For(registry.Action{Name: "build"}, "/tmp/repo", "")
	if got := strings.Join(fg.Args, " "); strings.Contains(got, "systemd-run") {
		t.Errorf("plain action went detached: %q", got)
	}
	bg := For(registry.Action{Name: "switch", Detach: true}, "/tmp/repo", "")
	if got := strings.Join(bg.Args, " "); !strings.Contains(got, "systemd-run") {
		t.Errorf("detached action ran in the foreground: %q", got)
	}
}

func TestForPassesParamToForegroundPath(t *testing.T) {
	c := For(registry.Action{Name: "anon-run", ParamName: "CMD"}, "/tmp/repo", "id")
	var found bool
	for _, e := range c.Env {
		if e == "CMD=id" {
			found = true
		}
	}
	if !found {
		t.Errorf("CMD not in env: %v", c.Env)
	}
}
