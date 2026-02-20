package exec

import (
	"fmt"
	"io"
	"os"
	osexec "os/exec"
	"strings"

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

func (b *BorderedExec) Run() error {
	f, ok := b.stdout.(*os.File)
	if !ok {
		return b.runPlain()
	}
	w, h, err := term.GetSize(int(f.Fd()))
	if err != nil || h < 4 || w < 20 {
		return b.runPlain()
	}

	// Clear screen and draw borders
	fmt.Fprint(b.stdout, "\x1b[2J\x1b[H")
	b.drawTopBorder(w)
	b.drawBottomBorder(w, h)

	// Set scroll region (rows 2 to h-1, 1-indexed)
	fmt.Fprintf(b.stdout, "\x1b[2;%dr", h-1)
	// Move cursor into scroll region
	fmt.Fprint(b.stdout, "\x1b[2;1H")

	// Run subprocess
	b.Cmd.Stdin = b.stdin
	b.Cmd.Stdout = b.stdout
	b.Cmd.Stderr = b.stderr
	cmdErr := b.Cmd.Run()

	// Restore: reset scroll region, clear screen
	fmt.Fprint(b.stdout, "\x1b[r\x1b[2J\x1b[H")

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
