// Package banner plays the ttfx-rs vhstape animation before the cockpit.
package banner

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"

	"github.com/charmbracelet/x/term"
)

var letter = map[rune][]string{
	'V': {"█   █", "█   █", "█   █", "█   █", " █ █ ", " █ █ ", "  █  "},
	'o': {"     ", "     ", " ███ ", "█   █", "█   █", "█   █", " ███ "},
	'l': {"  █  ", "  █  ", "  █  ", "  █  ", "  █  ", "  █  ", "  █  "},
	'n': {"     ", "     ", "███  ", "█  █ ", "█  █ ", "█  █ ", "█  █ "},
	'i': {"     ", "  █  ", "     ", "  █  ", "  █  ", "  █  ", "  █  "},
	'x': {"     ", "     ", "█   █", " █ █ ", "  █  ", " █ █ ", "█   █"},
}

// Play runs the animation when stdout is a terminal and ttfx-rs is found
// ($VOLINIT_TTFX, else PATH). Ctrl-C skips the animation, not volinit.
func Play() error {
	if !term.IsTerminal(os.Stdout.Fd()) {
		return nil
	}
	binary := os.Getenv("VOLINIT_TTFX")
	if binary == "" {
		var err error
		if binary, err = exec.LookPath("ttfx-rs"); err != nil {
			return nil
		}
	}
	width, height, err := term.GetSize(os.Stdout.Fd())
	if err != nil {
		width, height = 80, 24
	}

	// Catching SIGINT here leaves the child's default handler intact.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	defer signal.Stop(sig)

	cmd := exec.Command(binary, args()...)
	cmd.Stdin = strings.NewReader(artworkFor(width, height))
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil && len(sig) == 0 {
		return fmt.Errorf("ttfx-rs: %w", err)
	}
	return nil
}

func artworkFor(width, height int) string {
	// The 7-row wordmark is 35 columns; below that, plain text.
	if width < 36 || height < 9 {
		return center([]string{"Volnix", "by lowcache"}, width, height)
	}
	scale := max(1, min(width/36, (height-2)/7, 3))
	var rows []string
	for y := range 7 {
		var row strings.Builder
		for i, ch := range "Volnix" {
			if i > 0 {
				row.WriteString(strings.Repeat(" ", scale))
			}
			for _, cell := range letter[ch][y] {
				row.WriteString(strings.Repeat(string(cell), scale))
			}
		}
		for range scale {
			rows = append(rows, row.String())
		}
	}
	pad := (len([]rune(rows[0])) - len("by lowcache")) / 2
	rows = append(rows, strings.Repeat(" ", pad)+"by lowcache")
	return center(rows, width, height)
}

// center pads rows to a full-screen block: ttfx-rs has no anchor flag and
// sizes its canvas to the input, so effects would fill only the padded part.
// One row is left free so the final cursor move doesn't scroll the screen.
func center(rows []string, width, height int) string {
	widest := 0
	for _, row := range rows {
		widest = max(widest, len([]rune(row)))
	}
	left := max(0, (width-widest)/2)
	blank := strings.Repeat(" ", width)
	out := make([]string, 0, height)
	for range max(0, (height-1-len(rows))/2) {
		out = append(out, blank)
	}
	for _, row := range rows {
		line := strings.Repeat(" ", left) + row
		out = append(out, line+strings.Repeat(" ", max(0, width-len([]rune(line)))))
	}
	for len(out) < height-1 {
		out = append(out, blank)
	}
	return strings.Join(out, "\n")
}

func args() []string {
	return []string{"--frame-rate", "120", "vhstape"}
}
