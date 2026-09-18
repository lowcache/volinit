package hero

import "testing"

func envFrom(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestNotATTYIsT0(t *testing.T) {
	if got := Detect(envFrom(map[string]string{"TERM": "xterm-kitty"}), false); got != T0 {
		t.Errorf("piped output must be T0, got %v", got)
	}
}

func TestDumbTerminalIsT0(t *testing.T) {
	if got := Detect(envFrom(map[string]string{"TERM": "dumb"}), true); got != T0 {
		t.Errorf("TERM=dumb must be T0, got %v", got)
	}
}

func TestTruecolorIsT1(t *testing.T) {
	env := envFrom(map[string]string{"TERM": "xterm-256color", "COLORTERM": "truecolor"})
	if got := Detect(env, true); got != T1 {
		t.Errorf("truecolor tty must be T1, got %v", got)
	}
}

func TestNoColortermStillT1WhenTTY(t *testing.T) {
	env := envFrom(map[string]string{"TERM": "xterm-256color"})
	if got := Detect(env, true); got != T1 {
		t.Errorf("a normal tty floors at T1, got %v", got)
	}
}
