package banner

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Endpoints on volnix; see ~/.nix-config nixos/modules/anonymous-mode.nix
// and nixos/phone-agent/default.nix.
const (
	anonReady = "/run/anon-mode/ready"
	netGateIf = "vm-netgate"
	torSocks  = "192.168.100.2:9050"
	phoneMCP  = "100.101.229.9:8462"
)

const (
	neutral = iota
	good
	bad
)

type item struct {
	label, value string
	state        int
}

// probe gathers the ticker items concurrently; network checks are bounded so
// the whole call finishes within a second.
func probe() []item {
	checks := []func() item{
		func() item { h, _ := os.Hostname(); return item{"host", h, neutral} },
		func() item { return item{"kernel", firstLine("/proc/sys/kernel/osrelease"), neutral} },
		func() item {
			f := strings.Fields(firstLine("/proc/uptime"))
			s, _ := strconv.ParseFloat(append(f, "0")[0], 64)
			return item{"up", since(time.Duration(s) * time.Second), neutral}
		},
		func() item {
			link, _ := os.Readlink("/nix/var/nix/profiles/system")
			v := "gen " + generation(link)
			if fi, err := os.Lstat(filepath.Join("/nix/var/nix/profiles", link)); err == nil {
				v += ", built " + since(time.Since(fi.ModTime())) + " ago"
			}
			return item{"nixos", v, neutral}
		},
		func() item {
			out, _ := exec.Command("nixos-version", "--configuration-revision").Output()
			return item{"rev", shortRev(string(out)), neutral}
		},
		func() item { return item{"ip", ifaceAddr(defaultIface(readFile("/proc/net/route"))), neutral} },
		func() item {
			_, err := os.Stat(anonReady)
			return onOff("anon", err == nil, "on", "off")
		},
		func() item {
			return onOff("net-gate", firstLine("/sys/class/net/"+netGateIf+"/operstate") == "up", "up", "down")
		},
		func() item { return onOff("tor", reachable(torSocks, 300*time.Millisecond), "up", "down") },
		func() item {
			return onOff("phone-agent", reachable(phoneMCP, 800*time.Millisecond), "connected", "disconnected")
		},
	}
	items := make([]item, len(checks))
	var wg sync.WaitGroup
	for i, check := range checks {
		wg.Go(func() { items[i] = check() })
	}
	wg.Wait()
	return items
}

func onOff(label string, ok bool, yes, no string) item {
	if ok {
		return item{label, yes, good}
	}
	return item{label, no, bad}
}

func readFile(path string) string {
	b, _ := os.ReadFile(path)
	return string(b)
}

func firstLine(path string) string {
	line, _, _ := strings.Cut(readFile(path), "\n")
	return strings.TrimSpace(line)
}

func reachable(addr string, timeout time.Duration) bool {
	c, err := net.DialTimeout("tcp", addr, timeout)
	if err == nil {
		c.Close()
	}
	return err == nil
}

// since renders a duration as "3h12m" or "2d2h".
func since(d time.Duration) string {
	m := int(d.Minutes())
	switch {
	case m >= 24*60:
		return fmt.Sprintf("%dd%dh", m/(24*60), m%(24*60)/60)
	case m >= 60:
		return fmt.Sprintf("%dh%dm", m/60, m%60)
	}
	return fmt.Sprintf("%dm", m)
}

// defaultIface finds the interface holding the IPv4 default route in
// /proc/net/route text.
func defaultIface(route string) string {
	for _, line := range strings.Split(route, "\n")[1:] {
		if f := strings.Fields(line); len(f) > 1 && f[1] == "00000000" {
			return f[0]
		}
	}
	return ""
}

func ifaceAddr(name string) string {
	if iface, err := net.InterfaceByName(name); err == nil {
		addrs, _ := iface.Addrs()
		for _, a := range addrs {
			if n, ok := a.(*net.IPNet); ok && n.IP.To4() != nil {
				return n.IP.String()
			}
		}
	}
	return "none"
}

// generation extracts N from a "system-N-link" profile link.
func generation(link string) string {
	n, ok := strings.CutPrefix(filepath.Base(link), "system-")
	if n, ok2 := strings.CutSuffix(n, "-link"); ok && ok2 {
		return n
	}
	return ""
}

// shortRev abbreviates a configuration revision, marking dirty trees with "*".
func shortRev(rev string) string {
	rev = strings.TrimSpace(rev)
	clean, dirty := strings.CutSuffix(rev, "-dirty")
	if len(clean) != 40 {
		if rev == "" {
			return "unknown"
		}
		return rev
	}
	if dirty {
		return clean[:7] + "*"
	}
	return clean[:7]
}
