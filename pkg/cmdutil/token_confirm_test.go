package cmdutil

import (
	"errors"
	"io"
	"strings"
	"testing"

	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
)

// eofReader yields no data and reports io.EOF, simulating a closed stdin.
type eofReader struct{}

func (r *eofReader) Read(p []byte) (int, error) {
	return 0, io.EOF
}

// TestConfirmTokenDisclosureNilIORejects verifies a nil IOStreams is rejected
// with a usage error instead of panicking.
func TestConfirmTokenDisclosureNilIORejects(t *testing.T) {
	err := ConfirmTokenDisclosure(nil, "example.com")
	if err == nil {
		t.Fatal("ConfirmTokenDisclosure(nil, ...) = nil, want usage error")
	}
	if !strings.Contains(err.Error(), "requires interactive confirmation") {
		t.Fatalf("error = %q, want 'requires interactive confirmation'", err.Error())
	}
	if ExitCode(err) != ExitUsage {
		t.Fatalf("ExitCode() = %d, want %d", ExitCode(err), ExitUsage)
	}
}

// TestConfirmTokenDisclosureNonInteractiveRejects verifies a non-promptable
// stream is rejected with a usage error.
func TestConfirmTokenDisclosureNonInteractiveRejects(t *testing.T) {
	streams, _, _, _ := iostreams.Test()

	err := ConfirmTokenDisclosure(streams, "example.com")
	if err == nil {
		t.Fatal("ConfirmTokenDisclosure(non-interactive) = nil, want usage error")
	}
	if !strings.Contains(err.Error(), "requires interactive confirmation") {
		t.Fatalf("error = %q, want 'requires interactive confirmation'", err.Error())
	}
}

// TestConfirmTokenDisclosureEOFWithEmptyInput verifies EOF without input is
// treated as a non-interactive abort.
func TestConfirmTokenDisclosureEOFWithEmptyInput(t *testing.T) {
	streams, _, _, _ := iostreams.TestTTY()
	streams.In = &eofReader{}

	err := ConfirmTokenDisclosure(streams, "example.com")
	if err == nil {
		t.Fatal("ConfirmTokenDisclosure(EOF, empty input) = nil, want usage error")
	}
	if !strings.Contains(err.Error(), "requires interactive confirmation") {
		t.Fatalf("error = %q, want 'requires interactive confirmation'", err.Error())
	}
}

// TestConfirmTokenDisclosureMismatchRejects verifies a non-matching input is
// rejected.
func TestConfirmTokenDisclosureMismatchRejects(t *testing.T) {
	streams, _, _, _ := iostreams.TestTTY()
	streams.In = strings.NewReader("not-the-host\n")

	err := ConfirmTokenDisclosure(streams, "example.com")
	if err == nil {
		t.Fatal("ConfirmTokenDisclosure(mismatch) = nil, want usage error")
	}
	if !strings.Contains(err.Error(), "confirmation did not match") {
		t.Fatalf("error = %q, want 'confirmation did not match'", err.Error())
	}
	if ExitCode(err) != ExitUsage {
		t.Fatalf("ExitCode() = %d, want %d", ExitCode(err), ExitUsage)
	}
}

// TestConfirmTokenDisclosureExactMatchSucceeds verifies typing the exact
// hostname confirms the disclosure.
func TestConfirmTokenDisclosureExactMatchSucceeds(t *testing.T) {
	streams, _, _, _ := iostreams.TestTTY()
	streams.In = strings.NewReader("example.com\n")

	if err := ConfirmTokenDisclosure(streams, "example.com"); err != nil {
		t.Fatalf("ConfirmTokenDisclosure(match) = %v, want nil", err)
	}
}

// TestConfirmTokenDisclosureSurroundingWhitespaceStillMatches verifies
// whitespace-trimmed input still counts as a match.
func TestConfirmTokenDisclosureSurroundingWhitespaceStillMatches(t *testing.T) {
	streams, _, _, _ := iostreams.TestTTY()
	streams.In = strings.NewReader("  example.com  \n")

	if err := ConfirmTokenDisclosure(streams, "example.com"); err != nil {
		t.Fatalf("ConfirmTokenDisclosure(whitespace match) = %v, want nil", err)
	}
}

// TestConfirmTokenDisclosureReadErrorSurfaces verifies a non-EOF read failure
// is surfaced as a read error rather than a misleading mismatch.
func TestConfirmTokenDisclosureReadErrorSurfaces(t *testing.T) {
	streams, _, _, _ := iostreams.TestTTY()
	streams.In = &failingReader{data: "", err: errors.New("corrupt terminal")}

	err := ConfirmTokenDisclosure(streams, "example.com")
	if err == nil {
		t.Fatal("ConfirmTokenDisclosure(read error) = nil, want error")
	}
	if !strings.Contains(err.Error(), "failed to read confirmation") {
		t.Fatalf("error = %q, want 'failed to read confirmation'", err.Error())
	}
	if ExitCode(err) != ExitUsage {
		t.Fatalf("ExitCode() = %d, want %d", ExitCode(err), ExitUsage)
	}
}
