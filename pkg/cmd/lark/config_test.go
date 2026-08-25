package lark

import (
	"strings"
	"testing"

	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
)

func TestConfigSetRun_Friendly(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	io, _, out, _ := iostreams.Test()
	opts := &configSetOptions{IO: io, DefaultChat: "oc_abc"}
	if err := configSetRun(opts); err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(out.String(), "oc_abc") {
		t.Errorf("out = %q, want oc_abc", out.String())
	}
}

func TestConfigSetRun_PersistsAndEnvOverrides(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GC_CONFIG_DIR", dir)
	t.Setenv("GC_LARK_DEFAULT_CHAT_ID", "")

	io, _, _, _ := iostreams.Test()
	if err := configSetRun(&configSetOptions{IO: io, DefaultChat: "oc_persist"}); err != nil {
		t.Fatalf("set err = %v", err)
	}

	// env now overrides; get should reflect env, not persisted.
	t.Setenv("GC_LARK_DEFAULT_CHAT_ID", "oc_env")
	io2, _, out2, _ := iostreams.Test()
	if err := configGetRun(&configGetOptions{IO: io2}); err != nil {
		t.Fatalf("get err = %v", err)
	}
	if !strings.Contains(out2.String(), "oc_env") {
		t.Errorf("get out = %q, want oc_env (env overrides persisted)", out2.String())
	}
}

func TestConfigGetRun_NothingConfigured(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_LARK_DEFAULT_CHAT_ID", "")
	io, _, out, _ := iostreams.Test()
	if err := configGetRun(&configGetOptions{IO: io}); err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(out.String(), "no default chat configured") {
		t.Errorf("out = %q, want not-configured hint", out.String())
	}
}

func TestConfigGetRun_JSON(t *testing.T) {
	t.Setenv("GC_CONFIG_DIR", t.TempDir())
	t.Setenv("GC_LARK_DEFAULT_CHAT_ID", "oc_json")
	io, _, out, _ := iostreams.Test()
	if err := configGetRun(&configGetOptions{IO: io, JSON: true}); err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(out.String(), `"default_chat": "oc_json"`) {
		t.Errorf("out = %q, want default_chat oc_json", out.String())
	}
}

func TestConfigSetCommand_RequiresDefaultChat(t *testing.T) {
	t.Parallel()
	f := cmdutil.TestFactory()
	cmd := newCmdConfig(f, nil, nil)
	// Run "config set" without --default-chat through the parent command so
	// cobra's MarkFlagRequired check fires.
	cmd.SetArgs([]string{"set"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when --default-chat missing")
	}
}
