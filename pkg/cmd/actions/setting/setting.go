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

const actionsSettingRiskWarning = "Warning: This command calls a GitCode Web API endpoint that is not documented in the official GitCode API reference. Use of this command is at your own risk."

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

	Repository string
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
	target  repositoryTarget
	setting *api.ActionsSetting
}

// NewCmdSetting creates the actions setting command.
func NewCmdSetting(f *cmdutil.Factory, runF func(*SettingOptions) error) *cobra.Command {
	opts := &SettingOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
	}

	cmd := &cobra.Command{
		Use:   "setting <enable|disable>",
		Short: "Enable or disable repository Actions",
		Long: heredoc.Doc(`
			Enable or disable GitCode Actions for one repository or every
			repository in an organization. Use -R owner/repo for one repository,
			or -R organization for all repositories in an organization.

			The command reads each repository's current Actions permission first,
			then changes only action_enabled and preserves the other permission
			fields. This is a dangerous operation: type y to confirm, or use
			--yes in a non-interactive environment.
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
			$ gitcode actions setting enable -R owner/repo

			# Disable Actions for one repository without an interactive prompt
			$ gitcode actions setting disable -R owner/repo --yes

			# Enable Actions for every repository in an organization
			$ gitcode actions setting enable -R my-org

			# Enable all repositories and return structured output
			$ gitcode actions setting enable -R my-org --yes --json

			# Non-interactive: pipe the web session JWT. Never echo or cat a
			# token literal (it leaks to shell history or disk); pipe it from
			# a secret manager instead.
			$ <print-token-from-secret-manager> | gitcode actions setting disable -R owner/repo --with-token --yes
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Action = args[0]
			if err := validateTarget(opts.Repository); err != nil {
				return err
			}
			if runF != nil {
				return runF(opts)
			}
			return settingRun(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Repository, "repo", "R", "", "Repository (owner/repo) or organization")
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
	if err := validateTarget(opts.Repository); err != nil {
		return err
	}

	client, err := cmdutil.AuthenticatedClientFromFactory(opts.HttpClient)
	if err != nil {
		return err
	}

	targets, organization, err := resolveTargets(client, opts.Repository)
	if err != nil {
		return err
	}

	jwt, err := resolveWebJWT(opts)
	if err != nil {
		return err
	}
	client.SetToken(jwt, "web-jwt")

	states, err := readStates(client, targets)
	if err != nil {
		return err
	}

	summary := buildSummary(action, organization, states, desired)
	changed := countChanged(states, desired)
	writePreview(opts.IO, action, states, desired, changed)
	if err := confirmSettingChange(opts); err != nil {
		return err
	}
	if changed == 0 {
		return writeSummary(opts, summary)
	}
	if err := updateStates(client, states, desired, action); err != nil {
		return err
	}

	for i := range summary.Repositories {
		if summary.Repositories[i].PreviousActionEnabled != desired {
			summary.Repositories[i].Status = "updated"
		}
	}
	return writeSummary(opts, summary)
}

// resolveWebJWT obtains the web session JWT required by the Actions setting
// API. With --with-token the JWT is read from standard input; otherwise an
// interactive terminal is required, and non-interactive runs fail fast
// instead of blocking on stdin.
func resolveWebJWT(opts *SettingOptions) (string, error) {
	if opts.WithToken {
		return validateWebJWT(readLineFrom(opts.IO))
	}
	if !opts.IO.CanPrompt() {
		return "", cmdutil.NewUsageError("the Actions setting API requires the gitcode.com web session JWT; run in an interactive terminal to paste it, or provide it via --with-token")
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

func readStates(client *api.Client, targets []repositoryTarget) ([]repositoryState, error) {
	states := make([]repositoryState, 0, len(targets))
	for _, target := range targets {
		setting, err := api.GetActionsSetting(client, target.ProjectID)
		if err != nil {
			return nil, fmt.Errorf("failed to read Actions permission for %s: %w", target.Name, err)
		}
		if setting.ActionEnabled == nil {
			return nil, fmt.Errorf("Actions permission for %s does not contain action_enabled", target.Name)
		}
		states = append(states, repositoryState{target: target, setting: setting})
	}
	return states, nil
}

func confirmSettingChange(opts *SettingOptions) error {
	return cmdutil.ConfirmOrAbort(cmdutil.ConfirmOptions{
		IO:       opts.IO,
		Yes:      opts.Yes,
		Expected: "y",
		Prompt:   fmt.Sprintf("%s %s\nType y to confirm: ", opts.IO.ColorScheme().WarningIcon(), actionsSettingRiskWarning),
	})
}

func updateStates(client *api.Client, states []repositoryState, desired bool, action string) error {
	for i := range states {
		if *states[i].setting.ActionEnabled == desired {
			continue
		}
		updated := desired
		states[i].setting.ActionEnabled = &updated
		if err := api.UpdateActionsSetting(client, states[i].target.ProjectID, states[i].setting); err != nil {
			return fmt.Errorf("failed to %s Actions for %s: %w", action, states[i].target.Name, err)
		}
	}
	return nil
}

func validateTarget(repository string) error {
	if strings.TrimSpace(repository) == "" {
		return cmdutil.NewUsageError("-R/--repo is required (owner/repo or organization)")
	}
	return nil
}

func actionEnabledValue(action string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "enable":
		return true, nil
	case "disable":
		return false, nil
	default:
		return false, cmdutil.NewUsageError("action must be enable or disable")
	}
}

func resolveTargets(client *api.Client, repository string) ([]repositoryTarget, string, error) {
	repository = strings.TrimSpace(repository)
	if strings.Contains(repository, "/") {
		owner, name, err := cmdutil.ParseRepo(repository)
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

	organization := repository
	repositories, err := listAllOrganizationRepositories(client, organization)
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

func listAllOrganizationRepositories(client *api.Client, organization string) ([]api.Repository, error) {
	var repositories []api.Repository
	for page := 1; ; page++ {
		pageRepos, err := api.ListOrgRepos(client, organization, &api.RepoListOptions{
			Page:    page,
			PerPage: organizationRepoPageSize,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list repositories for organization %s: %w", organization, err)
		}
		repositories = append(repositories, pageRepos...)
		if len(pageRepos) < organizationRepoPageSize {
			return repositories, nil
		}
	}
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
		previous := *state.setting.ActionEnabled
		status := "unchanged"
		if previous != desired {
			status = "pending"
		}
		repositories = append(repositories, SettingResult{
			Repository:            state.target.Name,
			ProjectID:             state.target.ProjectID,
			PreviousActionEnabled: previous,
			ActionEnabled:         desired,
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
		if repository.Status == "updated" {
			updated++
		}
	}
	fmt.Fprintf(opts.IO.Out, "Actions %sd for %d/%d repositories\n", summary.Action, updated, len(summary.Repositories))
	return nil
}
