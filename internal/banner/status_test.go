package banner

import (
	"testing"
	"time"
)

func TestSince(t *testing.T) {
	for d, want := range map[time.Duration]string{
		40 * time.Second:               "0m",
		12 * time.Minute:               "12m",
		3*time.Hour + 12*time.Minute:   "3h12m",
		50*time.Hour + 30*time.Minute:  "2d2h",
		400*time.Hour + 59*time.Minute: "16d16h",
	} {
		if got := since(d); got != want {
			t.Errorf("since(%v) = %q, want %q", d, got, want)
		}
	}
}

func TestDefaultRouteIface(t *testing.T) {
	route := "Iface\tDestination\tGateway\tFlags\n" +
		"vm-netgate\t0064A8C0\t00000000\t0001\n" +
		"wlp4s0\t00000000\t0101A8C0\t0003\n"
	if got := defaultIface(route); got != "wlp4s0" {
		t.Errorf("defaultIface = %q, want wlp4s0", got)
	}
	if got := defaultIface("Iface\tDestination\n"); got != "" {
		t.Errorf("no default route should give empty iface, got %q", got)
	}
}

func TestGeneration(t *testing.T) {
	if got := generation("system-304-link"); got != "304" {
		t.Errorf("generation = %q, want 304", got)
	}
	if got := generation("/nix/store/abc-nixos-system"); got != "" {
		t.Errorf("non-generation link gave %q", got)
	}
}

func TestShortRev(t *testing.T) {
	for in, want := range map[string]string{
		"51eceb9a722573d67b03e6db75291d5d7d7c1265\n":       "51eceb9",
		"51eceb9a722573d67b03e6db75291d5d7d7c1265-dirty\n": "51eceb9*",
		"unknown": "unknown",
	} {
		if got := shortRev(in); got != want {
			t.Errorf("shortRev(%q) = %q, want %q", in, got, want)
		}
	}
}
