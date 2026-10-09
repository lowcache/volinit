package banner

import (
	"fmt"
	"strings"
)

const (
	tickerSep  = "  ◆  "
	tickerStep = 6 // frames per column: 20 columns/s at 120 fps, at any scale
	tickerFade = 4 // columns dimmed at each edge
	goodColor  = "80E0FF"
	badColor   = "8A008A"
)

// box is a screen region: 0-based row and column, and width.
type box struct{ y, x, w int }

type tglyph struct {
	ch     rune
	fg     rgb
	tinted bool
}

// ticker scrolls the status items through a box, colored by the wormhole's
// settled gradient so it reads as part of the wordmark. At scale 2 each glyph
// is drawn 2x2 cells with kitty's text-sizing protocol.
type ticker struct {
	box
	scale       int
	loop        []tglyph
	base        []rgb // gradient per glyph slot
	off, frames int
}

func newTicker(items []item, b box, h, w, scale int) *ticker {
	t := &ticker{box: b, scale: scale}
	add := func(s string, fg rgb, tinted bool) {
		for _, ch := range s {
			t.loop = append(t.loop, tglyph{ch, fg, tinted})
		}
	}
	for _, it := range items {
		add(it.label+" ", rgb{}, false)
		switch it.state {
		case good:
			add(it.value, hex(goodColor), true)
		case bad:
			add(it.value, hex(badColor), true)
		default:
			add(it.value, rgb{}, false)
		}
		add(tickerSep, rgb{}, false)
	}
	g := newGradient(9, hex("80E0FF"), hex("00D1FF"), hex("8A008A"))
	for i := range b.w / scale {
		t.base = append(t.base, g.radial(b.y, b.x+i*scale, h, w))
	}
	return t
}

// step is the number of frames per glyph.
func (t *ticker) step() int { return tickerStep * t.scale }

// advance moves one frame and reports when a full loop has scrolled past.
func (t *ticker) advance() bool {
	if t.frames++; t.frames%t.step() != 0 {
		return false
	}
	t.off = (t.off + 1) % len(t.loop)
	return t.off == 0
}

func (t *ticker) cells() []cell {
	row := make([]cell, len(t.base))
	fade := max(1, tickerFade/t.scale)
	for i := range row {
		g := t.loop[(t.off+i)%len(t.loop)]
		fg := t.base[i]
		if g.tinted {
			fg = g.fg
		}
		if e := min(i, len(row)-1-i); e < fade {
			fg = fg.dim(float64(e+1) / float64(fade+1))
		}
		row[i] = cell{g.ch, fg, g.ch != ' '}
	}
	return row
}

func (t *ticker) render() string {
	var b strings.Builder
	if t.scale == 1 {
		renderRow(&b, t.y, t.x, t.cells())
		return b.String()
	}
	fmt.Fprintf(&b, "\x1b[%d;%dH", t.y+1, t.x+1)
	var run strings.Builder
	flush := func() {
		if run.Len() > 0 {
			fmt.Fprintf(&b, "\x1b]66;s=%d;%s\a", t.scale, run.String())
			run.Reset()
		}
	}
	var last rgb
	for i, c := range t.cells() {
		if c.on && (i == 0 || c.fg != last) {
			flush()
			fmt.Fprintf(&b, "\x1b[38;2;%d;%d;%dm", c.fg.r, c.fg.g, c.fg.b)
			last = c.fg
		}
		run.WriteRune(c.ch)
	}
	flush()
	b.WriteString("\x1b[0m")
	return b.String()
}
