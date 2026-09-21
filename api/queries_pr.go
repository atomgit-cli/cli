package api

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// PullRequest represents a GitCode pull request
type PullRequest struct {
	ID           interface{}  `json:"id"`
	Number       int          `json:"number"`
	Title        string       `json:"title"`
	Body         string       `json:"body"`
	Description  string       `json:"description"`
	State        string       `json:"state"`
	HTMLURL      string       `json:"html_url"`
	DiffURL      string       `json:"diff_url"`
	PatchURL     string       `json:"patch_url"`
	User         *User        `json:"user"`
	Head         *PRBranch    `json:"head"`
	Base         *PRBranch    `json:"base"`
	Merged       bool         `json:"merged"`
	MergedAt     *string      `json:"merged_at"`
	Mergeable    *bool        `json:"mergeable"`
	MergeState   interface{}  `json:"mergeable_state"`
	Draft        bool         `json:"draft"`
	CreatedAt    FlexibleTime `json:"created_at"`
	UpdatedAt    FlexibleTime `json:"updated_at"`
	ClosedAt     *string      `json:"closed_at"`
	Comments     int          `json:"comments"`
	Commits      int          `json:"commits"`
	Additions    int          `json:"additions"`
	Deletions    int          `json:"deletions"`
	ChangedFiles int          `json:"changed_files"`
	Labels       []*Label     `json:"labels"`
	Assignees    []*User      `json:"assignees"`
	Reviewers    []*User      `json:"requested_reviewers"`
	Milestone    *Milestone   `json:"milestone"`
}

// IsMerged reports whether the pull request has been merged. GitCode may
// return merged as null, so state and merged_at are authoritative fallbacks.
func (pr PullRequest) IsMerged() bool {
	return pr.Merged || strings.EqualFold(pr.State, "merged") ||
		(pr.MergedAt != nil && strings.TrimSpace(*pr.MergedAt) != "")
}

// PRBranch represents a branch in a PR
type PRBranch struct {
	Label string      `json:"label"`
	Ref   string      `json:"ref"`
	SHA   string      `json:"sha"`
	Repo  *Repository `json:"repo"`
}

// PRComment represents a comment on a PR
type PRComment struct {
	ID           interface{}  `json:"id"`
	DiscussionID string       `json:"discussion_id"`
	Body         string       `json:"body"`
	User         *User        `json:"user"`
	CreatedAt    FlexibleTime `json:"created_at"`
	UpdatedAt    FlexibleTime `json:"updated_at"`
	CommentType  string       `json:"comment_type"`
	Resolved     bool         `json:"resolved"`
	DiffFile     string       `json:"diff_file"`
	DiffPosition interface{}  `json:"diff_position"`
}

// PRReview represents a review on a PR
type PRReview struct {
	ID          interface{}  `json:"id"`
	User        *User        `json:"user"`
	Body        string       `json:"body"`
	State       string       `json:"state"`
	SubmittedAt FlexibleTime `json:"submitted_at"`
}

// PRListOptions represents options for listing PRs
type PRListOptions struct {
	State     string `url:"state,omitempty"`
	Head      string `url:"head,omitempty"`
	Base      string `url:"base,omitempty"`
	Sort      string `url:"sort,omitempty"`
	Direction string `url:"direction,omitempty"`
	PerPage   int    `url:"per_page,omitempty"`
	Page      int    `url:"page,omitempty"`
	Milestone string `url:"milestone,omitempty"`
}

// CreatePROptions represents options for creating a PR.
//
// 跨仓库 PR（从 fork 提交到 upstream）通过 Head 使用 "<fork_owner>:<branch>"
// 形式表达源仓库，而不是单独的 fork_path 表单字段——GitCode v5 对 fork_path
// 的跨仓库创建会错误解析源（upstream 同名分支 → 0 commits）甚至直接 403。
// 详见 issue #259。
type CreatePROptions struct {
	Title  string   `json:"title"`
	Body   string   `json:"body,omitempty"`
	Head   string   `json:"head"`
	Base   string   `json:"base"`
	Draft  bool     `json:"draft,omitempty"`
	Labels []string `json:"labels,omitempty"`
}

// UpdatePROptions represents options for updating a PR
type UpdatePROptions struct {
	Title             string   `json:"title,omitempty"`
	Body              string   `json:"body,omitempty"`
	State             string   `json:"state,omitempty"`
	StateEvent        string   `json:"state_event,omitempty"`
	Base              string   `json:"base,omitempty"`
	Draft             *bool    `json:"draft,omitempty"`
	MilestoneNumber   int      `json:"milestone_number,omitempty"`
	Labels            []string `json:"labels,omitempty"`
	LabelsSet         bool     `json:"-"`
	CloseRelatedIssue *bool    `json:"close_related_issue,omitempty"`
}

// CreatePRCommentOptions represents options for creating a PR comment
type CreatePRCommentOptions struct {
	Body string `json:"body"`
	Path string `json:"path,omitempty"`
	// Position is the line number in the new version of the file (the new/right
	// side of the diff), not an offset within the diff hunk. The API resolves it
	// to the diff_position (new_line) of the comment.
	Position int `json:"position,omitempty"`
}

// CreatePRReviewOptions represents options for creating a PR review
type CreatePRReviewOptions struct {
	Body     string            `json:"body,omitempty"`
	Event    string            `json:"event"`
	Comments []PRReviewComment `json:"comments,omitempty"`
}

// PRReviewComment represents a comment in a review
type PRReviewComment struct {
	Path     string `json:"path"`
	Position int    `json:"position"`
	Body     string `json:"body"`
}

// MergePROptions represents options for merging a PR
type MergePROptions struct {
	CommitTitle   string `json:"commit_title,omitempty"`
	CommitMessage string `json:"commit_message,omitempty"`
	SHA           string `json:"sha,omitempty"`
	MergeMethod   string `json:"merge_method,omitempty"`
}

// ListPullRequests lists pull requests for a repository
func ListPullRequests(client *Client, owner, repo string, opts *PRListOptions) ([]PullRequest, error) {
	path := buildPRListPath(escapedRepoPath(owner, repo)+"/pulls", opts)

	var prs []PullRequest
	err := client.Get(path, &prs)
	if err != nil {
		return nil, err
	}
	for i := range prs {
		normalizePullRequest(&prs[i])
	}
	return prs, nil
}

func buildPRListPath(base string, opts *PRListOptions) string {
	if opts == nil {
		return base
	}
	return base + newQueryBuilder().
		Set("state", opts.State).
		Set("head", opts.Head).
		Set("base", opts.Base).
		Set("sort", opts.Sort).
		Set("direction", opts.Direction).
		SetInt("per_page", opts.PerPage).
		SetInt("page", opts.Page).
		Set("milestone", opts.Milestone).
		String()
}

// GetPullRequest fetches a PR by number
func GetPullRequest(client *Client, owner, repo string, number int) (*PullRequest, error) {
	var pr PullRequest
	err := client.Get(escapedRepoPath(owner, repo)+"/pulls/"+strconv.Itoa(number), &pr)
	if err != nil {
		return nil, err
	}
	normalizePullRequest(&pr)
	return &pr, nil
}

// CreatePullRequest creates a new PR
func CreatePullRequest(client *Client, owner, repo string, opts *CreatePROptions) (*PullRequest, error) {
	formValues := buildPRCreateFormValues(opts)

	var pr PullRequest
	err := client.PostForm(escapedRepoPath(owner, repo)+"/pulls", formValues, &pr)
	if err != nil {
		return nil, err
	}
	normalizePullRequest(&pr)
	if opts != nil && opts.Body != "" && pr.Body == "" && pr.Number > 0 {
		refreshed, refreshErr := GetPullRequest(client, owner, repo, pr.Number)
		if refreshErr == nil {
			return refreshed, nil
		}
	}
	return &pr, nil
}

func buildPRCreateFormValues(opts *CreatePROptions) url.Values {
	formValues := url.Values{}

	if opts == nil {
		return formValues
	}
	if opts.Title != "" {
		formValues.Set("title", opts.Title)
	}
	if opts.Body != "" {
		formValues.Set("body", opts.Body)
	}
	if opts.Head != "" {
		formValues.Set("head", opts.Head)
	}
	if opts.Base != "" {
		formValues.Set("base", opts.Base)
	}
	if opts.Draft {
		formValues.Set("draft", "true")
	}
	for _, label := range opts.Labels {
		trimmed := strings.TrimSpace(label)
		if trimmed != "" {
			formValues.Add("labels[]", trimmed)
		}
	}

	return formValues
}

// UpdatePullRequest updates an existing PR
func UpdatePullRequest(client *Client, owner, repo string, number int, opts *UpdatePROptions) (*PullRequest, error) {
	formValues := buildPRUpdateFormValues(opts)

	var pr PullRequest
	err := client.PatchForm(escapedRepoPath(owner, repo)+"/pulls/"+strconv.Itoa(number), formValues, &pr)
	if err != nil {
		return nil, err
	}
	normalizePullRequest(&pr)
	return &pr, nil
}

func normalizePullRequest(pr *PullRequest) {
	if pr == nil {
		return
	}
	pr.Merged = pr.IsMerged()
	if pr.Body == "" && pr.Description != "" {
		pr.Body = pr.Description
	}
	if pr.Description == "" && pr.Body != "" {
		pr.Description = pr.Body
	}
}

func buildPRUpdateFormValues(opts *UpdatePROptions) url.Values {
	formValues := url.Values{}

	if opts == nil {
		return formValues
	}
	if opts.Title != "" {
		formValues.Set("title", opts.Title)
	}
	if opts.Body != "" {
		formValues.Set("body", opts.Body)
	}
	if opts.State != "" {
		formValues.Set("state", opts.State)
	}
	if opts.StateEvent != "" {
		formValues.Set("state_event", opts.StateEvent)
	}
	if opts.Base != "" {
		formValues.Set("base", opts.Base)
	}
	if opts.Draft != nil {
		if *opts.Draft {
			formValues.Set("draft", "true")
		} else {
			formValues.Set("draft", "false")
		}
	}
	if opts.MilestoneNumber > 0 {
		formValues.Set("milestone_number", strconv.Itoa(opts.MilestoneNumber))
	}
	if opts.LabelsSet {
		labels := make([]string, 0, len(opts.Labels))
		for _, label := range opts.Labels {
			if trimmed := strings.TrimSpace(label); trimmed != "" {
				labels = append(labels, trimmed)
			}
		}
		value := strings.Join(labels, ",")
		if value == "" {
			// GitCode rejects labels= as a missing parameter. A lone comma is
			// parsed as an explicit empty label list and clears all labels.
			value = ","
		}
		formValues.Set("labels", value)
	} else {
		for _, label := range opts.Labels {
			trimmed := strings.TrimSpace(label)
			if trimmed != "" {
				formValues.Add("labels[]", trimmed)
			}
		}
	}
	if opts.CloseRelatedIssue != nil {
		if *opts.CloseRelatedIssue {
			formValues.Set("close_related_issue", "true")
		} else {
			formValues.Set("close_related_issue", "false")
		}
	}

	return formValues
}

func isPullRequestClosed(pr *PullRequest) bool {
	if pr == nil {
		return false
	}
	return strings.EqualFold(pr.State, "closed") || strings.EqualFold(pr.State, "close")
}

// ClosePullRequest closes a PR
func ClosePullRequest(client *Client, owner, repo string, number int) (*PullRequest, error) {
	pr, err := GetPullRequest(client, owner, repo, number)
	if err != nil {
		return nil, fmt.Errorf("failed to get PR: %w", err)
	}

	updated, err := UpdatePullRequest(client, owner, repo, number, &UpdatePROptions{
		State: "closed",
		Title: pr.Title,
	})
	if err != nil {
		return nil, err
	}
	if isPullRequestClosed(updated) {
		return updated, nil
	}

	verified, err := GetPullRequest(client, owner, repo, number)
	if err != nil {
		return nil, fmt.Errorf("failed to verify PR close state: %w", err)
	}
	if isPullRequestClosed(verified) {
		return verified, nil
	}

	return nil, fmt.Errorf("pull request #%d is still open after close request", number)
}

// ReopenPullRequest reopens a closed PR
func ReopenPullRequest(client *Client, owner, repo string, number int) (*PullRequest, error) {
	pr, err := GetPullRequest(client, owner, repo, number)
	if err != nil {
		return nil, fmt.Errorf("failed to get PR: %w", err)
	}

	updated, err := UpdatePullRequest(client, owner, repo, number, &UpdatePROptions{
		State: "open",
		Title: pr.Title,
	})
	if err != nil {
		return nil, err
	}
	if !isPullRequestClosed(updated) {
		return updated, nil
	}

	verified, err := GetPullRequest(client, owner, repo, number)
	if err != nil {
		return nil, fmt.Errorf("failed to verify PR reopen state: %w", err)
	}
	if !isPullRequestClosed(verified) {
		return verified, nil
	}

	return nil, fmt.Errorf("pull request #%d is still closed after reopen request", number)
}

// MergePullRequest merges a PR
func MergePullRequest(client *Client, owner, repo string, number int, opts *MergePROptions) (*PullRequest, error) {
	var pr PullRequest
	err := client.Put(escapedRepoPath(owner, repo)+"/pulls/"+strconv.Itoa(number)+"/merge", opts, &pr)
	if err != nil {
		return nil, err
	}
	normalizePullRequest(&pr)
	return &pr, nil
}

// ListPRComments lists comments on a PR
func ListPRComments(client *Client, owner, repo string, number int) ([]PRComment, error) {
	const perPage = 100

	var allComments []PRComment
	for page := 1; ; page++ {
		var comments []PRComment
		path := escapedRepoPath(owner, repo) + "/pulls/" + strconv.Itoa(number) + "/comments"

		err := client.Get(path+newQueryBuilder().SetInt("per_page", perPage).SetInt("page", page).String(), &comments)
		if err != nil {
			return nil, err
		}

		allComments = append(allComments, comments...)
		if len(comments) < perPage {
			break
		}
	}
	return allComments, nil
}

// CreatePRComment creates a comment on a PR
func CreatePRComment(client *Client, owner, repo string, number int, opts *CreatePRCommentOptions) (*PRComment, error) {
	var comment PRComment
	err := client.Post(escapedRepoPath(owner, repo)+"/pulls/"+strconv.Itoa(number)+"/comments", opts, &comment)
	if err != nil {
		return nil, err
	}
	return &comment, nil
}

// CreatePRReview creates a review on a PR
func CreatePRReview(client *Client, owner, repo string, number int, opts *CreatePRReviewOptions) (*PRReview, error) {
	var review PRReview
	err := client.Post(escapedRepoPath(owner, repo)+"/pulls/"+strconv.Itoa(number)+"/reviews", opts, &review)
	if err != nil {
		return nil, err
	}
	return &review, nil
}

// ReviewPROptions represents options for reviewing a PR
type ReviewPROptions struct {
	Force bool `json:"force,omitempty"` // Force approval (admin only)
}

// ReviewPR handles PR review (approve/force pass)
func ReviewPR(client *Client, owner, repo string, number int, opts *ReviewPROptions) error {
	return client.Post(escapedRepoPath(owner, repo)+"/pulls/"+strconv.Itoa(number)+"/review", opts, nil)
}

// EditPR updates a PR's information
func EditPR(client *Client, owner, repo string, number int, opts *UpdatePROptions) (*PullRequest, error) {
	return UpdatePullRequest(client, owner, repo, number, opts)
}

// TestPROptions represents options for PR test
type TestPROptions struct {
	Force bool `json:"force,omitempty"` // Force test pass (admin only)
}

// TestPR handles PR test
func TestPR(client *Client, owner, repo string, number int, opts *TestPROptions) error {
	return client.Post(escapedRepoPath(owner, repo)+"/pulls/"+strconv.Itoa(number)+"/test", opts, nil)
}

// EditPRCommentOptions represents options for editing a PR comment
type EditPRCommentOptions struct {
	Body string `json:"body"`
}

// EditPRComment edits a PR comment
func EditPRComment(client *Client, owner, repo string, commentID int, opts *EditPRCommentOptions) (*PRComment, error) {
	var comment PRComment
	err := client.Patch(escapedRepoPath(owner, repo)+"/pulls/comments/"+strconv.Itoa(commentID), opts, &comment)
	if err != nil {
		return nil, err
	}
	return &comment, nil
}

// DeletePRComment deletes a PR comment
func DeletePRComment(client *Client, owner, repo string, commentID int) error {
	return client.Delete(escapedRepoPath(owner, repo) + "/pulls/comments/" + strconv.Itoa(commentID))
}

// ReplyPRCommentOptions represents options for replying to a PR comment
type ReplyPRCommentOptions struct {
	Body string `json:"body"`
}

// ReplyPRCommentReply represents the response from replying to a PR comment
type ReplyPRCommentReply struct {
	ID     string `json:"id"`
	NoteID int    `json:"noteId"`
	Body   string `json:"body"`
}

// ReplyPRComment replies to a PR comment discussion
func ReplyPRComment(client *Client, owner, repo string, number int, discussionID string, opts *ReplyPRCommentOptions) (*ReplyPRCommentReply, error) {
	var result ReplyPRCommentReply
	err := client.Post(escapedRepoPath(owner, repo)+"/pulls/"+strconv.Itoa(number)+"/discussions/"+discussionID+"/comments", opts, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// ResolvePRCommentOptions represents options for resolving a PR comment
type ResolvePRCommentOptions struct {
	Resolved bool `json:"resolved"`
}

// ResolvePRComment updates the resolution status of a PR comment
func ResolvePRComment(client *Client, owner, repo string, number int, discussionID string, opts *ResolvePRCommentOptions) error {
	return client.Put(escapedRepoPath(owner, repo)+"/pulls/"+strconv.Itoa(number)+"/comments/"+discussionID, opts, nil)
}

// ListPRReviews lists reviews on a PR
func ListPRReviews(client *Client, owner, repo string, number int) ([]PRReview, error) {
	var reviews []PRReview
	err := client.Get(escapedRepoPath(owner, repo)+"/pulls/"+strconv.Itoa(number)+"/reviews", &reviews)
	if err != nil {
		return nil, err
	}
	return reviews, nil
}

// ListPRCommits lists commits in a PR
func ListPRCommits(client *Client, owner, repo string, number int) ([]Commit, error) {
	const perPage = 100

	var all []Commit
	for page := 1; ; page++ {
		var commits []Commit
		err := client.Get(escapedRepoPath(owner, repo)+"/pulls/"+strconv.Itoa(number)+"/commits"+newQueryBuilder().SetInt("per_page", perPage).SetInt("page", page).String(), &commits)
		if err != nil {
			return nil, err
		}
		all = append(all, commits...)
		if len(commits) < perPage {
			break
		}
	}
	return all, nil
}

// Commit represents a Git commit
type Commit struct {
	SHA       string      `json:"sha"`
	Message   string      `json:"message"`
	Commit    *CommitInfo `json:"commit"`
	Author    *User       `json:"author"`
	Committer *User       `json:"committer"`
}

// MessageText returns the best available commit message.
func (c Commit) MessageText() string {
	if c.Message != "" {
		return c.Message
	}
	if c.Commit != nil {
		return c.Commit.Message
	}
	return ""
}

// PRFilesResponse represents the response from PR files API
type PRFilesResponse struct {
	Code        int         `json:"code"`
	AddedLines  int         `json:"added_lines"`
	RemoveLines int         `json:"remove_lines"`
	Count       int         `json:"count"`
	DiffRefs    *PRDiffRefs `json:"diff_refs"`
	Diffs       []*PRDiff   `json:"diffs"`
}

// PRDiffRefs represents diff references
type PRDiffRefs struct {
	BaseSHA  string `json:"base_sha"`
	StartSHA string `json:"start_sha"`
	HeadSHA  string `json:"head_sha"`
}

// PRDiff represents a single file diff
type PRDiff struct {
	NewBlobID string         `json:"new_blob_id"`
	Statistic *PRStatistic   `json:"statistic"`
	Type      string         `json:"type"`
	Path      string         `json:"path"`
	OldPath   string         `json:"old_path"`
	NewPath   string         `json:"new_path"`
	View      int            `json:"view"`
	Head      *PRDiffHead    `json:"head"`
	Content   *PRDiffContent `json:"content"`
}

// PRStatistic represents file change statistics
type PRStatistic struct {
	Additions int `json:"additions"`
	Deletions int `json:"deletions"`
}

// PRDiffHead represents diff head info
type PRDiffHead struct {
	URL       string `json:"url"`
	CommitID  string `json:"commit_id"`
	Additions int    `json:"added_lines"`
	Deletions int    `json:"remove_lines"`
}

// PRDiffContent represents diff content
type PRDiffContent struct {
	Text []*PRDiffLine `json:"text"`
}

// PRDiffLine represents a single diff line
type PRDiffLine struct {
	LineContent string      `json:"line_content"`
	OldLine     interface{} `json:"old_line"` // can be string "..." or object
	NewLine     interface{} `json:"new_line"` // can be string "..." or object
	Type        string      `json:"type"`
}

// GetPRFiles gets the files and diffs of a PR
func GetPRFiles(client *Client, owner, repo string, number int) (*PRFilesResponse, error) {
	var result PRFilesResponse
	err := client.Get(escapedRepoPath(owner, repo)+"/pulls/"+strconv.Itoa(number)+"/files.json", &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// ListPRIssues lists issues linked to a pull request
func ListPRIssues(client *Client, owner, repo string, number int) ([]Issue, error) {
	var issues []Issue
	err := client.Get("/repos/"+url.PathEscape(owner)+"/"+url.PathEscape(repo)+"/pulls/"+strconv.Itoa(number)+"/issues", &issues)
	if err != nil {
		return nil, err
	}
	return issues, nil
}

// AddLabelsToPR adds labels to a pull request.
//
// It calls POST /repos/{owner}/{repo}/pulls/{number}/labels. The request body
// is the raw JSON array of label names (per the official GitCode OpenAPI), and
// the response is the array of applied labels.
func AddLabelsToPR(client *Client, owner, repo string, number int, labels []string) ([]Label, error) {
	var result []Label
	err := client.Post(escapedRepoPath(owner, repo)+"/pulls/"+strconv.Itoa(number)+"/labels", labels, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// RemoveLabelFromPR removes a label from a pull request.
//
// It calls DELETE /repos/{owner}/{repo}/pulls/{number}/labels/{name}.
func RemoveLabelFromPR(client *Client, owner, repo string, number int, label string) error {
	return client.Delete(escapedRepoPath(owner, repo) + "/pulls/" + strconv.Itoa(number) + "/labels/" + url.PathEscape(label))
}

// AssigneeRequest is the body for add assignee operations.
type AssigneeRequest struct {
	Assignees string `json:"assignees"`
}

// AddPRAssignees adds assignees to a pull request.
func AddPRAssignees(client *Client, owner, repo string, number int, users []string) error {
	return client.Post(escapedRepoPath(owner, repo)+"/pulls/"+strconv.Itoa(number)+"/assignees", &AssigneeRequest{Assignees: strings.Join(users, ",")}, nil)
}

// RemovePRAssignees removes assignees from a pull request via query parameter.
func RemovePRAssignees(client *Client, owner, repo string, number int, users []string) error {
	path := escapedRepoPath(owner, repo) + "/pulls/" + strconv.Itoa(number) + "/assignees"
	path += newQueryBuilder().Set("assignees", strings.Join(users, ",")).String()
	return client.Delete(path)
}

// ReviewerRequest is the body for add/remove reviewer operations.
type ReviewerRequest struct {
	Reviewers string `json:"reviewers"`
	Add       bool   `json:"add,omitempty"`
}

// AddPRReviewers adds reviewers to a pull request.
func AddPRReviewers(client *Client, owner, repo string, number int, users []string) error {
	return client.Post(escapedRepoPath(owner, repo)+"/pulls/"+strconv.Itoa(number)+"/reviewers", &ReviewerRequest{Reviewers: strings.Join(users, ","), Add: true}, nil)
}

// RemovePRReviewers removes reviewers from a pull request.
func RemovePRReviewers(client *Client, owner, repo string, number int, users []string) error {
	return client.REST("DELETE", escapedRepoPath(owner, repo)+"/pulls/"+strconv.Itoa(number)+"/reviewers", &ReviewerRequest{Reviewers: strings.Join(users, ",")}, nil)
}

// TesterRequest is the body for add/remove tester operations.
type TesterRequest struct {
	Testers string `json:"testers"`
	Add     bool   `json:"add,omitempty"`
}

// AddPRTesters adds testers to a pull request.
func AddPRTesters(client *Client, owner, repo string, number int, users []string) error {
	return client.Post(escapedRepoPath(owner, repo)+"/pulls/"+strconv.Itoa(number)+"/testers", &TesterRequest{Testers: strings.Join(users, ","), Add: true}, nil)
}

// RemovePRTesters removes testers from a pull request.
func RemovePRTesters(client *Client, owner, repo string, number int, users []string) error {
	return client.REST("DELETE", escapedRepoPath(owner, repo)+"/pulls/"+strconv.Itoa(number)+"/testers", &TesterRequest{Testers: strings.Join(users, ",")}, nil)
}

// PRMerger represents a user set as a pull request merger. Field shapes follow
// the official GitCode OpenAPI for PUT /repos/{owner}/{repo}/pulls/{number}/mergers.
type PRMerger struct {
	ID        int    `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url,omitempty"`
	ObjectID  string `json:"object_id"`
}

// MergerRequest is the body for setting PR mergers. Per the official OpenAPI
// the mergers field is a comma-separated list of user personal-space paths.
type MergerRequest struct {
	Mergers string `json:"mergers"`
}

// SetPRMergers sets the mergers of a pull request, replacing any previous set.
//
// It calls PUT /repos/{owner}/{repo}/pulls/{number}/mergers with a body of
// {"mergers": "user1,user2"} and returns the resulting merger list.
func SetPRMergers(client *Client, owner, repo string, number int, users []string) ([]*PRMerger, error) {
	var mergers []*PRMerger
	err := client.Put(escapedRepoPath(owner, repo)+"/pulls/"+strconv.Itoa(number)+"/mergers",
		&MergerRequest{Mergers: strings.Join(users, ",")}, &mergers)
	if err != nil {
		return nil, err
	}
	return mergers, nil
}
