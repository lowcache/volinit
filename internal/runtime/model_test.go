package runtime

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/lowcache/volinit/internal/registry"
	"github.com/lowcache/volinit/internal/theme"
)

func fixture() Model {
	return New([]registry.Repo{
		{Name: "cfg", Branch: registry.BranchSystem, Actions: []registry.Action{
			{Name: "switch"}, {Name: "build"},
		}},
	}, theme.Default())
}

func TestDownMovesTheCursor(t *testing.T) {
	m := fixture()
	next, _ := m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if next.(Model).cursor != 1 {
		t.Errorf("cursor = %d, want 1", next.(Model).cursor)
	}
}

func TestCursorStopsAtTheEnd(t *testing.T) {
	m := fixture()
	m.cursor = 1
	next, _ := m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if next.(Model).cursor != 1 {
		t.Errorf("cursor ran past the end: %d", next.(Model).cursor)
	}
}

func TestViewListsEveryAction(t *testing.T) {
	v := fixture().View()
	for _, want := range []string{"switch", "build"} {
		if !strings.Contains(v.Content, want) {
			t.Errorf("view missing %q:\n%s", want, v.Content)
		}
	}
}
