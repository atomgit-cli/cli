package api

import (
	"encoding/json"
	"fmt"
	"net/url"
)

// ActionsPlugin represents a plugin entry in the Actions plugin list response.
type ActionsPlugin struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
	Version     string `json:"version"`
}

// ActionsPluginVersionEntry represents a single version entry within the
// vision_content array of a plugin detail response.
type ActionsPluginVersionEntry struct {
	Version string `json:"version"`
	Readme  string `json:"readme"`
}

// ActionsPluginDetail represents the full plugin detail response.
type ActionsPluginDetail struct {
	Name          string                      `json:"name"`
	DisplayName   string                      `json:"display_name"`
	Description   string                      `json:"description"`
	VisionContent []ActionsPluginVersionEntry `json:"vision_content"`
}

// ActionsPluginsPage represents a paginated Actions plugin list response.
type ActionsPluginsPage struct {
	PageNum   int
	PageSize  int
	Total     int
	PageCount int
	Entries   []json.RawMessage
}

// ActionsListPluginsOptions controls pagination for ListActionsPlugins.
type ActionsListPluginsOptions struct {
	PerPage int
	Page    int
}

// webAPIHeaders returns headers required by the web-api.gitcode.com plugin
// directory endpoints, including a Referer for same-site request validation.
func webAPIHeaders() map[string]string {
	return map[string]string{
		"Referer": "https://gitcode.com/",
	}
}

// ListActionsPlugins lists official Actions plugins for a project.
//
// It calls GET https://web-api.gitcode.com/api/v2/projects/{project}/actions/plugins/all.
// The project path is the URL-encoded "owner/repo" form. The raw response body
// is returned so callers can preserve full API fields for --json output.
func ListActionsPlugins(client *Client, project string, opts *ActionsListPluginsOptions) ([]byte, error) {
	endpoint := "/api/v2/projects/" + url.PathEscape(project) + "/actions/plugins/all"
	if opts != nil {
		endpoint += newQueryBuilder().
			SetInt("per_page", opts.PerPage).
			SetInt("page", opts.Page).
			String()
	}

	resp, err := client.RawRESTToHost("GET", WebAPIHost, endpoint, nil, webAPIHeaders())
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// ViewActionsPlugin retrieves the detail of a specific Actions plugin.
//
// It calls
// GET https://web-api.gitcode.com/api/v2/projects/{project}/actions/plugins/detail?name={name}.
// The raw response body is returned so callers can preserve full API fields
// for --json output.
func ViewActionsPlugin(client *Client, project, name string) ([]byte, error) {
	endpoint := "/api/v2/projects/" + url.PathEscape(project) + "/actions/plugins/detail" +
		newQueryBuilder().Set("name", name).String()

	resp, err := client.RawRESTToHost("GET", WebAPIHost, endpoint, nil, webAPIHeaders())
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// ParseActionsPluginsList parses the raw list response into typed plugin
// entries. It handles both a plain JSON array and a paginated wrapper object
// with common field names ("content", "plugins", "list", or "data").
func ParseActionsPluginsList(raw []byte) ([]ActionsPlugin, error) {
	entries, err := ParseActionsPluginsListRaw(raw)
	if err != nil {
		return nil, err
	}
	plugins := make([]ActionsPlugin, 0, len(entries))
	for _, entry := range entries {
		var plugin ActionsPlugin
		if err := json.Unmarshal(entry, &plugin); err != nil {
			return nil, fmt.Errorf("failed to parse plugin entry: %w", err)
		}
		plugins = append(plugins, plugin)
	}
	return plugins, nil
}

// ParseActionsPluginsListRaw parses the raw list response into raw JSON
// entries, preserving full API fields for --json output. It handles both a
// plain JSON array and a paginated wrapper object with server metadata.
func ParseActionsPluginsListRaw(raw []byte) ([]json.RawMessage, error) {
	page, err := ParseActionsPluginsPage(raw)
	if err != nil {
		return nil, err
	}
	return page.Entries, nil
}

// ParseActionsPluginsPage parses a plain or paginated Actions plugin list
// response and retains the server pagination metadata.
func ParseActionsPluginsPage(raw []byte) (*ActionsPluginsPage, error) {
	var entries []json.RawMessage
	if err := json.Unmarshal(raw, &entries); err == nil {
		return &ActionsPluginsPage{
			Entries: entries,
		}, nil
	}

	var metadata struct {
		PageNum   int `json:"page_num"`
		PageSize  int `json:"page_size"`
		Total     int `json:"total"`
		PageCount int `json:"page_count"`
	}
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse plugins list response: %w", err)
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, fmt.Errorf("failed to parse plugins list response: %w", err)
	}
	entries, err := parsePluginListField(fields)
	if err != nil {
		return nil, err
	}
	return &ActionsPluginsPage{
		PageNum:   metadata.PageNum,
		PageSize:  metadata.PageSize,
		Total:     metadata.Total,
		PageCount: metadata.PageCount,
		Entries:   entries,
	}, nil
}

func parsePluginListField(fields map[string]json.RawMessage) ([]json.RawMessage, error) {
	for _, field := range []string{"content", "plugins", "list", "data"} {
		raw, ok := fields[field]
		if !ok {
			continue
		}
		if string(raw) == "null" {
			return []json.RawMessage{}, nil
		}
		var entries []json.RawMessage
		if err := json.Unmarshal(raw, &entries); err != nil {
			return nil, fmt.Errorf("failed to parse plugins list response field %s: %w", field, err)
		}
		return entries, nil
	}
	return nil, fmt.Errorf("failed to parse plugins list response: missing plugin list field")
}

// CountPluginsEntries returns the number of plugin entries in a raw list
// response, used to detect short pages during pagination.
func CountPluginsEntries(raw []byte) int {
	entries, err := ParseActionsPluginsListRaw(raw)
	if err != nil {
		return 0
	}
	return len(entries)
}
