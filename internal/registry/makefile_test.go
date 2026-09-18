package registry

import (
	"os"
	"testing"
)

func TestParseHashHashExtractsSectionsAndTargets(t *testing.T) {
	src, err := os.ReadFile("testdata/hashhash.mk")
	if err != nil {
		t.Fatal(err)
	}
	got := ParseHashHash(string(src))
	if len(got) != 3 {
		t.Fatalf("got %d actions, want 3", len(got))
	}
	if got[0].Name != "switch" {
		t.Errorf("Name = %q, want switch", got[0].Name)
	}
	if got[0].Description != "Rebuild and switch system live" {
		t.Errorf("Description = %q", got[0].Description)
	}
	if got[0].Section != "System Operations" {
		t.Errorf("Section = %q, want System Operations", got[0].Section)
	}
	if got[2].Section != "Anonymous Mode" {
		t.Errorf("section did not switch, got %q", got[2].Section)
	}
}

func TestParseHashHashIgnoresPlainComments(t *testing.T) {
	got := ParseHashHash("# just a comment\nfoo:\n\techo hi\n")
	if len(got) != 0 {
		t.Fatalf("got %d actions, want 0", len(got))
	}
}
