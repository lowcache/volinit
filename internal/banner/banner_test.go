package banner

import (
	"fmt"
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

func canvas(width, height int) string {
	c, _ := artworkFor(width, height, 1)
	return c
}

func TestArtworkReservesTickerRowUnderWordmark(t *testing.T) {
	c, tb := artworkFor(120, 40, 1)
	lines, top := art(c)
	if len(lines) != 21 || !strings.Contains(lines[0], "███") {
		t.Fatalf("large terminal did not use the full-size wordmark: %d rows", len(lines))
	}
	if tb.y != top+len(lines)+1 {
		t.Errorf("ticker row = %d, want one blank row under the wordmark (%d)", tb.y, top+len(lines)+1)
	}
	if want := (120 - 105) / 2; tb.x != want || tb.w != 105 {
		t.Errorf("ticker box = %+v, want x=%d w=105", tb, want)
	}
	if row := strings.Split(c, "\n")[tb.y]; strings.TrimSpace(row) != "" {
		t.Errorf("ticker row is not blank on the canvas: %q", row)
	}
}

func TestArtworkReservesTwoRowsForDoubleSizeTicker(t *testing.T) {
	c, tb := artworkFor(120, 40, 2)
	rows := strings.Split(c, "\n")
	lines, top := art(c)
	if tb.y != top+len(lines)+1 {
		t.Errorf("ticker row = %d, want %d", tb.y, top+len(lines)+1)
	}
	if tb.y+1 >= len(rows) || strings.TrimSpace(rows[tb.y]+rows[tb.y+1]) != "" {
		t.Errorf("both ticker rows must be blank and on the canvas")
	}
}

func TestArtworkFillsTheScreen(t *testing.T) {
	// The wormhole centers on the canvas, so the canvas must span the screen.
	rows := strings.Split(canvas(224, 55), "\n")
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
	lines, top := art(canvas(120, 40))
	if want := (39 - len(lines) - 2) / 2; top != want {
		t.Errorf("top padding = %d, want %d", top, want)
	}
	left, right := 120, 120
	for _, line := range lines {
		left = min(left, len(line)-len(strings.TrimLeft(line, " ")))
		right = min(right, 120-len([]rune(line)))
	}
	if left-right > 1 || right-left > 1 {
		t.Errorf("wordmark not horizontally centered: left %d, right %d", left, right)
	}
}

func TestArtworkFallsBackOnNarrowTerminals(t *testing.T) {
	c, tb := artworkFor(30, 8, 1)
	lines, _ := art(c)
	if len(lines) != 1 || strings.TrimSpace(lines[0]) != "Volnix" {
		t.Fatalf("narrow viewport should use compact text, got %q", lines)
	}
	if tb.w != 30 || tb.x != 0 {
		t.Errorf("narrow ticker should span the terminal, got %+v", tb)
	}
	if _, tb := artworkFor(30, 3, 1); tb.w != 0 {
		t.Errorf("a 3-row terminal has no room for the ticker, got %+v", tb)
	}
}

func TestEveryScaleFitsItsBox(t *testing.T) {
	for scale := 1; scale <= 3; scale++ {
		b, err := wordmarks.ReadFile(fmt.Sprintf("art/volnix-%d.txt", scale))
		if err != nil {
			t.Fatal(err)
		}
		rows := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
		if len(rows) != 7*scale {
			t.Errorf("scale %d: %d rows, want %d", scale, len(rows), 7*scale)
		}
		for i, row := range rows {
			if n := len([]rune(row)); n > 35*scale {
				t.Errorf("scale %d row %d: %d columns, want <= %d", scale, i, n, 35*scale)
			}
		}
	}
}
