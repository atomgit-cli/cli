package cmdutil

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const FlagEnumAnnotation = "gc.enum"
const TopicAnnotation = "gc.topic"

// NoUpdateLifecycleAnnotation marks protocol commands whose stderr the
// calling program passes through to the user (git credential helpers): the
// root update lifecycle (first-run notice, pending summaries) must skip
// them so its output never leaks into the protocol stream (issue 647).
// Presence semantics: any non-empty value opts out; the value is not
// matched against a "true" literal — write any non-empty string.
const NoUpdateLifecycleAnnotation = "gc/no-update-lifecycle"

// StandardTopics is the list of standard topic categories.
var StandardTopics = []string{
	"auth",
	"commits",
	"issues",
	"labels",
	"milestones",
	"precommit",
	"pull-requests",
	"releases",
	"repo",
}

// SetFlagEnum records a stable enum set for schema/export consumers. It
// returns an error (e.g. when the flag is not registered on the command)
// instead of panicking, so callers can decide how to handle it.
func SetFlagEnum(cmd *cobra.Command, name string, values ...string) error {
	if err := cmd.Flags().SetAnnotation(name, FlagEnumAnnotation, values); err != nil {
		return fmt.Errorf("failed to annotate flag %q: %w", name, err)
	}
	return nil
}

// SetFlagEnumOrWarn records a stable enum set for schema/export consumers and
// degrades an annotation failure to a stderr warning instead of crashing the
// process. Use this from command constructors (NewCmd*) whose signature
// (*cobra.Command) cannot propagate an error; the annotation is non-functional
// metadata for schema/export, so a missing enum does not break the command.
func SetFlagEnumOrWarn(cmd *cobra.Command, name string, values ...string) {
	if err := SetFlagEnum(cmd, name, values...); err != nil {
		fmt.Fprintf(os.Stderr, "gc: warning: %v\n", err)
	}
}

// SetTopicAnnotation adds a topic annotation to a command.
func SetTopicAnnotation(cmd *cobra.Command, topic string) {
	if cmd.Annotations == nil {
		cmd.Annotations = make(map[string]string)
	}
	cmd.Annotations[TopicAnnotation] = topic
}
