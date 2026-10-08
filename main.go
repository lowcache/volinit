package main

import (
	"fmt"
	"os"

	"github.com/lowcache/volinit/internal/banner"
)

func main() {
	if err := banner.Play(); err != nil {
		fmt.Fprintln(os.Stderr, "volinit:", err)
	}
}
