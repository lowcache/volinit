package theme

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

// Palette holds the Material 3 roles noctalia renders. Missing values fall
// back to Default so a fresh machine with no template still looks right.
type Palette struct {
	Primary          string `toml:"primary"`
	OnPrimary        string `toml:"on_primary"`
	Surface          string `toml:"surface"`
	OnSurface        string `toml:"on_surface"`
	SurfaceVariant   string `toml:"surface_variant"`
	SurfaceContainer string `toml:"surface_container"`
	Outline          string `toml:"outline"`
	Error            string `toml:"error"`
	Neutral          [18]string
}

// Default is the compiled-in palette, used when noctalia has written nothing.
func Default() Palette {
	return Palette{
		Primary: "#e5c799", OnPrimary: "#161311",
		Surface: "#161311", OnSurface: "#dcd8cd",
		SurfaceVariant: "#231e1a", SurfaceContainer: "#231e1a",
		Outline: "#6b6057", Error: "#cf767c",
	}
}

// Load reads a noctalia-rendered palette. A missing file is not an error:
// volinit runs on every shell and must never fail because a theme is absent.
func Load(path string) (Palette, error) {
	p := Default()
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return p, nil
		}
		return p, err
	}
	var flat map[string]any
	if _, err := toml.Decode(string(raw), &flat); err != nil {
		return p, err
	}
	if _, err := toml.Decode(string(raw), &p); err != nil {
		return p, err
	}
	for i := 0; i < 18; i++ {
		if v, ok := flat[fmt.Sprintf("neutral_%d", i)].(string); ok {
			p.Neutral[i] = v
		}
	}
	return p, nil
}

// Hash keys the hero frame cache: a palette change must invalidate it.
func (p Palette) Hash() string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%#v", p)))
	return hex.EncodeToString(sum[:8])
}
