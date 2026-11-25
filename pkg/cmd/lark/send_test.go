package lark

import (
	"errors"
	"os"
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
	"gitcode.com/gitcode-cli/cli/pkg/larkcli"
)

// stubRun builds a RunFunc returning a fixed result regardless of args.
func stubRun(stdout, stderr string, exit int) larkcli.RunFunc {
	return func(args []string) (*larkcli.Result, error) {
		return &larkcli.Result{Stdout: []byte(stdout), Stderr: []byte(stderr), ExitCode: exit}, nil
	}
}

func runSend(t *testing.T, opts *sendOptions) (string, string, error) {
	t.Helper()
	io, _, out, _ := iostreams.Test()
	opts.IO = io
	err := sendRun(opts)
	return out.String(), "", err
}

func isUsageError(err error) bool {
	var ce *cmdutil.CLIError
	return errors.As(err, &ce) && ce.Code == cmdutil.ExitUsage
}

func contains(t *testing.T, s, want string, msg string) {
	t.Helper()
	if !strings.Contains(s, want) {
		t.Errorf("%s: %q does not contain %q", msg, s, want)
	}
}

func TestSendRun_NoContentIsUsageError(t *testing.T) {
	t.Parallel()
	if err := sendRun(&sendOptions{ChatID: "oc_x"}); !isUsageError(err) {
		t.Fatalf("err = %v, want usage error", err)
	}
}

func TestSendRun_MultipleContentIsUsageError(t *testing.T) {
	t.Parallel()
	err := sendRun(&sendOptions{ChatID: "oc_x", Text: "a", Markdown: "b"})
	if !isUsageError(err) {
		t.Fatalf("err = %v, want usage error", err)
	}
}

func TestSendRun_NoChatIDIsUsageError(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_LARK_DEFAULT_CHAT_ID", "")
	if err := sendRun(&sendOptions{Text: "hi"}); !isUsageError(err) {
		t.Fatalf("err = %v, want usage error", err)
	}
}

func TestSendRun_EnvDefaultChatResolves(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_LARK_DEFAULT_CHAT_ID", "oc_env")
	_, _, err := runSend(t, &sendOptions{
		Text:    "hello",
		DryRun:  true,
		LarkRun: stubRun(`{"ok":true,"identity":"user","data":{}}`, "", 0),
	})
	if err != nil {
		t.Fatalf("sendRun err = %v", err)
	}
}

func TestSendRun_DryRunJSON(t *testing.T) {
	t.Parallel()
	out, _, err := runSend(t, &sendOptions{
		ChatID: "oc_x", Text: "hello world", DryRun: true, JSON: true,
		LarkRun: stubRun("", "", 0),
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	contains(t, out, `"target": "chat oc_x"`, "json target")
	contains(t, out, `"preview": "hello world"`, "json preview")
	contains(t, out, "--text", "json cmd")
	contains(t, out, "hello world", "json cmd")
	contains(t, out, "--json", "json cmd")
}

func TestSendRun_DryRunTextQuotesSpaces(t *testing.T) {
	t.Parallel()
	out, _, err := runSend(t, &sendOptions{
		ChatID: "oc_x", Text: "hello world", DryRun: true,
		LarkRun: stubRun("", "", 0),
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	contains(t, out, `--text "hello world" --json`, "dry-run cmd")
}

func TestSendRun_SuccessFriendly(t *testing.T) {
	t.Parallel()
	out, _, err := runSend(t, &sendOptions{
		ChatID: "oc_x", Text: "hi",
		LarkRun: stubRun(`{"ok":true,"identity":"user","data":{"message_id":"om_1"}}`, "", 0),
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	contains(t, out, "✓ Sent to chat oc_x", "friendly")
}

func TestSendRun_SuccessJSON(t *testing.T) {
	t.Parallel()
	out, _, err := runSend(t, &sendOptions{
		ChatID: "oc_x", Text: "hi", JSON: true,
		LarkRun: stubRun(`{"ok":true,"identity":"user","data":{"message_id":"om_1"}}`, "", 0),
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	contains(t, out, `"ok": true`, "json ok")
	contains(t, out, `"chat_id": "oc_x"`, "json chat (fallback to target)")
}

func TestSendRun_LarkCLINonZeroExit(t *testing.T) {
	t.Parallel()
	_, _, err := runSend(t, &sendOptions{
		ChatID: "oc_x", Text: "hi",
		LarkRun: stubRun("", `{"ok":false,"error":{"message":"chat not found"}}`, 1),
	})
	if err == nil {
		t.Fatal("err = nil, want error")
	}
	contains(t, err.Error(), "chat not found", "err msg")
	contains(t, err.Error(), "exit 1", "err exit")
}

func TestSendRun_SecretScanBlocksSend(t *testing.T) {
	t.Setenv("GC_TOKEN", "secretvalue123")
	_, _, err := runSend(t, &sendOptions{
		ChatID: "oc_x", Text: "leaking secretvalue123 here",
		LarkRun: stubRun(`{"ok":true,"identity":"user"}`, "", 0),
	})
	if err == nil {
		t.Fatal("err = nil, want secret scan block")
	}
	contains(t, err.Error(), "secret", "secret err")
}

func TestSendRun_BodyFileStdin(t *testing.T) {
	t.Parallel()
	io, _, out, _ := iostreams.Test()
	io.In = strings.NewReader("from stdin body")
	opts := &sendOptions{
		ChatID: "oc_x", BodyFile: "-", DryRun: true,
		LarkRun: stubRun("", "", 0), IO: io,
	}
	if err := sendRun(opts); err != nil {
		t.Fatalf("err = %v", err)
	}
	contains(t, out.String(), "from stdin body", "stdin body")
}

func TestSendRun_MarkdownContent(t *testing.T) {
	t.Parallel()
	out, _, err := runSend(t, &sendOptions{
		ChatID: "oc_x", Markdown: "## H", DryRun: true, JSON: true,
		LarkRun: stubRun("", "", 0),
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	contains(t, out, "--markdown", "markdown cmd")
	contains(t, out, "## H", "markdown cmd")
}

func TestShellQuote(t *testing.T) {
	t.Parallel()
	tests := []struct{ in, want string }{
		{"plain", "plain"},
		{"", `""`},
		{"with space", `"with space"`},
		{`with"quote`, `"with\"quote"`},
		{`back\slash`, `"back\\slash"`},
	}
	for _, tt := range tests {
		tt := tt
		if got := shellQuote(tt.in); got != tt.want {
			t.Errorf("shellQuote(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestSendRun_UserIDTarget(t *testing.T) {
	t.Parallel()
	out, _, err := runSend(t, &sendOptions{
		UserID: "ou_user1", Text: "hi", DryRun: true,
		LarkRun: stubRun("", "", 0),
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	contains(t, out, "--user-id ou_user1", "user-id arg in dry-run")
	// Friendly target label only appears on real send success, not dry-run.
}

func TestSendRun_ToSelfResolvesOpenID(t *testing.T) {
	t.Parallel()
	// The LarkRun stub returns an auth-status payload (with openId) for the
	// auth status call, and a send-success envelope for the send call. Both
	// are returned regardless of args, which is fine for this unit test.
	authStatus := `{"ok":true,"identities":{"user":{"openId":"ou_self123"}}}`
	out, _, err := runSend(t, &sendOptions{
		ToSelf: true, Text: "hi", DryRun: true,
		LarkRun: stubRun(authStatus, "", 0),
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	contains(t, out, "--user-id ou_self123", "resolved open_id")
	contains(t, out, "--as bot", "to-self defaults to bot identity")
}

func TestSendRun_ToSelfMissingOpenID(t *testing.T) {
	t.Parallel()
	_, _, err := runSend(t, &sendOptions{
		ToSelf: true, Text: "hi",
		LarkRun: stubRun(`{"ok":true,"identities":{"user":{}}}`, "", 0),
	})
	if err == nil {
		t.Fatal("err = nil, want error when open_id missing")
	}
	contains(t, err.Error(), "open_id", "err mentions open_id")
}

func TestResolveSelfOpenID_AltSnakeCase(t *testing.T) {
	t.Parallel()
	// Defensive: tolerate open_id if lark-cli ever changes casing.
	authStatus := `{"ok":true,"identities":{"user":{"open_id":"ou_snake"}}}`
	out, _, err := runSend(t, &sendOptions{
		ToSelf: true, Text: "hi", DryRun: true,
		LarkRun: stubRun(authStatus, "", 0),
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	contains(t, out, "--user-id ou_snake", "snake-case open_id resolved")
}

func TestSendRun_BodyFileRealFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := dir + "/body.txt"
	if err := os.WriteFile(path, []byte("from a real file body"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	out, _, err := runSend(t, &sendOptions{
		ChatID: "oc_x", BodyFile: path, DryRun: true,
		LarkRun: stubRun("", "", 0),
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	contains(t, out, "from a real file body", "body-file content")
}

func TestSendRun_FileAttachment(t *testing.T) {
	t.Parallel()
	out, _, err := runSend(t, &sendOptions{
		ChatID: "oc_x", File: "./report.pdf", DryRun: true, JSON: true,
		LarkRun: stubRun("", "", 0),
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	contains(t, out, "--file", "file arg")
	contains(t, out, "report.pdf", "file path")
}

func TestSendRun_EnvelopeFallbackChatID(t *testing.T) {
	t.Parallel()
	// lark-cli returns data.chat_id; success JSON should surface it verbatim.
	out, _, err := runSend(t, &sendOptions{
		ChatID: "oc_target", Text: "hi", JSON: true,
		LarkRun: stubRun(`{"ok":true,"identity":"bot","data":{"chat_id":"oc_resp","message_id":"om_1"}}`, "", 0),
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	contains(t, out, `"chat_id": "oc_resp"`, "envelope chat_id preferred")
}

func TestSendRun_EmptyResponseExitZero(t *testing.T) {
	t.Parallel()
	// stdout empty + exit 0 → "unexpected empty response" error.
	_, _, err := runSend(t, &sendOptions{
		ChatID: "oc_x", Text: "hi",
		LarkRun: stubRun("", "", 0),
	})
	if err == nil {
		t.Fatal("err = nil, want error for empty response")
	}
	contains(t, err.Error(), "unexpected empty response", "empty resp msg")
}

func TestSendRun_NonJSONStderrExitNonZero(t *testing.T) {
	t.Parallel()
	// Non-JSON stderr + exit 1 → falls back to trimmed stderr text, exit 1
	// (no auth indicator → generic error).
	_, _, err := runSend(t, &sendOptions{
		ChatID: "oc_x", Text: "hi",
		LarkRun: stubRun("", "raw network error text", 1),
	})
	if err == nil {
		t.Fatal("err = nil, want error")
	}
	contains(t, err.Error(), "raw network error text", "non-json stderr fallback")
}

func TestSendRun_AuthFailureMapsToExit4(t *testing.T) {
	t.Parallel()
	// "missing required scope" is an auth/scope failure → exit 4.
	_, _, err := runSend(t, &sendOptions{
		ChatID: "oc_x", Text: "hi",
		LarkRun: stubRun("", `{"ok":false,"error":{"message":"missing required scope(s): im:message.send_as_user"}}`, 3),
	})
	if err == nil {
		t.Fatal("err = nil, want error")
	}
	var ce *cmdutil.CLIError
	if !errors.As(err, &ce) {
		t.Fatalf("err type %T, want *cmdutil.CLIError", err)
	}
	if ce.Code != cmdutil.ExitAuth {
		t.Errorf("exit code = %d, want %d (auth)", ce.Code, cmdutil.ExitAuth)
	}
}

func TestSendRun_MembershipFailureMapsToExit4(t *testing.T) {
	t.Parallel()
	// "out of the chat" membership failure → permission → exit 4.
	_, _, err := runSend(t, &sendOptions{
		ChatID: "oc_x", Text: "hi",
		LarkRun: stubRun("", `{"ok":false,"error":{"message":"Bot/User can NOT be out of the chat."}}`, 1),
	})
	if err == nil {
		t.Fatal("err = nil, want error")
	}
	var ce *cmdutil.CLIError
	if !errors.As(err, &ce) || ce.Code != cmdutil.ExitAuth {
		t.Errorf("err = %v, want exit %d (auth/permission)", err, cmdutil.ExitAuth)
	}
}

func TestSendCommand_TargetsMutuallyExclusive(t *testing.T) {
	t.Parallel()
	f := cmdutil.TestFactory()
	cmd := newCmdSend(f, nil)
	cmd.SetArgs([]string{"--to-self", "--chat-id", "oc_x", "--text", "hi"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected mutual-exclusion error")
	}
}

func TestClassifyAuthError(t *testing.T) {
	t.Parallel()
	authMsgs := []string{
		"missing required scope(s): im:message.send_as_user",
		"Bot/User can NOT be out of the chat.",
		"not logged in",
		"no authority to manage",
	}
	for _, m := range authMsgs {
		m := m
		var ce *cmdutil.CLIError
		if err := classifyAuthError(m); !errors.As(err, &ce) || ce.Code != cmdutil.ExitAuth {
			t.Errorf("classifyAuthError(%q) = %v, want exit %d", m, err, cmdutil.ExitAuth)
		}
	}
	// Generic message stays exit 1 (no CLIError wrapping → plain error).
	plain := classifyAuthError("raw network timeout")
	var ce *cmdutil.CLIError
	if errors.As(plain, &ce) {
		t.Errorf("classifyAuthError(generic) wrapped as CLIError, want plain error")
	}
}
