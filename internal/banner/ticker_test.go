package banner

import (
	"regexp"
	"strings"
	"testing"
)

func text(row []cell) string {
	var b strings.Builder
	for _, c := range row {
		b.WriteRune(c.ch)
	}
	return b.String()
}

var testItems = []item{{"host", "volnix", neutral}, {"tor", "down", bad}}

func TestTickerScrollsAndLoops(t *testing.T) {
	tk := newTicker(testItems, box{y: 2, x: 3, w: 12}, 10, 40, 1)
	loop := "host volnix" + tickerSep + "tor down" + tickerSep
	if got := text(tk.cells()); got != loop[:12] {
		t.Fatalf("first frame = %q, want %q", got, loop[:12])
	}
	for range tickerStep {
		tk.advance()
	}
	if got := text(tk.cells()); got != loop[1:13] {
		t.Fatalf("after one step = %q, want %q", got, loop[1:13])
	}
	n := len([]rune(loop))*tickerStep - tickerStep
	for i := 1; i < n; i++ {
		if tk.advance() {
			t.Fatalf("reported a full loop early, at frame %d of %d", i, n)
		}
	}
	if !tk.advance() {
		t.Fatal("did not report the full loop")
	}
}

func TestTickerTintsStatusValues(t *testing.T) {
	tk := newTicker(testItems, box{y: 2, x: 0, w: 60}, 10, 60, 1)
	row := tk.cells()
	i := strings.Index(text(row), "down")
	if row[i].fg != hex(badColor) {
		t.Errorf("status value colored %v, want %v", row[i].fg, hex(badColor))
	}
	if j := strings.Index(text(row), "tor"); row[j].fg != tk.base[j] {
		t.Errorf("label colored %v, want gradient %v", row[j].fg, tk.base[j])
	}
}

func TestRenderRowDrawsAtTheBox(t *testing.T) {
	tk := newTicker(testItems, box{y: 4, x: 7, w: 12}, 10, 40, 1)
	out := tk.render()
	if !strings.HasPrefix(out, "\x1b[5;8H") {
		t.Errorf("ticker row should start at row 5 col 8, got %q", out[:min(len(out), 12)])
	}
}

func TestDoubleSizeTickerUsesKittyTextSizing(t *testing.T) {
	tk := newTicker(testItems, box{y: 4, x: 7, w: 25}, 10, 40, 2)
	if n := len(tk.cells()); n != 12 {
		t.Fatalf("25 columns at 2x hold %d glyphs, want 12", n)
	}
	out := tk.render()
	if !strings.HasPrefix(out, "\x1b[5;8H") {
		t.Errorf("ticker should start at row 5 col 8, got %q", out[:min(len(out), 12)])
	}
	var shown strings.Builder
	for _, m := range regexp.MustCompile(`\x1b\]66;s=2;([^\a]*)\a`).FindAllStringSubmatch(out, -1) {
		shown.WriteString(m[1])
	}
	if got := shown.String(); got != "host volnix " {
		t.Errorf("scaled text = %q, want the first 12 glyphs", got)
	}
	steps := 0
	for !tk.advance() {
		steps++
	}
	if want := len([]rune("host volnix"+tickerSep+"tor down"+tickerSep))*tickerStep*2 - 1; steps != want {
		t.Errorf("full loop took %d frames, want %d (half the glyph rate)", steps, want)
	}
}
