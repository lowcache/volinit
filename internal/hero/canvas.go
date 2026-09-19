package hero

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/lowcache/volinit/internal/theme"
)

// Role is what a mark means, which decides its colour. Higher roles win a
// cell shared with lower ones, since a cell carries one foreground.
type Role uint8

const (
	RoleNone  Role = iota
	RoleFaint      // hatching, leaders, construction lines: outline
	RoleInk        // part edges and lettering: on_surface
)

func (r Role) color(p theme.Palette) string {
	switch r {
	case RoleInk:
		return p.OnSurface
	case RoleFaint:
		return p.Outline
	}
	return ""
}

// Canvas is a grid of braille cells, each 2 dots wide and 4 tall, plus a
// text overlay that replaces a cell's braille where set.
type Canvas struct {
	cols, rows int
	dots       []Role // DotW × DotH, row-major
	text       []rune // cols × rows; 0 = no overlay
	textRole   []Role
}

func NewCanvas(cols, rows int) *Canvas {
	cols, rows = max(cols, 0), max(rows, 0)
	return &Canvas{
		cols:     cols,
		rows:     rows,
		dots:     make([]Role, 8*cols*rows),
		text:     make([]rune, cols*rows),
		textRole: make([]Role, cols*rows),
	}
}

func (c *Canvas) DotW() int { return 2 * c.cols }
func (c *Canvas) DotH() int { return 4 * c.rows }

// Set marks one dot; RoleNone clears it. Dots off the canvas are ignored so
// geometry may run past the edge.
func (c *Canvas) Set(x, y int, r Role) {
	if x < 0 || y < 0 || x >= c.DotW() || y >= c.DotH() {
		return
	}
	c.dots[y*c.DotW()+x] = r
}

// At reports one dot's role, RoleNone off the canvas.
func (c *Canvas) At(x, y int) Role {
	if x < 0 || y < 0 || x >= c.DotW() || y >= c.DotH() {
		return RoleNone
	}
	return c.dots[y*c.DotW()+x]
}

// Line draws both endpoints inclusive (Bresenham). dash is an on/off run
// pattern in dots, repeated along the line; nil is solid.
func (c *Canvas) Line(x0, y0, x1, y1 int, r Role, dash []int) {
	period := 0
	for _, d := range dash {
		period += d
	}
	dx, dy := abs(x1-x0), -abs(y1-y0)
	sx, sy := 1, 1
	if x0 > x1 {
		sx = -1
	}
	if y0 > y1 {
		sy = -1
	}
	err := dx + dy
	for i := 0; ; i++ {
		if on(dash, period, i) {
			c.Set(x0, y0, r)
		}
		if x0 == x1 && y0 == y1 {
			return
		}
		e2 := 2 * err // once per step: both tests use the same error
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

// on reports whether step i of a dashed line is inked.
func on(dash []int, period, i int) bool {
	if period == 0 {
		return true
	}
	i %= period
	for k, d := range dash {
		if i < d {
			return k%2 == 0
		}
		i -= d
	}
	return true
}

// Text overlays s from cell (col, row), clipped to the canvas.
func (c *Canvas) Text(col, row int, s string, r Role) {
	if row < 0 || row >= c.rows {
		return
	}
	for _, ch := range s {
		if col >= 0 && col < c.cols {
			c.text[row*c.cols+col] = ch
			c.textRole[row*c.cols+col] = r
		}
		col++
	}
}

// brailleBit is the U+2800-block bit for the dot at [dy][dx] within a cell.
var brailleBit = [4][2]rune{{0x01, 0x08}, {0x02, 0x10}, {0x04, 0x20}, {0x40, 0x80}}

// cell returns a cell's glyph and the highest role among its marks.
func (c *Canvas) cell(col, row int) (rune, Role) {
	i := row*c.cols + col
	if c.text[i] != 0 {
		return c.text[i], c.textRole[i]
	}
	var mask rune
	role := RoleNone
	for dy := 0; dy < 4; dy++ {
		for dx := 0; dx < 2; dx++ {
			r := c.dots[(4*row+dy)*c.DotW()+2*col+dx]
			if r == RoleNone {
				continue
			}
			mask |= brailleBit[dy][dx]
			role = max(role, r)
		}
	}
	if mask == 0 {
		return ' ', RoleNone
	}
	return 0x2800 + mask, role
}

// Plain is the canvas as glyphs with no escapes: rows joined by newlines,
// each exactly cols wide.
func (c *Canvas) Plain() string {
	var b strings.Builder
	for row := 0; row < c.rows; row++ {
		if row > 0 {
			b.WriteByte('\n')
		}
		for col := 0; col < c.cols; col++ {
			ch, _ := c.cell(col, row)
			b.WriteRune(ch)
		}
	}
	return b.String()
}

// Render is Plain with each run of same-role cells coloured from the
// palette. An unparseable palette colour falls back to the default fg.
func (c *Canvas) Render(p theme.Palette) string {
	sgr := map[Role]string{}
	for _, r := range []Role{RoleFaint, RoleInk} {
		if rgb, ok := parseHex(r.color(p)); ok {
			sgr[r] = fmt.Sprintf("\x1b[38;2;%d;%d;%dm", rgb[0], rgb[1], rgb[2])
		}
	}
	const reset = "\x1b[39m"
	var b strings.Builder
	for row := 0; row < c.rows; row++ {
		if row > 0 {
			b.WriteByte('\n')
		}
		cur := RoleNone
		for col := 0; col < c.cols; col++ {
			ch, r := c.cell(col, row)
			if r != cur {
				if s, ok := sgr[r]; ok {
					b.WriteString(s)
				} else {
					b.WriteString(reset)
				}
				cur = r
			}
			b.WriteRune(ch)
		}
		if cur != RoleNone {
			b.WriteString(reset)
		}
	}
	return b.String()
}

func parseHex(s string) ([3]uint8, bool) {
	var rgb [3]uint8
	if len(s) != 7 || s[0] != '#' {
		return rgb, false
	}
	for i := range rgb {
		v, err := strconv.ParseUint(s[1+2*i:3+2*i], 16, 8)
		if err != nil {
			return rgb, false
		}
		rgb[i] = uint8(v)
	}
	return rgb, true
}

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}
