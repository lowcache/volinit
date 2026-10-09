// Package banner plays the Volnix wormhole welcome.
package banner

import (
	"bufio"
	"embed"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/charmbracelet/x/term"
)

// wordmarks holds the Decima Nova Heavy wordmark traced in quadrant blocks at
// three scales: volnix-N.txt is 35N columns by 7N rows.
//
//go:embed art/volnix-*.txt
var wordmarks embed.FS

const frameRate = 120

// Play runs the wormhole, then scrolls the status ticker until a key is
// pressed. The key is consumed; without a terminal on stdin the ticker makes
// one pass. Any key or Ctrl-C during the wormhole skips straight to the end.
func Play() error {
	if !term.IsTerminal(os.Stdout.Fd()) {
		return nil
	}
	width, height, err := term.GetSize(os.Stdout.Fd())
	if err != nil {
		width, height = 80, 24
	}
	// kitty draws the ticker at 2x with its text-sizing protocol.
	scale := 1
	if os.Getenv("KITTY_WINDOW_ID") != "" || os.Getenv("TERM") == "xterm-kitty" {
		scale = 2
	}
	canvas, tb := artworkFor(width, height, scale)
	rows := strings.Split(canvas, "\n")
	fx := newWormhole(rows)
	status := make(chan []item, 1)
	go func() { status <- probe() }()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	defer signal.Stop(sig)
	keys := make(chan struct{}, 1)
	interactive := false
	if term.IsTerminal(os.Stdin.Fd()) {
		if st, err := term.MakeRaw(os.Stdin.Fd()); err == nil {
			defer term.Restore(os.Stdin.Fd(), st)
			interactive = true
			go func() { os.Stdin.Read(make([]byte, 64)); keys <- struct{}{} }()
		}
	}
	clock := time.NewTicker(time.Second / frameRate)
	defer clock.Stop()
	// wait paces one frame and reports whether the viewer asked to stop.
	wait := func() bool {
		select {
		case <-sig:
		case <-keys:
		case <-clock.C:
			return false
		}
		return true
	}

	out := bufio.NewWriterSize(os.Stdout, 64<<10)
	defer fmt.Fprintf(os.Stdout, "\x1b[%d;1H\x1b[?25h", len(rows)+1)
	// Scroll existing output away, then draw from the top with the cursor hidden.
	out.WriteString(strings.Repeat("\n", len(rows)) + "\x1b[H\x1b[?25l")
	quit := false
	for !quit && !fx.tick() {
		out.WriteString(render(fx.cells()))
		if err := out.Flush(); err != nil {
			return err
		}
		quit = wait()
	}
	fx.finish()
	fx.tick()
	out.WriteString(render(fx.cells()))
	if !quit && tb.w > 0 {
		tk := newTicker(<-status, tb, len(rows), width, scale)
		for {
			if tk.frames%tk.step() == 0 {
				out.WriteString(tk.render())
				if err := out.Flush(); err != nil {
					return err
				}
			}
			if (tk.advance() && !interactive) || wait() {
				break
			}
		}
	}
	return out.Flush()
}

// artworkFor lays out the full-screen canvas: the wordmark, a blank row, and
// tickRows reserved for the ticker, whose position it returns.
func artworkFor(width, height, tickRows int) (string, box) {
	w := width
	rows := []string{strings.Repeat(" ", max(0, (width-6)/2)) + "Volnix"}
	// The 7-row wordmark is 35 columns; below that, plain text.
	if width >= 36 && height >= 9+tickRows {
		scale := max(1, min(width/36, (height-2-tickRows)/7, 3))
		b, _ := wordmarks.ReadFile(fmt.Sprintf("art/volnix-%d.txt", scale))
		rows = strings.Split(strings.TrimRight(string(b), "\n"), "\n")
		w = 35 * scale
	}
	if height < 3+tickRows {
		canvas, _, _ := center(rows, width, height)
		return canvas, box{}
	}
	rows = append(rows, "")
	for range tickRows {
		rows = append(rows, strings.Repeat(" ", w))
	}
	canvas, top, left := center(rows, width, height)
	return canvas, box{top + len(rows) - tickRows, left, w}
}

// center pads rows to a full-screen block so the wormhole's center is the
// screen's, returning the block's top row and left column. One row is left
// free for the cursor once the effect settles.
func center(rows []string, width, height int) (string, int, int) {
	widest := 0
	for _, row := range rows {
		widest = max(widest, len([]rune(row)))
	}
	left := max(0, (width-widest)/2)
	top := max(0, (height-1-len(rows))/2)
	blank := strings.Repeat(" ", width)
	out := make([]string, 0, height)
	for range top {
		out = append(out, blank)
	}
	for _, row := range rows {
		line := strings.Repeat(" ", left) + row
		out = append(out, line+strings.Repeat(" ", max(0, width-len([]rune(line)))))
	}
	for len(out) < height-1 {
		out = append(out, blank)
	}
	return strings.Join(out, "\n"), top, left
}
