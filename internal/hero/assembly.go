package hero

import (
	"fmt"
	"math"

	"github.com/lowcache/volinit/internal/theme"
)

// Part is one plate of the assembly, named as the wiki's parts list names it.
type Part struct {
	Name     string
	Volatile bool // above the tmpfs datum: re-derived every boot, drawn hatched
}

// Volnix is the general-assembly stack from wiki.infernalcode.com, base first.
var Volnix = []Part{
	{"UEFI + Lanzaboote", false},
	{"CachyOS kernel", false},
	{"Hybrid graphics", false},
	{"/persist", false},
	{"tmpfs root", true},
	{"sops-nix + age", true},
	{"MicroVM gateways", true},
	{"CUDA AI stack", true},
	{"home-manager", true},
	{"niri + Noctalia v5", true},
}

// Phone is the detached sub-assembly, reached over the tailnet.
var Phone = Part{Name: "Nix-on-Droid phone tier"}

const (
	datum     = 4 // tmpfs root: the first volatile plate
	phoneJoin = 6 // MicroVM gateways, where the tailnet meets the phone

	thickRatio = 0.14 // plate thickness per unit R, from the wiki drawing
	gapRatio   = 0.46 // fully exploded gap between plates per unit R
	phoneRatio = 0.5  // phone plate size per unit R

	margin        = 2  // dots kept clear at every canvas edge
	minLabelR     = 8  // smallest plate that still carries lettering (pitch >= 4 dots)
	leaderDots    = 6  // leader length from a plate's right vertex
	labelCols     = 23 // "(10) niri + Noctalia v5"
	labelZone     = leaderDots + 2 + 2*labelCols
	phoneNameCols = 12 // "Nix-on-Droid"
	tieDots       = 8  // room for the tailnet tie between phone and stack
	hatchPitch    = 5
)

var (
	phoneName  = []string{"Nix-on-Droid", "phone tier"}
	centreLine = []int{6, 2, 1, 2} // long dash, short dash: drafting centre line
	leader     = []int{2, 2}
)

// Pose places the assembly in dot coordinates: CX is the axis, CY the
// stack's vertical centre, R the plate half-depth. Spread is 1 fully
// exploded, 0 assembled.
type Pose struct {
	CX, CY, R int
	Spread    float64
}

func thickness(r int) int { return max(2, int(math.Round(thickRatio*float64(r)))) }

func (p Pose) thick() int { return thickness(p.R) }

// pitch is the vertical distance between successive plates' centres.
func (p Pose) pitch() int {
	return p.thick() + int(math.Round(p.Spread*gapRatio*float64(p.R)))
}

// height is the stack's screen height, top vertex to bottom edge.
func (p Pose) height() int { return 2*p.R + p.thick() + (len(Volnix)-1)*p.pitch() }

func (p Pose) top() int { return p.CY - p.height()/2 }

// center is the y of plate i's top-face centre; plate 0 is the base.
func (p Pose) center(i int) int { return p.top() + p.R + (len(Volnix)-1-i)*p.pitch() }

// Lerp moves from a toward b; t is 0 at a and 1 at b.
func Lerp(a, b Pose, t float64) Pose {
	mix := func(x, y int) int { return x + int(math.Round(t*float64(y-x))) }
	return Pose{
		CX:     mix(a.CX, b.CX),
		CY:     mix(a.CY, b.CY),
		R:      mix(a.R, b.R),
		Spread: a.Spread + t*(b.Spread-a.Spread),
	}
}

// GreetingPose is the full-bleed pose: fully exploded and as large as the
// canvas allows. labels reports room for leaders, part names and the phone;
// without that room the bare stack is centred.
func GreetingPose(cols, rows int) (p Pose, labels bool) {
	w, h := 2*cols, 4*rows
	for r := h; r >= minLabelR; r-- {
		p = Pose{R: r, Spread: 1, CY: h / 2}
		if p.height() <= h-2*margin && margin+phoneZone(r)+4*r+1+labelZone+margin <= w {
			p.CX = margin + phoneZone(r) + 2*r
			return p, true
		}
	}
	return Pose{CX: w / 2, CY: h / 2, R: fit(w, h, 1), Spread: 1}, false
}

// StripPose is the assembled pose, centred in a cols×rows sidebar strip.
func StripPose(cols, rows int) Pose {
	w, h := 2*cols, 4*rows
	return Pose{CX: w / 2, CY: h / 2, R: fit(w, h, 0), Spread: 0}
}

// openSpread is how far past fully exploded the opening sequence begins.
// At this spread the outer plates start beyond the canvas and fall inward;
// Canvas.Set drops the dots that land outside.
const openSpread = 1.9

// OpeningPose is the greeting over-exploded: where the opening sequence
// starts, and the only pose in it that is not the greeting's own.
func OpeningPose(cols, rows int) Pose {
	p, _ := GreetingPose(cols, rows)
	p.Spread = openSpread
	return p
}

// fit is the largest R whose stack fits a w×h dot canvas inside the margin,
// or 0 when none does.
func fit(w, h int, spread float64) int {
	for r := h; r >= 2; r-- {
		p := Pose{R: r, Spread: spread}
		if 4*r+1 <= w-2*margin && p.height() <= h-2*margin {
			return r
		}
	}
	return 0
}

func phoneR(r int) int { return max(2, int(math.Round(phoneRatio*float64(r)))) }

// phoneZone is the width left of the stack given to the phone and its tie.
func phoneZone(r int) int { return max(4*phoneR(r)+1, 2*phoneNameCols) + tieDots }

// Draw renders the assembly at p onto a fresh cols×rows canvas.
func Draw(cols, rows int, p Pose, labels bool) *Canvas {
	c := NewCanvas(cols, rows)
	if p.R < 2 {
		return c // too small to read as anything
	}
	// The axis goes down first; every plate erases it where it passes behind.
	c.Line(p.CX, p.top()-p.R/2, p.CX, p.top()+p.height()+p.R/2, RoleFaint, centreLine)
	if labels {
		// The datum runs through the gap under tmpfs root: volatile above.
		y := p.center(datum) + p.thick() + (p.pitch()-p.thick())/2
		c.Line(margin, y, p.CX+2*p.R+leaderDots, y, RoleFaint, centreLine)
	}
	for i, part := range Volnix {
		plate(c, p.CX, p.center(i), p.R, p.thick(), part.Volatile)
	}
	if labels {
		annotate(c, p)
	}
	return c
}

// Frame renders the assembly at p into exactly cols×rows coloured cells.
func Frame(cols, rows int, p Pose, labels bool, pal theme.Palette) string {
	return Draw(cols, rows, p, labels).Render(pal)
}

// plate draws one plate centred at (cx, cy): a 2:1 rhombus top face 4r wide
// over a side band t deep. It first erases its silhouette, hiding whatever
// lower plate lies behind it.
func plate(c *Canvas, cx, cy, r, t int, hatched bool) {
	for dx := -2 * r; dx <= 2*r; dx++ {
		in := (abs(dx) + 1) / 2
		for y := cy - r + in; y <= cy+r+t-in; y++ {
			c.Set(cx+dx, y, RoleNone)
		}
	}
	if hatched {
		for dx := -2*r + 2; dx <= 2*r-2; dx++ {
			in := (abs(dx)+1)/2 + 1
			for y := cy - r + in; y <= cy+r-in; y++ {
				if mod(cx+dx-y, hatchPitch) == 0 {
					c.Set(cx+dx, y, RoleFaint)
				}
			}
		}
	}
	ink := func(x0, y0, x1, y1 int) { c.Line(x0, y0, x1, y1, RoleInk, nil) }
	ink(cx, cy-r, cx+2*r, cy) // top face
	ink(cx+2*r, cy, cx, cy+r)
	ink(cx, cy+r, cx-2*r, cy)
	ink(cx-2*r, cy, cx, cy-r)
	ink(cx-2*r, cy, cx-2*r, cy+t) // side band
	ink(cx, cy+r, cx, cy+r+t)
	ink(cx+2*r, cy, cx+2*r, cy+t)
	ink(cx-2*r, cy+t, cx, cy+r+t)
	ink(cx, cy+r+t, cx+2*r, cy+t)
}

// annotate adds the drawing's lettering: a dashed leader and a numbered
// balloon per plate, and the phone tied to the gateways it joins.
func annotate(c *Canvas, p Pose) {
	x := p.CX + 2*p.R + 1
	col := (x + leaderDots + 2) / 2
	for i, part := range Volnix {
		y := p.center(i)
		c.Line(x, y, x+leaderDots, y, RoleFaint, leader)
		c.Text(col, y/4, fmt.Sprintf("(%02d) %s", i+1, part.Name), RoleInk)
	}

	pr := phoneR(p.R)
	px := margin + max(4*pr+1, 2*phoneNameCols)/2
	py := p.center(phoneJoin)
	c.Line(px+2*pr+1, py, p.CX-2*p.R-1, py, RoleFaint, leader)
	plate(c, px, py, pr, thickness(pr), Phone.Volatile)
	balloon := fmt.Sprintf("(%02d)", len(Volnix)+1)
	c.Text(px/2-len(balloon)/2, (py-pr)/4-1, balloon, RoleInk)
	for k, line := range phoneName {
		c.Text(px/2-len(line)/2, (py+pr+thickness(pr))/4+1+k, line, RoleInk)
	}
}

func mod(a, n int) int { return (a%n + n) % n }
