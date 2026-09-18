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
	if len(got) != 5 {
		t.Fatalf("got %d actions, want 5", len(got))
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
	// Check that new targets with dashes are also parsed
	if got[3].Name != "serve" {
		t.Errorf("got[3].Name = %q, want serve", got[3].Name)
	}
	if got[3].Description != "Development server at localhost" {
		t.Errorf("got[3].Description = %q", got[3].Description)
	}
	if got[3].Section != "Blog Operations" {
		t.Errorf("got[3].Section = %q, want Blog Operations", got[3].Section)
	}
	if got[4].Name != "deploy" {
		t.Errorf("got[4].Name = %q, want deploy", got[4].Name)
	}
}

func TestParseHashHashIgnoresPlainComments(t *testing.T) {
	got := ParseHashHash("# just a comment\nfoo:\n\techo hi\n")
	if len(got) != 0 {
		t.Fatalf("got %d actions, want 0", len(got))
	}
}

func TestParseHelpTargetReadsEchoedLines(t *testing.T) {
	src, err := os.ReadFile("testdata/helptarget.mk")
	if err != nil {
		t.Fatal(err)
	}
	got := ParseHelpTarget(string(src))
	if len(got) != 3 {
		t.Fatalf("got %d actions, want 3", len(got))
	}
	if got[0].Name != "serve" {
		t.Errorf("Name = %q, want serve", got[0].Name)
	}
	if got[0].Description != "Live preview incl. drafts (http://localhost:1313)" {
		t.Errorf("Description = %q", got[0].Description)
	}
	if got[2].Name != "deploy" {
		t.Errorf("Name = %q, want deploy", got[2].Name)
	}
}

func TestParseHelpTargetStopsAtRecipeEnd(t *testing.T) {
	src := "help:\n\t@echo \"make a   first\"\n\nother:\n\t@echo \"make b   second\"\n"
	got := ParseHelpTarget(src)
	if len(got) != 1 {
		t.Fatalf("got %d actions, want 1 — must not read past the help recipe", len(got))
	}
}

func TestTargetLineRegexBothForms(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		wantOK  bool
		wantName string
		wantDesc string
	}{
		// Colon + dots form (nix-config dialect)
		{
			name:     "colon with dots",
			line:     "## :switch: ..........: Rebuild and switch",
			wantOK:   true,
			wantName: "switch",
			wantDesc: "Rebuild and switch",
		},
		// No colon + dashes form (blog dialect)
		{
			name:     "no colon with dashes",
			line:     "## serve: -----: Development server",
			wantOK:   true,
			wantName: "serve",
			wantDesc: "Development server",
		},
		// Mixed: colon + dashes
		{
			name:     "colon with dashes",
			line:     "## :deploy: ---: Deploy to production",
			wantOK:   true,
			wantName: "deploy",
			wantDesc: "Deploy to production",
		},
		// Mixed: no colon + dots
		{
			name:     "no colon with dots",
			line:     "## build: .....: Build the system",
			wantOK:   true,
			wantName: "build",
			wantDesc: "Build the system",
		},
		// Section header should NOT match
		{
			name:    "section header",
			line:    "## System Operations",
			wantOK:  false,
		},
		// Plain comment should NOT match
		{
			name:    "plain comment",
			line:    "# just a comment",
			wantOK:  false,
		},
		// Missing description should NOT match
		{
			name:   "missing description",
			line:   "## :switch: ....:",
			wantOK: false,
		},
		// Missing trailing colon should NOT match
		{
			name:   "missing trailing colon",
			line:   "## :switch: ..... Something",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := targetLine.FindStringSubmatch(tt.line)
			got := m != nil
			if got != tt.wantOK {
				t.Fatalf("match = %v, want %v", got, tt.wantOK)
			}
			if got {
				if m[1] != tt.wantName {
					t.Errorf("name = %q, want %q", m[1], tt.wantName)
				}
				if m[2] != tt.wantDesc {
					t.Errorf("description = %q, want %q", m[2], tt.wantDesc)
				}
			}
		})
	}
}


func TestParseBareTargetsRejectsNonTargets(t *testing.T) {
	src := "VER := 1\n" +
		"VER ?= 2\n" +
		".PHONY: all build\n" +
		"%.o: %.c\n" +
		"\tcc -c $<\n" +
		"# comment: not a target\n" +
		"build: VER\n" +
		"\techo build\n" +
		"build:\n" +
		"test-all:\n"
	var got []string
	for _, a := range ParseBareTargets(src) {
		got = append(got, a.Name)
	}
	want := []string{"build", "test-all"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
