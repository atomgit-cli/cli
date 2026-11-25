package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"gitcode.com/gitcode-cli/cli/pkg/testutil"
)

func TestSSHKeyQueries(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		method    string
		path      string
		response  string
		run       func(*Client) error
		wantBody  string
		wantQuery string
	}{
		{
			name: "list", method: http.MethodGet, path: "/api/v5/user/keys",
			response:  `[{"id":1,"title":"work","key":"ssh-ed25519 AAAA"}]`,
			wantQuery: "page=2&per_page=30",
			run: func(client *Client) error {
				keys, err := ListSSHKeys(client, 2, 30)
				if err == nil && (len(keys) != 1 || keys[0].ID != 1) {
					t.Fatalf("keys = %+v", keys)
				}
				return err
			},
		},
		{
			name: "list object example", method: http.MethodGet, path: "/api/v5/user/keys",
			response:  `{"id":6,"title":"example","key":"ssh-ed25519 CCCC"}`,
			wantQuery: "page=1&per_page=20",
			run: func(client *Client) error {
				keys, err := ListSSHKeys(client, 1, 20)
				if err == nil && (len(keys) != 1 || keys[0].ID != 6) {
					t.Fatalf("keys = %+v", keys)
				}
				return err
			},
		},
		{
			name: "create", method: http.MethodPost, path: "/api/v5/user/keys",
			response: `{"id":2,"title":"laptop","key":"ssh-ed25519 BBBB"}`,
			wantBody: `{"title":"laptop","key":"ssh-ed25519 BBBB"}`,
			run: func(client *Client) error {
				key, err := CreateSSHKey(client, &CreateSSHKeyOptions{Title: "laptop", Key: "ssh-ed25519 BBBB"})
				if err == nil && key.ID != 2 {
					t.Fatalf("key = %+v", key)
				}
				return err
			},
		},
		{
			name: "view object", method: http.MethodGet, path: "/api/v5/user/keys/3",
			response: `{"id":3,"title":"desktop"}`,
			run: func(client *Client) error {
				key, err := GetSSHKey(client, 3)
				if err == nil && key.ID != 3 {
					t.Fatalf("key = %+v", key)
				}
				return err
			},
		},
		{
			name: "view array", method: http.MethodGet, path: "/api/v5/user/keys/4",
			response: `[{"id":4,"title":"tablet"}]`,
			run: func(client *Client) error {
				key, err := GetSSHKey(client, 4)
				if err == nil && key.ID != 4 {
					t.Fatalf("key = %+v", key)
				}
				return err
			},
		},
		{
			name: "delete", method: http.MethodDelete, path: "/api/v5/user/keys/5",
			run: func(client *Client) error { return DeleteSSHKey(client, 5) },
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			client := NewClientFromHTTP(&http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != tt.method || req.URL.Path != tt.path {
					t.Fatalf("request = %s %s", req.Method, req.URL.Path)
				}
				if req.URL.RawQuery != tt.wantQuery {
					t.Fatalf("query = %q, want %q", req.URL.RawQuery, tt.wantQuery)
				}
				if tt.wantBody != "" {
					body, _ := io.ReadAll(req.Body)
					var got, want any
					_ = json.Unmarshal(body, &got)
					_ = json.Unmarshal([]byte(tt.wantBody), &want)
					if !equalJSON(got, want) {
						t.Fatalf("body = %s, want %s", body, tt.wantBody)
					}
				}
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(tt.response))}, nil
			})})
			if err := tt.run(client); err != nil {
				t.Fatalf("run() error = %v", err)
			}
		})
	}
}

func equalJSON(a, b any) bool {
	left, _ := json.Marshal(a)
	right, _ := json.Marshal(b)
	return string(left) == string(right)
}

func TestGetSSHKeyRejectsEmptyArray(t *testing.T) {
	t.Parallel()
	client := NewClientFromHTTP(&http.Client{Transport: testutil.NewRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("[]"))}, nil
	})})
	if _, err := GetSSHKey(client, 9); err == nil || !strings.Contains(err.Error(), "empty response") {
		t.Fatalf("error = %v, want empty response", err)
	}
}

func TestSSHKeyQueriesPropagateHTTPError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		method string
		run    func(*Client) error
	}{
		{name: "list", method: http.MethodGet, run: func(client *Client) error {
			_, err := ListSSHKeys(client, 1, 30)
			return err
		}},
		{name: "create", method: http.MethodPost, run: func(client *Client) error {
			_, err := CreateSSHKey(client, &CreateSSHKeyOptions{Title: "work", Key: "ssh-ed25519 QUFBQQ=="})
			return err
		}},
		{name: "view", method: http.MethodGet, run: func(client *Client) error {
			_, err := GetSSHKey(client, 42)
			return err
		}},
		{name: "delete", method: http.MethodDelete, run: func(client *Client) error {
			return DeleteSSHKey(client, 42)
		}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			client := NewClientFromHTTP(&http.Client{Transport: testutil.NewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != tt.method {
					t.Fatalf("method = %s, want %s", req.Method, tt.method)
				}
				return &http.Response{
					StatusCode: http.StatusForbidden,
					Status:     http.StatusText(http.StatusForbidden),
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader("{\"message\":\"forbidden\"}")),
				}, nil
			})})
			if err := tt.run(client); err == nil || !strings.Contains(err.Error(), "forbidden") {
				t.Fatalf("error = %v", err)
			}
		})
	}
}
