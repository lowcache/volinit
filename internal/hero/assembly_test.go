package hero

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/lowcache/volinit/internal/theme"
)

func TestVolnixMatchesTheWiki(t *testing.T) {
	if len(Volnix) != 10 {
		t.Fatalf("the wiki draws ten plates, got %d", len(Volnix))
	}
	if Volnix[0].Name != "UEFI + Lanzaboote" || Volnix[9].Name != "niri + Noctalia v5" {
		t.Errorf("stack runs boot chain to shell, got %q .. %q", Volnix[0].Name, Volnix[9].Name)
	}
	for i, p := range Volnix {
		if want := i >= datum; p.Volatile != want {
			t.Errorf("%s: volatile = %v, want %v (tmpfs root is the datum)", p.Name, p.Volatile, want)
		}
	}
	if Volnix[datum].Name != "tmpfs root" || Volnix[phoneJoin].Name != "MicroVM gateways" {
		t.Error("datum or phone join points at the wrong plate")
	}
}

func TestPlateHatchesOnlyWhenVolatile(t *testing.T) {
	for _, hatched := range []bool{true, false} {
		c := NewCanvas(40, 20)
		plate(c, 40, 30, 16, 3, hatched)
		faint := 0
		for y := 0; y < c.DotH(); y++ {
			for x := 0; x < c.DotW(); x++ {
				if c.At(x, y) == RoleFaint {
					faint++
				}
			}
		}
		if hatched && faint == 0 {
			t.Error("a volatile plate drew no hatching")
		}
		if !hatched && faint != 0 {
			t.Errorf("a persistent plate drew %d hatch dots", faint)
		}
	}
}

// The top plate has nothing above it, so the only marks inside its face may
// be its own hatching: ink there is a lower plate showing through.
func TestTopPlateFaceIsHatchedAndOpaque(t *testing.T) {
	p := StripPose(24, 40)
	c := Draw(24, 40, p, false)
	cy := p.center(len(Volnix) - 1)
	hatch := 0
	for y := cy - p.R; y <= cy+p.R; y++ {
		for x := p.CX - 2*p.R; x <= p.CX+2*p.R; x++ {
			if abs(x-p.CX)/2+abs(y-cy) > p.R-2 {
				continue // on or next to the face's own edges
			}
			switch c.At(x, y) {
			case RoleInk:
				t.Fatalf("ink at (%d,%d) inside the top face: a lower plate shows through", x, y)
			case RoleFaint:
				hatch++
			}
		}
	}
	if hatch == 0 {
		t.Error("the top plate is volatile but its face carries no hatching")
	}
}

func TestFrameIsExactlyTheRequestedSize(t *testing.T) {
	for _, g := range [][2]int{{200, 50}, {80, 24}, {30, 40}, {24, 40}, {5, 3}} {
		cols, rows := g[0], g[1]
		greet, labels := GreetingPose(cols, rows)
		for _, c := range []*Canvas{Draw(cols, rows, greet, labels), Draw(cols, rows, StripPose(cols, rows), false)} {
			lines := strings.Split(c.Plain(), "\n")
			if len(lines) != rows {
				t.Fatalf("%dx%d: %d lines", cols, rows, len(lines))
			}
			for i, l := range lines {
				if n := utf8.RuneCountInString(l); n != cols {
					t.Fatalf("%dx%d: line %d is %d wide", cols, rows, i, n)
				}
			}
		}
	}
}

func TestGreetingLettersThePartsWhenThereIsRoom(t *testing.T) {
	p, labels := GreetingPose(200, 50)
	if !labels {
		t.Fatal("200x50 has room for the parts list")
	}
	out := Draw(200, 50, p, true).Plain()
	for _, want := range []string{"(01) UEFI + Lanzaboote", "(10) niri + Noctalia v5", "(11)", "Nix-on-Droid", "phone tier"} {
		if !strings.Contains(out, want) {
			t.Errorf("greeting missing %q", want)
		}
	}
}

func TestNarrowGreetingIsTheBareStack(t *testing.T) {
	p, labels := GreetingPose(30, 40)
	if labels {
		t.Fatal("30 columns cannot hold the parts list")
	}
	out := Draw(30, 40, p, false).Plain()
	if strings.Contains(out, "(") {
		t.Error("a bare stack carries no lettering")
	}
	if strings.TrimSpace(strings.ReplaceAll(out, "\n", "")) == "" {
		t.Error("the bare stack drew nothing")
	}
}

func TestStripPoseFitsTheStrip(t *testing.T) {
	p := StripPose(24, 40)
	if p.R < 2 {
		t.Fatalf("a 24x40 strip should fit the stack, R = %d", p.R)
	}
	if p.CX-2*p.R < 0 || p.CX+2*p.R >= 48 || p.top() < 0 || p.top()+p.height() >= 160 {
		t.Errorf("stack overflows the strip: %+v", p)
	}
	if p.Spread != 0 {
		t.Error("the strip holds the assembly closed")
	}
}

func TestLerpHitsBothEnds(t *testing.T) {
	a, _ := GreetingPose(200, 50)
	b := StripPose(24, 50)
	if Lerp(a, b, 0) != a || Lerp(a, b, 1) != b {
		t.Error("the morph must start at the greeting and end at the strip exactly")
	}
}

// The greeting renders live on every shell start; this budget is what lets
// the disk cache go unbuilt. Its log line is the evidence for that decision.
func TestGreetingFrameWithinBudget(t *testing.T) {
	const cols, rows, runs = 240, 70, 20
	const budget = 4 * time.Millisecond
	p, labels := GreetingPose(cols, rows)
	pal := theme.Default()
	Frame(cols, rows, p, labels, pal) // warm up
	start := time.Now()
	for i := 0; i < runs; i++ {
		Frame(cols, rows, p, labels, pal)
	}
	avg := time.Since(start) / runs
	t.Logf("greeting frame %dx%d: %v", cols, rows, avg)
	if avg > budget {
		t.Fatalf("frame took %v, budget %v", avg, budget)
	}
}

func TestOpeningPoseOvershootsTheGreeting(t *testing.T) {
	const cols, rows = 200, 50
	greet, _ := GreetingPose(cols, rows)
	open := OpeningPose(cols, rows)
	if open.CX != greet.CX || open.CY != greet.CY || open.R != greet.R {
		t.Errorf("the opening must sit where the greeting sits: %+v vs %+v", open, greet)
	}
	if open.Spread <= greet.Spread {
		t.Errorf("the opening starts past fully exploded, Spread = %v", open.Spread)
	}
}

// The over-exploded stack runs off the canvas by design and Canvas.Set drops
// what lands outside. What must not happen is a torn frame or a blank one.
func TestOverExplodedStackStaysInsideItsFrame(t *testing.T) {
	for _, g := range [][2]int{{200, 50}, {80, 24}, {30, 40}} {
		cols, rows := g[0], g[1]
		out := Draw(cols, rows, OpeningPose(cols, rows), false).Plain()
		lines := strings.Split(out, "\n")
		if len(lines) != rows {
			t.Fatalf("%dx%d: %d lines", cols, rows, len(lines))
		}
		for i, l := range lines {
			if n := utf8.RuneCountInString(l); n != cols {
				t.Fatalf("%dx%d: line %d is %d wide", cols, rows, i, n)
			}
		}
		if strings.TrimSpace(strings.ReplaceAll(out, "\n", "")) == "" {
			t.Errorf("%dx%d: the opening frame drew nothing", cols, rows)
		}
	}
}
