package api

import (
	"net/http"
	"testing"

	"gitcode.com/gitcode-cli/cli/pkg/testutil"
)

// GitCode nests most merge gate fields inside "merge_request_setting" and
// repeats only a few of them at the top level. Decoding such a response into
// the flat PRSettings struct used to leave every wrapped field at its zero
// value, so `repo pr-settings view` reported can_reopen=false on repositories
// where it was true.
func TestGetPRSettingsUnwrapsMergeRequestSetting(t *testing.T) {
	var gotPath string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		_, _ = w.Write([]byte(`{
			"merge_request_setting": {
				"disable_merge_by_self": false,
				"can_force_merge": true,
				"approval_required_reviewers": 2,
				"delete_source_branch_when_merged": true,
				"close_issue_when_mr_merged": true,
				"can_reopen": true,
				"is_check_cla": true,
				"forbidden_pr_related_issue_closed": false
			},
			"only_allow_merge_if_all_discussions_are_resolved": true,
			"only_allow_merge_if_pipeline_succeeds": false,
			"merge_method": "merge"
		}`))
	})
	client := NewClientFromHTTP(testutil.NewTestHTTPClient(handler))

	settings, err := GetPRSettings(client, "owner", "repo")
	if err != nil {
		t.Fatalf("GetPRSettings() error = %v", err)
	}
	if gotPath != "/api/v5/repos/owner/repo/pull_request_settings" {
		t.Fatalf("path = %q", gotPath)
	}
	if !settings.CloseIssueWhenMrMerged {
		t.Errorf("CloseIssueWhenMrMerged = false, want true")
	}
	if !settings.CanForceMerge {
		t.Errorf("CanForceMerge = false, want true")
	}
	if !settings.CanReopen {
		t.Errorf("CanReopen = false, want true")
	}
	if !settings.DeleteSourceBranchWhenMerged {
		t.Errorf("DeleteSourceBranchWhenMerged = false, want true")
	}
	if !settings.IsCheckCla {
		t.Errorf("IsCheckCla = false, want true")
	}
	if settings.ApprovalRequiredReviewers != 2 {
		t.Errorf("ApprovalRequiredReviewers = %d, want 2", settings.ApprovalRequiredReviewers)
	}
	if settings.DisableMergeBySelf {
		t.Errorf("DisableMergeBySelf = true, want false")
	}
	if settings.ForbiddenPrRelatedIssueClosed {
		t.Errorf("ForbiddenPrRelatedIssueClosed = true, want false")
	}
	if !settings.OnlyAllowMergeIfAllDiscussionsResolved {
		t.Errorf("OnlyAllowMergeIfAllDiscussionsResolved = false, want true")
	}
	if settings.OnlyAllowMergeIfPipelineSucceeds {
		t.Errorf("OnlyAllowMergeIfPipelineSucceeds = true, want false")
	}
	if settings.MergeMethod != "merge" {
		t.Errorf("MergeMethod = %q, want %q", settings.MergeMethod, "merge")
	}
}

func TestGetPRSettingsAcceptsFlatResponse(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"close_issue_when_mr_merged": true,
			"disable_merge_by_self": true,
			"merge_method": "ff"
		}`))
	})
	client := NewClientFromHTTP(testutil.NewTestHTTPClient(handler))

	settings, err := GetPRSettings(client, "owner", "repo")
	if err != nil {
		t.Fatalf("GetPRSettings() error = %v", err)
	}
	if !settings.CloseIssueWhenMrMerged {
		t.Errorf("CloseIssueWhenMrMerged = false, want true")
	}
	if !settings.DisableMergeBySelf {
		t.Errorf("DisableMergeBySelf = false, want true")
	}
	if settings.MergeMethod != "ff" {
		t.Errorf("MergeMethod = %q, want %q", settings.MergeMethod, "ff")
	}
}

// Mixed shapes carry some fields only at the top level and the rest only
// inside "merge_request_setting"; both halves must survive decoding.
func TestGetPRSettingsMergesMixedResponse(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"merge_request_setting": {
				"close_issue_when_mr_merged": true
			},
			"can_force_merge": true,
			"merge_method": "ff"
		}`))
	})
	client := NewClientFromHTTP(testutil.NewTestHTTPClient(handler))

	settings, err := GetPRSettings(client, "owner", "repo")
	if err != nil {
		t.Fatalf("GetPRSettings() error = %v", err)
	}
	if !settings.CloseIssueWhenMrMerged {
		t.Errorf("CloseIssueWhenMrMerged = false, want true (wrapped field)")
	}
	if !settings.CanForceMerge {
		t.Errorf("CanForceMerge = false, want true (top-level field)")
	}
	if settings.MergeMethod != "ff" {
		t.Errorf("MergeMethod = %q, want %q", settings.MergeMethod, "ff")
	}
}

// A null wrapper must not clobber the flat fields around it.
func TestGetPRSettingsNullWrapperKeepsFlatFields(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"merge_request_setting": null,
			"close_issue_when_mr_merged": true,
			"merge_method": "ff"
		}`))
	})
	client := NewClientFromHTTP(testutil.NewTestHTTPClient(handler))

	settings, err := GetPRSettings(client, "owner", "repo")
	if err != nil {
		t.Fatalf("GetPRSettings() error = %v", err)
	}
	if !settings.CloseIssueWhenMrMerged {
		t.Errorf("CloseIssueWhenMrMerged = false, want true")
	}
	if settings.MergeMethod != "ff" {
		t.Errorf("MergeMethod = %q, want %q", settings.MergeMethod, "ff")
	}
}
