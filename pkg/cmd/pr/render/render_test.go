package render

import (
	"bytes"
	"testing"

	"gitcode.com/gitcode-cli/cli/api"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
)

func TestReplyBlock(t *testing.T) {
	io, _, _, _ := iostreams.Test()
	var buf bytes.Buffer
	replies := []api.PRCommentReply{
		{ID: "11", Body: "line 1\nline 2", User: &api.User{Login: "bob"}, CreatedAt: api.FlexibleTime{}},
		{ID: "12", Body: "solo", User: nil},
	}
	ReplyBlock(&buf, io.ColorScheme(), replies, "   ", func(api.FlexibleTime) string { return "T" })

	got := buf.String()
	for _, want := range []string{
		"   Replies (2):",
		"     ↳ ID: 11, bob at T",
		"       line 1",
		"       line 2",
		"     ↳ ID: 12, unknown at T",
		"       solo",
	} {
		if !bytes.Contains([]byte(got), []byte(want)) {
			t.Fatalf("ReplyBlock() output missing %q: %q", want, got)
		}
	}
}

func TestReplyBlockEmptyWritesNothing(t *testing.T) {
	io, _, _, _ := iostreams.Test()
	var buf bytes.Buffer
	ReplyBlock(&buf, io.ColorScheme(), nil, "  ", func(api.FlexibleTime) string { return "T" })
	if buf.Len() != 0 {
		t.Fatalf("ReplyBlock() with no replies wrote %q, want empty", buf.String())
	}
}

func TestAcceptStatusText(t *testing.T) {
	cases := map[string]string{
		"true":     "approved",
		"reject":   "rejected",
		"":         "pending",
		"optional": "pending",
	}
	for in, want := range cases {
		if got := AcceptStatusText(in); got != want {
			t.Errorf("AcceptStatusText(%q) = %q, want %q", in, got, want)
		}
	}
}
