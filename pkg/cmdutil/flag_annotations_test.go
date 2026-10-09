package cmdutil

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestSetFlagEnumSetsAnnotationOnRegisteredFlag(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("state", "", "issue state")
	if err := SetFlagEnum(cmd, "state", "open", "closed"); err != nil {
		t.Fatalf("SetFlagEnum() error = %v, want nil", err)
	}
	got, ok := cmd.Flags().Lookup("state").Annotations[FlagEnumAnnotation]
	if !ok {
		t.Fatal("annotation not set")
	}
	if strings.Join(got, ",") != "open,closed" {
		t.Fatalf("annotation = %v, want [open closed]", got)
	}
}

func TestSetFlagEnumReturnsErrorOnUnregisteredFlag(t *testing.T) {
	cmd := &cobra.Command{}
	// "missing" is not a registered flag → SetAnnotation fails.
	err := SetFlagEnum(cmd, "missing", "a", "b")
	if err == nil {
		t.Fatal("SetFlagEnum() error = nil, want error for unregistered flag")
	}
	if !strings.Contains(err.Error(), "failed to annotate flag") {
		t.Fatalf("error = %q, want 'failed to annotate flag'", err.Error())
	}
}

// TestSetFlagEnumOrWarnDoesNotPanicOnUnregisteredFlag is the core #415 guard:
// a constructor calling SetFlagEnumOrWarn with a bad flag must NOT crash the
// CLI (the previous implementation panic'd).
func TestSetFlagEnumOrWarnDoesNotPanicOnUnregisteredFlag(t *testing.T) {
	cmd := &cobra.Command{}
	stderr := captureStderr(t, func() {
		SetFlagEnumOrWarn(cmd, "missing", "a", "b")
	})
	if !strings.Contains(stderr, "warning") {
		t.Fatalf("stderr = %q, want a warning about the failed annotation", stderr)
	}
}

func TestSetFlagEnumOrWarnSilentOnRegisteredFlag(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("state", "", "issue state")
	stderr := captureStderr(t, func() {
		SetFlagEnumOrWarn(cmd, "state", "open", "closed")
	})
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty (annotation succeeded)", stderr)
	}
	got, ok := cmd.Flags().Lookup("state").Annotations[FlagEnumAnnotation]
	if !ok {
		t.Fatal("annotation not set")
	}
	if strings.Join(got, ",") != "open,closed" {
		t.Fatalf("annotation = %v, want [open closed]", got)
	}
}

// TestSetTopicAnnotationInitializesNilMap verifies SetTopicAnnotation creates
// the Annotations map when it is nil instead of panicking.
func TestSetTopicAnnotationInitializesNilMap(t *testing.T) {
	cmd := &cobra.Command{}
	if cmd.Annotations != nil {
		t.Fatal("precondition: Annotations should be nil on a fresh command")
	}

	SetTopicAnnotation(cmd, "issues")

	if cmd.Annotations == nil {
		t.Fatal("Annotations = nil after SetTopicAnnotation, want initialized map")
	}
	if got := cmd.Annotations[TopicAnnotation]; got != "issues" {
		t.Fatalf("topic annotation = %q, want %q", got, "issues")
	}
}

// TestSetTopicAnnotationOverwritesExistingTopic verifies a second call
// replaces the previous topic value.
func TestSetTopicAnnotationOverwritesExistingTopic(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Annotations = map[string]string{"unrelated": "value"}

	SetTopicAnnotation(cmd, "repo")

	if got := cmd.Annotations[TopicAnnotation]; got != "repo" {
		t.Fatalf("topic annotation = %q, want %q", got, "repo")
	}
	if got := cmd.Annotations["unrelated"]; got != "value" {
		t.Fatalf("unrelated annotation = %q, want preserved", got)
	}
}

// captureStderr runs fn while os.Stderr is redirected to a pipe, returning the
// captured output. The previous process-global os.Stderr is restored.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer r.Close()
	old := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = old }()

	done := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	fn()
	_ = w.Close()
	return <-done
}
