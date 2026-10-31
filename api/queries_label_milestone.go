package api

import (
	"net/url"
	"strconv"
)

// CreateLabelOptions represents options for creating a label
type CreateLabelOptions struct {
	Name        string `json:"name"`
	Color       string `json:"color,omitempty"`
	Description string `json:"description,omitempty"`
}

// UpdateLabelOptions represents options for updating a label
type UpdateLabelOptions struct {
	NewName     string `json:"new_name,omitempty"`
	Color       string `json:"color,omitempty"`
	Description string `json:"description,omitempty"`
}

// CreateMilestoneOptions represents options for creating a milestone
type CreateMilestoneOptions struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	State       string `json:"state,omitempty"`
	DueOn       string `json:"due_on,omitempty"`
}

// UpdateMilestoneOptions represents options for updating a milestone
type UpdateMilestoneOptions struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	State       string `json:"state,omitempty"`
	DueOn       string `json:"due_on,omitempty"`
}

// CreateLabel creates a new label in a repository
func CreateLabel(client *Client, owner, repo string, opts *CreateLabelOptions) (*Label, error) {
	var label Label
	// GitCode API requires query parameters, not JSON body
	path := escapedRepoPath(owner, repo) + "/labels?name=" + url.QueryEscape(opts.Name)
	if opts.Color != "" {
		path += "&color=" + url.QueryEscape(opts.Color)
	}
	if opts.Description != "" {
		path += "&description=" + url.QueryEscape(opts.Description)
	}
	err := client.Post(path, nil, &label)
	if err != nil {
		return nil, err
	}
	return &label, nil
}

// UpdateLabel updates an existing label.
//
// Per api-doc/openapi.json, PATCH /repos/{owner}/{repo}/labels/{original_name}
// takes name and color as query parameters with no request body; the endpoint
// does not support updating description.
func UpdateLabel(client *Client, owner, repo, name string, opts *UpdateLabelOptions) (*Label, error) {
	escapedName := url.PathEscape(name)
	path := escapedRepoPath(owner, repo) + "/labels/" + escapedName
	q := url.Values{}
	if opts.NewName != "" {
		q.Set("name", opts.NewName)
	}
	if opts.Color != "" {
		q.Set("color", opts.Color)
	}
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}
	var label Label
	err := client.Patch(path, nil, &label)
	if err != nil {
		return nil, err
	}
	return &label, nil
}

// DeleteLabel deletes a label
func DeleteLabel(client *Client, owner, repo, name string) error {
	escapedName := url.PathEscape(name)
	return client.Delete(escapedRepoPath(owner, repo) + "/labels/" + escapedName)
}

// AddLabelsToIssue adds labels to an issue
func AddLabelsToIssue(client *Client, owner, repo string, number int, labels []string) ([]Label, error) {
	var result []Label
	err := client.Post(escapedRepoPath(owner, repo)+"/issues/"+strconv.Itoa(number)+"/labels", map[string]interface{}{
		"labels": labels,
	}, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// RemoveLabelFromIssue removes a label from an issue
func RemoveLabelFromIssue(client *Client, owner, repo string, number int, label string) error {
	escapedLabel := url.PathEscape(label)
	return client.Delete(escapedRepoPath(owner, repo) + "/issues/" + strconv.Itoa(number) + "/labels/" + escapedLabel)
}

// SetIssueLabels sets (replaces) labels on an issue
func SetIssueLabels(client *Client, owner, repo string, number int, labels []string) ([]Label, error) {
	var result []Label
	err := client.Put(escapedRepoPath(owner, repo)+"/issues/"+strconv.Itoa(number)+"/labels", map[string]interface{}{
		"labels": labels,
	}, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ClearIssueLabels removes all labels from an issue
func ClearIssueLabels(client *Client, owner, repo string, number int) error {
	return client.Delete(escapedRepoPath(owner, repo) + "/issues/" + strconv.Itoa(number) + "/labels")
}

// CreateMilestone creates a new milestone
func CreateMilestone(client *Client, owner, repo string, opts *CreateMilestoneOptions) (*Milestone, error) {
	var milestone Milestone
	err := client.Post(escapedRepoPath(owner, repo)+"/milestones", opts, &milestone)
	if err != nil {
		return nil, err
	}
	return &milestone, nil
}

// GetMilestone fetches a milestone by number
func GetMilestone(client *Client, owner, repo string, number int) (*Milestone, error) {
	var milestone Milestone
	err := client.Get(escapedRepoPath(owner, repo)+"/milestones/"+strconv.Itoa(number), &milestone)
	if err != nil {
		return nil, err
	}
	return &milestone, nil
}

// UpdateMilestone updates an existing milestone
func UpdateMilestone(client *Client, owner, repo string, number int, opts *UpdateMilestoneOptions) (*Milestone, error) {
	var milestone Milestone
	err := client.Patch(escapedRepoPath(owner, repo)+"/milestones/"+strconv.Itoa(number), opts, &milestone)
	if err != nil {
		return nil, err
	}
	return &milestone, nil
}

// DeleteMilestone deletes a milestone
func DeleteMilestone(client *Client, owner, repo string, number int) error {
	return client.Delete(escapedRepoPath(owner, repo) + "/milestones/" + strconv.Itoa(number))
}

// CloseMilestone closes a milestone
func CloseMilestone(client *Client, owner, repo string, number int) (*Milestone, error) {
	return UpdateMilestone(client, owner, repo, number, &UpdateMilestoneOptions{State: "closed"})
}

// OpenMilestone reopens a milestone
func OpenMilestone(client *Client, owner, repo string, number int) (*Milestone, error) {
	return UpdateMilestone(client, owner, repo, number, &UpdateMilestoneOptions{State: "open"})
}
