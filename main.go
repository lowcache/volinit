package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lowcache/volinit/internal/registry"
	"github.com/lowcache/volinit/internal/runtime"
	"github.com/lowcache/volinit/internal/theme"
)

// configDir honours XDG_CONFIG_HOME, falling back to ~/.config. The noctalia
// template writes the palette under $XDG_CONFIG_HOME; reading ~/.config
// unconditionally agreed with it only by coincidence, and disagreed silently.
func configDir(home string) string {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return d
	}
	return filepath.Join(home, ".config")
}

func main() {
	// Everything below degrades rather than fails — volinit runs on every
	// interactive shell and must not block one. But it says what it lost:
	// a blank screen with no explanation is the failure mode to avoid.
	var notices []string
	note := func(format string, a ...any) {
		notices = append(notices, "volinit: "+fmt.Sprintf(format, a...))
	}

	home, err := os.UserHomeDir()
	if err != nil {
		note("no home directory (%v); discovery has nothing to walk", err)
	}
	roots := []string{filepath.Join(home, ".nix-config"), filepath.Join(home, "CodeRepo")}

	repos := registry.Discover(roots)
	for i := range repos {
		if err := registry.ApplySidecar(&repos[i]); err != nil {
			fmt.Fprintln(os.Stderr, "volinit:", err)
			note("%v", err)
		}
	}
	if len(repos) == 0 {
		note("nothing found under %s", strings.Join(roots, ", "))
	}

	if len(os.Args) > 1 && os.Args[1] == "doctor" {
		for _, n := range notices {
			fmt.Fprintln(os.Stderr, n)
		}
		for _, w := range registry.Doctor(repos) {
			fmt.Println(w)
		}
		return
	}

	// Loaded after the doctor branch, which never uses it.
	palettePath := filepath.Join(configDir(home), "volinit", "palette.toml")
	p, err := theme.Load(palettePath)
	if err != nil {
		note("%s: %v; using the built-in palette", palettePath, err)
	}

	if err := runtime.Run(repos, p, notices); err != nil {
		fmt.Fprintln(os.Stderr, "volinit:", err)
		os.Exit(1)
	}
}
