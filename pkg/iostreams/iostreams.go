// Package iostreams provides input/output stream management
package iostreams

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"

	"golang.org/x/term"
)

// IOStreams holds the standard input, output, and error streams
type IOStreams struct {
	In     io.Reader
	Out    io.Writer
	ErrOut io.Writer

	colorEnabled  bool
	isTerminal    func(io.Writer) bool
	isInputTTY    func(io.Reader) bool
	pager         string
	pagerCmd      *exec.Cmd
	noInteractive bool
}

// SetNoInteractive disables interactive prompts. When true, CanPrompt always
// returns false, forcing commands to require explicit --yes or fail in
// non-interactive (agent/script) contexts.
func (s *IOStreams) SetNoInteractive(v bool) {
	s.noInteractive = v
}

// NoInteractive returns whether non-interactive mode was explicitly requested.
func (s *IOStreams) NoInteractive() bool {
	return s != nil && s.noInteractive
}

// System returns IOStreams connected to standard input, output, and error
func System() *IOStreams {
	return &IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,

		colorEnabled: !isColorDisabled(),
		isTerminal:   isTerminal,
		isInputTTY:   isInputTerminal,
		pager:        os.Getenv("PAGER"),
	}
}

// ColorEnabled returns true if color output is enabled
func (s *IOStreams) ColorEnabled() bool {
	return s.colorEnabled && s.isTerminal(s.Out)
}

// ColorScheme returns a ColorScheme for output
func (s *IOStreams) ColorScheme() *ColorScheme {
	if !s.ColorEnabled() {
		return &ColorScheme{noColor: true}
	}
	return &ColorScheme{noColor: false}
}

// SetPager sets the pager command
func (s *IOStreams) SetPager(pager string) {
	s.pager = pager
}

// StartPager starts a pager for output
func (s *IOStreams) StartPager() error {
	if s.pager == "" {
		return nil
	}

	parts, err := splitCommandLine(s.pager)
	if err != nil {
		return err
	}
	if len(parts) == 0 {
		return nil
	}

	pagerCmd := exec.Command(parts[0], parts[1:]...)
	pagerCmd.Stdout = os.Stdout
	pagerCmd.Stderr = os.Stderr

	stdin, err := pagerCmd.StdinPipe()
	if err != nil {
		return err
	}

	if err := pagerCmd.Start(); err != nil {
		return err
	}

	s.pagerCmd = pagerCmd
	s.Out = stdin
	return nil
}

func splitCommandLine(raw string) ([]string, error) {
	var (
		args      []string
		current   []rune
		inSingle  bool
		inDouble  bool
		escaping  bool
		wasQuoted bool
	)

	flush := func() {
		if len(current) == 0 && !wasQuoted {
			return
		}
		args = append(args, string(current))
		current = current[:0]
		wasQuoted = false
	}

	for _, r := range raw {
		switch {
		case escaping:
			current = append(current, r)
			escaping = false
		case r == '\\' && !inSingle:
			escaping = true
		case r == '\'' && !inDouble:
			inSingle = !inSingle
			wasQuoted = true
		case r == '"' && !inSingle:
			inDouble = !inDouble
			wasQuoted = true
		case (r == ' ' || r == '\t' || r == '\n') && !inSingle && !inDouble:
			flush()
		default:
			current = append(current, r)
		}
	}

	if escaping {
		current = append(current, '\\')
	}
	if inSingle || inDouble {
		return nil, fmt.Errorf("invalid pager command: unterminated quote")
	}
	flush()
	return args, nil
}

// StopPager stops the pager
func (s *IOStreams) StopPager() error {
	if closer, ok := s.Out.(io.Closer); ok {
		if err := closer.Close(); err != nil {
			return err
		}
	}
	if s.pagerCmd != nil {
		return s.pagerCmd.Wait()
	}
	return nil
}

func isColorDisabled() bool {
	return os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb"
}

func isTerminal(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		fi, err := f.Stat()
		if err != nil {
			return false
		}
		return (fi.Mode() & os.ModeCharDevice) != 0
	}
	return false
}

func isInputTerminal(r io.Reader) bool {
	if f, ok := r.(*os.File); ok {
		// A char-device check alone misreads /dev/null (and NUL on Windows)
		// as a terminal, so confirm with a real TTY ioctl instead.
		return term.IsTerminal(int(f.Fd()))
	}
	return false
}

// IsStdinTTY returns true if stdin is a terminal
func (s *IOStreams) IsStdinTTY() bool {
	if s == nil {
		return false
	}
	if s.isInputTTY != nil {
		return s.isInputTTY(s.In)
	}
	return false
}

// IsStdoutTTY returns true if stdout is a terminal
func (s *IOStreams) IsStdoutTTY() bool {
	return s.isTerminal(s.Out)
}

// IsStderrTTY returns true if stderr is a terminal
func (s *IOStreams) IsStderrTTY() bool {
	return s.isTerminal(s.ErrOut)
}

// CanPromptForInput returns true when this IOStreams instance can prompt safely.
func (s *IOStreams) CanPrompt() bool {
	if s == nil || s.noInteractive {
		return false
	}
	return s.IsStdinTTY() && os.Getenv("GC_TEST_DISABLE_PROMPT") == ""
}

// IsInputTTY returns true if input is from a terminal
func IsInputTTY() bool {
	// A char-device check alone misreads /dev/null (and NUL on Windows)
	// as a terminal, so confirm with a real TTY ioctl instead.
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// CanPromptForInput returns true if we can prompt for input
func CanPromptForInput() bool {
	return IsInputTTY() && os.Getenv("GC_TEST_DISABLE_PROMPT") == ""
}

// Test returns IOStreams suitable for testing
func Test() (*IOStreams, *bytes.Buffer, *bytes.Buffer, *bytes.Buffer) {
	in := &bytes.Buffer{}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	return &IOStreams{
		In:         in,
		Out:        out,
		ErrOut:     errOut,
		isTerminal: func(io.Writer) bool { return false },
		isInputTTY: func(io.Reader) bool { return false },
	}, in, out, errOut
}

// TestTTY returns IOStreams suitable for testing with TTY enabled (IsStdoutTTY returns true).
func TestTTY() (*IOStreams, *bytes.Buffer, *bytes.Buffer, *bytes.Buffer) {
	in := &bytes.Buffer{}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	return &IOStreams{
		In:         in,
		Out:        out,
		ErrOut:     errOut,
		isTerminal: func(io.Writer) bool { return true },
		isInputTTY: func(io.Reader) bool { return true },
	}, in, out, errOut
}
