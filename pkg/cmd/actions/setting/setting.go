// Package setting implements the actions setting command.
package setting

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"gitcode.com/gitcode-cli/cli/api"
	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
)

const organizationRepoPageSize = 100

// organizationRepoMaxPages bounds organization listing so a server that keeps
// returning non-empty pages cannot make the command loop forever.
const organizationRepoMaxPages = 1000

// Per-repository result statuses reported in the summary output.
const (
	statusUnchanged = "unchanged"
	statusUpdated   = "updated"
	statusFailed    = "failed"
)

const actionsSettingRiskMessage = "This command calls a GitCode Web API endpoint that is not documented in the official GitCode API reference. Use of this command is at your own risk."

type terminalPasswordReader func(int) ([]byte, error)

// webJWTHelpText explains why the command needs the web session JWT and
// where to obtain it. It is shown before the interactive paste prompt.
const webJWTHelpText = `The Actions settings endpoint requires a GitCode web-session JWT;
a classic personal access token cannot be used for this request.
To obtain the JWT:
  1. Open https://gitcode.com and sign in with an account that can manage
     the target repository.
  2. Click the account avatar in the top-right corner.
  3. Press F12 and switch to the Console tab.
  4. Run:
       copy(localStorage.getItem('access_token'))
  5. Paste the copied value at the next prompt and press Enter.
The JWT is entered without terminal echo and held in memory only for this
command. Do not share it or save it to a file.`

// SettingOptions configures the actions setting command.
type SettingOptions struct {
	IO         *iostreams.IOStreams
	HttpClient func() (*http.Client, error)
	BaseRepo   func() (string, error)

	Repository string
	Org        string
	Action     string
	WithToken  bool
	Yes        bool
	JSON       bool

	readPassword terminalPasswordReader
}

// SettingResult represents the result for one repository.
type SettingResult struct {
	Repository            string `json:"repository"`
	ProjectID             string `json:"project_id"`
	PreviousActionEnabled bool   `json:"previous_action_enabled"`
	ActionEnabled         bool   `json:"action_enabled"`
	Status                string `json:"status"`
}

// SettingSummary represents the result of an Actions permission update.
type SettingSummary struct {
	Action        string          `json:"action"`
	ActionEnabled bool            `json:"action_enabled"`
	Organization  string          `json:"organization,omitempty"`
	Repositories  []SettingResult `json:"repositories"`
}

type repositoryTarget struct {
	Name      string
	ProjectID string
}

type repositoryState struct {
	target   repositoryTarget
	setting  *api.ActionsSetting
	previous bool
	status   string
	err      error
}

// NewCmdSetting creates the actions setting command.
func NewCmdSetting(f *cmdutil.Factory, runF func(*SettingOptions) error) *cobra.Command {
	opts := &SettingOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		BaseRepo:   f.BaseRepo,
	}

	cmd := &cobra.Command{
		Use:   "setting <enable|disable>",
		Short: "Enable or disable repository Actions",
		Long: heredoc.Doc(`
			Enable or disable GitCode Actions for one repository or for every
			repository in an organization. Use -R owner/repo for one repository,
			or --org <name> for all repositories in an organization. When neither
			is given, the repository is inferred from the current git repository,
			like the rest of the actions command family.

			The command reads each repository's current Actions permission first,
			then changes only action_enabled and preserves the other permission
			fields. This is a dangerous operation: type the repository or
			organization name to confirm, or use --yes in a non-interactive
			environment. When no repository needs a change, the command reports
			the result without asking for confirmation.
			Before confirmation, the command warns that it uses a Web API endpoint
			not documented in the official GitCode API reference.

			The Actions settings endpoint requires a GitCode web-session JWT; a
			classic personal access token cannot be used for this request. The
			command explains how to obtain it from a signed-in GitCode browser
			session. Interactive input is hidden and held in memory only for this
			command. In non-interactive environments, pipe it via --with-token
			and never place a token literal on the command line.
		`),
		Example: heredoc.Doc(`
			# Enable Actions for one repository
			$ gc actions setting enable -R owner/repo

			# Enable Actions for the current repository
			$ gc actions setting enable

			# Disable Actions for one repository without an interactive prompt
			$ gc actions setting disable -R owner/repo --yes

			# Enable Actions for every repository in an organization
			$ gc actions setting enable --org my-org

			# Enable all repositories and return structured output
			$ gc actions setting enable --org my-org --yes --json

			# Non-interactive: pipe the web session JWT. Never echo or cat a
			# token literal (it leaks to shell history or disk); pipe it from
			# a secret manager instead.
			$ <print-token-from-secret-manager> | gc actions setting disable -R owner/repo --with-token --yes
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Action = args[0]
			if err := validateTarget(opts); err != nil {
				return err
			}
			if runF != nil {
				return runF(opts)
			}
			return settingRun(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Repository, "repo", "R", "", "Repository (owner/repo, defaults to the current repository)")
	cmd.Flags().StringVar(&opts.Org, "org", "", "Organization name; batch every repository in it (mutually exclusive with -R)")
	cmd.Flags().BoolVarP(&opts.Yes, "yes", "y", false, "Skip confirmation (required in non-interactive mode)")
	cmd.Flags().BoolVar(&opts.WithToken, "with-token", false, "Read the gitcode.com web session JWT from standard input")
	cmdutil.AddJSONFlag(cmd, &opts.JSON)

	return cmd
}

func settingRun(opts *SettingOptions) error {
	action := strings.ToLower(strings.TrimSpace(opts.Action))
	desired, err := actionEnabledValue(action)
	if err != nil {
		return err
	}
	opts.Action = action
	if err := validateTarget(opts); err != nil {
		return err
	}
	if err := applyRepositoryDefault(opts); err != nil {
		return err
	}

	// Resolve the web JWT before any API call: the non-interactive check is a
	// parameter validation, so a missing JWT must fail fast instead of running
	// all the read requests first.
	jwt, err := resolveWebJWT(opts)
	if err != nil {
		return err
	}

	client, err := cmdutil.AuthenticatedClientFromFactory(opts.HttpClient)
	if err != nil {
		return err
	}

	targets, organization, err := resolveTargets(opts.IO, client, opts)
	if err != nil {
		return err
	}

	client.SetToken(jwt, "web-jwt")

	states, err := readStates(opts.IO, client, targets)
	if err != nil {
		return err
	}

	changed := countChanged(states, desired)
	writePreview(opts.IO, action, states, desired, changed)
	// Nothing to change means no write happens, so there is nothing to confirm.
	if changed > 0 {
		if err := confirmSettingChange(opts, confirmationTarget(organization, targets)); err != nil {
			return err
		}
	}

	failures := applyStates(client, states, desired, action)
	summary := buildSummary(action, organization, states, desired)
	if err := writeSummary(opts, summary); err != nil {
		return err
	}
	if failures > 0 {
		writeFailureDetails(opts.IO, states)
		return cmdutil.NewCLIError(cmdutil.ExitError,
			fmt.Sprintf("failed to %s Actions for %d of %d repositories; see the per-repository status above", action, failures, len(states)), nil)
	}
	return nil
}

// resolveWebJWT obtains the web session JWT required by the Actions setting
// API. With --with-token the JWT is read from standard input; if that input is
// still an interactive terminal, the typed value is read hidden so it does not
// echo. Otherwise an interactive terminal is required, and non-interactive runs
// fail fast instead of blocking on stdin.
func resolveWebJWT(opts *SettingOptions) (string, error) {
	if opts.WithToken && !opts.IO.CanPrompt() {
		return validateWebJWT(readLineFrom(opts.IO))
	}
	if !opts.IO.CanPrompt() {
		return "", cmdutil.NewUsageError("the Actions setting API requires the gitcode.com web session JWT; run in an interactive terminal to paste it, or provide it via --with-token")
	}
	if opts.WithToken {
		fmt.Fprintln(opts.IO.ErrOut, "--with-token is reading from an interactive terminal; input stays hidden.")
	}
	fmt.Fprintln(opts.IO.ErrOut, webJWTHelpText)
	fmt.Fprint(opts.IO.ErrOut, "Web session JWT (input hidden; press Enter when complete): ")
	raw, err := readHiddenLineFrom(opts.IO, opts.readPassword)
	if err != nil {
		return "", cmdutil.NewCLIError(cmdutil.ExitUsage, "failed to read web session JWT", err)
	}
	return validateWebJWT(raw)
}

func readHiddenLineFrom(ioStreams *iostreams.IOStreams, passwordReader terminalPasswordReader) (string, error) {
	in := io.Reader(os.Stdin)
	if ioStreams != nil && ioStreams.In != nil {
		in = ioStreams.In
	}

	file, ok := in.(*os.File)
	if !ok && passwordReader == nil {
		return "", fmt.Errorf("hidden JWT input requires a terminal")
	}
	if passwordReader == nil {
		passwordReader = terminalPasswordReader(term.ReadPassword)
	}
	fd := -1
	if file != nil {
		fd = int(file.Fd())
	}
	value, err := passwordReader(fd)
	if ioStreams != nil && ioStreams.ErrOut != nil {
		fmt.Fprintln(ioStreams.ErrOut)
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(value)), nil
}

// readLineFrom reads exactly one line without buffering ahead, so a later
// confirmation prompt can still read the remaining stdin content.
func readLineFrom(ioStreams *iostreams.IOStreams) string {
	in := ioStreams.In
	if in == nil {
		in = os.Stdin
	}
	var line strings.Builder
	buf := make([]byte, 1)
	for {
		n, err := in.Read(buf)
		if n > 0 {
			if buf[0] == '\n' {
				break
			}
			line.WriteByte(buf[0])
		}
		if err != nil {
			break
		}
	}
	return strings.TrimSpace(line.String())
}

// validateWebJWT performs local-only checks: the value must look like a JWT
// and must not be expired. The value is never logged.
func validateWebJWT(raw string) (string, error) {
	jwt := strings.TrimSpace(raw)
	if jwt == "" {
		return "", cmdutil.NewUsageError("no web session JWT provided; refresh GitCode, run copy(localStorage.getItem('access_token')) in the Console, and retry")
	}
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" {
		return "", cmdutil.NewAuthError("the provided value is not a valid GitCode web-session JWT (expected three dot-separated segments); refresh GitCode and copy access_token from the Console")
	}
	payload, err := decodeJWTPayload(parts[1])
	if err != nil {
		return "", cmdutil.NewAuthError("failed to decode the web-session JWT payload; refresh GitCode and copy access_token from the Console")
	}
	if exp, ok := payload["exp"].(float64); ok {
		expiresAt := time.Unix(int64(exp), 0)
		if time.Now().After(expiresAt) {
			return "", cmdutil.NewAuthError(fmt.Sprintf("the JWT expired at %s; refresh gitcode.com in the browser and copy a fresh access_token", expiresAt.Format("2006-01-02 15:04 MST")))
		}
	}
	return jwt, nil
}

func decodeJWTPayload(segment string) (map[string]any, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(segment)
	if err != nil {
		decoded, err = base64.URLEncoding.DecodeString(segment)
		if err != nil {
			return nil, err
		}
	}
	var payload map[string]any
	if err := json.Unmarshal(decoded, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func readStates(ioStreams *iostreams.IOStreams, client *api.Client, targets []repositoryTarget) ([]repositoryState, error) {
	// A batch read issues one request per repository; announce it so the
	// operator is not left staring at a silent terminal.
	if ioStreams != nil && ioStreams.ErrOut != nil && len(targets) > 1 {
		fmt.Fprintf(ioStreams.ErrOut, "Reading current Actions settings for %d repositories...\n", len(targets))
	}
	states := make([]repositoryState, 0, len(targets))
	for _, target := range targets {
		setting, err := api.GetActionsSetting(client, target.ProjectID)
		if err != nil {
			return nil, fmt.Errorf("failed to read Actions permission for %s: %w", target.Name, err)
		}
		if setting.ActionEnabled == nil {
			return nil, fmt.Errorf("Actions permission for %s does not contain action_enabled", target.Name)
		}
		states = append(states, repositoryState{target: target, setting: setting, previous: *setting.ActionEnabled})
	}
	return states, nil
}

// confirmationTarget returns the identifier the operator must type to confirm,
// matching the actions command family convention of typing the affected target.
func confirmationTarget(organization string, targets []repositoryTarget) string {
	if strings.TrimSpace(organization) != "" {
		return organization
	}
	if len(targets) > 0 {
		return targets[0].Name
	}
	return ""
}

func confirmSettingChange(opts *SettingOptions, target string) error {
	cs := opts.IO.ColorScheme()
	expected := fmt.Sprintf("%s Actions for %s", opts.Action, target)
	return cmdutil.ConfirmOrAbort(cmdutil.ConfirmOptions{
		IO:       opts.IO,
		Yes:      opts.Yes,
		Expected: expected,
		Prompt:   fmt.Sprintf("%s %s: %s\n%s Type %q to confirm: ", cs.WarningIcon(), cs.Yellow("Warning"), actionsSettingRiskMessage, cs.WarningIcon(), expected),
	})
}

// applyStates updates every state that is not already at the desired value. A
// failure is recorded on the state and the loop continues, so a batch run still
// reports the outcome of every repository instead of stopping at the first
// error; already applied changes are kept. The failure count drives the exit
// code.
func applyStates(client *api.Client, states []repositoryState, desired bool, action string) int {
	failures := 0
	for i := range states {
		if *states[i].setting.ActionEnabled == desired {
			states[i].status = statusUnchanged
			continue
		}
		previous := *states[i].setting.ActionEnabled
		updated := desired
		states[i].setting.ActionEnabled = &updated
		if err := api.UpdateActionsSetting(client, states[i].target.ProjectID, states[i].setting); err != nil {
			states[i].setting.ActionEnabled = &previous
			states[i].status = statusFailed
			states[i].err = fmt.Errorf("failed to %s Actions for %s: %w", action, states[i].target.Name, err)
			failures++
			continue
		}
		states[i].status = statusUpdated
	}
	return failures
}

// validateTarget checks the explicit target flags. An empty target is allowed
// here: settingRun fills it in from the current repository, matching the rest
// of the actions command family.
func validateTarget(opts *SettingOptions) error {
	repository := strings.TrimSpace(opts.Repository)
	organization := strings.TrimSpace(opts.Org)
	switch {
	case repository != "" && organization != "":
		return cmdutil.NewUsageError("-R/--repo and --org are mutually exclusive")
	case organization != "":
		if strings.Contains(organization, "/") {
			return cmdutil.NewUsageError("--org takes an organization name without '/'")
		}
		return nil
	case repository == "":
		return nil
	}

	owner, name, err := cmdutil.ParseRepo(repository)
	if err != nil {
		return err
	}
	if strings.TrimSpace(owner) == "" || strings.TrimSpace(name) == "" {
		return cmdutil.NewUsageError(fmt.Sprintf("invalid repository %q: expected owner/repo with both segments non-empty", repository))
	}
	return nil
}

// applyRepositoryDefault fills in the current repository when neither -R nor
// --org was given. Resolution only reads local git state, so it still runs
// before any network call or JWT prompt.
func applyRepositoryDefault(opts *SettingOptions) error {
	if strings.TrimSpace(opts.Repository) != "" || strings.TrimSpace(opts.Org) != "" {
		return nil
	}
	repository, err := cmdutil.ResolveRepo("", opts.BaseRepo)
	if err != nil {
		return err
	}
	opts.Repository = repository
	return nil
}

func actionEnabledValue(action string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "enable":
		return true, nil
	case "disable":
		return false, nil
	default:
		return false, cmdutil.NewUsageError(fmt.Sprintf("invalid action %q: must be enable or disable", action))
	}
}

func resolveTargets(ioStreams *iostreams.IOStreams, client *api.Client, opts *SettingOptions) ([]repositoryTarget, string, error) {
	organization := strings.TrimSpace(opts.Org)
	if organization != "" {
		repositories, err := listAllOrganizationRepositories(ioStreams, client, organization)
		if err != nil {
			return nil, "", err
		}
		targets := make([]repositoryTarget, 0, len(repositories))
		for _, repo := range repositories {
			projectID, err := projectIDFromRepository(&repo)
			if err != nil {
				return nil, "", fmt.Errorf("failed to determine project id for %s: %w", repositoryDisplayName(organization, repo), err)
			}
			targets = append(targets, repositoryTarget{
				Name:      repositoryDisplayName(organization, repo),
				ProjectID: projectID,
			})
		}
		return targets, organization, nil
	}

	owner, name, err := cmdutil.ParseRepo(strings.TrimSpace(opts.Repository))
	if err != nil {
		return nil, "", err
	}
	repo, err := api.GetRepo(client, owner, name)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read repository %s/%s: %w", owner, name, err)
	}
	projectID, err := projectIDFromRepository(repo)
	if err != nil {
		return nil, "", fmt.Errorf("failed to determine project id for %s/%s: %w", owner, name, err)
	}
	return []repositoryTarget{{Name: owner + "/" + name, ProjectID: projectID}}, "", nil
}

// listAllOrganizationRepositories lists every repository in an organization.
//
// Pagination stops on the first empty page rather than the first short page: a
// server that clamps per_page below the requested size would otherwise make the
// first page look like the last one and silently drop repositories. The page
// cap guards against a server that keeps returning entries forever.
func listAllOrganizationRepositories(ioStreams *iostreams.IOStreams, client *api.Client, organization string) ([]api.Repository, error) {
	repositories := make([]api.Repository, 0)
	for page := 1; page <= organizationRepoMaxPages; page++ {
		// Listing a large organization takes several round trips; report each
		// page so the operator sees progress instead of a silent terminal.
		if ioStreams != nil && ioStreams.ErrOut != nil {
			fmt.Fprintf(ioStreams.ErrOut, "Fetching repositories in organization %s (page %d)...\n", organization, page)
		}
		pageRepos, err := api.ListOrgRepos(client, organization, &api.RepoListOptions{
			Page:    page,
			PerPage: organizationRepoPageSize,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list repositories for organization %s: %w", organization, err)
		}
		if len(pageRepos) == 0 {
			return repositories, nil
		}
		repositories = append(repositories, pageRepos...)
	}
	return nil, fmt.Errorf("failed to list repositories for organization %s: exceeded %d pages", organization, organizationRepoMaxPages)
}

func projectIDFromRepository(repository *api.Repository) (string, error) {
	if repository == nil || repository.ID == nil {
		return "", fmt.Errorf("repository id is missing")
	}

	switch id := repository.ID.(type) {
	case string:
		if strings.TrimSpace(id) == "" {
			return "", fmt.Errorf("repository id is empty")
		}
		return id, nil
	case json.Number:
		return id.String(), nil
	case float64:
		if id < 0 || id != math.Trunc(id) || id > math.MaxInt64 {
			return "", fmt.Errorf("repository id is not an integer")
		}
		return strconv.FormatInt(int64(id), 10), nil
	case int:
		return strconv.Itoa(id), nil
	case int64:
		return strconv.FormatInt(id, 10), nil
	default:
		return "", fmt.Errorf("unsupported repository id type %T", repository.ID)
	}
}

func repositoryDisplayName(organization string, repository api.Repository) string {
	if strings.TrimSpace(repository.FullName) != "" {
		return repository.FullName
	}
	return strings.TrimSuffix(organization, "/") + "/" + repository.Name
}

func countChanged(states []repositoryState, desired bool) int {
	count := 0
	for _, state := range states {
		if *state.setting.ActionEnabled != desired {
			count++
		}
	}
	return count
}

func buildSummary(action, organization string, states []repositoryState, desired bool) SettingSummary {
	repositories := make([]SettingResult, 0, len(states))
	for _, state := range states {
		status := state.status
		if status == "" {
			status = statusUnchanged
		}
		repositories = append(repositories, SettingResult{
			Repository:            state.target.Name,
			ProjectID:             state.target.ProjectID,
			PreviousActionEnabled: state.previous,
			ActionEnabled:         *state.setting.ActionEnabled,
			Status:                status,
		})
	}
	return SettingSummary{
		Action:        action,
		ActionEnabled: desired,
		Organization:  organization,
		Repositories:  repositories,
	}
}

func writePreview(ioStreams *iostreams.IOStreams, action string, states []repositoryState, desired bool, changed int) {
	if ioStreams == nil {
		return
	}
	fmt.Fprintf(ioStreams.ErrOut, "Actions will be %sd for %d of %d repositories:\n", action, changed, len(states))
	for _, state := range states {
		if *state.setting.ActionEnabled != desired {
			fmt.Fprintf(ioStreams.ErrOut, "  %s: %t -> %t\n", state.target.Name, *state.setting.ActionEnabled, desired)
			continue
		}
		if changed != len(states) {
			fmt.Fprintf(ioStreams.ErrOut, "  %s: skipped (Actions already %s)\n", state.target.Name, actionState(*state.setting.ActionEnabled))
		}
	}
}

func actionState(enabled bool) string {
	if enabled {
		return "enabled"
	}
	return "disabled"
}

func writeSummary(opts *SettingOptions, summary SettingSummary) error {
	if opts.JSON {
		return cmdutil.WriteJSON(opts.IO.Out, summary)
	}
	updated := 0
	for _, repository := range summary.Repositories {
		if repository.Status == statusUpdated {
			updated++
		}
	}
	fmt.Fprintf(opts.IO.Out, "Actions %sd for %d/%d repositories\n", summary.Action, updated, len(summary.Repositories))
	for _, repository := range summary.Repositories {
		fmt.Fprintf(opts.IO.Out, "  %s: %s\n", repository.Repository, repository.Status)
	}
	return nil
}

// writeFailureDetails reports why each repository failed on stderr so a batch
// run leaves an actionable trace for every repository.
func writeFailureDetails(ioStreams *iostreams.IOStreams, states []repositoryState) {
	if ioStreams == nil {
		return
	}
	for _, state := range states {
		if state.err != nil {
			fmt.Fprintln(ioStreams.ErrOut, state.err)
		}
	}
}
