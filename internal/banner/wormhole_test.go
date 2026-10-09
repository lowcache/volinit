package banner

import (
	"regexp"
	"strings"
	"testing"
)

var testRows = []string{
	"          ",
	"  █   █   ",
	"   █ █  x ",
	"    █     ",
	"          ",
}

func plain(cells [][]cell) []string {
	out := make([]string, len(cells))
	for y, row := range cells {
		var b strings.Builder
		for _, c := range row {
			if c.on {
				b.WriteRune(c.ch)
			} else {
				b.WriteRune(' ')
			}
		}
		out[y] = b.String()
	}
	return out
}

func TestWormholeHoldsTheArtFirst(t *testing.T) {
	fx := newWormhole(testRows)
	fx.tick()
	if got := plain(fx.cells()); strings.Join(got, "\n") != strings.Join(testRows, "\n") {
		t.Fatalf("hold frame differs from input:\n%s", strings.Join(got, "\n"))
	}
}

func TestWormholeSettlesWithEveryGlyphHome(t *testing.T) {
	fx := newWormhole(testRows)
	frames := 0
	for !fx.tick() {
		if frames++; frames > 20000 {
			t.Fatal("wormhole never finished")
		}
	}
	if frames <= holdFrames {
		t.Fatalf("finished after %d frames, before the hold ended", frames)
	}
	cells := fx.cells()
	if got := plain(cells); strings.Join(got, "\n") != strings.Join(testRows, "\n") {
		t.Fatalf("final frame differs from input:\n%s", strings.Join(got, "\n"))
	}
	for _, g := range fx.glyphs {
		if c := cells[g.homeY][g.homeX]; c.fg != g.final {
			t.Errorf("glyph at %d,%d colored %v, want final %v", g.homeY, g.homeX, c.fg, g.final)
		}
	}
}

func TestFinishSkipsToTheFinalFrame(t *testing.T) {
	fx := newWormhole(testRows)
	fx.tick()
	fx.finish()
	if !fx.tick() {
		t.Fatal("tick after finish should report done")
	}
	if got := plain(fx.cells()); strings.Join(got, "\n") != strings.Join(testRows, "\n") {
		t.Fatalf("skipped frame differs from input:\n%s", strings.Join(got, "\n"))
	}
}

func TestRadialGradientRunsCenterToCorner(t *testing.T) {
	g := newGradient(9, hex("80E0FF"), hex("00D1FF"), hex("8A008A"))
	if c := g.radial(5, 10, 10, 20); c != hex("80E0FF") {
		t.Errorf("center = %v, want first stop", c)
	}
	if c := g.radial(0, 0, 10, 20); c != hex("8A008A") {
		t.Errorf("corner = %v, want last stop", c)
	}
	if len(g) != 19 {
		t.Errorf("spectrum has %d colors, want 19", len(g))
	}
}

var ansi = regexp.MustCompile(`\x1b\[[0-9;?]*[A-Za-z]`)

func TestRenderRepaintsWholeRows(t *testing.T) {
	fx := newWormhole(testRows)
	fx.tick()
	out := render(fx.cells())
	if !strings.HasPrefix(out, "\x1b[?2026h") || !strings.HasSuffix(out, "\x1b[?2026l") {
		t.Error("frame is not wrapped in a synchronized update")
	}
	var text []string
	for _, r := range ansi.Split(out, -1) {
		if r != "" {
			text = append(text, r)
		}
	}
	if joined := strings.Join(text, ""); joined != strings.Join(testRows, "") {
		t.Errorf("rendered text = %q, want rows concatenated", joined)
	}
}
