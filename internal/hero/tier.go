// Package hero owns the visual identity: tier detection, art, the frame
// cache and the morph between the greeting and the working state.
package hero

// Tier is how much the terminal can render. Higher tiers are strictly
// additive: every tier can do everything below it.
type Tier int

const (
	T0 Tier = iota // plain text, no escapes — pipes, cron, dumb terminals
	T1             // truecolor character cells
	T2             // kitty graphics: prerendered image frames
	T3             // kitty graphics + live 3D
)

// Detect returns the floor tier from environment and TTY state alone.
// T2/T3 additionally require a terminal QUERY, which happens elsewhere —
// this never promotes above T1 so that a non-graphical terminal can never
// be mistaken for a graphical one on env vars alone.
func Detect(env func(string) string, isTTY bool) Tier {
	if !isTTY {
		return T0
	}
	switch env("TERM") {
	case "", "dumb":
		return T0
	}
	return T1
}
