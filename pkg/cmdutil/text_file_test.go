package cmdutil

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadTextFileStripsUTF8BOM(t *testing.T) {
	path := filepath.Join(t.TempDir(), "body.md")
	if err := os.WriteFile(path, []byte{0xef, 0xbb, 0xbf, 'b', 'o', 'd', 'y'}, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := ReadTextFile(path)
	if err != nil {
		t.Fatalf("ReadTextFile() error = %v", err)
	}
	if got != "body" {
		t.Fatalf("ReadTextFile() = %q, want %q", got, "body")
	}
}

func TestReadTextFileKeepsNonBOMContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "body.md")
	if err := os.WriteFile(path, []byte("plain body"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := ReadTextFile(path)
	if err != nil {
		t.Fatalf("ReadTextFile() error = %v", err)
	}
	if got != "plain body" {
		t.Fatalf("ReadTextFile() = %q, want %q", got, "plain body")
	}
}

func TestDecodeUserTextStripsUTF16LEBOM(t *testing.T) {
	got := DecodeUserText([]byte{0xff, 0xfe, 0x60, 0x4f, 0x7d, 0x59})
	if got != "你好" {
		t.Fatalf("DecodeUserText() = %q, want %q", got, "你好")
	}
}

func TestReadTextUsesDecodeUserText(t *testing.T) {
	got, err := ReadText(strings.NewReader("\xef\xbb\xbfbody"))
	if err != nil {
		t.Fatalf("ReadText() error = %v", err)
	}
	if got != "body" {
		t.Fatalf("ReadText() = %q, want %q", got, "body")
	}
}

func TestReadTextDoesNotApplyLossyPowerShellGuard(t *testing.T) {
	got, err := ReadText(strings.NewReader("Windows????-20260526"))
	if err != nil {
		t.Fatalf("ReadText() error = %v", err)
	}
	if got != "Windows????-20260526" {
		t.Fatalf("ReadText() = %q", got)
	}
}

func TestDecodeUserTextFallsBackToGB18030(t *testing.T) {
	got := DecodeUserText([]byte{0xc4, 0xe3, 0xba, 0xc3})
	if got != "你好" {
		t.Fatalf("DecodeUserText() = %q, want %q", got, "你好")
	}
}

func TestIsLikelyLossyPowerShellStdin(t *testing.T) {
	if !isLikelyLossyPowerShellStdin([]byte("Windows????-20260526"), "Windows????-20260526", "windows", false) {
		t.Fatal("expected lossy PowerShell stdin to be detected")
	}
}

func TestIsLikelyLossyPowerShellStdinCanBeBypassed(t *testing.T) {
	if isLikelyLossyPowerShellStdin([]byte("Windows????-20260526"), "Windows????-20260526", "windows", true) {
		t.Fatal("expected explicit lossy stdin bypass to be honored")
	}
}

func TestIsLikelyLossyPowerShellStdinIgnoresUTF8Text(t *testing.T) {
	raw := []byte("Windows中文正文")
	if isLikelyLossyPowerShellStdin(raw, string(raw), "windows", false) {
		t.Fatal("expected valid UTF-8 stdin text to be accepted")
	}
}

func TestIsLikelyLossyPowerShellStdinNonWindows(t *testing.T) {
	if isLikelyLossyPowerShellStdin([]byte("Linux????-20260526"), "Linux????-20260526", "linux", false) {
		t.Fatal("expected non-Windows stdin to be accepted")
	}
}

func TestReadBodyReturnsEmptyWhenNeitherSet(t *testing.T) {
	got, err := ReadBody("", "", nil)
	if err != nil {
		t.Fatalf("ReadBody() error = %v", err)
	}
	if got != "" {
		t.Fatalf("ReadBody() = %q, want empty", got)
	}
}

func TestReadBodyReturnsBodyWhenSet(t *testing.T) {
	got, err := ReadBody("hello world", "", nil)
	if err != nil {
		t.Fatalf("ReadBody() error = %v", err)
	}
	if got != "hello world" {
		t.Fatalf("ReadBody() = %q, want %q", got, "hello world")
	}
}

func TestReadBodyReturnsErrorWhenBothSet(t *testing.T) {
	_, err := ReadBody("body", "file.md", nil)
	if err == nil {
		t.Fatal("ReadBody() expected error when both body and bodyFile set, got nil")
	}
}

func TestReadBodyReadsFromStdinWhenBodyFileIsDash(t *testing.T) {
	got, err := ReadBody("", "-", strings.NewReader("  from stdin\n"))
	if err != nil {
		t.Fatalf("ReadBody() error = %v", err)
	}
	if got != "from stdin" {
		t.Fatalf("ReadBody() = %q, want %q", got, "from stdin")
	}
}

func TestReadBodyReadsFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "body.md")
	if err := os.WriteFile(path, []byte("  file content\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := ReadBody("", path, nil)
	if err != nil {
		t.Fatalf("ReadBody() error = %v", err)
	}
	if got != "file content" {
		t.Fatalf("ReadBody() = %q, want %q", got, "file content")
	}
}

func TestReadBodyReturnsErrorOnMissingFile(t *testing.T) {
	_, err := ReadBody("", "/nonexistent/path/body.md", nil)
	if err == nil {
		t.Fatal("ReadBody() expected error for nonexistent file, got nil")
	}
	if !strings.Contains(err.Error(), "failed to read file") {
		t.Fatalf("ReadBody() error = %q, want to contain %q", err.Error(), "failed to read file")
	}
}

func TestNewLossyPowerShellStdinErrorIncludesFlagAndExamples(t *testing.T) {
	err := newLossyPowerShellStdinError("--comment-file")
	if !errors.Is(err, ErrLossyPowerShellStdin) {
		t.Fatalf("error does not wrap ErrLossyPowerShellStdin: %v", err)
	}
	text := err.Error()
	for _, want := range []string{
		"--comment-file -",
		"Set-Content -Path comment.md",
		"gitcode pr review 1 -R owner/repo --comment-file",
		"GITCODE_CLI_ALLOW_LOSSY_STDIN=1",
		"正确用法",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("error %q does not contain %q", text, want)
		}
	}
}

func TestReadBodyRejectsInlineBodyContainingToken(t *testing.T) {
	t.Setenv("GC_TOKEN", "secret-token-abc123")
	_, err := ReadBody("this leaks secret-token-abc123", "", nil)
	if !errors.Is(err, ErrSecretDetected) {
		t.Fatalf("ReadBody() error = %v, want ErrSecretDetected", err)
	}
}

func TestReadBodyRejectsFileBodyContainingToken(t *testing.T) {
	t.Setenv("GC_TOKEN", "secret-token-abc123")
	path := filepath.Join(t.TempDir(), "body.md")
	if err := os.WriteFile(path, []byte("leaked: secret-token-abc123"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	_, err := ReadBody("", path, nil)
	if !errors.Is(err, ErrSecretDetected) {
		t.Fatalf("ReadBody() error = %v, want ErrSecretDetected", err)
	}
}

func TestReadTextRejectsContentContainingToken(t *testing.T) {
	t.Setenv("GC_TOKEN", "secret-token-abc123")
	_, err := ReadText(strings.NewReader("contains secret-token-abc123"))
	if !errors.Is(err, ErrSecretDetected) {
		t.Fatalf("ReadText() error = %v, want ErrSecretDetected", err)
	}
}

func TestReadTextFileRejectsContentContainingToken(t *testing.T) {
	t.Setenv("GC_TOKEN", "secret-token-abc123")
	path := filepath.Join(t.TempDir(), "body.md")
	if err := os.WriteFile(path, []byte("leaked: secret-token-abc123"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	_, err := ReadTextFile(path)
	if !errors.Is(err, ErrSecretDetected) {
		t.Fatalf("ReadTextFile() error = %v, want ErrSecretDetected", err)
	}
}

func TestReadTextFromFlagRejectsContentContainingToken(t *testing.T) {
	t.Setenv("GC_TOKEN", "secret-token-abc123")
	_, err := ReadTextFromFlag(strings.NewReader("contains secret-token-abc123"), "--body-file")
	if !errors.Is(err, ErrSecretDetected) {
		t.Fatalf("ReadTextFromFlag() error = %v, want ErrSecretDetected", err)
	}
}

func TestNormalizeLabels(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{name: "nil", in: nil, want: []string{}},
		{name: "empty", in: []string{}, want: []string{}},
		{name: "comma split", in: []string{"bug,enhancement"}, want: []string{"bug", "enhancement"}},
		{name: "already split", in: []string{"bug", "enhancement"}, want: []string{"bug", "enhancement"}},
		{name: "trim spaces", in: []string{" bug , enhancement "}, want: []string{"bug", "enhancement"}},
		{name: "drop empty", in: []string{"bug,,", ",enhancement,"}, want: []string{"bug", "enhancement"}},
		{name: "dedup", in: []string{"bug,bug,enhancement"}, want: []string{"bug", "enhancement"}},
		{name: "dedup across elements", in: []string{"bug", "bug"}, want: []string{"bug"}},
		{name: "preserves first-seen order", in: []string{"enhancement,bug"}, want: []string{"enhancement", "bug"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeLabels(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("NormalizeLabels(%v) = %v, want %v", tt.in, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("NormalizeLabels(%v)[%d] = %q, want %q", tt.in, i, got[i], tt.want[i])
				}
			}
		})
	}
}

// TestReadTextFromFlagSurfacesReadError verifies an io.ReadAll failure is
// returned as-is instead of being swallowed.
func TestReadTextFromFlagSurfacesReadError(t *testing.T) {
	readErr := errors.New("simulated stdin failure")
	_, err := ReadTextFromFlag(&failingReader{data: "", err: readErr}, "--body-file")
	if err == nil {
		t.Fatal("ReadTextFromFlag() error = nil, want read error")
	}
	if !errors.Is(err, readErr) {
		t.Fatalf("ReadTextFromFlag() error = %v, want the underlying read error", err)
	}
}
