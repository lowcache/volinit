package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lowcache/volinit/internal/registry"
	"github.com/lowcache/volinit/internal/runtime"
	"github.com/lowcache/volinit/internal/theme"
)

func main() {
	home, _ := os.UserHomeDir()
	roots := []string{filepath.Join(home, ".nix-config"), filepath.Join(home, "CodeRepo")}

	repos := registry.Discover(roots)
	for i := range repos {
		if err := registry.ApplySidecar(&repos[i]); err != nil {
			fmt.Fprintln(os.Stderr, "volinit:", err)
		}
	}
	p, _ := theme.Load(filepath.Join(home, ".config", "volinit", "palette.toml"))

	if len(os.Args) > 1 && os.Args[1] == "doctor" {
		for _, w := range registry.Doctor(repos) {
			fmt.Println(w)
		}
		return
	}
	if err := runtime.Run(repos, p); err != nil {
		fmt.Fprintln(os.Stderr, "volinit:", err)
		os.Exit(1)
	}
}
