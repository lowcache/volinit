package banner

import (
	"strings"
	"testing"
)

// art drops the blank padding so tests see the drawing itself, plus the
// index of its first row.
func art(s string) (lines []string, top int) {
	top = -1
	for i, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if top < 0 {
			top = i
		}
		lines = append(lines, strings.TrimRight(line, " "))
	}
	return lines, top
}

func TestArtworkScalesAndKeepsBylineSmall(t *testing.T) {
	lines, _ := art(artworkFor(120, 40))
	if len(lines) < 9 || !strings.Contains(lines[0], "███") {
		t.Fatalf("large terminal did not use the scaled block wordmark: %q", lines[0])
	}
	if strings.TrimSpace(lines[len(lines)-1]) != "by lowcache" {
		t.Fatalf("byline is not its own last line: %q", lines[len(lines)-1])
	}
}

func TestArtworkFillsTheScreen(t *testing.T) {
	// ttfx-rs sizes its canvas to the input, so the input must span the screen.
	rows := strings.Split(artworkFor(224, 55), "\n")
	if len(rows) != 54 {
		t.Errorf("got %d rows, want 54 (height-1)", len(rows))
	}
	for i, row := range rows {
		if n := len([]rune(row)); n != 224 {
			t.Fatalf("row %d is %d columns, want 224", i, n)
		}
	}
}

func TestArtworkIsCentered(t *testing.T) {
	lines, top := art(artworkFor(120, 40))
	if want := (39 - len(lines)) / 2; top != want {
		t.Errorf("top padding = %d, want %d", top, want)
	}
	left, right := 120, 120
	for _, line := range lines[:len(lines)-1] {
		left = min(left, len(line)-len(strings.TrimLeft(line, " ")))
		right = min(right, 120-len([]rune(line)))
	}
	if left-right > 1 || right-left > 1 {
		t.Errorf("wordmark not horizontally centered: left %d, right %d", left, right)
	}
}

func TestArtworkFallsBackOnNarrowTerminals(t *testing.T) {
	lines, _ := art(artworkFor(30, 8))
	if len(lines) != 2 || strings.TrimSpace(lines[0]) != "Volnix" || strings.TrimSpace(lines[1]) != "by lowcache" {
		t.Fatalf("narrow viewport should use compact text, got %q", lines)
	}
	for _, line := range lines {
		if len(line) > 30 {
			t.Errorf("compact line exceeds terminal width: %q", line)
		}
	}
}

func TestArgsMatchTtfxRSCLI(t *testing.T) {
	got := args()
	want := []string{"--frame-rate", "120", "vhstape"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("ttfx-rs args = %v, want %v", got, want)
	}
}

func TestArgsDoNotPassUnsupportedOptions(t *testing.T) {
	got := strings.Join(args(), " ")
	for _, unsupported := range []string{
		"--canvas-width", "--canvas-height", "--anchor-canvas", "--anchor-text",
		"--typing-speed", "--ciphertext-colors", "--final-gradient-stops", "--max-frames",
	} {
		if strings.Contains(got, unsupported) {
			t.Errorf("args contain unsupported ttfx-rs option %q: %s", unsupported, got)
		}
	}
}
