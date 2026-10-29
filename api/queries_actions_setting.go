package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
)

// ActionsSetting represents the repository-level Actions permission settings.
// Pointer fields preserve false values and allow fields absent from a response
// to remain absent from an update request.
type ActionsSetting struct {
	ActionEnabled            *bool `json:"action_enabled,omitempty"`
	BlockAllNewPipelines     *bool `json:"block_all_new_pipelines,omitempty"`
	BlockCrossRepoPRTriggers *bool `json:"block_cross_repo_pr_triggers,omitempty"`
}

// GetActionsSetting gets the repository-level Actions permission settings.
//
// It calls GET /api/v2/projects/{project_id}/actions/setting on the GitCode
// web API host. The endpoint currently returns either the setting object or an
// object envelope containing it in data.
func GetActionsSetting(client *Client, projectID string) (*ActionsSetting, error) {
	endpoint := actionsSettingEndpoint(projectID)
	resp, err := client.RawRESTToHost("GET", WebAPIHost, endpoint, nil, webAPIHeaders())
	if err != nil {
		return nil, err
	}

	settingBody, err := unwrapActionsSetting(resp.Body)
	if err != nil {
		return nil, err
	}

	var setting ActionsSetting
	if err := json.Unmarshal(settingBody, &setting); err != nil {
		return nil, fmt.Errorf("failed to parse actions setting response: %w", err)
	}
	return &setting, nil
}

// UpdateActionsSetting updates the repository-level Actions permission
// settings without sending a project_id in the request body.
//
// The PUT body is built only from the modelled fields above. The endpoint
// replaces the whole setting object, so a field the server returns but this
// struct does not model is not echoed back and will fall back to the server
// default. That is a known trade-off: the alternative would be to round-trip
// unknown fields verbatim, which risks sending back stale or read-only values.
func UpdateActionsSetting(client *Client, projectID string, setting *ActionsSetting) error {
	body, err := json.Marshal(setting)
	if err != nil {
		return fmt.Errorf("failed to marshal actions setting: %w", err)
	}
	_, err = client.RawRESTToHost("PUT", WebAPIHost, actionsSettingEndpoint(projectID), bytes.NewReader(body), webAPIHeaders())
	return err
}

func actionsSettingEndpoint(projectID string) string {
	return "/api/v2/projects/" + url.PathEscape(projectID) + "/actions/setting"
}

func unwrapActionsSetting(body []byte) ([]byte, error) {
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("failed to parse actions setting response: %w", err)
	}
	if len(envelope.Data) > 0 && string(envelope.Data) != "null" {
		return envelope.Data, nil
	}
	return body, nil
}
