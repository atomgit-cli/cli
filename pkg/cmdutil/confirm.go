package cmdutil

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
)

// ConfirmOptions controls destructive-action confirmation prompts.
type ConfirmOptions struct {
	IO       *iostreams.IOStreams
	Yes      bool
	Expected string
	Prompt   string
}

// ConfirmOrAbort enforces a shared confirmation flow.
func ConfirmOrAbort(opts ConfirmOptions) error {
	if opts.Yes {
		return nil
	}
	if opts.Expected == "" {
		return NewUsageError("missing confirmation target")
	}
	if opts.IO == nil || !opts.IO.CanPrompt() {
		return NewUsageError("confirmation required in non-interactive mode; rerun with --yes")
	}

	if opts.Prompt != "" {
		fmt.Fprint(opts.IO.ErrOut, opts.Prompt)
	}
	reader := bufio.NewReader(opts.IO.In)
	input, err := reader.ReadString('\n')
	if err != nil {
		// Surface read errors even when partial input was received (e.g. corrupt
		// terminal), instead of falling through to a misleading "did not match".
		// EOF with empty input signals a non-interactive context (#361).
		if err == io.EOF && strings.TrimSpace(input) == "" {
			return NewUsageError("confirmation required in non-interactive mode; rerun with --yes")
		}
		return NewCLIError(ExitUsage, "failed to read confirmation", err)
	}
	if strings.TrimSpace(input) != opts.Expected {
		return NewUsageError("confirmation did not match expected value")
	}
	return nil
}

// ConfirmYesNoOptions controls a shared y/N confirmation prompt.
type ConfirmYesNoOptions struct {
	IO     *iostreams.IOStreams
	Reader *bufio.Reader
	Prompt string
}

// ConfirmYesNo asks a y/N question and returns true only for y or yes.
func ConfirmYesNo(opts ConfirmYesNoOptions) (bool, error) {
	if opts.IO == nil || opts.Reader == nil || !opts.IO.CanPrompt() {
		return false, NewUsageError("yes/no confirmation requires an interactive terminal")
	}
	if opts.Prompt != "" {
		fmt.Fprint(opts.IO.ErrOut, opts.Prompt)
	}

	input, err := opts.Reader.ReadString('\n')
	if err != nil {
		if err == io.EOF && strings.TrimSpace(input) == "" {
			return false, nil
		}
		return false, NewCLIError(ExitUsage, "failed to read confirmation", err)
	}

	switch strings.ToLower(strings.TrimSpace(input)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}
