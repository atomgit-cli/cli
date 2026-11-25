package add

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
)

func TestNewCmdAddRequiresFlags(t *testing.T) {
	t.Parallel()
	cmd := NewCmdAdd(cmdutil.TestFactory(), func(*AddOptions) error { return nil })
	cmd.SetArgs(nil)
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected required flag error")
	}
}

func TestAddRunReadsFileAndWritesJSON(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	file := t.TempDir() + "/key.pub"
	if err := os.WriteFile(file, []byte("ssh-ed25519 QUFBQQ== test@example\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	f := cmdutil.TestFactory()
	out := &strings.Builder{}
	f.IOStreams.Out = out
	opts := &AddOptions{IO: f.IOStreams, Title: "laptop", KeyFile: file, JSON: true, HttpClient: func() (*http.Client, error) {
		return &http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			body, _ := io.ReadAll(req.Body)
			var got map[string]string
			if err := json.Unmarshal(body, &got); err != nil {
				t.Fatal(err)
			}
			if got["title"] != "laptop" || got["key"] != "ssh-ed25519 QUFBQQ== test@example" {
				t.Fatalf("body = %s", body)
			}
			return addResponse(http.StatusOK, "{\"id\":8,\"title\":\"laptop\",\"key\":\"ssh-ed25519 QUFBQQ== test@example\"}"), nil
		})}, nil
	}}
	if err := addRun(opts); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "\"id\": 8") {
		t.Fatalf("output = %s", out)
	}
}

func TestAddRunSanitizesTextOutput(t *testing.T) {
	t.Setenv("GC_TOKEN", "test-token")
	file := t.TempDir() + "/key.pub"
	if err := os.WriteFile(file, []byte("ssh-ed25519 QUFBQQ== comment"), 0o600); err != nil {
		t.Fatal(err)
	}
	f := cmdutil.TestFactory()
	out := &strings.Builder{}
	f.IOStreams.Out = out
	opts := &AddOptions{IO: f.IOStreams, Title: "laptop", KeyFile: file, HttpClient: func() (*http.Client, error) {
		return &http.Client{Transport: testutil.NewRoundTripFunc(func(*http.Request) (*http.Response, error) {
			return addResponse(http.StatusOK, `{"id":8,"title":"lap\u001b[31mtop"}`), nil
		})}, nil
	}}
	if err := addRun(opts); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "\x1b") || out.String() != "Added SSH key lap[31mtop (ID: 8).\n" {
		t.Fatalf("output = %q", out)
	}
}

func TestValidatePublicKey(t *testing.T) {
	t.Parallel()
	privateKeyMarker := strings.Join([]string{"-----BEGIN OPENSSH PRIVATE", "KEY-----"}, " ")
	tests := []struct {
		name    string
		key     string
		wantErr string
	}{
		{name: "ed25519", key: "ssh-ed25519 QUFBQQ== comment"},
		{name: "ecdsa", key: "ecdsa-sha2-nistp256 QUFBQQ=="},
		{name: "private", key: privateKeyMarker, wantErr: "not a private key"},
		{name: "multiline", key: "ssh-ed25519 QUFBQQ==\nsecond", wantErr: "single OpenSSH"},
		{name: "arbitrary", key: "not-a-key", wantErr: "single OpenSSH"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validatePublicKey(tt.key)
			if tt.wantErr == "" && err != nil {
				t.Fatalf("error = %v", err)
			}
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Fatalf("error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestAddRunRejectsEmptyFile(t *testing.T) {
	t.Parallel()
	file := t.TempDir() + "/empty.pub"
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	f := cmdutil.TestFactory()
	err := addRun(&AddOptions{IO: f.IOStreams, Title: "empty", KeyFile: file})
	if err == nil || !strings.Contains(err.Error(), "file is empty") {
		t.Fatalf("error = %v", err)
	}
}

func TestAddRunRejectsPrivateKey(t *testing.T) {
	t.Parallel()
	file := t.TempDir() + "/invalid-key"
	privateKeyMarker := strings.Join([]string{"PRIVATE", "KEY"}, " ") + " material"
	if err := os.WriteFile(file, []byte(privateKeyMarker), 0o600); err != nil {
		t.Fatal(err)
	}
	err := addRun(&AddOptions{IO: cmdutil.TestFactory().IOStreams, Title: "private", KeyFile: file})
	if err == nil || !strings.Contains(err.Error(), "not a private key") {
		t.Fatalf("error = %v", err)
	}
}

func TestAddRunRejectsMultipleLines(t *testing.T) {
	t.Parallel()
	file := t.TempDir() + "/keys.pub"
	content := "ssh-ed25519 AAAA first\nssh-ed25519 BBBB second"
	if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	err := addRun(&AddOptions{IO: cmdutil.TestFactory().IOStreams, Title: "multiple", KeyFile: file})
	if err == nil || !strings.Contains(err.Error(), "exactly one line") {
		t.Fatalf("error = %v", err)
	}
}

func addResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Status: http.StatusText(status), Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}
