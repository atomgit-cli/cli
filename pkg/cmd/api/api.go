// Package api implements the generic api command.
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/itchyny/gojq"
	"github.com/spf13/cobra"

	"gitcode.com/gitcode-cli/cli/api"
	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
)

// paginatePageLimit bounds --paginate so an endpoint that ignores the page
// parameter cannot loop forever.
const paginatePageLimit = 100

// defaultPaginatePerPage is the page size injected when the endpoint query
// does not already carry one.
const defaultPaginatePerPage = 100

type Options struct {
	IO         *iostreams.IOStreams
	HttpClient func() (*http.Client, error)

	Endpoint  string
	Method    string
	MethodSet bool
	Headers   []string
	Input     string

	// Include prints the response status line and headers before the body.
	Include bool
	// Paginate follows the page query parameter until an empty page.
	Paginate bool
	// Slurp merges paginated pages into a single JSON array (requires Paginate).
	Slurp bool
	// JQ filters the output through a jq expression.
	JQ string
	// Fields are repeated key=value pairs sent as a JSON request body.
	Fields []string
}

// NewCmdAPI creates the generic api command.
func NewCmdAPI(f *cmdutil.Factory, runF func(*Options) error) *cobra.Command {
	opts := &Options{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		Method:     http.MethodGet,
	}

	cmd := &cobra.Command{
		Use:   "api <endpoint>",
		Short: "Make an authenticated GitCode REST API request",
		Long: heredoc.Doc(`
			Make an authenticated GitCode REST API request.

			The endpoint may be written with or without a leading slash. If it does
			not start with /api/, the CLI prefixes /api/v5 automatically.
		`),
		Example: heredoc.Doc(`
			# Get PR changed files
			$ gc api repos/owner/repo/pulls/123/files

			# Query commits for a file on a branch
			$ gc api 'repos/owner/repo/commits?path=src/main.go&sha=main'

			# Send a PATCH request with a JSON body
			$ gc api repos/owner/repo/pulls/123 --method PATCH --input body.json

			# Inspect response headers (ETag, rate limit) and filter output
			$ gc api repos/owner/repo --include
			$ gc api user --jq .login

			# Fetch every page of an issue list, merged into one array
			$ gc api 'repos/owner/repo/issues?state=all&per_page=100' --paginate --slurp

			# Create a comment with simple JSON fields
			$ gc api repos/owner/repo/issues/1/comments -f body='hello'
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Endpoint = args[0]
			opts.MethodSet = cmd.Flags().Changed("method")
			if runF != nil {
				return runF(opts)
			}
			return run(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Method, "method", "X", http.MethodGet, "HTTP method")
	cmd.Flags().StringArrayVarP(&opts.Headers, "header", "H", nil, "Additional HTTP header (Name: value)")
	cmd.Flags().StringVar(&opts.Input, "input", "", "Read request body from file (use - for stdin)")
	cmd.Flags().BoolVar(&opts.Include, "include", false, "Include the response status line and headers in the output")
	cmd.Flags().BoolVar(&opts.Paginate, "paginate", false, "Follow the page query parameter until an empty page")
	cmd.Flags().BoolVar(&opts.Slurp, "slurp", false,
		"Merge paginated pages into a single JSON array (requires --paginate)")
	cmd.Flags().StringVar(&opts.JQ, "jq", "", "Filter the response body through a jq expression")
	cmd.Flags().StringArrayVarP(&opts.Fields, "field", "f", nil,
		"Add a key=value string field; fields are sent as a JSON request body")

	return cmd
}

func run(opts *Options) error {
	if err := validateFlagCombos(opts); err != nil {
		return err
	}

	jqCode, err := parseJQFilter(opts.JQ)
	if err != nil {
		return err
	}

	client, err := cmdutil.AuthenticatedClientFromFactory(opts.HttpClient)
	if err != nil {
		return err
	}

	body, closer, err := buildRequestBody(opts)
	if err != nil {
		return err
	}
	if closer != nil {
		defer closer.Close()
	}

	headers, err := parseHeaders(opts.Headers)
	if err != nil {
		return err
	}

	method := strings.ToUpper(strings.TrimSpace(opts.Method))
	if method == "" {
		method = http.MethodGet
	}
	if !opts.MethodSet && body != nil && method == http.MethodGet {
		method = http.MethodPost
	}

	if opts.Paginate {
		return runPaginated(opts, client, headers, jqCode)
	}

	resp, err := client.RawREST(method, opts.Endpoint, body, headers)
	if err != nil {
		return err
	}
	return writeResponse(opts, resp, jqCode)
}

// validateFlagCombos rejects flag combinations and formats that cannot work
// together before authentication or any network activity happens, so usage
// errors are reported with exit 2 even in unauthenticated environments.
func validateFlagCombos(opts *Options) error {
	if opts.Paginate {
		method := strings.ToUpper(strings.TrimSpace(opts.Method))
		if method != "" && method != http.MethodGet {
			return cmdutil.NewUsageError("--paginate requires a GET request")
		}
		if strings.TrimSpace(opts.Input) != "" {
			return cmdutil.NewUsageError("--paginate cannot be combined with --input")
		}
		if len(opts.Fields) > 0 {
			return cmdutil.NewUsageError("--paginate cannot be combined with --field")
		}
	} else if opts.Slurp {
		return cmdutil.NewUsageError("--slurp requires --paginate")
	}
	if opts.Slurp && strings.TrimSpace(opts.JQ) != "" {
		// gh rejects this combination too; silently dropping the filter
		// would hand consumers unfiltered bulk data.
		return cmdutil.NewUsageError("--slurp cannot be combined with --jq")
	}
	if len(opts.Fields) > 0 {
		if strings.TrimSpace(opts.Input) != "" {
			return cmdutil.NewUsageError("--field and --input cannot be combined; choose one request body source")
		}
		for _, raw := range opts.Fields {
			key, _, ok := strings.Cut(raw, "=")
			if !ok || strings.TrimSpace(key) == "" {
				return cmdutil.NewUsageError("--field must be in 'key=value' format")
			}
		}
	}
	return nil
}

// parseJQFilter compiles the --jq expression, or returns nil when unset.
func parseJQFilter(filter string) (*gojq.Code, error) {
	if strings.TrimSpace(filter) == "" {
		return nil, nil
	}
	query, err := gojq.Parse(filter)
	if err != nil {
		return nil, cmdutil.NewUsageError(fmt.Sprintf("invalid --jq filter: %v", err))
	}
	// Compile once (pagination reuses it per page). Never pass
	// WithEnvironLoader(os.Environ) here: it would let --jq expressions
	// read $ENV.GC_TOKEN and print the token to stdout.
	code, err := gojq.Compile(query)
	if err != nil {
		return nil, cmdutil.NewUsageError(fmt.Sprintf("invalid --jq filter: %v", err))
	}
	return code, nil
}

// buildRequestBody resolves the request body from --field pairs or --input.
// Field format validation happens earlier in validateFlagCombos.
func buildRequestBody(opts *Options) (io.Reader, io.Closer, error) {
	if len(opts.Fields) > 0 {
		obj := make(map[string]string, len(opts.Fields))
		for _, raw := range opts.Fields {
			key, value, _ := strings.Cut(raw, "=")
			obj[strings.TrimSpace(key)] = value
		}
		data, err := json.Marshal(obj)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to encode --field body: %w", err)
		}
		return bytes.NewReader(data), nil, nil
	}

	body, err := readInput(opts)
	if err != nil {
		return nil, nil, err
	}
	var closer io.Closer
	if body != nil {
		if rc, ok := body.(io.Closer); ok {
			closer = rc
		}
	}
	return body, closer, nil
}

// runPaginated fetches pages by injecting the page query parameter until a
// page comes back empty. Non-slurp output emits each page body as-is (or
// filtered); --slurp merges all items into one JSON array.
func runPaginated(opts *Options, client *api.Client, headers map[string]string, jqCode *gojq.Code) error {
	merged := make([]json.RawMessage, 0)
	var firstResp *api.RawResponse
	for page := 1; ; page++ {
		if page > paginatePageLimit {
			return pageLimitError(opts.Endpoint)
		}
		resp, err := fetchPage(client, opts.Endpoint, page, headers)
		if err != nil {
			return err
		}
		if firstResp == nil {
			firstResp = resp
		}
		if resp.StatusCode == http.StatusNotModified {
			return handleNotModified(opts, resp)
		}

		items, err := decodePageItems(resp)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			break
		}
		if opts.Slurp {
			merged = append(merged, items...)
			continue
		}
		if err := emitPage(opts, resp, jqCode); err != nil {
			return err
		}
	}

	if !opts.Slurp {
		return nil
	}
	// The include block for --slurp comes from the first page: it carries
	// the request-level metadata (e.g. ETag) of the collection start.
	if firstResp != nil && opts.Include {
		if err := writeInclude(opts.IO.Out, firstResp); err != nil {
			return err
		}
	}
	mergedBody, err := json.Marshal(merged)
	if err != nil {
		return fmt.Errorf("failed to merge paginated pages: %w", err)
	}
	return writeBody(opts.IO.Out, mergedBody)
}

// fetchPage performs one GET for the given page of the endpoint.
func fetchPage(client *api.Client, endpoint string, page int, headers map[string]string) (*api.RawResponse, error) {
	pageEndpoint, err := withPageQuery(endpoint, page)
	if err != nil {
		return nil, err
	}
	return client.RawREST(http.MethodGet, pageEndpoint, nil, headers)
}

// emitPage writes one non-slurp page honoring --include and --jq.
func emitPage(opts *Options, resp *api.RawResponse, jqCode *gojq.Code) error {
	if opts.Include {
		if err := writeInclude(opts.IO.Out, resp); err != nil {
			return err
		}
	}
	if jqCode != nil {
		return writeJQ(opts.IO.Out, jqCode, resp.Body)
	}
	return writeBody(opts.IO.Out, resp.Body)
}

// decodePageItems parses a page body as a JSON array of items.
func decodePageItems(resp *api.RawResponse) ([]json.RawMessage, error) {
	var items []json.RawMessage
	if err := json.Unmarshal(resp.Body, &items); err != nil {
		return nil, fmt.Errorf("--paginate requires a JSON array response, got %q: %w", previewBody(resp.Body), err)
	}
	return items, nil
}

// handleNotModified emits the 304 include block when requested and reports
// the non-zero "resource unchanged" error.
func handleNotModified(opts *Options, resp *api.RawResponse) error {
	if opts.Include {
		if err := writeInclude(opts.IO.Out, resp); err != nil {
			return err
		}
	}
	return notModifiedError()
}

// pageLimitError reports the --paginate safety cap.
func pageLimitError(endpoint string) error {
	return fmt.Errorf("--paginate exceeded the page limit of %d for %q: narrow the query "+
		"(per_page, filters) or iterate pages manually", paginatePageLimit, endpoint)
}

// withPageQuery returns the endpoint with page set and per_page defaulted.
func withPageQuery(endpoint string, page int) (string, error) {
	path, rawQuery, hasQuery := strings.Cut(endpoint, "?")
	q := url.Values{}
	if hasQuery {
		parsed, err := url.ParseQuery(rawQuery)
		if err != nil {
			return "", fmt.Errorf("invalid endpoint query: %w", err)
		}
		q = parsed
	}
	q.Set("page", strconv.Itoa(page))
	if q.Get("per_page") == "" {
		q.Set("per_page", strconv.Itoa(defaultPaginatePerPage))
	}
	return path + "?" + q.Encode(), nil
}

// writeResponse emits the response honoring --include and --jq.
func writeResponse(opts *Options, resp *api.RawResponse, jqCode *gojq.Code) error {
	if resp.StatusCode == http.StatusNotModified {
		return handleNotModified(opts, resp)
	}
	if opts.Include {
		if err := writeInclude(opts.IO.Out, resp); err != nil {
			return err
		}
	}
	if len(resp.Body) == 0 {
		return nil
	}
	if jqCode != nil {
		return writeJQ(opts.IO.Out, jqCode, resp.Body)
	}
	return writeBody(opts.IO.Out, resp.Body)
}

// notModifiedError reports a conditional request hit: non-zero exit with the
// status visible, so consumers can treat "304 + non-zero" as "no change".
func notModifiedError() error {
	return cmdutil.NewCLIError(cmdutil.ExitError,
		"HTTP 304 Not Modified (resource unchanged; keep the ETag from --include output)", nil)
}

// writeInclude prints the status line and response headers (sorted for
// deterministic output), then a blank line.
func writeInclude(w io.Writer, resp *api.RawResponse) error {
	status := http.StatusText(resp.StatusCode)
	if status == "" {
		status = strconv.Itoa(resp.StatusCode)
	}
	if _, err := fmt.Fprintf(w, "HTTP/1.1 %d %s\n", resp.StatusCode, status); err != nil {
		return err
	}
	keys := make([]string, 0, len(resp.Header))
	for key := range resp.Header {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		for _, value := range resp.Header[key] {
			if _, err := fmt.Fprintf(w, "%s: %s\n", key, value); err != nil {
				return err
			}
		}
	}
	_, err := fmt.Fprintln(w)
	return err
}

// writeJQ applies the compiled jq expression to the response body and writes
// one result per line; string results are printed raw, everything else as
// compact JSON.
func writeJQ(w io.Writer, code *gojq.Code, body []byte) error {
	var value interface{}
	if err := json.Unmarshal(body, &value); err != nil {
		return fmt.Errorf("failed to parse response body as JSON for --jq: %w", err)
	}
	iter := code.Run(value)
	for {
		result, ok := iter.Next()
		if !ok {
			return nil
		}
		if err, isErr := result.(error); isErr {
			return fmt.Errorf("--jq filter error: %v", err)
		}
		if s, isString := result.(string); isString {
			if _, err := fmt.Fprintln(w, s); err != nil {
				return err
			}
			continue
		}
		data, err := json.Marshal(result)
		if err != nil {
			return fmt.Errorf("failed to encode --jq result: %w", err)
		}
		if _, err := w.Write(data); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}
}

// writeBody writes the raw response body and guarantees a trailing newline.
func writeBody(w io.Writer, body []byte) error {
	if _, err := w.Write(body); err != nil {
		return err
	}
	if !bytes.HasSuffix(body, []byte("\n")) {
		_, err := fmt.Fprintln(w)
		return err
	}
	return nil
}

// previewBody shortens a body preview for error messages.
func previewBody(body []byte) string {
	const max = 80
	if len(body) > max {
		return string(body[:max]) + "..."
	}
	return string(body)
}

func readInput(opts *Options) (io.Reader, error) {
	if strings.TrimSpace(opts.Input) == "" {
		return nil, nil
	}
	if opts.Input == "-" {
		// Stream stdin directly without buffering it into memory.
		return io.NopCloser(opts.IO.In), nil
	}
	// Stream the file directly as the request body instead of reading it
	// fully into memory; this mirrors `gh api --input` (openUserFile) and
	// avoids unbounded memory use for large inputs.
	f, err := os.Open(opts.Input)
	if err != nil {
		return nil, fmt.Errorf("failed to open input file: %w", err)
	}
	return f, nil
}

func parseHeaders(values []string) (map[string]string, error) {
	headers := map[string]string{}
	for _, raw := range values {
		name, value, ok := strings.Cut(raw, ":")
		if !ok || strings.TrimSpace(name) == "" {
			return nil, cmdutil.NewUsageError("--header must be in 'Name: value' format")
		}
		headers[strings.TrimSpace(name)] = strings.TrimSpace(value)
	}
	return headers, nil
}
