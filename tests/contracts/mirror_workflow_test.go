package contracts

// Mirror workflow contract (issue 625).
//
// Scope: structural invariants of .github/workflows/ci.yml that silent YAML
// edits could break without any test signal - the job set, the cross-platform
// matrices, the test gating, the race-detector flag, the full-history secret
// scan, the read-only permissions, and SHA-pinned actions. The trigger
// surface is pinned by TestMirrorWorkflowPinsMainTriggers in
// ci_workflow_test.go.
//
// Deliberately NOT covered (declared per issue 625): step sequences inside
// jobs, env details, the codecov upload, the npm/wrapper smoke internals,
// and the docker build arguments. Those are behavioral details exercised by
// the CI runs themselves; pinning them here would couple the contract to
// implementation churn without guarding against silent structural
// regressions.

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

const mirrorWorkflowPath = "../../.github/workflows/ci.yml"

var mirrorRequiredJobs = []string{"lint", "secret-scan", "test", "build", "docker"}

// mirrorRequiredOS is the cross-platform coverage matrix: the mirror is the
// only CI source for macOS and Windows compatibility evidence, so every
// listed OS must stay in both the test and the build matrix.
var mirrorRequiredOS = []string{"ubuntu-latest", "macos-14", "windows-latest"}

// pinnedActionValuePattern matches a resolved uses value pinned to a full
// commit SHA. The line-oriented pinnedActionPattern in
// release_workflow_test.go matches the raw YAML line instead.
var pinnedActionValuePattern = regexp.MustCompile(`^[^@\s]+@[0-9a-f]{40}(?:\s+#.*)?$`)

func TestMirrorWorkflowContract(t *testing.T) {
	doc := readWorkflowAt(t, mirrorWorkflowPath)
	if problems := validateMirrorWorkflow(doc); len(problems) > 0 {
		t.Fatalf("GitHub mirror workflow contract violations:\n- %s", strings.Join(problems, "\n- "))
	}
}

func TestMirrorWorkflowContractRejectsRegressions(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*workflowDocument)
		want   string
	}{
		{name: "missing docker job", mutate: removeMirrorDockerJob, want: "missing required job"},
		{name: "unexpected job", mutate: addMirrorUnexpectedJob, want: "unexpected job"},
		{name: "test matrix drops windows", mutate: mutateMirrorDropTestWindows, want: "matrix"},
		{name: "build matrix drops macos", mutate: mutateMirrorDropBuildMacos, want: "matrix"},
		{name: "build no longer needs test", mutate: mutateMirrorDropBuildNeeds, want: "must need test"},
		{name: "race flag dropped", mutate: mutateMirrorStripRace, want: "race"},
		{name: "secret scan loses full history", mutate: mutateMirrorShallowSecretScan, want: "fetch-depth"},
		{name: "permissions dropped", mutate: mutateMirrorDropPermissions, want: "permissions"},
		{name: "unpinned action", mutate: mutateMirrorUnpinAction, want: "pinned"},
		{name: "fail-fast enabled on test matrix", mutate: mutateMirrorEnableFailFast, want: "fail-fast"},
		{name: "gitleaks loses redaction", mutate: mutateMirrorStripRedact, want: "--redact"},
		{name: "gitleaks redaction zeroed", mutate: mutateMirrorZeroRedact, want: "--redact"},
		{name: "job-level permission escalation", mutate: mutateMirrorElevateJobPermissions, want: "must not exceed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := readWorkflowAt(t, mirrorWorkflowPath)
			tt.mutate(&doc)
			if !containsProblem(validateMirrorWorkflow(doc), tt.want) {
				t.Fatalf("validator accepted regression; wanted problem containing %q", tt.want)
			}
		})
	}
}

func validateMirrorWorkflow(doc workflowDocument) []string {
	var problems []string
	problems = append(problems, validateMirrorJobs(doc)...)
	problems = append(problems, validateMirrorMatrices(doc)...)
	problems = append(problems, validateMirrorNeeds(doc)...)
	problems = append(problems, validateMirrorRace(doc)...)
	problems = append(problems, validateMirrorSecretScan(doc)...)
	problems = append(problems, validateMirrorPermissions(doc)...)
	problems = append(problems, validateMirrorActionPinning(doc)...)
	problems = append(problems, validateMirrorJobPermissions(doc)...)
	return problems
}

func validateMirrorJobs(doc workflowDocument) []string {
	var problems []string
	for _, jobID := range mirrorRequiredJobs {
		if _, ok := doc.Jobs[jobID]; !ok {
			problems = append(problems, fmt.Sprintf("missing required job %q", jobID))
		}
	}
	for jobID := range doc.Jobs {
		if !contains(mirrorRequiredJobs, jobID) {
			problems = append(problems, fmt.Sprintf("unexpected job %q", jobID))
		}
	}
	return problems
}

func validateMirrorMatrices(doc workflowDocument) []string {
	var problems []string
	for _, jobID := range []string{"test", "build"} {
		job, ok := doc.Jobs[jobID]
		if !ok {
			continue
		}
		for _, os := range mirrorRequiredOS {
			if !contains(job.Strategy.Matrix.OS, os) {
				problems = append(problems, fmt.Sprintf("job %q matrix must cover %q", jobID, os))
			}
		}
	}
	// The mirror is the only macOS/Windows evidence source, so a failing
	// platform must not silently cancel the remaining runs: GitHub's default
	// fail-fast would.
	if job, ok := doc.Jobs["test"]; ok {
		if job.Strategy.FailFast == nil || *job.Strategy.FailFast {
			problems = append(problems, "test job must set fail-fast: false so a failing platform does not cancel the others")
		}
	}
	return problems
}

func validateMirrorNeeds(doc workflowDocument) []string {
	var problems []string
	for _, jobID := range []string{"build", "docker"} {
		if job, ok := doc.Jobs[jobID]; ok && !contains(job.Needs, "test") {
			problems = append(problems, fmt.Sprintf("job %q must need test", jobID))
		}
	}
	return problems
}

func validateMirrorRace(doc workflowDocument) []string {
	job, ok := doc.Jobs["test"]
	if !ok {
		return nil
	}
	for _, step := range job.Steps {
		if strings.Contains(step.Run, "go test") && strings.Contains(step.Run, "-race") {
			return nil
		}
	}
	return []string{"test job must run go test with the -race flag"}
}

func validateMirrorSecretScan(doc workflowDocument) []string {
	job, ok := doc.Jobs["secret-scan"]
	if !ok {
		return nil
	}
	var problems []string
	fullHistory := false
	redacted := false
	for _, step := range job.Steps {
		if strings.Contains(step.Uses, "checkout") && stringValue(step.With["fetch-depth"]) == "0" {
			fullHistory = true
		}
		if strings.Contains(step.Run, "gitleaks") && strings.Contains(step.Run, "--redact=100") {
			// Findings must stay out of the public CI logs. The flag is a
			// percentage: --redact=0 disables redaction entirely while still
			// satisfying a bare --redact substring, so pin the full value.
			redacted = true
		}
	}
	if !fullHistory {
		problems = append(problems, "secret-scan checkout must use fetch-depth 0 for full history scanning")
	}
	if !redacted {
		problems = append(problems, "secret-scan gitleaks must run with --redact=100 so findings stay out of public logs")
	}
	return problems
}

func validateMirrorPermissions(doc workflowDocument) []string {
	if doc.Permissions["contents"] == "read" {
		return nil
	}
	return []string{"workflow permissions must be contents: read"}
}

func validateMirrorActionPinning(doc workflowDocument) []string {
	var problems []string
	for jobID, job := range doc.Jobs {
		for _, step := range job.Steps {
			if step.Uses == "" {
				continue
			}
			if !pinnedActionValuePattern.MatchString(step.Uses) {
				problems = append(problems, fmt.Sprintf("job %q action %q must be pinned to a commit SHA", jobID, step.Uses))
			}
		}
	}
	return problems
}

// validateMirrorJobPermissions keeps the least-privilege story airtight: no
// job may carry a permissions block beyond contents: read.
func validateMirrorJobPermissions(doc workflowDocument) []string {
	var problems []string
	for jobID, job := range doc.Jobs {
		for scope, value := range job.Permissions {
			if scope != "contents" || value != "read" {
				problems = append(problems, fmt.Sprintf("job %q permissions must not exceed contents: read (has %s: %s)", jobID, scope, value))
			}
		}
	}
	return problems
}

func removeMirrorDockerJob(doc *workflowDocument) {
	delete(doc.Jobs, "docker")
}

func addMirrorUnexpectedJob(doc *workflowDocument) {
	doc.Jobs["extra"] = workflowJob{}
}

func dropMirrorOS(doc *workflowDocument, jobID, os string) {
	job := doc.Jobs[jobID]
	filtered := make([]string, 0, len(job.Strategy.Matrix.OS))
	for _, value := range job.Strategy.Matrix.OS {
		if value != os {
			filtered = append(filtered, value)
		}
	}
	job.Strategy.Matrix.OS = filtered
	doc.Jobs[jobID] = job
}

func mutateMirrorDropTestWindows(doc *workflowDocument) {
	dropMirrorOS(doc, "test", "windows-latest")
}

func mutateMirrorDropBuildMacos(doc *workflowDocument) {
	dropMirrorOS(doc, "build", "macos-14")
}

func mutateMirrorDropBuildNeeds(doc *workflowDocument) {
	job := doc.Jobs["build"]
	job.Needs = nil
	doc.Jobs["build"] = job
}

func mutateMirrorStripRace(doc *workflowDocument) {
	job := doc.Jobs["test"]
	for i := range job.Steps {
		job.Steps[i].Run = strings.ReplaceAll(job.Steps[i].Run, " -race", "")
	}
	doc.Jobs["test"] = job
}

func mutateMirrorShallowSecretScan(doc *workflowDocument) {
	job := doc.Jobs["secret-scan"]
	for i := range job.Steps {
		if strings.Contains(job.Steps[i].Uses, "checkout") {
			if job.Steps[i].With == nil {
				job.Steps[i].With = map[string]any{}
			}
			job.Steps[i].With["fetch-depth"] = "1"
		}
	}
	doc.Jobs["secret-scan"] = job
}

func mutateMirrorDropPermissions(doc *workflowDocument) {
	doc.Permissions = nil
}

func mutateMirrorEnableFailFast(doc *workflowDocument) {
	job := doc.Jobs["test"]
	enabled := true
	job.Strategy.FailFast = &enabled
	doc.Jobs["test"] = job
}

func mutateMirrorStripRedact(doc *workflowDocument) {
	job := doc.Jobs["secret-scan"]
	for i := range job.Steps {
		job.Steps[i].Run = strings.ReplaceAll(job.Steps[i].Run, "--redact=100 ", "")
		job.Steps[i].Run = strings.ReplaceAll(job.Steps[i].Run, " --redact=100", "")
	}
	doc.Jobs["secret-scan"] = job
}

// mutateMirrorZeroRedact keeps a --redact flag but disables redaction: the
// value is a percentage, so 0 means findings print verbatim.
func mutateMirrorZeroRedact(doc *workflowDocument) {
	job := doc.Jobs["secret-scan"]
	for i := range job.Steps {
		job.Steps[i].Run = strings.ReplaceAll(job.Steps[i].Run, "--redact=100", "--redact=0")
	}
	doc.Jobs["secret-scan"] = job
}

func mutateMirrorElevateJobPermissions(doc *workflowDocument) {
	job := doc.Jobs["lint"]
	job.Permissions = map[string]string{"contents": "write"}
	doc.Jobs["lint"] = job
}

func mutateMirrorUnpinAction(doc *workflowDocument) {
	job := doc.Jobs["lint"]
	for i := range job.Steps {
		if strings.Contains(job.Steps[i].Uses, "checkout") {
			job.Steps[i].Uses = "actions/checkout@v5"
		}
	}
	doc.Jobs["lint"] = job
}
