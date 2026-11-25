package contracts

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// cleanupWorkflowDocument captures the scheduled artifact-cleanup workflow.
// It reuses workflowStep/workflowJob shapes where possible but needs its own
// trigger block, because the cleanup workflow is schedule-only.
type cleanupWorkflowDocument struct {
	On          cleanupTriggers               `yaml:"on"`
	Permissions map[string]string             `yaml:"permissions"`
	Env         map[string]any                `yaml:"env"`
	Jobs        map[string]cleanupWorkflowJob `yaml:"jobs"`
}

type cleanupTriggers struct {
	Schedule         []cleanupSchedule `yaml:"schedule"`
	WorkflowDispatch yaml.Node         `yaml:"workflow_dispatch"`
	Push             yaml.Node         `yaml:"push"`
	PullRequest      yaml.Node         `yaml:"pull_request"`
}

// triggerPresent reports whether a trigger key exists at all. A trigger with
// no body (workflow_dispatch:) decodes as a null scalar node, so decoding into
// any would lose the distinction between "absent" and "declared".
func triggerPresent(node yaml.Node) bool {
	return node.Kind != 0
}

type cleanupSchedule struct {
	Cron string `yaml:"cron"`
}

type cleanupWorkflowJob struct {
	RunsOn stringList     `yaml:"runs-on"`
	Steps  []workflowStep `yaml:"steps"`
}

func readCleanupWorkflow(t *testing.T) cleanupWorkflowDocument {
	t.Helper()
	path := filepath.Join("..", "..", ".gitcode", "workflows", "artifact-cleanup.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var doc cleanupWorkflowDocument
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return doc
}

// TestArtifactCleanupWorkflowContract pins the scheduled cleanup workflow:
// keep-together invariants between the upload retention window and the
// cleanup job (issue 666). Without this contract a future edit could raise
// retention-days in ci.yml while the pruner keeps deleting at the old window,
// or the pruner could silently stop running.
func TestArtifactCleanupWorkflowContract(t *testing.T) {
	doc := readCleanupWorkflow(t)
	ci := readWorkflow(t)
	if problems := validateCleanupWorkflow(doc, ci); len(problems) > 0 {
		t.Fatalf("artifact cleanup workflow contract violations:\n- %s", strings.Join(problems, "\n- "))
	}
}

func validateCleanupWorkflow(doc cleanupWorkflowDocument, ci workflowDocument) []string {
	var problems []string
	problems = append(problems, validateCleanupTriggers(doc)...)
	problems = append(problems, validateCleanupPermissions(doc)...)
	problems = append(problems, validateCleanupSteps(doc)...)
	problems = append(problems, validateCleanupRetention(doc, ci)...)
	return problems
}

func validateCleanupTriggers(doc cleanupWorkflowDocument) []string {
	var problems []string
	if len(doc.On.Schedule) != 1 {
		problems = append(problems, fmt.Sprintf("cleanup must have exactly one schedule trigger (got %d)", len(doc.On.Schedule)))
	}
	for _, schedule := range doc.On.Schedule {
		fields := strings.Fields(schedule.Cron)
		if len(fields) != 5 {
			problems = append(problems, fmt.Sprintf("schedule cron %q must be a five-field POSIX expression", schedule.Cron))
			continue
		}
		interval, ok := cronHourIntervalHours(fields[1])
		if !ok {
			problems = append(problems, fmt.Sprintf("schedule cron %q must use a numeric hour or a */N step", schedule.Cron))
			continue
		}
		if interval > cleanupMaxIntervalHours {
			problems = append(problems, fmt.Sprintf(
				"cleanup must run at least every %d hours to keep peak artifact stock under the namespace quota (cron %q = %dh)",
				cleanupMaxIntervalHours, schedule.Cron, interval))
		}
	}
	if !triggerPresent(doc.On.WorkflowDispatch) {
		problems = append(problems, "cleanup must stay manually dispatchable (workflow_dispatch)")
	}
	if triggerPresent(doc.On.Push) {
		problems = append(problems, "cleanup must not trigger on push")
	}
	if triggerPresent(doc.On.PullRequest) {
		problems = append(problems, "cleanup must not trigger on pull_request")
	}
	return problems
}

// cleanupMaxIntervalHours bounds how long artifacts can pile up between
// cleanups. At the observed peak volume (43 runs/day, ~8.94 MiB/run) the
// 1 GiB namespace quota is reached once the stock spans about 2.7 days, so
// the 2-day age window plus one cleanup interval must stay below that.
const cleanupMaxIntervalHours = 6

// cronHourIntervalHours returns the cleanup interval implied by the cron hour
// field. A numeric hour runs once a day; a */N step runs every N hours.
func cronHourIntervalHours(field string) (int, bool) {
	if field == "*" {
		return 1, true
	}
	if rest, ok := strings.CutPrefix(field, "*/"); ok {
		step, err := strconv.Atoi(rest)
		if err != nil || step < 1 || step > 24 {
			return 0, false
		}
		return step, true
	}
	hour, err := strconv.Atoi(field)
	if err != nil || hour < 0 || hour > 23 {
		return 0, false
	}
	return 24, true
}

func validateCleanupPermissions(doc cleanupWorkflowDocument) []string {
	var problems []string
	if got := doc.Permissions["repository"]; got != "read" {
		problems = append(problems, fmt.Sprintf("cleanup permissions must keep repository=read (got %q)", got))
	}
	if got := doc.Permissions["action"]; got != "write" {
		problems = append(problems, fmt.Sprintf("cleanup permissions must declare action=write to delete artifacts (got %q)", got))
	}
	for key := range doc.Permissions {
		if key != "repository" && key != "action" {
			problems = append(problems, fmt.Sprintf("cleanup must request least privilege, unexpected permission %q", key))
		}
	}
	return problems
}

func validateCleanupSteps(doc cleanupWorkflowDocument) []string {
	job, ok := doc.Jobs["cleanup"]
	if !ok {
		return []string{`missing required job "cleanup"`}
	}
	var problems []string
	var hasCheckout, hasGoSetup, hasPrune bool
	for _, step := range job.Steps {
		switch step.Uses {
		case "checkout":
			hasCheckout = true
		case "setup-go":
			hasGoSetup = true
			if got := stringValue(step.With["version"]); got != gitcodeRequiredGoVersion {
				problems = append(problems, fmt.Sprintf("cleanup setup-go must pin go version %q (got %q)", gitcodeRequiredGoVersion, got))
			}
		}
		if strings.Contains(step.Run, "python3 scripts/cleanup-artifacts.py") {
			hasPrune = true
			for _, want := range []string{`--repo "$ATOMGIT_REPOSITORY"`, `--retention-days "$RETENTION_DAYS"`} {
				if !strings.Contains(step.Run, want) {
					problems = append(problems, fmt.Sprintf("cleanup step must pass %s", want))
				}
			}
			if !strings.Contains(step.Run, `export GC_TOKEN="$ATOMGIT_TOKEN"`) {
				problems = append(problems, "cleanup step must export GC_TOKEN from the runner-injected ATOMGIT_TOKEN")
			}
			if strings.Contains(step.Run, "secrets.ATOMGIT_TOKEN") {
				problems = append(problems, "cleanup must not remap secrets.ATOMGIT_TOKEN (runner injects it as an env var)")
			}
		}
	}
	if !hasCheckout {
		problems = append(problems, "cleanup job must check out the repository")
	}
	if !hasGoSetup {
		problems = append(problems, "cleanup job must build gc with the pinned Go toolchain")
	}
	if !hasPrune {
		problems = append(problems, "cleanup job must run scripts/cleanup-artifacts.py")
	}
	return problems
}

// validateCleanupRetention ties the pruner window to the upload retention.
// Both sides drifting apart would either delete artifacts a reviewer still
// expects (pruner window below upload retention) or leave the quota exposed
// (pruner window above it).
func validateCleanupRetention(doc cleanupWorkflowDocument, ci workflowDocument) []string {
	var problems []string
	retention, err := strconv.Atoi(stringValue(doc.Env["RETENTION_DAYS"]))
	if err != nil || retention < 1 {
		problems = append(problems, fmt.Sprintf("cleanup RETENTION_DAYS must be a positive integer (got %q)", stringValue(doc.Env["RETENTION_DAYS"])))
		return problems
	}
	uploads := uploadRetentionDays(ci)
	if len(uploads) == 0 {
		return append(problems, "ci.yml must declare retention-days on its artifact uploads")
	}
	for _, upload := range uploads {
		if upload != retention {
			problems = append(problems, fmt.Sprintf("ci.yml upload retention-days=%d must match cleanup RETENTION_DAYS=%d", upload, retention))
		}
	}
	return problems
}

func uploadRetentionDays(doc workflowDocument) []int {
	var days []int
	for _, job := range doc.Jobs {
		for _, step := range job.Steps {
			if step.Uses != "upload-artifact" {
				continue
			}
			value, err := strconv.Atoi(stringValue(step.With["retention-days"]))
			if err != nil {
				continue
			}
			days = append(days, value)
		}
	}
	return days
}

func TestArtifactCleanupWorkflowContractRejectsRegressions(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*cleanupWorkflowDocument)
		want   string
	}{
		{
			name: "push trigger",
			mutate: func(doc *cleanupWorkflowDocument) {
				doc.On.Push = yaml.Node{Kind: yaml.ScalarNode}
			},
			want: "must not trigger on push",
		},
		{
			name: "missing manual dispatch",
			mutate: func(doc *cleanupWorkflowDocument) {
				doc.On.WorkflowDispatch = yaml.Node{}
			},
			want: "manually dispatchable",
		},
		{
			name: "missing schedule",
			mutate: func(doc *cleanupWorkflowDocument) {
				doc.On.Schedule = nil
			},
			want: "exactly one schedule trigger",
		},
		{
			name: "daily-only cadence",
			mutate: func(doc *cleanupWorkflowDocument) {
				doc.On.Schedule[0].Cron = "0 19 * * *"
			},
			want: "at least every 6 hours",
		},
		{
			name: "over-broad permissions",
			mutate: func(doc *cleanupWorkflowDocument) {
				doc.Permissions["repository"] = "write"
			},
			want: "repository=read",
		},
		{
			name: "secrets remap",
			mutate: func(doc *cleanupWorkflowDocument) {
				job := doc.Jobs["cleanup"]
				for i := range job.Steps {
					job.Steps[i].Run = `export GC_TOKEN="${{ secrets.ATOMGIT_TOKEN }}"` + "\n" + job.Steps[i].Run
				}
				doc.Jobs["cleanup"] = job
			},
			want: "secrets.ATOMGIT_TOKEN",
		},
		{
			name: "retention mismatch",
			mutate: func(doc *cleanupWorkflowDocument) {
				doc.Env["RETENTION_DAYS"] = "7"
			},
			want: "must match cleanup RETENTION_DAYS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := readCleanupWorkflow(t)
			ci := readWorkflow(t)
			tt.mutate(&doc)
			problems := strings.Join(validateCleanupWorkflow(doc, ci), "\n")
			if !strings.Contains(problems, tt.want) {
				t.Fatalf("expected violation %q, got:\n%s", tt.want, problems)
			}
		})
	}
}
