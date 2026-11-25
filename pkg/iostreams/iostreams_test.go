package iostreams

import (
	"bytes"
	"os"
	"testing"
)

func TestSplitCommandLine(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{
			name:  "simple",
			input: "less -R",
			want:  []string{"less", "-R"},
		},
		{
			name:  "double quotes",
			input: `delta --syntax-theme="GitHub Dark"`,
			want:  []string{"delta", "--syntax-theme=GitHub Dark"},
		},
		{
			name:  "single quotes",
			input: `pager --title 'hello world'`,
			want:  []string{"pager", "--title", "hello world"},
		},
		{
			name:  "escaped spaces",
			input: `pager /tmp/with\ spaces/file.txt`,
			want:  []string{"pager", "/tmp/with spaces/file.txt"},
		},
		{
			name:    "unterminated quote",
			input:   `pager "oops`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := splitCommandLine(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("splitCommandLine() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("splitCommandLine() = %#v, want %#v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("splitCommandLine() = %#v, want %#v", got, tt.want)
				}
			}
		})
	}
}

func TestCanPrompt(t *testing.T) {
	t.Run("default Test() returns false", func(t *testing.T) {
		io, _, _, _ := Test()
		if io.CanPrompt() {
			t.Fatal("Test() IOStreams should not be promptable")
		}
	})

	t.Run("TestTTY returns true by default", func(t *testing.T) {
		io, _, _, _ := TestTTY()
		if !io.CanPrompt() {
			t.Fatal("TestTTY() IOStreams should be promptable by default")
		}
	})

	t.Run("SetNoInteractive disables prompting", func(t *testing.T) {
		io, _, _, _ := TestTTY()
		io.SetNoInteractive(true)
		if io.CanPrompt() {
			t.Fatal("CanPrompt() should return false after SetNoInteractive(true)")
		}
	})

	t.Run("SetNoInteractive false restores prompting", func(t *testing.T) {
		io, _, _, _ := TestTTY()
		io.SetNoInteractive(true)
		io.SetNoInteractive(false)
		if !io.CanPrompt() {
			t.Fatal("CanPrompt() should return true after SetNoInteractive(false)")
		}
	})
}

func TestCanPromptNilSafe(t *testing.T) {
	var s *IOStreams
	if s.CanPrompt() {
		t.Fatal("nil IOStreams CanPrompt() = true, want false")
	}
}

func TestIsInputTerminal(t *testing.T) {
	t.Run("/dev/null is not a terminal", func(t *testing.T) {
		f, err := os.Open(os.DevNull)
		if err != nil {
			t.Skipf("cannot open /dev/null: %v", err)
		}
		t.Cleanup(func() { f.Close() })
		if isInputTerminal(f) {
			t.Fatal("isInputTerminal(/dev/null) = true, want false (/dev/null is a char device but not a terminal)")
		}
	})

	t.Run("regular file is not a terminal", func(t *testing.T) {
		f, err := os.CreateTemp(t.TempDir(), "stdin")
		if err != nil {
			t.Fatalf("temp file: %v", err)
		}
		t.Cleanup(func() { f.Close() })
		if isInputTerminal(f) {
			t.Fatal("isInputTerminal(regular file) = true, want false")
		}
	})

	t.Run("pipe is not a terminal", func(t *testing.T) {
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatalf("pipe: %v", err)
		}
		t.Cleanup(func() { r.Close(); w.Close() })
		if isInputTerminal(r) {
			t.Fatal("isInputTerminal(pipe) = true, want false")
		}
	})

	t.Run("non-file reader is not a terminal", func(t *testing.T) {
		if isInputTerminal(bytes.NewReader(nil)) {
			t.Fatal("isInputTerminal(bytes.Reader) = true, want false")
		}
	})
}

func TestIsInputTTYDevNull(t *testing.T) {
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Skipf("cannot open /dev/null: %v", err)
	}
	t.Cleanup(func() { f.Close() })

	oldStdin := os.Stdin
	os.Stdin = f
	t.Cleanup(func() { os.Stdin = oldStdin })

	if IsInputTTY() {
		t.Fatal("IsInputTTY() with /dev/null stdin = true, want false")
	}
}
