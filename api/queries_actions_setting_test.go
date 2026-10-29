package api

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"gitcode.com/gitcode-cli/cli/pkg/testutil"
)

func TestGetActionsSettingUnwrapsData(t *testing.T) {
	var gotPath string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		if got := r.Header.Get("Referer"); got != "https://gitcode.com/" {
			t.Errorf("Referer = %q, want %q", got, "https://gitcode.com/")
		}
		_, _ = w.Write([]byte(`{"data":{"action_enabled":true,"block_all_new_pipelines":false,"block_cross_repo_pr_triggers":true}}`))
	})
	client := NewClientFromHTTP(testutil.NewTestHTTPClient(handler))

	setting, err := GetActionsSetting(client, "12345")
	if err != nil {
		t.Fatalf("GetActionsSetting() error = %v", err)
	}
	if gotPath != "/api/v2/projects/12345/actions/setting" {
		t.Fatalf("path = %q", gotPath)
	}
	if setting.ActionEnabled == nil || !*setting.ActionEnabled {
		t.Fatalf("ActionEnabled = %#v, want true", setting.ActionEnabled)
	}
	if setting.BlockAllNewPipelines == nil || *setting.BlockAllNewPipelines {
		t.Fatalf("BlockAllNewPipelines = %#v, want false", setting.BlockAllNewPipelines)
	}
	if setting.BlockCrossRepoPRTriggers == nil || !*setting.BlockCrossRepoPRTriggers {
		t.Fatalf("BlockCrossRepoPRTriggers = %#v, want true", setting.BlockCrossRepoPRTriggers)
	}
}

func TestGetActionsSettingAcceptsFlatResponse(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"action_enabled":false}`))
	})
	client := NewClientFromHTTP(testutil.NewTestHTTPClient(handler))

	setting, err := GetActionsSetting(client, "12345")
	if err != nil {
		t.Fatalf("GetActionsSetting() error = %v", err)
	}
	if setting.ActionEnabled == nil || *setting.ActionEnabled {
		t.Fatalf("ActionEnabled = %#v, want false", setting.ActionEnabled)
	}
	if setting.BlockAllNewPipelines != nil || setting.BlockCrossRepoPRTriggers != nil {
		t.Fatalf("unexpected absent fields: %#v", setting)
	}
}

func TestUpdateActionsSettingPreservesReadFields(t *testing.T) {
	var gotPath string
	var gotMethod string
	var gotBody map[string]any
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		gotMethod = r.Method
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
			return
		}
		if err := json.Unmarshal(body, &gotBody); err != nil {
			t.Errorf("decode body: %v", err)
		}
		if got := r.Header.Get("Referer"); got != "https://gitcode.com/" {
			t.Errorf("Referer = %q, want %q", got, "https://gitcode.com/")
		}
		w.WriteHeader(http.StatusNoContent)
	})
	client := NewClientFromHTTP(testutil.NewTestHTTPClient(handler))
	client.SetToken("test-token", "env")

	setting := &ActionsSetting{
		ActionEnabled:            boolPointer(false),
		BlockAllNewPipelines:     boolPointer(true),
		BlockCrossRepoPRTriggers: boolPointer(false),
	}
	if err := UpdateActionsSetting(client, "12345", setting); err != nil {
		t.Fatalf("UpdateActionsSetting() error = %v", err)
	}
	if gotMethod != http.MethodPut || gotPath != "/api/v2/projects/12345/actions/setting" {
		t.Fatalf("request = %s %s", gotMethod, gotPath)
	}
	if len(gotBody) != 3 {
		t.Fatalf("body fields = %#v, want only three setting fields", gotBody)
	}
	if gotBody["action_enabled"] != false || gotBody["block_all_new_pipelines"] != true || gotBody["block_cross_repo_pr_triggers"] != false {
		t.Fatalf("body = %#v", gotBody)
	}
	if _, ok := gotBody["project_id"]; ok {
		t.Fatalf("body unexpectedly contains project_id: %#v", gotBody)
	}
}

func TestActionsSettingEndpointEscapesProjectID(t *testing.T) {
	var gotPath string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		_, _ = w.Write([]byte(`{"action_enabled":true}`))
	})
	client := NewClientFromHTTP(testutil.NewTestHTTPClient(handler))

	if _, err := GetActionsSetting(client, "owner/repo"); err != nil {
		t.Fatalf("GetActionsSetting() error = %v", err)
	}
	if gotPath != "/api/v2/projects/owner%2Frepo/actions/setting" {
		t.Fatalf("path = %q", gotPath)
	}
}

func boolPointer(value bool) *bool {
	return &value
}
