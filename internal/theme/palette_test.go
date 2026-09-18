package theme

import "testing"

func TestLoadReadsM3Roles(t *testing.T) {
	p, err := Load("testdata/palette.toml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if p.Primary != "#e5c799" {
		t.Errorf("Primary = %q, want #e5c799", p.Primary)
	}
	if p.OnSurface != "#dcd8cd" {
		t.Errorf("OnSurface = %q, want #dcd8cd", p.OnSurface)
	}
	if p.Neutral[1] != "#13100e" {
		t.Errorf("Neutral[1] = %q, want #13100e", p.Neutral[1])
	}
}

func TestMissingFileReturnsDefaultPalette(t *testing.T) {
	p, err := Load("testdata/does-not-exist.toml")
	if err != nil {
		t.Fatalf("missing file must not error, got %v", err)
	}
	if p.Primary == "" {
		t.Fatal("default palette must be populated")
	}
}

func TestHashChangesWithPalette(t *testing.T) {
	a, _ := Load("testdata/palette.toml")
	b := a
	b.Primary = "#ffffff"
	if a.Hash() == b.Hash() {
		t.Fatal("hash must change when a role changes")
	}
}
