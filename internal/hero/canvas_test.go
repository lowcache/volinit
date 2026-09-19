package hero

import (
	"strings"
	"testing"

	"github.com/lowcache/volinit/internal/theme"
)

func TestBrailleDotsEncode(t *testing.T) {
	c := NewCanvas(1, 1)
	c.Set(0, 0, RoleInk)
	if got := c.Plain(); got != "⠁" {
		t.Fatalf("top-left dot = %q, want U+2801", got)
	}
	c.Set(1, 3, RoleInk)
	if got := c.Plain(); got != "⢁" {
		t.Fatalf("adding the bottom-right dot = %q, want U+2881", got)
	}
}

func TestPlainRowsAreFullWidth(t *testing.T) {
	if got := NewCanvas(3, 2).Plain(); got != "   \n   " {
		t.Fatalf("blank 3x2 = %q", got)
	}
}

func TestOutOfRangeDotsAreIgnored(t *testing.T) {
	c := NewCanvas(2, 2)
	c.Set(-1, 0, RoleInk)
	c.Set(0, -1, RoleInk)
	c.Set(4, 0, RoleInk)
	c.Set(0, 8, RoleInk)
	if strings.TrimSpace(strings.ReplaceAll(c.Plain(), "\n", "")) != "" {
		t.Fatal("a dot outside the canvas landed inside it")
	}
}

func TestLineReachesBothEnds(t *testing.T) {
	c := NewCanvas(10, 10)
	c.Line(1, 2, 17, 9, RoleInk, nil)
	if c.At(1, 2) != RoleInk || c.At(17, 9) != RoleInk {
		t.Fatal("line missed an endpoint")
	}
}

func TestDashedLineLeavesGaps(t *testing.T) {
	c := NewCanvas(4, 1)
	c.Line(0, 0, 7, 0, RoleFaint, []int{2, 2})
	want := []Role{RoleFaint, RoleFaint, RoleNone, RoleNone, RoleFaint, RoleFaint, RoleNone, RoleNone}
	for x, w := range want {
		if got := c.At(x, 0); got != w {
			t.Errorf("dot %d = %v, want %v", x, got, w)
		}
	}
}

func TestInkWinsASharedCell(t *testing.T) {
	c := NewCanvas(1, 1)
	c.Set(0, 0, RoleFaint)
	c.Set(1, 0, RoleInk)
	got := c.Render(theme.Default())
	if !strings.Contains(got, "\x1b[38;2;220;216;205m") { // on_surface #dcd8cd
		t.Errorf("shared cell not drawn in ink: %q", got)
	}
	if strings.Contains(got, "\x1b[38;2;107;96;87m") { // outline #6b6057
		t.Errorf("shared cell also drew the faint colour: %q", got)
	}
}

func TestTextReplacesBraille(t *testing.T) {
	c := NewCanvas(4, 1)
	c.Line(0, 0, 7, 3, RoleInk, nil)
	c.Text(1, 0, "ab", RoleInk)
	c.Text(3, 0, "overflow", RoleInk)
	got := []rune(c.Plain())
	if len(got) != 4 || got[1] != 'a' || got[2] != 'b' || got[3] != 'o' {
		t.Fatalf("text overlay = %q", string(got))
	}
}

func TestBadPaletteColourEmitsNoEscape(t *testing.T) {
	c := NewCanvas(1, 1)
	c.Set(0, 0, RoleInk)
	p := theme.Default()
	p.OnSurface = "not-a-colour"
	if got := c.Render(p); strings.Contains(got, "38;2") {
		t.Errorf("an unparseable colour emitted an escape: %q", got)
	}
}
