package exec

import (
	"fmt"
	"io"
	"os"
	osexec "os/exec"
	"strings"

	"github.com/creack/pty"
	"golang.org/x/term"
)

// BorderedExec wraps a command in a bordered terminal frame.
// Top border displays a title, bottom border displays shortcuts.
// The subprocess runs inside a scroll region between the borders.
// Implements tea.ExecCommand.
type BorderedExec struct {
	Title string
	Cmd   *osexec.Cmd

	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

func (b *BorderedExec) SetStdin(r io.Reader)  { b.stdin = r }
func (b *BorderedExec) SetStdout(w io.Writer) { b.stdout = w }
func (b *BorderedExec) SetStderr(w io.Writer) { b.stderr = w }

// escScanner filters PTY output, intercepting screen-clearing escape
// sequences and redrawing borders to prevent them from being erased.
type escScanner struct {
	out    io.Writer
	buf    []byte // CSI parameter accumulator
	state  int    // 0=normal, 1=ESC, 2=CSI
	normal []byte // accumulated normal bytes for bulk write
	redraw func()
}

func (s *escScanner) Write(p []byte) (int, error) {
	for _, ch := range p {
		switch s.state {
		case 0: // normal
			if ch == 0x1b {
				s.flushNormal()
				s.state = 1
			} else {
				s.normal = append(s.normal, ch)
			}
		case 1: // after ESC
			switch ch {
			case '[':
				s.state = 2
				s.buf = s.buf[:0]
			case 'c':
				// RIS (Reset to Initial State) — redraw borders
				s.redraw()
				s.state = 0
			default:
				s.out.Write([]byte{0x1b, ch})
				s.state = 0
			}
		case 2: // CSI parameters
			if (ch >= 0x20 && ch <= 0x2f) || (ch >= 0x30 && ch <= 0x3f) {
				s.buf = append(s.buf, ch)
			} else if ch >= 0x40 && ch <= 0x7e {
				s.handleCSI(ch)
				s.state = 0
			} else {
				// Invalid CSI — pass through
				s.out.Write([]byte{0x1b, '['})
				s.out.Write(s.buf)
				s.out.Write([]byte{ch})
				s.state = 0
			}
		}
	}
	s.flushNormal()
	return len(p), nil
}

func (s *escScanner) flushNormal() {
	if len(s.normal) > 0 {
		s.out.Write(s.normal)
		s.normal = s.normal[:0]
	}
}

func (s *escScanner) handleCSI(final byte) {
	params := string(s.buf)
	switch {
	case final == 'J' && (params == "2" || params == "3"):
		// ED 2 (erase display) or ED 3 (erase scrollback) — redraw
		s.redraw()
	case final == 'r' && params == "":
		// DECSTBM reset (no params) — redraw to re-apply scroll region
		s.redraw()
	case final == 'p' && params == "!":
		// DECSTR (Soft Terminal Reset) — redraw to re-apply scroll region and DECOM
		s.redraw()
	case final == 'l' && params == "?6":
		// DECOM disable — suppress to keep origin mode active
	default:
		s.out.Write([]byte{0x1b, '['})
		s.out.Write(s.buf)
		s.out.Write([]byte{final})
	}
}

func (b *BorderedExec) fullRedraw(w, h int) {
	fmt.Fprint(b.stdout, "\x1b[?6l")         // disable DECOM for absolute positioning
	fmt.Fprint(b.stdout, "\x1b[2J\x1b[H")    // clear screen, cursor home
	b.drawTopBorder(w)
	b.drawBottomBorder(w, h)
	fmt.Fprintf(b.stdout, "\x1b[2;%dr", h-1) // scroll region rows 2..h-1
	fmt.Fprint(b.stdout, "\x1b[?6h")         // enable DECOM
	fmt.Fprint(b.stdout, "\x1b[H")           // cursor to scroll region origin
}

func (b *BorderedExec) scanAndForward(ptmx *os.File, w, h int) {
	scanner := &escScanner{
		out:    b.stdout,
		redraw: func() { b.fullRedraw(w, h) },
	}
	buf := make([]byte, 4096)
	for {
		n, err := ptmx.Read(buf)
		if n > 0 {
			scanner.Write(buf[:n])
		}
		if err != nil {
			return
		}
	}
}

func (b *BorderedExec) Run() error {
	f, ok := b.stdout.(*os.File)
	if !ok {
		return b.runPlain()
	}
	w, h, err := term.GetSize(int(f.Fd()))
	if err != nil || h < 4 || w < 20 {
		return b.runPlain()
	}

	// Set real terminal to raw mode for PTY forwarding
	oldState, err := term.MakeRaw(int(f.Fd()))
	if err != nil {
		return b.runPlain()
	}
	defer term.Restore(int(f.Fd()), oldState)

	// Start subprocess in a PTY sized to the content area
	ptmx, err := pty.StartWithSize(b.Cmd, &pty.Winsize{
		Rows: uint16(h - 2),
		Cols: uint16(w),
	})
	if err != nil {
		return b.runPlain()
	}
	defer ptmx.Close()

	// Draw borders and set up scroll region + DECOM
	b.fullRedraw(w, h)

	// Forward stdin → PTY in background
	go io.Copy(ptmx, b.stdin)

	// Forward PTY → filtered stdout (blocks until PTY EOF)
	b.scanAndForward(ptmx, w, h)

	// Wait for subprocess to finish
	cmdErr := b.Cmd.Wait()

	// Restore: disable DECOM, reset scroll region, clear screen
	fmt.Fprint(b.stdout, "\x1b[?6l\x1b[r\x1b[2J\x1b[H")

	return cmdErr
}

func (b *BorderedExec) runPlain() error {
	b.Cmd.Stdin = b.stdin
	b.Cmd.Stdout = b.stdout
	b.Cmd.Stderr = b.stderr
	return b.Cmd.Run()
}

func (b *BorderedExec) drawTopBorder(w int) {
	// ╭─ title ──────────╮
	title := " " + b.Title + " "
	fill := w - len([]rune(title)) - 3 // 2 corners + 1 leading ─
	if fill < 0 {
		fill = 0
	}
	fmt.Fprintf(b.stdout, "\x1b[1;1H\x1b[38;5;8m╭─%s%s╮\x1b[0m", title, strings.Repeat("─", fill))
}

func (b *BorderedExec) drawBottomBorder(w, h int) {
	// ╰─ Ctrl+D:Exit ───╯
	shortcuts := " Ctrl+D:Exit "
	fill := w - len([]rune(shortcuts)) - 3
	if fill < 0 {
		fill = 0
	}
	fmt.Fprintf(b.stdout, "\x1b[%d;1H\x1b[38;5;8m╰─%s%s╯\x1b[0m", h, shortcuts, strings.Repeat("─", fill))
}
