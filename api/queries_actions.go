package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
)

// ActionsActor represents the actor who triggered a workflow run.
type ActionsActor struct {
	ID       string `json:"id"`
	ObjectID string `json:"object_id"`
	Login    string `json:"login"`
	Name     string `json:"name"`
}

// WorkflowRun represents a single pipeline run record.
type WorkflowRun struct {
	WorkflowRunID string        `json:"workflow_run_id"`
	WorkflowID    string        `json:"workflow_id"`
	WorkflowName  string        `json:"workflow_name"`
	FilePath      string        `json:"file_path"`
	Title         string        `json:"title"`
	Status        string        `json:"status"`
	Event         string        `json:"event"`
	RunNumber     int           `json:"run_number"`
	HeadBranch    string        `json:"head_branch"`
	HeadSHA       string        `json:"head_sha"`
	Actor         *ActionsActor `json:"actor"`
	StartTime     int64         `json:"start_time"`
	EndTime       int64         `json:"end_time"`
	PauseTime     int64         `json:"pause_time"`
}

// WorkflowRunsResponse represents the response from listing workflow runs.
type WorkflowRunsResponse struct {
	TotalCount   int           `json:"total_count"`
	WorkflowRuns []WorkflowRun `json:"workflow_runs"`
}

// ActionsListRunsOptions represents the filter options for listing workflow runs.
//
// The fields mirror the query parameters accepted by the GitCode Actions v8 API.
// The optional access_token query parameter is intentionally omitted: the CLI
// authenticates through the standard Bearer header instead of exposing the
// token in the request URL.
type ActionsListRunsOptions struct {
	Event         string
	Status        string
	Branch        string
	Executor      string
	PullRequestID string
	WorkflowID    string
	WorkflowName  string
	PerPage       int
	Page          int
	StartTime     int64
	EndTime       int64
}

// ListActionsRuns lists all pipeline run records for a repository.
//
// It calls the GitCode Actions v8 endpoint
// GET /api/v8/repos/{owner}/{repo}/actions/runs. Unlike most queries that use
// the default v5 prefix, the Actions API lives under v8, so the request is
// issued via RawREST with a full /api/v8 path which the client preserves
// verbatim (see Client.rawURL).
func ListActionsRuns(client *Client, owner, repo string, opts *ActionsListRunsOptions) (*WorkflowRunsResponse, error) {
	endpoint := "/api/v8/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) + "/actions/runs"
	if opts != nil {
		endpoint += newQueryBuilder().
			Set("event", opts.Event).
			Set("status", opts.Status).
			Set("branch", opts.Branch).
			Set("executor", opts.Executor).
			Set("pull_request_id", opts.PullRequestID).
			Set("workflow_id", opts.WorkflowID).
			Set("workflow_name", opts.WorkflowName).
			SetInt("per_page", opts.PerPage).
			SetInt("page", opts.Page).
			SetInt64("startTime", opts.StartTime).
			SetInt64("endTime", opts.EndTime).
			String()
	}

	resp, err := client.RawREST("GET", endpoint, nil, nil)
	if err != nil {
		return nil, err
	}

	var result WorkflowRunsResponse
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse workflow runs response: %w", err)
	}
	return &result, nil
}

// WorkflowRunStep represents a single step within a workflow job.
type WorkflowRunStep struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Task           string `json:"task"`
	Identifier     string `json:"identifier"`
	Status         string `json:"status"`
	Sequence       int    `json:"sequence"`
	JobRunID       string `json:"job_run_id"`
	LastDispatchID string `json:"last_dispatch_id"`
	StartTime      int64  `json:"start_time"`
	EndTime        int64  `json:"end_time"`
}

// WorkflowRunJob represents a job within a workflow run stage.
type WorkflowRunJob struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Identifier      string            `json:"identifier"`
	Status          string            `json:"status"`
	Sequence        int               `json:"sequence"`
	JobType         string            `json:"job_type"`
	Resource        string            `json:"resource"`
	Condition       string            `json:"condition"`
	IsSelect        bool              `json:"is_select"`
	DependsOn       []string          `json:"depends_on"`
	StartTime       int64             `json:"start_time"`
	EndTime         int64             `json:"end_time"`
	ExecuteCostTime int64             `json:"execute_cost_time"`
	ExecID          string            `json:"exec_id"`
	LastDispatchID  string            `json:"last_dispatch_id"`
	Steps           []WorkflowRunStep `json:"steps"`
}

// WorkflowRunStage represents a stage within a workflow run.
type WorkflowRunStage struct {
	ID         string           `json:"id"`
	Category   string           `json:"category"`
	Name       string           `json:"name"`
	Identifier string           `json:"identifier"`
	Status     string           `json:"status"`
	Sequence   int              `json:"sequence"`
	RunAlways  bool             `json:"run_always"`
	FailFast   bool             `json:"fail_fast"`
	IsSelect   bool             `json:"is_select"`
	DependsOn  []string         `json:"depends_on"`
	StartTime  int64            `json:"start_time"`
	EndTime    int64            `json:"end_time"`
	PauseTime  int64            `json:"pause_time"`
	Jobs       []WorkflowRunJob `json:"jobs"`
}

// WorkflowRunDetail represents the detail of a single pipeline run.
//
// It embeds WorkflowRun for the base run fields and adds the detail-only
// fields (exist_in_default_branch and stages). Only the fields needed for the
// human-facing view are modeled; deep or nullable execution fields (pre/post
// hooks, parallel, condition, inputs, env, ...) are left unmodeled and
// preserved verbatim through the raw response returned alongside this struct
// for faithful --json output.
type WorkflowRunDetail struct {
	WorkflowRun
	ExistInDefaultBranch bool               `json:"exist_in_default_branch"`
	Stages               []WorkflowRunStage `json:"stages"`
}

// GetActionsRun fetches the detail of a single pipeline run.
//
// It calls GET /api/v8/repos/{owner}/{repo}/actions/runs/{run_id}. It returns
// both a typed detail (for the human-facing view) and the raw response body
// (for faithful --json output that preserves every API field verbatim,
// including the deep stage/job/step execution fields that the typed struct
// does not model).
func GetActionsRun(client *Client, owner, repo, runID string) (*WorkflowRunDetail, json.RawMessage, error) {
	endpoint := "/api/v8/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) + "/actions/runs/" + url.PathEscape(runID)

	resp, err := client.RawREST("GET", endpoint, nil, nil)
	if err != nil {
		return nil, nil, err
	}

	var detail WorkflowRunDetail
	if err := json.Unmarshal(resp.Body, &detail); err != nil {
		return nil, nil, fmt.Errorf("failed to parse workflow run detail response: %w", err)
	}
	return &detail, resp.Body, nil
}

// StopActionsRun stops a running pipeline run.
//
// It calls POST /api/v8/repos/{owner}/{repo}/actions/runs/{run_id}/stop.
// The endpoint is idempotent: stopping a run that has already finished still
// returns success.
func StopActionsRun(client *Client, owner, repo, runID string) error {
	endpoint := "/api/v8/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) + "/actions/runs/" + url.PathEscape(runID) + "/stop"

	_, err := client.RawREST("POST", endpoint, nil, nil)
	return err
}

// WorkflowRunJobsResponse represents the response from listing the jobs of a
// workflow run.
type WorkflowRunJobsResponse struct {
	TotalCount int              `json:"total_count"`
	Jobs       []WorkflowRunJob `json:"jobs"`
}

// ListActionsRunJobs lists the jobs of a single pipeline run.
//
// It calls GET /api/v8/repos/{owner}/{repo}/actions/runs/{run_id}/jobs. The
// job items share the WorkflowRunJob type used by the run detail response
// (the job shape is identical in both endpoints).
func ListActionsRunJobs(client *Client, owner, repo, runID string) (*WorkflowRunJobsResponse, error) {
	endpoint := "/api/v8/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) + "/actions/runs/" + url.PathEscape(runID) + "/jobs"

	resp, err := client.RawREST("GET", endpoint, nil, nil)
	if err != nil {
		return nil, err
	}

	var result WorkflowRunJobsResponse
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse workflow run jobs response: %w", err)
	}
	return &result, nil
}

// GetActionsJob fetches the detail of a single workflow job.
//
// It calls GET /api/v8/repos/{owner}/{repo}/actions/runs/{run_id}/jobs/{job_id}.
// It returns both the typed job (for the human-facing view) and the raw
// response body (for faithful --json output that preserves the deep step
// execution fields the typed struct does not model), mirroring GetActionsRun.
func GetActionsJob(client *Client, owner, repo, runID, jobID string) (*WorkflowRunJob, json.RawMessage, error) {
	endpoint := "/api/v8/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) + "/actions/runs/" + url.PathEscape(runID) + "/jobs/" + url.PathEscape(jobID)

	resp, err := client.RawREST("GET", endpoint, nil, nil)
	if err != nil {
		return nil, nil, err
	}

	var job WorkflowRunJob
	if err := json.Unmarshal(resp.Body, &job); err != nil {
		return nil, nil, fmt.Errorf("failed to parse workflow job response: %w", err)
	}
	return &job, resp.Body, nil
}

// GetActionsJobLog downloads the log of a single workflow job.
//
// It calls GET /api/v8/repos/{owner}/{repo}/actions/runs/{run_id}/jobs/{job_id}/download_log.
// The endpoint returns the raw log content (Content-Type */*, no JSON schema),
// so this returns the response body bytes verbatim. Accept is set to */* so
// the server returns the log rather than a JSON representation.
func GetActionsJobLog(client *Client, owner, repo, runID, jobID string) ([]byte, error) {
	endpoint := "/api/v8/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) + "/actions/runs/" + url.PathEscape(runID) + "/jobs/" + url.PathEscape(jobID) + "/download_log"

	resp, err := client.RawREST("GET", endpoint, nil, map[string]string{"Accept": "*/*"})
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// Artifact represents a workflow run artifact.
type Artifact struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	SizeBytes     int64  `json:"size_bytes"`
	WorkflowID    string `json:"workflow_id"`
	WorkflowRunID string `json:"workflow_run_id"`
	Digest        string `json:"digest"`
	ExpiresAt     string `json:"expires_at"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

// ArtifactsResponse represents the response from listing artifacts.
type ArtifactsResponse struct {
	TotalCount int        `json:"total_count"`
	Artifacts  []Artifact `json:"artifacts"`
}

// ActionsListArtifactsOptions represents the filter options for listing artifacts.
//
// The fields mirror the query parameters accepted by the GitCode Actions v8
// artifacts endpoint (name filter, sort/direction, pagination). The optional
// access_token query parameter is intentionally omitted: the CLI authenticates
// through the standard Bearer header.
type ActionsListArtifactsOptions struct {
	Name      string
	Sort      string
	Direction string
	Page      int
	PerPage   int
}

// ListActionsArtifacts lists the artifacts of a repository.
//
// It calls GET /api/v8/repos/{owner}/{repo}/actions/artifacts. The endpoint
// supports name filtering, sort (created) and direction, plus pagination.
func ListActionsArtifacts(client *Client, owner, repo string, opts *ActionsListArtifactsOptions) (*ArtifactsResponse, error) {
	endpoint := "/api/v8/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) + "/actions/artifacts"
	if opts != nil {
		endpoint += newQueryBuilder().
			Set("name", opts.Name).
			Set("sort", opts.Sort).
			Set("direction", opts.Direction).
			SetInt("per_page", opts.PerPage).
			SetInt("page", opts.Page).
			String()
	}

	resp, err := client.RawREST("GET", endpoint, nil, nil)
	if err != nil {
		return nil, err
	}
	var result ArtifactsResponse
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse artifacts response: %w", err)
	}
	return &result, nil
}

// ListActionsRunArtifacts lists the artifacts produced by a specific run.
//
// It calls GET /api/v8/repos/{owner}/{repo}/actions/runs/{run_id}/artifacts.
// The response shape (Artifact items, total_count) is identical to the
// repository-level artifacts endpoint, so it reuses ArtifactsResponse and
// ActionsListArtifactsOptions.
func ListActionsRunArtifacts(client *Client, owner, repo, runID string, opts *ActionsListArtifactsOptions) (*ArtifactsResponse, error) {
	endpoint := "/api/v8/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) + "/actions/runs/" + url.PathEscape(runID) + "/artifacts"
	if opts != nil {
		endpoint += newQueryBuilder().
			Set("name", opts.Name).
			Set("sort", opts.Sort).
			Set("direction", opts.Direction).
			SetInt("per_page", opts.PerPage).
			SetInt("page", opts.Page).
			String()
	}

	resp, err := client.RawREST("GET", endpoint, nil, nil)
	if err != nil {
		return nil, err
	}
	var result ArtifactsResponse
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse run artifacts response: %w", err)
	}
	return &result, nil
}

// GetActionsArtifact fetches the detail of a single artifact.
//
// It calls GET /api/v8/repos/{owner}/{repo}/actions/artifacts/{artifact_id}.
// It returns both the typed artifact (for the human-facing view) and the raw
// response body (for faithful --json output), mirroring GetActionsRun/GetActionsJob.
func GetActionsArtifact(client *Client, owner, repo, artifactID string) (*Artifact, json.RawMessage, error) {
	endpoint := "/api/v8/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) + "/actions/artifacts/" + url.PathEscape(artifactID)

	resp, err := client.RawREST("GET", endpoint, nil, nil)
	if err != nil {
		return nil, nil, err
	}

	var artifact Artifact
	if err := json.Unmarshal(resp.Body, &artifact); err != nil {
		return nil, nil, fmt.Errorf("failed to parse artifact response: %w", err)
	}
	return &artifact, resp.Body, nil
}

// DownloadActionsArtifact downloads an artifact as a ZIP archive.
//
// It calls GET /api/v8/repos/{owner}/{repo}/actions/artifacts/{artifact_id}/zip.
// The endpoint returns a 302 redirect to a pre-signed download URL; the HTTP
// client follows the redirect automatically. The response body is the raw ZIP
// archive bytes (binary, not JSON).
func DownloadActionsArtifact(client *Client, owner, repo, artifactID string) ([]byte, error) {
	endpoint := "/api/v8/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) + "/actions/artifacts/" + url.PathEscape(artifactID) + "/zip"

	resp, err := client.RawREST("GET", endpoint, nil, map[string]string{"Accept": "*/*"})
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// DeleteActionsArtifact deletes a single artifact.
//
// It calls DELETE /api/v8/repos/{owner}/{repo}/actions/artifacts/{artifact_id}.
// The endpoint returns 204 No Content on success.
func DeleteActionsArtifact(client *Client, owner, repo, artifactID string) error {
	endpoint := "/api/v8/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) + "/actions/artifacts/" + url.PathEscape(artifactID)

	_, err := client.RawREST("DELETE", endpoint, nil, nil)
	return err
}

// ValidateWorkflowRequest is the JSON request body for the workflow validate endpoint.
type ValidateWorkflowRequest struct {
	Base64Content string `json:"base64_content"`
}

// WorkflowPosition is a line/column position in a workflow YAML file.
type WorkflowPosition struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

// WorkflowDiagnosticRange is a line/column range reported by the validator.
type WorkflowDiagnosticRange struct {
	Start WorkflowPosition `json:"start"`
	End   WorkflowPosition `json:"end"`
}

// WorkflowDiagnostic is a single validation failure cause.
type WorkflowDiagnostic struct {
	Range    WorkflowDiagnosticRange `json:"range"`
	Severity string                  `json:"severity"`
	Message  string                  `json:"message"`
}

// ValidateWorkflowResponse is the response from the workflow validate endpoint.
type ValidateWorkflowResponse struct {
	Valid       bool                 `json:"valid"`
	Diagnostics []WorkflowDiagnostic `json:"diagnostics"`
}

// ValidateActionsWorkflow validates workflow YAML content through the Actions
// v8 validate endpoint.
//
// It calls POST /api/v8/repos/{owner}/{repo}/actions/workflows/validate and
// base64-encodes the content into the required base64_content field.
// Authentication uses the standard Bearer header; no token is placed in the
// query string.
func ValidateActionsWorkflow(client *Client, owner, repo string, content []byte) (*ValidateWorkflowResponse, json.RawMessage, error) {
	endpoint := "/api/v8/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) + "/actions/workflows/validate"

	body, err := json.Marshal(ValidateWorkflowRequest{
		Base64Content: base64.StdEncoding.EncodeToString(content),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal validate request: %w", err)
	}

	resp, err := client.RawREST("POST", endpoint, bytes.NewReader(body), nil)
	if err != nil {
		return nil, nil, err
	}

	var result ValidateWorkflowResponse
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, nil, fmt.Errorf("failed to parse validate response: %w", err)
	}
	return &result, resp.Body, nil
}

// RunnerGroup represents a single Actions runner group in an organization.
type RunnerGroup struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	RunnerGroupName string `json:"runner_group_name"`
	NamespaceID     string `json:"namespace_id"`
	Creator         string `json:"creator"`
	CreateTime      int64  `json:"create_time"`
	RunnerCount     int    `json:"runner_count"`
	NamespaceType   string `json:"namespace_type"`
	ShareAll        bool   `json:"share_all"`
}

// RunnerGroupsResponse represents the response from listing org runner groups.
type RunnerGroupsResponse struct {
	TotalCount   int           `json:"total_count"`
	RunnerGroups []RunnerGroup `json:"runner_groups"`
}

// ListOrgRunnerGroupsOptions represents the filter options for listing runner groups.
//
// The fields mirror the query parameters accepted by the GitCode Actions v8
// endpoint GET /api/v8/orgs/{org}/actions/runner-groups. The optional
// access_token query parameter is intentionally omitted: the CLI authenticates
// through the standard Bearer header.
type ListOrgRunnerGroupsOptions struct {
	Keyword string
	Page    int
	PerPage int
}

// ListOrgRunnerGroups lists all runner groups in an organization.
//
// It calls GET /api/v8/orgs/{org}/actions/runner-groups. Unlike most queries
// that use the default v5 prefix, the Actions API lives under v8, so the
// request is issued via RawREST with a full /api/v8 path.
func ListOrgRunnerGroups(client *Client, org string, opts *ListOrgRunnerGroupsOptions) (*RunnerGroupsResponse, error) {
	endpoint := "/api/v8/orgs/" + url.PathEscape(org) + "/actions/runner-groups"
	if opts != nil {
		endpoint += newQueryBuilder().
			Set("keyword", opts.Keyword).
			SetInt("per_page", opts.PerPage).
			SetInt("page", opts.Page).
			String()
	}

	resp, err := client.RawREST("GET", endpoint, nil, nil)
	if err != nil {
		return nil, err
	}
	var result RunnerGroupsResponse
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse runner groups response: %w", err)
	}
	return &result, nil
}

// RunnerGroupDetail represents the detailed information of a single runner group.
type RunnerGroupDetail struct {
	RunnerGroupID           string `json:"runner_group_id"`
	RunnerGroupName         string `json:"runner_group_name"`
	ShareAll                bool   `json:"share_all"`
	ShareAllPublicRepos     bool   `json:"share_all_public_repos"`
	ExplicitSharedRepoCount int    `json:"explicit_shared_repo_count"`
	CreatedAt               int64  `json:"created_at"`
	UpdatedAt               int64  `json:"updated_at"`
}

// GetOrgRunnerGroup fetches the detail of a single runner group.
//
// It calls GET /api/v8/orgs/{org}/actions/runner-groups/{runner_group_id}.
// It returns both the typed detail (for the human-facing view) and the raw
// response body (for faithful --json output), mirroring GetActionsArtifact.
func GetOrgRunnerGroup(client *Client, org, runnerGroupID string) (*RunnerGroupDetail, json.RawMessage, error) {
	endpoint := "/api/v8/orgs/" + url.PathEscape(org) + "/actions/runner-groups/" + url.PathEscape(runnerGroupID)

	resp, err := client.RawREST("GET", endpoint, nil, nil)
	if err != nil {
		return nil, nil, err
	}

	var detail RunnerGroupDetail
	if err := json.Unmarshal(resp.Body, &detail); err != nil {
		return nil, nil, fmt.Errorf("failed to parse runner group response: %w", err)
	}
	return &detail, resp.Body, nil
}

// RunnerLabel represents a label attached to a runner.
type RunnerLabel struct {
	LabelName  string `json:"label_name"`
	LabelValue string `json:"label_value"`
	LabelColor string `json:"label_color"`
}

// Runner represents a single host runner in a runner group.
type Runner struct {
	ID            string        `json:"id"`
	RunnerGroupID string        `json:"runner_group_id"`
	RunnerName    string        `json:"runner_name"`
	Name          string        `json:"name"`
	WorkDir       string        `json:"work_dir"`
	Labels        []RunnerLabel `json:"labels"`
}

// RunnersResponse represents the response from listing runners in a runner group.
type RunnersResponse struct {
	TotalCount int      `json:"total_count"`
	Runners    []Runner `json:"runners"`
}

// ListRunnerGroupRunnersOptions represents the filter options for listing runners.
//
// The fields mirror the query parameters accepted by the GitCode Actions v8
// endpoint GET /api/v8/orgs/{org}/actions/runner-groups/{runner_group_id}/runners.
// The optional access_token query parameter is intentionally omitted.
type ListRunnerGroupRunnersOptions struct {
	Keyword string
	Page    int
	PerPage int
}

// ListRunnerGroupRunners lists all host runners in a runner group.
//
// It calls GET /api/v8/orgs/{org}/actions/runner-groups/{runner_group_id}/runners.
func ListRunnerGroupRunners(client *Client, org, runnerGroupID string, opts *ListRunnerGroupRunnersOptions) (*RunnersResponse, error) {
	endpoint := "/api/v8/orgs/" + url.PathEscape(org) + "/actions/runner-groups/" + url.PathEscape(runnerGroupID) + "/runners"
	if opts != nil {
		endpoint += newQueryBuilder().
			Set("keyword", opts.Keyword).
			SetInt("per_page", opts.PerPage).
			SetInt("page", opts.Page).
			String()
	}

	resp, err := client.RawREST("GET", endpoint, nil, nil)
	if err != nil {
		return nil, err
	}
	var result RunnersResponse
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse runners response: %w", err)
	}
	return &result, nil
}

// ListRepoRunners lists all host runners in a repository.
//
// It calls GET /api/v8/repos/{owner}/{repo}/actions/runners. The response
// shape (RunnersResponse with Runner items) is identical to the org-level
// runner-group runners endpoint.
func ListRepoRunners(client *Client, owner, repo string, opts *ListRunnerGroupRunnersOptions) (*RunnersResponse, error) {
	endpoint := "/api/v8/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) + "/actions/runners"
	if opts != nil {
		endpoint += newQueryBuilder().
			Set("keyword", opts.Keyword).
			SetInt("per_page", opts.PerPage).
			SetInt("page", opts.Page).
			String()
	}

	resp, err := client.RawREST("GET", endpoint, nil, nil)
	if err != nil {
		return nil, err
	}
	var result RunnersResponse
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse repo runners response: %w", err)
	}
	return &result, nil
}

// RunnerSet represents a single K8S runner set in a runner group.
type RunnerSet struct {
	ID             string        `json:"id"`
	RunnerGroupID  string        `json:"runner_group_id"`
	Name           string        `json:"name"`
	Status         string        `json:"status"`
	RequiredLabels []RunnerLabel `json:"required_labels"`
}

// RunnerSetsResponse represents the response from listing K8S runner sets.
type RunnerSetsResponse struct {
	TotalCount int         `json:"total_count"`
	RunnerSets []RunnerSet `json:"runner_sets"`
}

// ListRunnerGroupRunnerSets lists all K8S runner sets in a runner group.
//
// It calls GET /api/v8/orgs/{org}/actions/runner-groups/{runner_group_id}/runner-sets.
func ListRunnerGroupRunnerSets(client *Client, org, runnerGroupID string, opts *ListRunnerGroupRunnersOptions) (*RunnerSetsResponse, error) {
	endpoint := "/api/v8/orgs/" + url.PathEscape(org) + "/actions/runner-groups/" + url.PathEscape(runnerGroupID) + "/runner-sets"
	if opts != nil {
		endpoint += newQueryBuilder().
			Set("keyword", opts.Keyword).
			SetInt("per_page", opts.PerPage).
			SetInt("page", opts.Page).
			String()
	}

	resp, err := client.RawREST("GET", endpoint, nil, nil)
	if err != nil {
		return nil, err
	}
	var result RunnerSetsResponse
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse runner sets response: %w", err)
	}
	return &result, nil
}

// SharedNamespace represents a namespace sharing record for a runner group.
type SharedNamespace struct {
	ID              string `json:"id"`
	RunnerGroupID   string `json:"runner_group_id"`
	FromNamespaceID string `json:"from_namespace_id"`
	ToNamespaceID   string `json:"to_namespace_id"`
	Type            string `json:"type"`
	CreateTime      int64  `json:"create_time"`
	UpdateTime      int64  `json:"update_time"`
}

// SharedNamespacesResponse represents the response from listing shared namespaces.
type SharedNamespacesResponse struct {
	TotalCount       int               `json:"total_count"`
	SharedNamespaces []SharedNamespace `json:"shared_namespaces"`
}

// ListRunnerGroupSharedNamespaces lists namespaces that have access to a runner group.
//
// It calls GET /api/v8/orgs/{org}/actions/runner-groups/{runner_group_id}/shared-namespaces.
func ListRunnerGroupSharedNamespaces(client *Client, org, runnerGroupID string, opts *ListRunnerGroupRunnersOptions) (*SharedNamespacesResponse, error) {
	endpoint := "/api/v8/orgs/" + url.PathEscape(org) + "/actions/runner-groups/" + url.PathEscape(runnerGroupID) + "/shared-namespaces"
	if opts != nil {
		endpoint += newQueryBuilder().
			Set("keyword", opts.Keyword).
			SetInt("per_page", opts.PerPage).
			SetInt("page", opts.Page).
			String()
	}

	resp, err := client.RawREST("GET", endpoint, nil, nil)
	if err != nil {
		return nil, err
	}
	var result SharedNamespacesResponse
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse shared namespaces response: %w", err)
	}
	return &result, nil
}

// ListRepoRunnerSets lists all K8S runner sets in a repository.
//
// It calls GET /api/v8/repos/{owner}/{repo}/actions/runner-sets. The response
// shape (RunnerSetsResponse with RunnerSet items) is identical to the org-level
// runner-group runner-sets endpoint.
func ListRepoRunnerSets(client *Client, owner, repo string, opts *ListRunnerGroupRunnersOptions) (*RunnerSetsResponse, error) {
	endpoint := "/api/v8/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) + "/actions/runner-sets"
	if opts != nil {
		endpoint += newQueryBuilder().
			Set("keyword", opts.Keyword).
			SetInt("per_page", opts.PerPage).
			SetInt("page", opts.Page).
			String()
	}

	resp, err := client.RawREST("GET", endpoint, nil, nil)
	if err != nil {
		return nil, err
	}
	var result RunnerSetsResponse
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse repo runner sets response: %w", err)
	}
	return &result, nil
}

// ListRepoSharedRunners lists all shared host runners available to a repository.
//
// It calls GET /api/v8/repos/{owner}/{repo}/actions/runners/shared-runners.
// The response shape (RunnersResponse with Runner items) is identical to the
// repo-level runners endpoint.
func ListRepoSharedRunners(client *Client, owner, repo string, opts *ListRunnerGroupRunnersOptions) (*RunnersResponse, error) {
	endpoint := "/api/v8/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) + "/actions/runners/shared-runners"
	if opts != nil {
		endpoint += newQueryBuilder().
			Set("keyword", opts.Keyword).
			SetInt("per_page", opts.PerPage).
			SetInt("page", opts.Page).
			String()
	}

	resp, err := client.RawREST("GET", endpoint, nil, nil)
	if err != nil {
		return nil, err
	}
	var result RunnersResponse
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse shared runners response: %w", err)
	}
	return &result, nil
}

// ListRepoSharedRunnerSets lists all shared K8S runner sets available to a repository.
//
// It calls GET /api/v8/repos/{owner}/{repo}/actions/shared-runner-sets.
// The response shape (RunnerSetsResponse with RunnerSet items) is identical
// to the repo-level runner-sets endpoint.
func ListRepoSharedRunnerSets(client *Client, owner, repo string, opts *ListRunnerGroupRunnersOptions) (*RunnerSetsResponse, error) {
	endpoint := "/api/v8/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) + "/actions/shared-runner-sets"
	if opts != nil {
		endpoint += newQueryBuilder().
			Set("keyword", opts.Keyword).
			SetInt("per_page", opts.PerPage).
			SetInt("page", opts.Page).
			String()
	}

	resp, err := client.RawREST("GET", endpoint, nil, nil)
	if err != nil {
		return nil, err
	}
	var result RunnerSetsResponse
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse shared runner sets response: %w", err)
	}
	return &result, nil
}

// Workflow represents a GitCode Actions workflow.
type Workflow struct {
	WorkflowID string `json:"workflow_id"`
	FilePath   string `json:"file_path"`
	Name       string `json:"name"`
	State      string `json:"state"`
}

// WorkflowsResponse represents the response from listing workflows.
type WorkflowsResponse struct {
	TotalCount int        `json:"total_count"`
	Workflows  []Workflow `json:"workflows"`
}

// ListActionsWorkflows lists all workflows in a repository.
//
// It calls GET /api/v8/repos/{owner}/{repo}/actions/workflows.
func ListActionsWorkflows(client *Client, owner, repo string, page, perPage int) (*WorkflowsResponse, error) {
	endpoint := "/api/v8/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) + "/actions/workflows"
	endpoint += newQueryBuilder().
		SetInt("per_page", perPage).
		SetInt("page", page).
		String()

	resp, err := client.RawREST("GET", endpoint, nil, nil)
	if err != nil {
		return nil, err
	}

	var result WorkflowsResponse
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse workflows response: %w", err)
	}
	return &result, nil
}

// DispatchWorkflowRequest is the request body for triggering a workflow run.
type DispatchWorkflowRequest struct {
	Ref    string            `json:"ref"`
	Inputs map[string]string `json:"inputs"`
}

// DispatchWorkflowResponse represents the response from triggering a workflow run.
type DispatchWorkflowResponse struct {
	WorkflowID    string `json:"workflow_id"`
	WorkflowRunID string `json:"workflow_run_id"`
}

// DispatchActionsWorkflow triggers a workflow_dispatch run for a workflow.
//
// It calls POST /api/v8/repos/{owner}/{repo}/actions/workflows/{workflow_id}/dispatches.
func DispatchActionsWorkflow(client *Client, owner, repo, workflowID string, req *DispatchWorkflowRequest) (*DispatchWorkflowResponse, error) {
	endpoint := "/api/v8/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) +
		"/actions/workflows/" + url.PathEscape(workflowID) + "/dispatches"

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal dispatch request: %w", err)
	}

	resp, err := client.RawREST("POST", endpoint, bytes.NewReader(body), nil)
	if err != nil {
		return nil, err
	}

	var result DispatchWorkflowResponse
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse dispatch response: %w", err)
	}
	return &result, nil
}
