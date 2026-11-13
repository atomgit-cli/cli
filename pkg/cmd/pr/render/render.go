// Package render holds text rendering shared between pr subcommands so the
// same data always renders the same way across commands (issue 652: the
// comments listing and pr view share the nested-reply rendering; the
// reviews command and the view summary share the status classification).
package render

import (
	"fmt"
	"io"
	"strings"

	"gitcode.com/gitcode-cli/cli/api"
	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
	"gitcode.com/gitcode-cli/cli/pkg/iostreams"
)

// ReplyBlock writes a parent comment's nested replies: a "Replies (N):"
// header, then one "↳ ID: ..., author at time" line and indented body lines
// per reply, all offset by indent. timeFn formats a reply's creation time.
// Replies carry no discussion_id of their own; the parent comment owns it.
func ReplyBlock(out io.Writer, cs *iostreams.ColorScheme, replies []api.PRDiscussionReply, indent string, timeFn func(api.FlexibleTime) string) {
	if len(replies) == 0 {
		return
	}
	fmt.Fprintf(out, "%sReplies (%d):\n", indent, len(replies))
	for _, reply := range replies {
		author := "unknown"
		if reply.User != nil {
			author = reply.User.Login
		}
		replyID := cmdutil.FormatAPIID(reply.ID)
		fmt.Fprintf(out, "%s  ↳ ID: %s, %s at %s\n", indent, cs.Cyan(replyID), cs.Bold(author), timeFn(reply.CreatedAt))
		for _, line := range strings.Split(reply.Body, "\n") {
			fmt.Fprintf(out, "%s    %s\n", indent, line)
		}
	}
}

// AcceptStatusText maps the API accept_status enum to display text:
// true → approved, reject → rejected, anything else → pending.
func AcceptStatusText(status string) string {
	switch status {
	case "true":
		return "approved"
	case "reject":
		return "rejected"
	default:
		return "pending"
	}
}
