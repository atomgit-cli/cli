package api

import "encoding/json"

// PRSettings represents the pull request settings for a repository.
type PRSettings struct {
	ApprovalRequiredReviewersEnable        bool   `json:"approval_required_reviewers_enable"`
	ApprovalRequiredReviewers              int    `json:"approval_required_reviewers"`
	OnlyAllowMergeIfAllDiscussionsResolved bool   `json:"only_allow_merge_if_all_discussions_are_resolved"`
	OnlyAllowMergeIfPipelineSucceeds       bool   `json:"only_allow_merge_if_pipeline_succeeds"`
	DisableMergeBySelf                     bool   `json:"disable_merge_by_self"`
	CanForceMerge                          bool   `json:"can_force_merge"`
	AddNotesAfterMerged                    bool   `json:"add_notes_after_merged"`
	MarkAutoMergedMrAsClosed               bool   `json:"mark_auto_merged_mr_as_closed"`
	CanReopen                              bool   `json:"can_reopen"`
	DeleteSourceBranchWhenMerged           bool   `json:"delete_source_branch_when_merged"`
	DisableSquashMerge                     bool   `json:"disable_squash_merge"`
	AutoSquashMerge                        bool   `json:"auto_squash_merge"`
	MergeMethod                            string `json:"merge_method"`
	SquashMergeWithNoMergeCommit           bool   `json:"squash_merge_with_no_merge_commit"`
	MergedCommitAuthor                     string `json:"merged_commit_author"`
	ApprovalRequiredApprovers              int    `json:"approval_required_approvers"`
	ApprovalApproverIDs                    string `json:"approval_approver_ids"`
	ApprovalTesterIDs                      string `json:"approval_tester_ids"`
	ApprovalRequiredTesters                int    `json:"approval_required_testers"`
	IsCheckCla                             bool   `json:"is_check_cla"`
	IsAllowLiteMergeRequest                bool   `json:"is_allow_lite_merge_request"`
	LiteMergeRequestPrefixTitle            string `json:"lite_merge_request_prefix_title"`
	CloseIssueWhenMrMerged                 bool   `json:"close_issue_when_mr_merged"`
	ForbiddenPrRelatedIssueClosed          bool   `json:"forbidden_pr_related_issue_closed"`
}

// UnmarshalJSON accepts every response shape served for
// /repos/{owner}/{repo}/pull_request_settings. GitCode nests most merge
// gate fields inside a "merge_request_setting" object and repeats only a few
// of them at the top level; older or mirrored shapes flatten every field, and
// mixed shapes split fields between the two places. Decoding any of those
// into the flat struct above would silently leave the misplaced fields at
// their zero value, so the flat fields are decoded first and the wrapped
// fields are overlaid on top. A field carried in both places keeps the
// wrapped value.
func (s *PRSettings) UnmarshalJSON(data []byte) error {
	type plain PRSettings

	var base plain
	if err := json.Unmarshal(data, &base); err != nil {
		return err
	}

	var wrapper struct {
		MergeRequestSetting json.RawMessage `json:"merge_request_setting"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return err
	}
	// encoding/json already treats a null unmarshaled into a struct as a
	// no-op, so the null check below is belt-and-braces: it keeps a future
	// decode-replace refactor (decode into a fresh struct, then assign) from
	// letting a null wrapper clobber the flat fields.
	if len(wrapper.MergeRequestSetting) > 0 && string(wrapper.MergeRequestSetting) != "null" {
		if err := json.Unmarshal(wrapper.MergeRequestSetting, &base); err != nil {
			return err
		}
	}

	*s = PRSettings(base)
	return nil
}

// GetPRSettings fetches the pull request settings for a repository.
func GetPRSettings(client *Client, owner, repo string) (*PRSettings, error) {
	var settings PRSettings
	err := client.Get(escapedRepoPath(owner, repo)+"/pull_request_settings", &settings)
	if err != nil {
		return nil, err
	}
	return &settings, nil
}

// UpdatePRSettingsRequest is the request body for updating PR settings.
// Uses pointer types for bool/int fields to distinguish "not provided" from zero values.
type UpdatePRSettingsRequest struct {
	ApprovalRequiredReviewersEnable        *bool  `json:"approval_required_reviewers_enable,omitempty"`
	ApprovalRequiredReviewers              *int   `json:"approval_required_reviewers,omitempty"`
	OnlyAllowMergeIfAllDiscussionsResolved *bool  `json:"only_allow_merge_if_all_discussions_are_resolved,omitempty"`
	OnlyAllowMergeIfPipelineSucceeds       *bool  `json:"only_allow_merge_if_pipeline_succeeds,omitempty"`
	DisableMergeBySelf                     *bool  `json:"disable_merge_by_self,omitempty"`
	CanForceMerge                          *bool  `json:"can_force_merge,omitempty"`
	DeleteSourceBranchWhenMerged           *bool  `json:"delete_source_branch_when_merged,omitempty"`
	DisableSquashMerge                     *bool  `json:"disable_squash_merge,omitempty"`
	MergeMethod                            string `json:"merge_method,omitempty"`
	ApprovalRequiredApprovers              *int   `json:"approval_required_approvers,omitempty"`
	ApprovalApproverIDs                    string `json:"approval_approver_ids,omitempty"`
	ApprovalTesterIDs                      string `json:"approval_tester_ids,omitempty"`
	ApprovalRequiredTesters                *int   `json:"approval_required_testers,omitempty"`
	IsCheckCla                             *bool  `json:"is_check_cla,omitempty"`
	IsAllowLiteMergeRequest                *bool  `json:"is_allow_lite_merge_request,omitempty"`
	LiteMergeRequestPrefixTitle            string `json:"lite_merge_request_prefix_title,omitempty"`
	CloseIssueWhenMrMerged                 *bool  `json:"close_issue_when_mr_merged,omitempty"`
	ForbiddenPrRelatedIssueClosed          *bool  `json:"forbidden_pr_related_issue_closed,omitempty"`
}

// UpdatePRSettings updates the pull request settings for a repository.
// After updating, it re-reads the full settings via GET to return a complete object.
func UpdatePRSettings(client *Client, owner, repo string, req *UpdatePRSettingsRequest) (*PRSettings, error) {
	err := client.Put(escapedRepoPath(owner, repo)+"/pull_request_settings", req, nil)
	if err != nil {
		return nil, err
	}
	return GetPRSettings(client, owner, repo)
}
