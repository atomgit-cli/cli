package gitcredential

import (
	"errors"
	"io"
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/config"
)

type failingIO struct{}

func (failingIO) Read([]byte) (int, error)  { return 0, errors.New("private reader details") }
func (failingIO) Write([]byte) (int, error) { return 0, errors.New("private writer details") }

func TestReadRequest(t *testing.T) {
	for _, input := range []string{"=value\n", "host=gitcode.com\x00\n", "protocol=https\nprotocol=http\n", strings.Repeat("extension="+strings.Repeat("a", 1000)+"\n", 70)} {
		if _, err := readRequest(strings.NewReader(input)); err == nil {
			t.Fatal("invalid request accepted")
		}
	}
	if _, err := readRequest(failingIO{}); err == nil || strings.Contains(err.Error(), "private") {
		t.Fatal("reader error not safely handled")
	}
	fields, err := readRequest(strings.NewReader("protocol=https\r\nhost=gitcode.com\r\nwwwauth[]=one\r\nwwwauth[]=two\r\n\r\nhost=evil.example\n"))
	if err != nil || fields["host"] != "gitcode.com" {
		t.Fatal("CRLF, extensions, or terminator mishandled")
	}
}

func TestCredentialFailuresAndFallback(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_TOKEN", "")
	t.Setenv("GITCODE_TOKEN", "")
	f := cmdutil.TestFactory()
	out := &strings.Builder{}
	f.IOStreams.Out = out
	cfg := config.New()
	f.Config = func() (config.Config, error) { return cfg, nil }
	run := func() error {
		f.IOStreams.In = strings.NewReader("protocol=https\nhost=gitcode.com\n\n")
		return credentialRun(f)
	}
	if err := run(); err != nil || out.Len() != 0 {
		t.Fatal("missing login must be silent")
	}
	t.Setenv("GITCODE_TOKEN", "fallback-value")
	if err := run(); err != nil || out.String() != "username=oauth2\npassword=fallback-value\n\n" {
		t.Fatal("environment-only fallback failed")
	}
	out.Reset()
	f.IOStreams.Out = failingIO{}
	if err := run(); err == nil || strings.Contains(err.Error(), "private") {
		t.Fatal("writer error not safely handled")
	}
	f.IOStreams.Out = io.Discard
	f.Config = func() (config.Config, error) { return nil, errors.New("private config details") }
	if err := run(); err == nil || strings.Contains(err.Error(), "private") {
		t.Fatal("config error not safely handled")
	}
}

func TestReadRequestCountsCRLFBytes(t *testing.T) {
	// Each line is 4 wire bytes but ScanLines returns only 2 text bytes.
	input := strings.Repeat("x=\r\n", maxRequestBytes/4+1)
	if _, err := readRequest(strings.NewReader(input)); err == nil {
		t.Fatal("accepted more than 64 KiB of CRLF input")
	}
}
