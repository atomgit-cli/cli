// Package api provides GitCode API client
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"
)

const (
	// DefaultHost is the default GitCode host
	DefaultHost = "api.gitcode.com"
	// DefaultAPIVersion is the default API version
	DefaultAPIVersion = "v5"
	// WebAPIHost is the host for GitCode web API (v2) endpoints such as the
	// Actions plugin directory. It is separate from the default REST host and
	// is whitelisted so that RawRESTToHost can target it safely.
	WebAPIHost = "web-api.gitcode.com"
)

// allowedGitCodeHosts is the whitelist of GitCode API hosts that may be
// targeted explicitly via RawRESTToHost. This preserves SSRF protection for
// arbitrary hosts while enabling access to known GitCode API surfaces.
var allowedGitCodeHosts = map[string]bool{
	DefaultHost: true,
	WebAPIHost:  true,
}

// Client is a GitCode API client
type Client struct {
	httpClient  *http.Client
	host        string
	token       string
	tokenSource string
}

// NewClient creates a new API client
func NewClient(httpClient *http.Client, host, token string) *Client {
	return &Client{
		httpClient: httpClient,
		host:       apiHostForGitCodeHost(host),
		token:      token,
	}
}

// NewClientFromHTTP creates a client from an existing HTTP client
func NewClientFromHTTP(httpClient *http.Client) *Client {
	return &Client{
		httpClient: httpClient,
		host:       DefaultHost,
	}
}

// SetToken sets the authentication token
func (c *Client) SetToken(token, source string) {
	c.token = token
	c.tokenSource = source
}

// SetHost sets the API host
func (c *Client) SetHost(host string) {
	c.host = apiHostForGitCodeHost(host)
}

// Host returns the current host
func (c *Client) Host() string {
	return c.host
}

func apiHostForGitCodeHost(host string) string {
	if host == "" || host == "gitcode.com" {
		return DefaultHost
	}
	if host == DefaultHost {
		return host
	}
	if len(host) > 4 && host[:4] == "api." {
		return host
	}
	return "api." + host
}

// Token returns the current token
func (c *Client) Token() string {
	return c.token
}

// REST performs a REST API call
func (c *Client) REST(method, path string, body interface{}, response interface{}) error {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	reqURL := fmt.Sprintf("https://%s/api/%s%s", c.host, DefaultAPIVersion, path)
	req, err := http.NewRequest(method, reqURL, reqBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	// Add authentication
	if c.token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Check for errors
	if resp.StatusCode >= 400 {
		return decodeAPIError(respBody, resp.StatusCode, resp.Status)
	}

	// Parse response
	if response != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, response); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}
	}

	return nil
}

// RawResponse represents a raw API response body and metadata.
type RawResponse struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

// RawREST performs a REST API call and returns the raw response body.
func (c *Client) RawREST(method, endpoint string, body io.Reader, headers map[string]string) (*RawResponse, error) {
	reqURL, err := c.rawURL(endpoint)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(strings.ToUpper(method), reqURL, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		if strings.TrimSpace(key) != "" {
			req.Header.Set(strings.TrimSpace(key), value)
		}
	}
	if c.token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, decodeAPIError(respBody, resp.StatusCode, resp.Status)
	}

	return &RawResponse{
		StatusCode: resp.StatusCode,
		Header:     resp.Header.Clone(),
		Body:       respBody,
	}, nil
}

// RawRESTToHost performs a REST request to a specific GitCode API host.
// The host must be in the allowedGitCodeHosts whitelist, which preserves
// SSRF protection for arbitrary hosts while enabling access to known GitCode
// API surfaces that live on separate hosts (e.g. web-api.gitcode.com).
// The endpoint should be a relative path beginning with "/".
func (c *Client) RawRESTToHost(method, host, endpoint string, body io.Reader, headers map[string]string) (*RawResponse, error) {
	if !allowedGitCodeHosts[host] {
		return nil, fmt.Errorf("host %s is not a known GitCode API host", host)
	}
	if !strings.HasPrefix(endpoint, "/") {
		endpoint = "/" + endpoint
	}
	reqURL := fmt.Sprintf("https://%s%s", host, endpoint)
	req, err := http.NewRequest(strings.ToUpper(method), reqURL, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		if strings.TrimSpace(key) != "" {
			req.Header.Set(strings.TrimSpace(key), value)
		}
	}
	if c.token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode >= 400 {
		err := decodeAPIError(respBody, resp.StatusCode, resp.Status)
		// Endpoints on the web API host authenticate with the browser session
		// JWT, so the generic "run gc auth login" advice would send the user
		// after a credential that cannot work here. Replace that advice instead
		// of appending a second, contradictory instruction.
		if resp.StatusCode == http.StatusUnauthorized && host == WebAPIHost {
			if apiErr, ok := err.(*APIError); ok {
				apiErr.Guidance = fmt.Sprintf("The %s web API rejected the credential. This host authenticates with the gitcode.com web session JWT, not a classic personal access token: refresh gitcode.com, run copy(localStorage.getItem('access_token')) in the browser Console, and pass the value with --with-token.", host)
				return nil, apiErr
			}
		}
		return nil, err
	}

	return &RawResponse{
		StatusCode: resp.StatusCode,
		Header:     resp.Header.Clone(),
		Body:       respBody,
	}, nil
}

func (c *Client) rawURL(endpoint string) (string, error) {
	endpoint = strings.TrimSpace(endpoint)
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		parsed, err := url.Parse(endpoint)
		if err != nil {
			return "", fmt.Errorf("invalid api endpoint: %w", err)
		}
		if !strings.EqualFold(parsed.Host, c.host) {
			return "", fmt.Errorf("api endpoint host must be %s", c.host)
		}
		return endpoint, nil
	}
	if !strings.HasPrefix(endpoint, "/") {
		endpoint = "/" + endpoint
	}
	if strings.HasPrefix(endpoint, "/api/") {
		return fmt.Sprintf("https://%s%s", c.host, endpoint), nil
	}
	return fmt.Sprintf("https://%s/api/%s%s", c.host, DefaultAPIVersion, endpoint), nil
}

// Get performs a GET request
func (c *Client) Get(path string, response interface{}) error {
	return c.REST("GET", path, nil, response)
}

// GetText performs a GET request and returns the raw response body as text.
func (c *Client) GetText(path string) (string, error) {
	reqURL := fmt.Sprintf("https://%s/api/%s%s", c.host, DefaultAPIVersion, path)
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "*/*")
	if c.token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return "", decodeAPIError(respBody, resp.StatusCode, resp.Status)
	}

	return string(respBody), nil
}

// Post performs a POST request
func (c *Client) Post(path string, body interface{}, response interface{}) error {
	return c.REST("POST", path, body, response)
}

// PostForm performs a POST request with form data.
func (c *Client) PostForm(path string, formValues url.Values, response interface{}) error {
	reqBody := formValues.Encode()

	reqURL := fmt.Sprintf("https://%s/api/%s%s", c.host, DefaultAPIVersion, path)
	req, err := http.NewRequest("POST", reqURL, bytes.NewReader([]byte(reqBody)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if c.token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return decodeAPIError(respBody, resp.StatusCode, resp.Status)
	}

	if response != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, response); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}
	}

	return nil
}

// Put performs a PUT request
func (c *Client) Put(path string, body interface{}, response interface{}) error {
	return c.REST("PUT", path, body, response)
}

// Patch performs a PATCH request
func (c *Client) Patch(path string, body interface{}, response interface{}) error {
	return c.REST("PATCH", path, body, response)
}

// Delete performs a DELETE request
func (c *Client) Delete(path string) error {
	return c.REST("DELETE", path, nil, nil)
}

// PatchForm performs a PATCH request with form data
func (c *Client) PatchForm(path string, formValues url.Values, response interface{}) error {
	reqBody := formValues.Encode()

	reqURL := fmt.Sprintf("https://%s/api/%s%s", c.host, DefaultAPIVersion, path)
	req, err := http.NewRequest("PATCH", reqURL, bytes.NewReader([]byte(reqBody)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Add authentication
	if c.token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Check for errors
	if resp.StatusCode >= 400 {
		return decodeAPIError(respBody, resp.StatusCode, resp.Status)
	}

	// Parse response
	if response != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, response); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}
	}

	return nil
}

// UploadToURL uploads a file to an external URL with custom headers
func (c *Client) UploadToURL(uploadURL, filename string, content []byte, contentType string, headers map[string]string) error {
	req, err := http.NewRequest("PUT", uploadURL, bytes.NewReader(content))
	if err != nil {
		return fmt.Errorf("failed to create upload request: %w", err)
	}

	// Set Content-Type
	req.Header.Set("Content-Type", contentType)

	// Set custom headers from API response
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Check for errors
	if resp.StatusCode >= 400 {
		const maxBodyPreview = 512
		preview := string(respBody)
		if len(preview) > maxBodyPreview {
			cutoff := maxBodyPreview
			for cutoff > 0 && !utf8.RuneStart(preview[cutoff]) {
				cutoff--
			}
			preview = preview[:cutoff] + "...(truncated)"
		}
		return fmt.Errorf("upload failed: %s - %s", resp.Status, preview)
	}

	return nil
}

// UploadAsset uploads a file to a release
func (c *Client) UploadAsset(path, filename string, content []byte, contentType string) (*ReleaseAsset, error) {
	reqURL := fmt.Sprintf("https://%s/api/%s%s?name=%s", c.host, DefaultAPIVersion, path, url.QueryEscape(filename))
	req, err := http.NewRequest("POST", reqURL, bytes.NewReader(content))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", contentType)

	// Add authentication
	if c.token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upload failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for errors
	if resp.StatusCode >= 400 {
		return nil, decodeAPIError(respBody, resp.StatusCode, resp.Status)
	}

	// Parse response
	var asset ReleaseAsset
	if err := json.Unmarshal(respBody, &asset); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &asset, nil
}

// APIError represents an API error
type APIError struct {
	StatusCode    int    `json:"-"`
	ErrorCode     int    `json:"error_code"`
	Message       string `json:"message"`
	ErrorMessage  string `json:"error_message"`
	ErrorName     string `json:"error"`
	ErrorCodeName string `json:"error_code_name"`

	// Guidance replaces the status-specific guidance that Error appends. A
	// caller sets it when the generic advice would be wrong for the endpoint,
	// for example the personal-access-token hint on a 401 from a host that
	// authenticates with a browser session JWT.
	Guidance string `json:"-"`
}

func (e *APIError) Error() string {
	var msg string
	if e.Message != "" {
		msg = e.Message
	} else if e.ErrorMessage != "" {
		msg = e.ErrorMessage
	} else {
		msg = "unknown error"
	}

	// Caller-supplied guidance replaces the generic status guidance so the two
	// never contradict each other.
	if e.Guidance != "" {
		return fmt.Sprintf("HTTP %d: %s\n\n%s", e.StatusCode, msg, e.Guidance)
	}

	// Add actionable guidance for permission errors
	if e.StatusCode == 403 {
		return fmt.Sprintf("HTTP %d: %s\n\nYou don't have permission for this action. Possible reasons:\n- You are not the owner or a project maintainer\n- The project requires specific permissions\n\nFor permission details, check the project settings or contact a maintainer.", e.StatusCode, msg)
	}

	// Add actionable guidance for authentication errors
	if e.StatusCode == 401 {
		return fmt.Sprintf("HTTP %d: %s\n\nYour token may be invalid or expired. Try:\n- Run `gc auth login` to re-authenticate\n- Check that GC_TOKEN or GITCODE_TOKEN is set correctly", e.StatusCode, msg)
	}

	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, msg)
}

// decodeAPIError attempts to decode an API error from the response body.
// Returns *APIError if the body contains a recognized error structure,
// otherwise returns a generic error with the HTTP status.
func decodeAPIError(respBody []byte, statusCode int, status string) error {
	var apiErr APIError
	if err := json.Unmarshal(respBody, &apiErr); err == nil &&
		(apiErr.Message != "" || apiErr.ErrorMessage != "" || apiErr.ErrorName != "" || apiErr.ErrorCodeName != "") {
		apiErr.StatusCode = statusCode
		return &apiErr
	}
	return &APIError{StatusCode: statusCode, Message: status}
}

// BuildURL builds a URL with path parameters
func BuildURL(base string, params map[string]string) string {
	for k, v := range params {
		base = replacePath(base, k, v)
	}
	return base
}

func replacePath(urlStr, key, value string) string {
	placeholder := "{" + key + "}"
	return strings.ReplaceAll(urlStr, placeholder, url.PathEscape(value))
}
