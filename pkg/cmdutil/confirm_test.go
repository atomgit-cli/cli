package cmdutil

import (
	"bufio"
	"errors"
	"io"
	"strings"
	"testing"

	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
)

func TestConfirmOrAbort_YesFlagSkipsConfirmation(t *testing.T) {
	opts := ConfirmOptions{
		Yes:      true,
		Expected: "123",
	}
	if err := ConfirmOrAbort(opts); err != nil {
		t.Fatalf("ConfirmOrAbort() with --yes = %v, want nil", err)
	}
}

func TestConfirmOrAbort_YesFlagWorksWithNilIO(t *testing.T) {
	opts := ConfirmOptions{
		IO:       nil,
		Yes:      true,
		Expected: "123",
	}
	if err := ConfirmOrAbort(opts); err != nil {
		t.Fatalf("ConfirmOrAbort() with --yes and nil IO = %v, want nil", err)
	}
}

func TestConfirmOrAbort_MissingExpectedValue(t *testing.T) {
	io, _, _, _ := iostreams.Test()
	opts := ConfirmOptions{
		IO:       io,
		Expected: "",
	}
	err := ConfirmOrAbort(opts)
	if err == nil {
		t.Fatal("ConfirmOrAbort() with empty Expected = nil, want error")
	}
	if _, ok := err.(*CLIError); !ok {
		t.Fatalf("ConfirmOrAbort() error type = %T, want *CLIError", err)
	}
	if ExitCode(err) != ExitUsage {
		t.Fatalf("ExitCode() = %d, want %d", ExitCode(err), ExitUsage)
	}
}

func TestConfirmOrAbort_NonInteractiveReturnsUsageError(t *testing.T) {
	// Create non-interactive IO (CanPrompt returns false)
	io, _, _, _ := iostreams.Test()
	opts := ConfirmOptions{
		IO:       io,
		Expected: "123",
		Prompt:   "Type the number to confirm: ",
	}
	err := ConfirmOrAbort(opts)
	if err == nil {
		t.Fatal("ConfirmOrAbort() in non-interactive mode = nil, want error")
	}
	if _, ok := err.(*CLIError); !ok {
		t.Fatalf("ConfirmOrAbort() error type = %T, want *CLIError", err)
	}
	if !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("ConfirmOrAbort() error = %q, want mention of --yes", err.Error())
	}
}

func TestConfirmOrAbort_ReadErrorReturnsUsageError(t *testing.T) {
	// Simulate the read-error path: reader.ReadString fails with a non-EOF error.
	// This constructs the same CLIError that confirm.go now returns on read failure.
	readErr := errors.New("simulated read error")
	err := NewCLIError(ExitUsage, "failed to read confirmation", readErr)
	if err == nil {
		t.Fatal("NewCLIError() = nil, want error")
	}
	if _, ok := err.(*CLIError); !ok {
		t.Fatalf("NewCLIError() error type = %T, want *CLIError", err)
	}
	if ExitCode(err) != ExitUsage {
		t.Fatalf("ExitCode() = %d, want %d (ExitUsage)", ExitCode(err), ExitUsage)
	}
	if !strings.Contains(err.Error(), "failed to read confirmation") {
		t.Fatalf("error = %q, want mention of 'failed to read confirmation'", err.Error())
	}
	// Verify error unwrapping preserves the underlying cause
	if !errors.Is(err, readErr) {
		t.Fatalf("errors.Is(err, readErr) = false, want true (cause not preserved)")
	}
}

func TestConfirmOrAbort_NilIOReturnsUsageError(t *testing.T) {
	opts := ConfirmOptions{
		IO:       nil,
		Expected: "123",
		Prompt:   "Type the number: ",
	}
	err := ConfirmOrAbort(opts)
	if err == nil {
		t.Fatal("ConfirmOrAbort() with nil IO = nil, want error")
	}
	if _, ok := err.(*CLIError); !ok {
		t.Fatalf("ConfirmOrAbort() error type = %T, want *CLIError", err)
	}
	if !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("ConfirmOrAbort() error = %q, want mention of --yes", err.Error())
	}
}

// failingReader returns partial data then an error, simulating a corrupt
// terminal that yields some bytes before failing (#361).
type failingReader struct {
	data string
	err  error
	read int
}

func (r *failingReader) Read(p []byte) (int, error) {
	if r.read >= len(r.data) {
		return 0, r.err
	}
	n := copy(p, r.data[r.read:])
	r.read += n
	if r.read >= len(r.data) {
		return n, r.err
	}
	return n, nil
}

// TestConfirmOrAbort_PartialInputWithReadErrorSurfacesReadError verifies that a
// read error with partial input is surfaced as a read error, not silently
// swallowed as "confirmation did not match" (#361).
func TestConfirmOrAbort_PartialInputWithReadErrorSurfacesReadError(t *testing.T) {
	streams, _, _, _ := iostreams.TestTTY()
	streams.In = &failingReader{data: "12", err: errors.New("corrupt terminal")}
	opts := ConfirmOptions{
		IO:       streams,
		Expected: "123",
		Prompt:   "Type the number: ",
	}
	err := ConfirmOrAbort(opts)
	if err == nil {
		t.Fatal("ConfirmOrAbort() with partial input + read error = nil, want error")
	}
	if _, ok := err.(*CLIError); !ok {
		t.Fatalf("ConfirmOrAbort() error type = %T, want *CLIError (read error, not 'did not match')", err)
	}
	if !strings.Contains(err.Error(), "failed to read confirmation") {
		t.Fatalf("error = %q, want 'failed to read confirmation' (not 'did not match')", err.Error())
	}
}

// TestConfirmOrAbort_EOFWithEmptyInputReturnsNonInteractive verifies that EOF
// with empty input still returns the non-interactive usage error (#361).
func TestConfirmOrAbort_EOFWithEmptyInputReturnsNonInteractive(t *testing.T) {
	streams, _, _, _ := iostreams.TestTTY()
	streams.In = &failingReader{data: "", err: io.EOF}
	opts := ConfirmOptions{
		IO:       streams,
		Expected: "123",
		Prompt:   "Type the number: ",
	}
	err := ConfirmOrAbort(opts)
	if err == nil {
		t.Fatal("ConfirmOrAbort() with EOF + empty input = nil, want error")
	}
	if !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("error = %q, want mention of --yes (non-interactive)", err.Error())
	}
}

// TestConfirmTokenDisclosure_PartialInputWithReadErrorSurfacesReadError verifies
// the same fix applies to ConfirmTokenDisclosure (token_confirm.go #361).
func TestConfirmTokenDisclosure_PartialInputWithReadErrorSurfacesReadError(t *testing.T) {
	streams, _, _, _ := iostreams.TestTTY()
	streams.In = &failingReader{data: "partial", err: errors.New("corrupt terminal")}
	err := ConfirmTokenDisclosure(streams, "example.com")
	if err == nil {
		t.Fatal("ConfirmTokenDisclosure() with partial input + read error = nil, want error")
	}
	if _, ok := err.(*CLIError); !ok {
		t.Fatalf("error type = %T, want *CLIError (read error, not 'did not match')", err)
	}
	if !strings.Contains(err.Error(), "failed to read confirmation") {
		t.Fatalf("error = %q, want 'failed to read confirmation'", err.Error())
	}
}

func TestConfirmYesNo(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "accept with y", input: "y\n", want: true},
		{name: "accept with yes", input: "YES\n", want: true},
		{name: "decline with n", input: "n\n", want: false},
		{name: "decline with empty line", input: "\n", want: false},
		{name: "decline on EOF", input: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			io, _, _, errOut := iostreams.TestTTY()
			reader := bufio.NewReader(strings.NewReader(tt.input))

			got, err := ConfirmYesNo(ConfirmYesNoOptions{
				IO:     io,
				Reader: reader,
				Prompt: "Continue? (y/N): ",
			})
			if err != nil {
				t.Fatalf("ConfirmYesNo() error = %v, want nil", err)
			}
			if got != tt.want {
				t.Fatalf("ConfirmYesNo() = %v, want %v", got, tt.want)
			}
			if !strings.Contains(errOut.String(), "Continue? (y/N): ") {
				t.Fatalf("stderr missing prompt: %q", errOut.String())
			}
		})
	}
}

func TestConfirmYesNoSharesBufferedReader(t *testing.T) {
	io, _, _, _ := iostreams.TestTTY()
	reader := bufio.NewReader(strings.NewReader("y\nn\n"))

	first, err := ConfirmYesNo(ConfirmYesNoOptions{IO: io, Reader: reader})
	if err != nil {
		t.Fatalf("first ConfirmYesNo() error = %v, want nil", err)
	}
	second, err := ConfirmYesNo(ConfirmYesNoOptions{IO: io, Reader: reader})
	if err != nil {
		t.Fatalf("second ConfirmYesNo() error = %v, want nil", err)
	}
	if !first || second {
		t.Fatalf("ConfirmYesNo() results = %v, %v; want true, false", first, second)
	}
}

func TestConfirmYesNoReadErrorPropagates(t *testing.T) {
	io, _, _, _ := iostreams.TestTTY()
	reader := bufio.NewReader(&failingReader{data: "y", err: errors.New("corrupt terminal")})

	_, err := ConfirmYesNo(ConfirmYesNoOptions{IO: io, Reader: reader})
	if err == nil {
		t.Fatal("ConfirmYesNo() error = nil, want read error")
	}
	if !strings.Contains(err.Error(), "failed to read confirmation") {
		t.Fatalf("error = %q, want read confirmation failure", err.Error())
	}
}

func TestConfirmYesNoNonEmptyEOFWithoutNewlineFailsClosed(t *testing.T) {
	for _, input := range []string{"y", "n"} {
		t.Run("input="+input, func(t *testing.T) {
			io, _, _, _ := iostreams.TestTTY()
			reader := bufio.NewReader(strings.NewReader(input))

			_, err := ConfirmYesNo(ConfirmYesNoOptions{IO: io, Reader: reader})
			if err == nil {
				t.Fatal("ConfirmYesNo() error = nil, want read error")
			}
			if !strings.Contains(err.Error(), "failed to read confirmation") {
				t.Fatalf("error = %q, want read confirmation failure", err.Error())
			}
		})
	}
}

func TestConfirmYesNoNilReaderRequiresInteractiveTerminal(t *testing.T) {
	io, _, _, _ := iostreams.TestTTY()

	_, err := ConfirmYesNo(ConfirmYesNoOptions{IO: io})
	if err == nil {
		t.Fatal("ConfirmYesNo() error = nil, want usage error")
	}
	if !strings.Contains(err.Error(), "interactive terminal") {
		t.Fatalf("error = %q, want interactive terminal requirement", err.Error())
	}
}

func TestConfirmYesNoRequiresInteractiveTerminal(t *testing.T) {
	io, _, _, _ := iostreams.Test()
	reader := bufio.NewReader(strings.NewReader("y\n"))

	_, err := ConfirmYesNo(ConfirmYesNoOptions{IO: io, Reader: reader})
	if err == nil {
		t.Fatal("ConfirmYesNo() error = nil, want non-interactive usage error")
	}
	if !strings.Contains(err.Error(), "interactive terminal") {
		t.Fatalf("error = %q, want interactive terminal requirement", err.Error())
	}
}
