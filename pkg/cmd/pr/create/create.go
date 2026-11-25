// Package create implements the pr create command
package create

import (
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitcode.com/gitcode-cli/cli/api"
	"gitcode.com/gitcode-cli/cli/pkg/browser"
	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
)

type CreateOptions struct {
	IO             *iostreams.IOStreams
	HttpClient     func() (*http.Client, error)
	BaseRepo       func() (string, error)
	Branch         func() (string, error)
	ExecGitCommand func(string, ...string) (string, error)
	CreatePR       func(*api.Client, string, string, *api.CreatePROptions) (*api.PullRequest, error)
	OpenBrowser    func(string) error

	// Arguments
	Repository string

	// Flags
	Title    string
	Body     string
	BodyFile string
	Head     string
	Base     string
	Draft    bool
	Fill     bool
	Web      bool
	JSON     bool
	Fork     string // 跨仓库 PR：fork 项目路径【owner/repo】
	Labels   []string

	// Issue 关联：Issues 保存 --issue 的原始值（"12"、"#12"、"12,13"），
	// 由 createRun 解析为 api.CreatePROptions.IssueNumbers。
	Issues               []string
	CloseRelatedIssue    bool
	CloseRelatedIssueSet bool
}

// NewCmdCreate creates the create command
func NewCmdCreate(f *cmdutil.Factory, runF func(*CreateOptions) error) *cobra.Command {
	opts := &CreateOptions{
		IO:             f.IOStreams,
		HttpClient:     f.HttpClient,
		BaseRepo:       f.BaseRepo,
		Branch:         f.Branch,
		ExecGitCommand: execGitCommand,
		CreatePR:       api.CreatePullRequest,
		OpenBrowser:    browser.Open,
	}

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a pull request",
		Long: heredoc.Doc(`
			Create a new pull request in a GitCode repository.
		`),
		Example: heredoc.Doc(`
			# Create a PR with title and body (uses current branch as head)
			$ gc pr create -R owner/repo --title "Feature" --body "Description"

			# Create a PR with body from file
			$ gc pr create -R owner/repo --title "Feature" --body-file description.md

			# Create a PR with body from stdin
			$ cat description.md | gc pr create -R owner/repo --title "Feature" --body-file -

			# Create a PR with specific head and base branches
			$ gc pr create -R owner/repo --head feature-branch --base main --title "Feature" --body "Description"

			# Create a draft PR
			$ gc pr create -R owner/repo --title "Feature" --draft

			# Create a PR with labels
			$ gc pr create -R owner/repo --title "Feature" --labels bug,enhancement

			# Create a PR linked to existing issues
			$ gc pr create -R owner/repo --title "Feature" --issue 12,13

			# Explicitly close the linked issues when the PR is merged
			$ gc pr create -R owner/repo --title "Feature" --issue '#12' --close-related-issue

			# Keep the linked issues open on merge
			$ gc pr create -R owner/repo --title "Feature" --issue 12 --close-related-issue=false

			# Create a cross-repo PR (from fork to upstream)
			$ gc pr create -R upstream/repo --fork myfork/repo --head feature-branch --title "Feature"

			# Output as JSON
			$ gc pr create -R owner/repo --title "Feature" --json
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			// --close-related-issue is tri-state: absent keeps the repository
			// setting, bare means true, =false opts out explicitly.
			opts.CloseRelatedIssueSet = cmd.Flags().Changed("close-related-issue")
			if runF != nil {
				return runF(opts)
			}
			return createRun(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Repository, "repo", "R", "", "Repository (owner/repo)")
	cmd.Flags().StringVarP(&opts.Title, "title", "t", "", "Title for the PR")
	cmd.Flags().StringVarP(&opts.Body, "body", "b", "", "Body for the PR")
	cmd.Flags().StringVar(&opts.BodyFile, "body-file", "", "Read body from file (use - for stdin)")
	cmd.Flags().StringVarP(&opts.Head, "head", "H", "", "Head branch (default: current branch)")
	cmd.Flags().StringVarP(&opts.Base, "base", "B", "main", "Base branch")
	cmd.Flags().BoolVarP(&opts.Draft, "draft", "d", false, "Create as draft")
	cmd.Flags().BoolVarP(&opts.Fill, "fill", "f", false, "Fill from last commit")
	cmd.Flags().BoolVarP(&opts.Web, "web", "w", false, "Open in browser")
	cmdutil.AddJSONFlag(cmd, &opts.JSON)
	cmd.Flags().StringVarP(&opts.Fork, "fork", "F", "", "Fork repository path for cross-repo PR (owner/repo)")
	cmd.Flags().StringSliceVarP(&opts.Labels, "labels", "l", nil, "Add labels (comma-separated)")
	cmd.Flags().StringSliceVarP(&opts.Issues, "issue", "i", nil,
		"Link existing issues at creation (comma-separated, accepts 12, #12, or 12,13)")
	cmd.Flags().BoolVar(&opts.CloseRelatedIssue, "close-related-issue", false,
		"Close linked issues when the PR is merged (default: repository setting); use =false to keep them open")

	return cmd
}

func createRun(opts *CreateOptions) error {
	cs := opts.IO.ColorScheme()

	if opts.JSON && opts.Web {
		return cmdutil.NewUsageError("cannot use --json with --web")
	}

	// Validate --issue before any request so a bad value never reaches the API.
	issueNumbers, err := parseIssueNumbers(opts.Issues)
	if err != nil {
		return err
	}
	var closeRelatedIssue *bool
	if opts.CloseRelatedIssueSet {
		closeRelatedIssue = &opts.CloseRelatedIssue
	}

	httpClient, err := opts.HttpClient()
	if err != nil {
		return fmt.Errorf("failed to create HTTP client: %w", err)
	}
	client, err := cmdutil.AuthenticatedClient(httpClient)
	if err != nil {
		return err
	}

	// Get repository
	repository, err := cmdutil.ResolveRepo(opts.Repository, opts.BaseRepo)
	if err != nil {
		return err
	}
	owner, repo, err := parseRepo(repository)
	if err != nil {
		return err
	}

	// Validate required fields
	if opts.Fill {
		if err := fillFromLastCommit(opts); err != nil {
			return err
		}
	}

	if opts.Title == "" {
		return cmdutil.NewUsageError("title is required. Use --title flag")
	}

	body, err := getBody(opts)
	if err != nil {
		return err
	}

	// Auto-detect head branch if not specified
	head := opts.Head
	if head == "" {
		if opts.Branch == nil {
			return cmdutil.NewUsageError("head branch is required. Use --head flag")
		}
		output, err := opts.Branch()
		if err != nil {
			return cmdutil.NewUsageError("could not determine current branch. Use --head flag")
		}
		head = strings.TrimSpace(output)
		if head == "" || head == "HEAD" {
			return cmdutil.NewUsageError("could not determine current branch. Use --head flag")
		}
	}

	// Express a cross-repo (fork) source via head="<fork_owner>/<fork_repo>:<branch>"
	// (GitCode v5 resolves this form; the "owner:branch" form from #259 is no
	// longer resolved — see #495), instead of a fork_path form field.
	head, warning, err := resolveHead(head, opts.Fork)
	if err != nil {
		return err
	}
	if warning != "" {
		fmt.Fprintf(opts.IO.ErrOut, "warning: %s\n", warning)
	}

	// Create PR
	pr, err := opts.CreatePR(client, owner, repo, &api.CreatePROptions{
		Title:             opts.Title,
		Body:              body,
		Head:              head,
		Base:              opts.Base,
		Draft:             opts.Draft,
		Labels:            opts.Labels,
		IssueNumbers:      issueNumbers,
		CloseRelatedIssue: closeRelatedIssue,
	})
	if err != nil {
		return fmt.Errorf("failed to create PR: %w", err)
	}

	if opts.JSON {
		if body != "" && pr.Body == "" && pr.Number > 0 {
			fmt.Fprintf(opts.IO.ErrOut, "warning: GitCode create response did not include PR body, and the follow-up read did not return it. 警告：PR 描述未能从远端返回；请运行 `gitcode pr view %d -R %s/%s --json` 核验远端 body。\n", pr.Number, owner, repo)
		}
		return cmdutil.WriteJSON(opts.IO.Out, pr)
	}
	fmt.Fprintf(opts.IO.Out, "%s Created PR #%d in %s/%s\n", cs.Green("✓"), pr.Number, owner, repo)
	fmt.Fprintf(opts.IO.Out, "  %s\n", pr.HTMLURL)
	if opts.Web {
		fmt.Fprintf(opts.IO.Out, "Opening %s in your browser.\n", pr.HTMLURL)
		if err := opts.OpenBrowser(pr.HTMLURL); err != nil {
			if opts.IO.IsStdoutTTY() {
				return fmt.Errorf("failed to open browser: %w", err)
			}
			fmt.Fprintf(opts.IO.ErrOut, "Failed to open browser: %v\n", err)
		}
	}
	return nil
}

func fillFromLastCommit(opts *CreateOptions) error {
	output, err := opts.ExecGitCommand("git", "log", "-1", "--pretty=%B")
	if err != nil {
		return fmt.Errorf("failed to fill from last commit: %w", err)
	}

	message := strings.TrimSpace(output)
	if message == "" {
		return fmt.Errorf("failed to fill from last commit: commit message is empty")
	}

	subject, body := splitCommitMessage(message)
	if opts.Title == "" {
		opts.Title = subject
	}
	if opts.Body == "" {
		opts.Body = body
	}
	if opts.BodyFile != "" {
		opts.Body = ""
	}

	return nil
}

func getBody(opts *CreateOptions) (string, error) {
	return cmdutil.ReadBody(opts.Body, opts.BodyFile, opts.IO.In)
}

func splitCommitMessage(message string) (string, string) {
	lines := strings.Split(strings.ReplaceAll(message, "\r\n", "\n"), "\n")
	subject := strings.TrimSpace(lines[0])
	if len(lines) == 1 {
		return subject, ""
	}

	body := strings.TrimSpace(strings.Join(lines[1:], "\n"))
	return subject, body
}

func execGitCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

func parseRepo(repo string) (string, string, error) {
	return cmdutil.ParseRepo(repo)
}

// parseIssueNumbers normalizes --issue values ("12", "#12", "12,13") into
// positive issue numbers, dropping duplicates while preserving input order.
// A nil slice means --issue was not used.
func parseIssueNumbers(values []string) ([]int, error) {
	if values == nil {
		return nil, nil
	}
	if len(values) == 0 {
		return nil, cmdutil.NewUsageError("invalid --issue value: expected at least one issue number")
	}
	numbers := make([]int, 0, len(values))
	seen := make(map[int]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(value), "#"))
		number, err := strconv.Atoi(trimmed)
		if err != nil || number <= 0 {
			return nil, cmdutil.NewUsageError(fmt.Sprintf(
				"invalid --issue value %q: expected a positive issue number like 12 or #12", value))
		}
		if _, ok := seen[number]; ok {
			continue
		}
		seen[number] = struct{}{}
		numbers = append(numbers, number)
	}
	return numbers, nil
}

// resolveHead expresses a cross-repo PR source for GitCode.
//
// When --fork is supplied and head is a bare branch name, it returns the
// "owner/repo:branch" form, which GitCode v5 resolves to the fork's branch
// even when the upstream has a same-name branch. (The "owner:branch" form used
// earlier per #259 is no longer resolved by the server — see #495.) A head
// that already carries an "owner:branch" or "owner/repo:branch" prefix is
// returned unchanged so callers can override the inferred fork explicitly.
//
// The returned warning is non-empty when an explicit head owner overrides a
// conflicting --fork owner (likely a typo), so the caller can surface it.
func resolveHead(head, fork string) (resolved, warning string, err error) {
	if fork == "" {
		return head, "", nil
	}

	// fork must be "owner/repo" so we can build "owner/repo:branch" (#495).
	idx := strings.Index(fork, "/")
	if idx <= 0 || idx == len(fork)-1 {
		return "", "", cmdutil.NewUsageError("invalid --fork value; expected owner/repo")
	}
	forkOwner := fork[:idx]

	// An explicit head carrying ":" (owner:branch or owner/repo:branch) wins.
	// Flag a mismatched --fork owner (compare the owner prefix before any '/').
	if colon := strings.Index(head, ":"); colon >= 0 {
		headOwner := head[:colon]
		if slash := strings.Index(headOwner, "/"); slash >= 0 {
			headOwner = headOwner[:slash]
		}
		if headOwner != forkOwner {
			warning = fmt.Sprintf("--head owner %q overrides --fork owner %q", headOwner, forkOwner)
		}
		return head, warning, nil
	}

	// Build "owner/repo:branch" — GitCode v5 resolves this cross-repo form.
	return fork + ":" + head, "", nil
}
