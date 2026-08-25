package view

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
	"time"

	"gitcode.com/gitcode-cli/cli/api"
	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
	"gitcode.com/gitcode-cli/cli/pkg/output"
	"gitcode.com/gitcode-cli/cli/pkg/testutil"
)

func TestNewCmdViewTimeFormatEnumAnnotation(t *testing.T) {
	t.Parallel()
	f := cmdutil.TestFactory()
	cmd := NewCmdView(f, nil)
	flag := cmd.Flags().Lookup("time-format")
	if flag == nil {
		t.Fatal("time-format flag not found")
	}
	got := strings.Join(flag.Annotations[cmdutil.FlagEnumAnnotation], ",")
	if got != "absolute,relative" {
		t.Fatalf("time-format enum = %q", got)
	}
}

func TestNewCmdView(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "view issue",
			args:    []string{"123"},
			wantErr: false,
		},
		{
			name:    "view with comments",
			args:    []string{"123", "--comments"},
			wantErr: false,
		},
		{
			name:    "view with relative time",
			args:    []string{"123", "--time-format", "relative"},
			wantErr: false,
		},
		{
			name:    "no issue number",
			args:    []string{},
			wantErr: true,
		},
		{
			name:    "invalid issue number",
			args:    []string{"abc"},
			wantErr: true,
		},
		{
			name:    "invalid time format",
			args:    []string{"123", "--time-format", "yaml"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := cmdutil.TestFactory()
			cmd := NewCmdView(f, func(opts *ViewOptions) error {
				return nil
			})
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseTimeFormat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		want    output.TimeFormat
		wantErr bool
	}{
		{name: "default", input: "", want: output.TimeFormatAbsolute},
		{name: "absolute", input: "absolute", want: output.TimeFormatAbsolute},
		{name: "relative", input: "relative", want: output.TimeFormatRelative},
		{name: "invalid", input: "yaml", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseTimeFormat(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("parseTimeFormat() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseTimeFormat() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("parseTimeFormat() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatViewTime(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)
	ts := api.FlexibleTime{Time: now.Add(-2 * time.Hour)}

	if got := formatViewTime(ts.Time, output.TimeFormatAbsolute, now); got != "2026-03-26 10:00" {
		t.Fatalf("absolute format = %q", got)
	}
	if got := formatViewTime(ts.Time, output.TimeFormatRelative, now); got != "2 hours ago" {
		t.Fatalf("relative format = %q", got)
	}
	if got := formatViewTime(time.Time{}, output.TimeFormatAbsolute, now); got != "unknown" {
		t.Fatalf("zero time format = %q", got)
	}
}

func TestRenderIssueView(t *testing.T) {
	t.Parallel()
	io, _, _, _ := iostreams.Test()
	now := time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)
	closed := api.FlexibleTime{Time: now.Add(-time.Hour)}
	issue := &api.Issue{
		Title:     "Improve issue details",
		Number:    "42",
		State:     "closed",
		HTMLURL:   "https://gitcode.com/owner/repo/issues/42",
		Body:      "Issue body",
		Comments:  1,
		CreatedAt: api.FlexibleTime{Time: now.Add(-3 * time.Hour)},
		UpdatedAt: api.FlexibleTime{Time: now.Add(-30 * time.Minute)},
		ClosedAt:  &closed,
		User:      &api.User{Login: "alice"},
		Milestone: &api.Milestone{Title: "v1.0"},
		Assignees: []*api.User{&api.User{Login: "bob"}},
		Labels:    []*api.Label{&api.Label{Name: "bug"}},
	}
	comments := []api.IssueComment{
		{
			Body:      "Looks good",
			User:      &api.User{Login: "carol"},
			CreatedAt: api.FlexibleTime{Time: now.Add(-15 * time.Minute)},
		},
	}

	var buf bytes.Buffer
	if err := renderIssueView(&buf, io.ColorScheme(), issue, comments, output.TimeFormatRelative, now); err != nil {
		t.Fatalf("renderIssueView() error = %v", err)
	}

	output := buf.String()
	for _, want := range []string{
		"Improve issue details #42",
		"State: closed",
		"Author: alice",
		"Created: 3 hours ago",
		"Updated: 30 minutes ago",
		"Closed: 1 hour ago",
		"Milestone: v1.0",
		"Assignees: bob",
		"Labels: bug",
		"Comments: 1",
		"Issue body",
		"--- Comments (1) ---",
		"carol at 15 minutes ago",
		"Looks good",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("renderIssueView() output missing %q: %s", want, output)
		}
	}
}

func TestViewRunRejectsJSONWithWebBeforeAuth(t *testing.T) {
	t.Parallel()
	io, _, _, _ := testutil.NewTestIOStreams()
	err := viewRun(&ViewOptions{
		IO:     io,
		Web:    true,
		JSON:   true,
		Number: 1,
		HttpClient: func() (*http.Client, error) {
			t.Fatal("HttpClient should not be called when --json and --web conflict")
			return nil, nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "cannot use --json with --web") {
		t.Fatalf("viewRun() error = %v, want json/web usage error", err)
	}
}
