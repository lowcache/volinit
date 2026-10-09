package banner

// Port of the ttfx-rs wormhole effect (github.com/louzt/ttfx-rs, MIT OR
// Apache-2.0): glyphs are pulled into the center, then burst back home.

import (
	"fmt"
	"math"
	"math/rand/v2"
	"strconv"
	"strings"
)

type rgb struct{ r, g, b uint8 }

func hex(s string) rgb {
	v, _ := strconv.ParseUint(s, 16, 32)
	return rgb{uint8(v >> 16), uint8(v >> 8), uint8(v)}
}

func lerp(a, b rgb, t float64) rgb {
	t = max(0, min(1, t))
	mix := func(x, y uint8) uint8 { return uint8(float64(x) + (float64(y)-float64(x))*t) }
	return rgb{mix(a.r, b.r), mix(a.g, b.g), mix(a.b, b.b)}
}

// dim scales HSL lightness by f, keeping hue and saturation.
func (c rgb) dim(f float64) rgb {
	r, g, b := float64(c.r)/255, float64(c.g)/255, float64(c.b)/255
	hi, lo := max(r, g, b), min(r, g, b)
	l := (hi + lo) / 2
	var h, s float64
	if hi != lo {
		d := hi - lo
		if l > 0.5 {
			s = d / (2 - hi - lo)
		} else {
			s = d / (hi + lo)
		}
		switch hi {
		case r:
			h = (g - b) / d
			if g < b {
				h += 6
			}
		case g:
			h = (b-r)/d + 2
		default:
			h = (r-g)/d + 4
		}
		h /= 6
	}
	l = max(0, min(1, l*f))
	if s == 0 {
		v := uint8(l * 255)
		return rgb{v, v, v}
	}
	q := l + s - l*s
	if l < 0.5 {
		q = l * (1 + s)
	}
	p := 2*l - q
	return rgb{uint8(hue(p, q, h+1.0/3) * 255), uint8(hue(p, q, h) * 255), uint8(hue(p, q, h-1.0/3) * 255)}
}

func hue(p, q, h float64) float64 {
	if h < 0 {
		h++
	} else if h > 1 {
		h--
	}
	switch {
	case h < 1.0/6:
		return p + (q-p)*6*h
	case h < 1.0/2:
		return q
	case h < 2.0/3:
		return p + (q-p)*(2.0/3-h)*6
	}
	return p
}

// gradient is the precomputed spectrum: `steps` colors per pair of stops.
type gradient []rgb

func newGradient(steps int, stops ...rgb) gradient {
	var g gradient
	for i := range len(stops) - 1 {
		for s := range steps {
			g = append(g, lerp(stops[i], stops[i+1], float64(s)/float64(steps)))
		}
	}
	return append(g, stops[len(stops)-1])
}

// radial picks a color by distance from the canvas center to its corner.
func (g gradient) radial(y, x, h, w int) rgb {
	cy, cx := float64(h)/2, float64(w)/2
	t := math.Hypot(float64(y)-cy, float64(x)-cx) / math.Hypot(cy, cx)
	return g[int(max(0, min(1, t))*float64(len(g)-1))]
}

func inExpo(t float64) float64 {
	if t == 0 {
		return 0
	}
	return math.Pow(2, 10*t-10)
}

func outExpo(t float64) float64 {
	if t == 1 {
		return 1
	}
	return 1 - math.Pow(2, -10*t)
}

const holdFrames = 60

const (
	holding = iota
	pulling
	expanding
	done
)

type glyph struct {
	homeY, homeX        int
	y, x                float64
	ch                  rune
	hold, final, accent rgb
	pull, pullSpeed     float64
	expand, expandSpeed float64
}

type wormhole struct {
	glyphs        []glyph
	w, h          int
	cy, cx        float64
	phase, frames int
}

func newWormhole(rows []string) *wormhole {
	fx := &wormhole{h: len(rows)}
	for _, row := range rows {
		fx.w = max(fx.w, len([]rune(row)))
	}
	fx.cy, fx.cx = float64(fx.h)/2, float64(fx.w)/2
	holdG := newGradient(9, hex("8A008A"), hex("00D1FF"), hex("80E0FF"))
	finalG := newGradient(9, hex("80E0FF"), hex("00D1FF"), hex("8A008A"))
	for y, row := range rows {
		for x, ch := range []rune(row) {
			if ch == ' ' {
				continue
			}
			dy, dx := float64(y)-fx.cy, float64(x)-fx.cx
			dist := max(1, math.Hypot(dy, dx))
			// The burst accent runs light blue → white around the center.
			t := math.Mod((math.Atan2(dy, dx)+math.Pi)/(2*math.Pi), 1)
			fx.glyphs = append(fx.glyphs, glyph{
				homeY: y, homeX: x, y: float64(y), x: float64(x), ch: ch,
				hold:        holdG.radial(y, x, fx.h, fx.w),
				final:       finalG.radial(y, x, fx.h, fx.w),
				accent:      rgb{uint8(130 + 125*t), uint8(210 + 45*t), 255},
				pullSpeed:   (0.08 + 0.06*rand.Float64()) / dist,
				expandSpeed: (0.15 + 0.10*rand.Float64()) / dist,
			})
		}
	}
	if len(fx.glyphs) == 0 {
		fx.phase = done
	}
	return fx
}

// finish skips straight to the settled frame.
func (fx *wormhole) finish() { fx.phase = done }

// tick advances one frame and reports whether the effect has settled.
func (fx *wormhole) tick() bool {
	switch fx.phase {
	case done:
		return true
	case holding:
		if fx.frames++; fx.frames >= holdFrames {
			fx.phase = pulling
		}
	case pulling:
		all := true
		for i := range fx.glyphs {
			g := &fx.glyphs[i]
			if g.pull < 1 {
				g.pull = min(1, g.pull+g.pullSpeed)
				t := inExpo(g.pull)
				g.y = float64(g.homeY) + (fx.cy-float64(g.homeY))*t
				g.x = float64(g.homeX) + (fx.cx-float64(g.homeX))*t
			}
			all = all && g.pull >= 1
		}
		if all {
			for i := range fx.glyphs {
				fx.glyphs[i].y, fx.glyphs[i].x = fx.cy, fx.cx
			}
			fx.phase = expanding
		}
	case expanding:
		all := true
		for i := range fx.glyphs {
			g := &fx.glyphs[i]
			if g.expand < 1 {
				g.expand = min(1, g.expand+g.expandSpeed)
				t := outExpo(g.expand)
				g.y = fx.cy + (float64(g.homeY)-fx.cy)*t
				g.x = fx.cx + (float64(g.homeX)-fx.cx)*t
			}
			all = all && g.expand >= 1
		}
		if all {
			fx.phase = done
		}
	}
	return false
}

type cell struct {
	ch rune
	fg rgb
	on bool
}

// cells lays out the current frame; later glyphs win shared cells.
func (fx *wormhole) cells() [][]cell {
	grid := make([][]cell, fx.h)
	for y := range grid {
		grid[y] = make([]cell, fx.w)
	}
	white := rgb{255, 255, 255}
	for _, g := range fx.glyphs {
		y, x, fg := g.homeY, g.homeX, g.final
		switch fx.phase {
		case holding:
			fg = g.hold
		case pulling:
			if g.pull >= 1 {
				continue
			}
			y, x, fg = int(math.Round(g.y)), int(math.Round(g.x)), g.hold.dim(1-g.pull)
		case expanding:
			// The burst flashes white → accent → final color.
			y, x = int(math.Round(g.y)), int(math.Round(g.x))
			if g.expand < 0.35 {
				fg = lerp(white, g.accent, g.expand/0.35)
			} else {
				fg = lerp(g.accent, g.final, (g.expand-0.35)/0.65)
			}
		}
		if y >= 0 && x >= 0 && y < fx.h && x < fx.w {
			grid[y][x] = cell{g.ch, fg, true}
		}
	}
	return grid
}

// render draws a frame from the top-left as one synchronized update,
// repainting every cell so no stale glyphs survive.
func render(grid [][]cell) string {
	var b strings.Builder
	b.WriteString("\x1b[?2026h")
	for y, row := range grid {
		renderRow(&b, y, 0, row)
	}
	b.WriteString("\x1b[?2026l")
	return b.String()
}

// renderRow draws cells starting at 0-based row y, column x.
func renderRow(b *strings.Builder, y, x int, row []cell) {
	fmt.Fprintf(b, "\x1b[%d;%dH", y+1, x+1)
	colored := false
	var last rgb
	for _, c := range row {
		switch {
		case !c.on:
			if colored {
				b.WriteString("\x1b[0m")
				colored = false
			}
			b.WriteRune(' ')
			continue
		case !colored || c.fg != last:
			fmt.Fprintf(b, "\x1b[38;2;%d;%d;%dm", c.fg.r, c.fg.g, c.fg.b)
			colored, last = true, c.fg
		}
		b.WriteRune(c.ch)
	}
	if colored {
		b.WriteString("\x1b[0m")
	}
}
