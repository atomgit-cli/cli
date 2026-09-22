package api

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
)

// Discussion represents a GitCode organization discussion (discuss).
//
// Field shapes follow the official GitCode OpenAPI
// (gc-api-doc: GET /api/v5/orgs/{org}/discuss and /discuss/{number}).
// Nested objects that the CLI does not need to interpret (category, namespace,
// vote, vote_options, labels) are kept as raw JSON so --json output passes
// them through verbatim.
type Discussion struct {
	ID            string          `json:"id"`
	Number        int             `json:"number"`
	Title         string          `json:"title"`
	MdContent     string          `json:"md_content,omitempty"` // detail only
	CreatedAt     string          `json:"created_at"`
	UpdatedAt     string          `json:"updated_at"`
	Author        *User           `json:"author"`
	IsLock        int             `json:"is_lock"`
	IsPin         int             `json:"is_pin"`
	IsCategoryPin int             `json:"is_category_pin"`
	IsClosed      int             `json:"is_closed"`
	IsAnswered    int             `json:"is_answered"`
	CommentTotal  int             `json:"comment_total"`
	Category      json.RawMessage `json:"category,omitempty"`
	Namespace     json.RawMessage `json:"namespace,omitempty"`
	Vote          json.RawMessage `json:"vote,omitempty"`         // detail only
	VoteOptions   json.RawMessage `json:"vote_options,omitempty"` // detail only
	Labels        json.RawMessage `json:"labels,omitempty"`
}

// ListOrgDiscussionsOptions filters and paginates ListOrgDiscussions.
type ListOrgDiscussionsOptions struct {
	Page      int
	PerPage   int
	Sort      string // created (default) or comment_size
	Direction string // asc or desc (default desc)
	Search    string // filter by title/description
}

// ListOrgDiscussions lists discussions in an organization.
//
// It calls GET /api/v5/orgs/{org}/discuss with optional pagination, sort,
// direction, and search query parameters (per the official OpenAPI).
func ListOrgDiscussions(client *Client, org string, opts *ListOrgDiscussionsOptions) ([]*Discussion, error) {
	endpoint := "/orgs/" + url.PathEscape(org) + "/discuss"
	if opts != nil {
		endpoint += newQueryBuilder().
			SetInt("page", opts.Page).
			SetInt("per_page", opts.PerPage).
			Set("sort", opts.Sort).
			Set("direction", opts.Direction).
			Set("search", opts.Search).
			String()
	}

	var discussions []*Discussion
	if err := client.Get(endpoint, &discussions); err != nil {
		return nil, fmt.Errorf("failed to list org discussions: %w", err)
	}
	return discussions, nil
}

// GetOrgDiscussion fetches a single organization discussion by number.
//
// It calls GET /api/v5/orgs/{org}/discuss/{number}.
func GetOrgDiscussion(client *Client, org string, number int) (*Discussion, error) {
	endpoint := "/orgs/" + url.PathEscape(org) + "/discuss/" + strconv.Itoa(number)

	var d Discussion
	if err := client.Get(endpoint, &d); err != nil {
		return nil, fmt.Errorf("failed to get org discussion: %w", err)
	}
	return &d, nil
}

// ListRepoDiscussions lists discussions in a repository (project-level
// discussions). It calls GET /api/v5/repos/{owner}/{repo}/discuss with the
// same optional pagination/sort/direction/search parameters as the org-level
// endpoint (per the official OpenAPI).
func ListRepoDiscussions(client *Client, owner, repo string, opts *ListOrgDiscussionsOptions) ([]*Discussion, error) {
	endpoint := "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) + "/discuss"
	if opts != nil {
		endpoint += newQueryBuilder().
			SetInt("page", opts.Page).
			SetInt("per_page", opts.PerPage).
			Set("sort", opts.Sort).
			Set("direction", opts.Direction).
			Set("search", opts.Search).
			String()
	}

	var discussions []*Discussion
	if err := client.Get(endpoint, &discussions); err != nil {
		return nil, fmt.Errorf("failed to list repo discussions: %w", err)
	}
	return discussions, nil
}

// GetRepoDiscussion fetches a single repository (project-level) discussion by
// number. It calls GET /api/v5/repos/{owner}/{repo}/discuss/{number}.
func GetRepoDiscussion(client *Client, owner, repo string, number int) (*Discussion, error) {
	endpoint := "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) + "/discuss/" + strconv.Itoa(number)

	var d Discussion
	if err := client.Get(endpoint, &d); err != nil {
		return nil, fmt.Errorf("failed to get repo discussion: %w", err)
	}
	return &d, nil
}

// DiscussionComment represents a GitCode discussion comment or comment reply.
// Both the comment-list and reply-list endpoints return the same shape (per
// the official OpenAPI), so a single struct covers both.
type DiscussionComment struct {
	ID         string `json:"id"`
	CreatedAt  string `json:"created_at"`
	Author     *User  `json:"author"`
	Content    string `json:"content"`
	MdContent  string `json:"md_content,omitempty"`
	IsHide     int    `json:"is_hide"`
	ReplyTotal int    `json:"reply_total"`
	LikeTotal  int    `json:"like_total"`
	IsLike     bool   `json:"is_like"`
	IsDeleted  int    `json:"is_deleted"`
	IsRemark   int    `json:"is_remark"`
}

// ListDiscussionCommentsOptions filters and paginates discussion comment
// listing. Order applies to comment lists only (replies do not accept it).
type ListDiscussionCommentsOptions struct {
	Page    int
	PerPage int
	Order   string // time_asc, time_desc, hot_desc (comments only)
}

// ListOrgDiscussionComments lists comments on an organization discussion.
// GET /api/v5/orgs/{org}/discuss/{number}/comment
func ListOrgDiscussionComments(client *Client, org string, number int, opts *ListDiscussionCommentsOptions) ([]*DiscussionComment, error) {
	endpoint := "/orgs/" + url.PathEscape(org) + "/discuss/" + strconv.Itoa(number) + "/comment"
	endpoint = appendDiscussionCommentParams(endpoint, opts)
	var comments []*DiscussionComment
	if err := client.Get(endpoint, &comments); err != nil {
		return nil, fmt.Errorf("failed to list org discussion comments: %w", err)
	}
	return comments, nil
}

// ListOrgDiscussionCommentReplies lists replies to a comment on an
// organization discussion.
// GET /api/v5/orgs/{org}/discuss/{number}/comment/{comment_id}/reply
func ListOrgDiscussionCommentReplies(client *Client, org string, number int, commentID string, opts *ListDiscussionCommentsOptions) ([]*DiscussionComment, error) {
	endpoint := "/orgs/" + url.PathEscape(org) + "/discuss/" + strconv.Itoa(number) + "/comment/" + url.PathEscape(commentID) + "/reply"
	endpoint = appendDiscussionCommentParams(endpoint, opts)
	var replies []*DiscussionComment
	if err := client.Get(endpoint, &replies); err != nil {
		return nil, fmt.Errorf("failed to list org discussion comment replies: %w", err)
	}
	return replies, nil
}

// ListRepoDiscussionComments lists comments on a repository (project-level)
// discussion. GET /api/v5/repos/{owner}/{repo}/discuss/{number}/comment
func ListRepoDiscussionComments(client *Client, owner, repo string, number int, opts *ListDiscussionCommentsOptions) ([]*DiscussionComment, error) {
	endpoint := "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) + "/discuss/" + strconv.Itoa(number) + "/comment"
	endpoint = appendDiscussionCommentParams(endpoint, opts)
	var comments []*DiscussionComment
	if err := client.Get(endpoint, &comments); err != nil {
		return nil, fmt.Errorf("failed to list repo discussion comments: %w", err)
	}
	return comments, nil
}

// ListRepoDiscussionCommentReplies lists replies to a comment on a repository
// (project-level) discussion.
// GET /api/v5/repos/{owner}/{repo}/discuss/{number}/comment/{comment_id}/reply
func ListRepoDiscussionCommentReplies(client *Client, owner, repo string, number int, commentID string, opts *ListDiscussionCommentsOptions) ([]*DiscussionComment, error) {
	endpoint := "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) + "/discuss/" + strconv.Itoa(number) + "/comment/" + url.PathEscape(commentID) + "/reply"
	endpoint = appendDiscussionCommentParams(endpoint, opts)
	var replies []*DiscussionComment
	if err := client.Get(endpoint, &replies); err != nil {
		return nil, fmt.Errorf("failed to list repo discussion comment replies: %w", err)
	}
	return replies, nil
}

// appendDiscussionCommentParams appends page/per_page/order query params when
// set. Order is included only when non-empty (callers for reply endpoints
// leave it empty since the API does not accept order there).
func appendDiscussionCommentParams(endpoint string, opts *ListDiscussionCommentsOptions) string {
	if opts == nil {
		return endpoint
	}
	return endpoint + newQueryBuilder().
		SetInt("page", opts.Page).
		SetInt("per_page", opts.PerPage).
		Set("order", opts.Order).
		String()
}

// --- Write operations (POST / PATCH / DELETE) ---

// CreateOrgDiscussionOptions specifies the parameters for creating an
// organization discussion. Per the official OpenAPI all fields are required.
type CreateOrgDiscussionOptions struct {
	Title        string `json:"title"`
	MdContent    string `json:"md_content,omitempty"`
	CategoryName string `json:"category_name,omitempty"`
}

// CreateOrgDiscussion creates a new discussion in an organization.
// POST /api/v5/orgs/{org}/discuss
func CreateOrgDiscussion(client *Client, org string, opts *CreateOrgDiscussionOptions) (*Discussion, error) {
	endpoint := "/orgs/" + url.PathEscape(org) + "/discuss"
	var d Discussion
	if err := client.Post(endpoint, opts, &d); err != nil {
		return nil, fmt.Errorf("failed to create org discussion: %w", err)
	}
	return &d, nil
}

// UpdateOrgDiscussionOptions specifies the parameters for updating an
// organization discussion. Per the official OpenAPI all fields are optional.
type UpdateOrgDiscussionOptions struct {
	Title        string `json:"title,omitempty"`
	MdContent    string `json:"md_content,omitempty"`
	CategoryName string `json:"category_name,omitempty"`
}

// UpdateOrgDiscussion updates an existing organization discussion.
// PUT /api/v5/orgs/{org}/discuss/{number}
func UpdateOrgDiscussion(client *Client, org string, number int, opts *UpdateOrgDiscussionOptions) (*Discussion, error) {
	endpoint := "/orgs/" + url.PathEscape(org) + "/discuss/" + strconv.Itoa(number)
	var d Discussion
	if err := client.Put(endpoint, opts, &d); err != nil {
		return nil, fmt.Errorf("failed to update org discussion: %w", err)
	}
	return &d, nil
}

// DeleteOrgDiscussion deletes an organization discussion.
// DELETE /api/v5/orgs/{org}/discuss/{number}
func DeleteOrgDiscussion(client *Client, org string, number int) error {
	endpoint := "/orgs/" + url.PathEscape(org) + "/discuss/" + strconv.Itoa(number)
	if err := client.Delete(endpoint); err != nil {
		return fmt.Errorf("failed to delete org discussion: %w", err)
	}
	return nil
}

// CreateOrgDiscussionCommentOptions specifies the parameters for creating a
// comment on an organization discussion.
type CreateOrgDiscussionCommentOptions struct {
	MdContent string `json:"md_content"`
}

// CreateOrgDiscussionComment creates a comment on an organization discussion.
// POST /api/v5/orgs/{org}/discuss/{number}/comment
func CreateOrgDiscussionComment(client *Client, org string, number int, opts *CreateOrgDiscussionCommentOptions) (*DiscussionComment, error) {
	endpoint := "/orgs/" + url.PathEscape(org) + "/discuss/" + strconv.Itoa(number) + "/comment"
	var c DiscussionComment
	if err := client.Post(endpoint, opts, &c); err != nil {
		return nil, fmt.Errorf("failed to create org discussion comment: %w", err)
	}
	return &c, nil
}

// UpdateOrgDiscussionCommentOptions specifies the parameters for updating a
// comment on an organization discussion.
type UpdateOrgDiscussionCommentOptions struct {
	MdContent string `json:"md_content"`
}

// UpdateOrgDiscussionComment updates a comment on an organization discussion.
// PUT /api/v5/orgs/{org}/discuss/{number}/comment/{id}
func UpdateOrgDiscussionComment(client *Client, org string, number int, commentID string, opts *UpdateOrgDiscussionCommentOptions) (*DiscussionComment, error) {
	endpoint := "/orgs/" + url.PathEscape(org) + "/discuss/" + strconv.Itoa(number) + "/comment/" + url.PathEscape(commentID)
	var c DiscussionComment
	if err := client.Put(endpoint, opts, &c); err != nil {
		return nil, fmt.Errorf("failed to update org discussion comment: %w", err)
	}
	return &c, nil
}

// DeleteOrgDiscussionComment deletes a comment on an organization discussion.
// DELETE /api/v5/orgs/{org}/discuss/{number}/comment/{id}
func DeleteOrgDiscussionComment(client *Client, org string, number int, commentID string) error {
	endpoint := "/orgs/" + url.PathEscape(org) + "/discuss/" + strconv.Itoa(number) + "/comment/" + url.PathEscape(commentID)
	if err := client.Delete(endpoint); err != nil {
		return fmt.Errorf("failed to delete org discussion comment: %w", err)
	}
	return nil
}

// ReplyOrgDiscussionCommentOptions specifies the parameters for replying to a
// comment on an organization discussion.
type ReplyOrgDiscussionCommentOptions struct {
	MdContent string `json:"md_content"`
}

// ReplyOrgDiscussionComment replies to a comment on an organization discussion.
// POST /api/v5/orgs/{org}/discuss/{number}/comment/{id}/reply
func ReplyOrgDiscussionComment(client *Client, org string, number int, commentID string, opts *ReplyOrgDiscussionCommentOptions) (*DiscussionComment, error) {
	endpoint := "/orgs/" + url.PathEscape(org) + "/discuss/" + strconv.Itoa(number) + "/comment/" + url.PathEscape(commentID) + "/reply"
	var c DiscussionComment
	if err := client.Post(endpoint, opts, &c); err != nil {
		return nil, fmt.Errorf("failed to reply org discussion comment: %w", err)
	}
	return &c, nil
}

// CreateRepoDiscussionOptions specifies the parameters for creating a
// repository discussion.
type CreateRepoDiscussionOptions struct {
	Title     string `json:"title"`
	MdContent string `json:"md_content,omitempty"`
}

// CreateRepoDiscussion creates a new discussion in a repository.
// POST /api/v5/repos/{owner}/{repo}/discuss
func CreateRepoDiscussion(client *Client, owner, repo string, opts *CreateRepoDiscussionOptions) (*Discussion, error) {
	endpoint := "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) + "/discuss"
	var d Discussion
	if err := client.Post(endpoint, opts, &d); err != nil {
		return nil, fmt.Errorf("failed to create repo discussion: %w", err)
	}
	return &d, nil
}

// UpdateRepoDiscussionOptions specifies the parameters for updating a
// repository discussion.
type UpdateRepoDiscussionOptions struct {
	Title     string `json:"title,omitempty"`
	MdContent string `json:"md_content,omitempty"`
}

// UpdateRepoDiscussion updates an existing repository discussion.
// PATCH /api/v5/repos/{owner}/{repo}/discuss/{number}
func UpdateRepoDiscussion(client *Client, owner, repo string, number int, opts *UpdateRepoDiscussionOptions) (*Discussion, error) {
	endpoint := "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) + "/discuss/" + strconv.Itoa(number)
	var d Discussion
	if err := client.Patch(endpoint, opts, &d); err != nil {
		return nil, fmt.Errorf("failed to update repo discussion: %w", err)
	}
	return &d, nil
}

// DeleteRepoDiscussion deletes a repository discussion.
// DELETE /api/v5/repos/{owner}/{repo}/discuss/{number}
func DeleteRepoDiscussion(client *Client, owner, repo string, number int) error {
	endpoint := "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) + "/discuss/" + strconv.Itoa(number)
	if err := client.Delete(endpoint); err != nil {
		return fmt.Errorf("failed to delete repo discussion: %w", err)
	}
	return nil
}

// CreateRepoDiscussionCommentOptions specifies the parameters for creating a
// comment on a repository discussion.
type CreateRepoDiscussionCommentOptions struct {
	MdContent string `json:"md_content"`
}

// CreateRepoDiscussionComment creates a comment on a repository discussion.
// POST /api/v5/repos/{owner}/{repo}/discuss/{number}/comment
func CreateRepoDiscussionComment(client *Client, owner, repo string, number int, opts *CreateRepoDiscussionCommentOptions) (*DiscussionComment, error) {
	endpoint := "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) + "/discuss/" + strconv.Itoa(number) + "/comment"
	var c DiscussionComment
	if err := client.Post(endpoint, opts, &c); err != nil {
		return nil, fmt.Errorf("failed to create repo discussion comment: %w", err)
	}
	return &c, nil
}

// UpdateRepoDiscussionCommentOptions specifies the parameters for updating a
// comment on a repository discussion.
type UpdateRepoDiscussionCommentOptions struct {
	MdContent string `json:"md_content"`
}

// UpdateRepoDiscussionComment updates a comment on a repository discussion.
// PATCH /api/v5/repos/{owner}/{repo}/discuss/{number}/comment/{id}
func UpdateRepoDiscussionComment(client *Client, owner, repo string, number int, commentID string, opts *UpdateRepoDiscussionCommentOptions) (*DiscussionComment, error) {
	endpoint := "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) + "/discuss/" + strconv.Itoa(number) + "/comment/" + url.PathEscape(commentID)
	var c DiscussionComment
	if err := client.Patch(endpoint, opts, &c); err != nil {
		return nil, fmt.Errorf("failed to update repo discussion comment: %w", err)
	}
	return &c, nil
}

// DeleteRepoDiscussionComment deletes a comment on a repository discussion.
// DELETE /api/v5/repos/{owner}/{repo}/discuss/{number}/comment/{id}
func DeleteRepoDiscussionComment(client *Client, owner, repo string, number int, commentID string) error {
	endpoint := "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) + "/discuss/" + strconv.Itoa(number) + "/comment/" + url.PathEscape(commentID)
	if err := client.Delete(endpoint); err != nil {
		return fmt.Errorf("failed to delete repo discussion comment: %w", err)
	}
	return nil
}

// ReplyRepoDiscussionCommentOptions specifies the parameters for replying to a
// comment on a repository discussion.
type ReplyRepoDiscussionCommentOptions struct {
	MdContent string `json:"md_content"`
}

// ReplyRepoDiscussionComment replies to a comment on a repository discussion.
// POST /api/v5/repos/{owner}/{repo}/discuss/{number}/comment/{id}/reply
func ReplyRepoDiscussionComment(client *Client, owner, repo string, number int, commentID string, opts *ReplyRepoDiscussionCommentOptions) (*DiscussionComment, error) {
	endpoint := "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) + "/discuss/" + strconv.Itoa(number) + "/comment/" + url.PathEscape(commentID) + "/reply"
	var c DiscussionComment
	if err := client.Post(endpoint, opts, &c); err != nil {
		return nil, fmt.Errorf("failed to reply repo discussion comment: %w", err)
	}
	return &c, nil
}
