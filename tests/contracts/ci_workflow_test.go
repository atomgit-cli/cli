package contracts

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type workflowDocument struct {
	Jobs map[string]workflowJob `yaml:"jobs"`
}

type workflowJob struct {
	Needs     stringList     `yaml:"needs"`
	Steps     []workflowStep `yaml:"steps"`
	Env       map[string]any `yaml:"env"`
	Services  map[string]any `yaml:"services"`
	Container any            `yaml:"container"`
}

type stringList []string

func (values *stringList) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		var value string
		if err := node.Decode(&value); err != nil {
			return err
		}
		*values = []string{value}
	case yaml.SequenceNode:
		var decoded []string
		if err := node.Decode(&decoded); err != nil {
			return err
		}
		*values = decoded
	default:
		return fmt.Errorf("needs must be a string or list of strings")
	}
	return nil
}

type workflowStep struct {
	Name string         `yaml:"name"`
	ID   string         `yaml:"id"`
	Uses string         `yaml:"uses"`
	Run  string         `yaml:"run"`
	With map[string]any `yaml:"with"`
	Env  map[string]any `yaml:"env"`
}

type artifactContract struct {
	jobID    string
	stepID   string
	nameBase string
	path     string
}

var artifactContracts = []artifactContract{
	{jobID: "test", stepID: "coverage-artifact", nameBase: "coverage-linux-", path: "coverage.out"},
	{jobID: "build", stepID: "binary-artifact", nameBase: "gc-ubuntu-", path: "bin/gc"},
	{jobID: "package", stepID: "wheel-artifact", nameBase: "gc-wheel-", path: "dist/*.whl"},
}

func TestGitCodeWorkflowContract(t *testing.T) {
	doc := readWorkflow(t)
	if problems := validateWorkflow(doc); len(problems) > 0 {
		t.Fatalf("GitCode workflow contract violations:\n- %s", strings.Join(problems, "\n- "))
	}
}

func TestGitCodeWorkflowContractRejectsRegressions(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*workflowDocument)
		want   string
	}{
		{name: "missing package job", mutate: removePackageJob, want: "missing required job"},
		{name: "docker command", mutate: addDockerCommand, want: "must not depend on Docker"},
		{name: "docker action", mutate: addDockerAction, want: "must not depend on Docker"},
		{name: "docker container action", mutate: addDockerContainerAction, want: "must not depend on Docker"},
		{name: "generic container", mutate: addGenericContainer, want: "must not depend on Docker"},
		{name: "generic service", mutate: addGenericService, want: "must not depend on Docker"},
		{name: "docker service", mutate: addDockerService, want: "must not depend on Docker"},
		{name: "static resolver output", mutate: makeCoverageResolverStatic, want: "resolver"},
		{name: "static upload name", mutate: makeCoverageUploadStatic, want: "upload name"},
		{name: "missing setup-python", mutate: removePythonSetup, want: "setup-python"},
		{name: "missing wheel install", mutate: removeWheelInstall, want: "wheel entrypoints"},
		{name: "echo-only entrypoints", mutate: removeEntrypointExecutions, want: "wheel entrypoints"},
		{name: "late setup-python", mutate: movePythonSetupAfterSmoke, want: "before wheel smoke test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := readWorkflow(t)
			tt.mutate(&doc)
			if !containsProblem(validateWorkflow(doc), tt.want) {
				t.Fatalf("validator accepted regression; wanted problem containing %q", tt.want)
			}
		})
	}
}

func TestWorkflowNeedsAcceptsScalarForm(t *testing.T) {
	var job workflowJob
	if err := yaml.Unmarshal([]byte("needs: test\n"), &job); err != nil {
		t.Fatalf("parse scalar needs: %v", err)
	}
	if len(job.Needs) != 1 || job.Needs[0] != "test" {
		t.Fatalf("scalar needs = %#v, want [test]", job.Needs)
	}
}

func TestDockerDisabledMessageIsAllowed(t *testing.T) {
	doc := readWorkflow(t)
	job := doc.Jobs["package"]
	job.Steps = append(job.Steps, workflowStep{Run: `echo "Docker is disabled on this runner"`})
	doc.Jobs["package"] = job
	if containsProblem(validateWorkflow(doc), "Docker") {
		t.Fatal("validator treated a Docker-disabled message as a daemon dependency")
	}
}

func readWorkflow(t *testing.T) workflowDocument {
	t.Helper()
	path := filepath.Join("..", "..", ".gitcode", "workflows", "ci.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	var doc workflowDocument
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return doc
}

func validateWorkflow(doc workflowDocument) []string {
	var problems []string
	problems = append(problems, validateJobs(doc)...)
	problems = append(problems, validateDependencies(doc)...)
	problems = append(problems, validatePackage(doc)...)
	for _, contract := range artifactContracts {
		problems = append(problems, validateArtifact(doc, contract)...)
	}
	return problems
}

func validateJobs(doc workflowDocument) []string {
	required := []string{"lint", "security", "test", "build", "package"}
	var problems []string
	for _, jobID := range required {
		if _, ok := doc.Jobs[jobID]; !ok {
			problems = append(problems, fmt.Sprintf("missing required job %q", jobID))
		}
	}
	for jobID, job := range doc.Jobs {
		if !contains(required, jobID) {
			problems = append(problems, fmt.Sprintf("unexpected job %q", jobID))
		}
		if jobUsesDocker(job) {
			problems = append(problems, fmt.Sprintf("job %q must not depend on Docker", jobID))
		}
	}
	return problems
}

func validateDependencies(doc workflowDocument) []string {
	var problems []string
	for _, jobID := range []string{"build", "package"} {
		job, ok := doc.Jobs[jobID]
		if ok && !contains(job.Needs, "test") {
			problems = append(problems, fmt.Sprintf("job %q must need test", jobID))
		}
	}
	return problems
}

func validatePackage(doc workflowDocument) []string {
	job, ok := doc.Jobs["package"]
	if !ok {
		return nil
	}
	setupIndex, smokeIndex := -1, -1
	for i, step := range job.Steps {
		if step.Uses == "setup-python" && stringValue(step.With["python-version"]) == "3.12" {
			setupIndex = i
		}
		if hasWheelSmoke(step.Run) {
			smokeIndex = i
		}
	}
	var problems []string
	if setupIndex < 0 {
		problems = append(problems, "package job must use setup-python 3.12")
	}
	if smokeIndex < 0 {
		problems = append(problems, "package job must build, install, and smoke-test all wheel entrypoints")
	}
	if setupIndex >= 0 && smokeIndex >= 0 && setupIndex > smokeIndex {
		problems = append(problems, "setup-python must run before wheel smoke test")
	}
	return problems
}

func validateArtifact(doc workflowDocument, contract artifactContract) []string {
	job, ok := doc.Jobs[contract.jobID]
	if !ok {
		return nil
	}
	resolve, ok := findStepByID(job.Steps, contract.stepID)
	if !ok {
		return []string{fmt.Sprintf("job %q missing artifact resolver %q", contract.jobID, contract.stepID)}
	}
	var problems []string
	expectedResolver := fmt.Sprintf(`echo "name=%s${ATOMGIT_RUN_ID:?}" >> "$ATOMGIT_OUTPUT"`, contract.nameBase)
	if !isExactScript(resolve.Run, expectedResolver) {
		problems = append(problems, fmt.Sprintf("resolver %q must use runtime run ID and output file", contract.stepID))
	}
	expectedName := fmt.Sprintf("${{ steps.%s.outputs.name }}", contract.stepID)
	if !hasArtifactUpload(job.Steps, expectedName, contract.path) {
		problem := fmt.Sprintf(
			"job %q upload name/path must be %q and %q",
			contract.jobID, expectedName, contract.path,
		)
		problems = append(problems, problem)
	}
	return problems
}

func findStepByID(steps []workflowStep, id string) (workflowStep, bool) {
	for _, step := range steps {
		if step.ID == id {
			return step, true
		}
	}
	return workflowStep{}, false
}

func hasArtifactUpload(steps []workflowStep, name, path string) bool {
	for _, step := range steps {
		if step.Uses == "upload-artifact" && stringValue(step.With["name"]) == name &&
			stringValue(step.With["path"]) == path {
			return true
		}
	}
	return false
}

func hasWheelSmoke(run string) bool {
	requiredLines := []string{
		"python -m build --wheel --outdir dist/",
		"python -m pip install dist/*.whl",
		`echo "=== gc version ===" && gc version`,
		`echo "=== gitcode version ===" && gitcode version`,
		`echo "=== python -m gc_cli version ===" && python -m gc_cli version`,
	}
	lines := scriptLines(run)
	lastLine := -1
	for _, required := range requiredLines {
		index := indexOfLine(lines, required)
		if index <= lastLine {
			return false
		}
		lastLine = index
	}
	return true
}

func isExactScript(script, want string) bool {
	commands := scriptLines(script)
	return len(commands) == 1 && commands[0] == want
}

func scriptLines(script string) []string {
	var commands []string
	for _, line := range strings.Split(script, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			commands = append(commands, trimmed)
		}
	}
	return commands
}

func indexOfLine(lines []string, want string) int {
	for i, line := range lines {
		if line == want {
			return i
		}
	}
	return -1
}

func jobUsesDocker(job workflowJob) bool {
	if containsDockerDaemonReference(job.Env) || len(job.Services) > 0 || hasConfiguredValue(job.Container) {
		return true
	}
	for _, step := range job.Steps {
		if isDockerAction(step.Uses) || shellUsesDocker(step.Run) || containsDockerDaemonReference(step.With) ||
			containsDockerDaemonReference(step.Env) {
			return true
		}
	}
	return false
}

const dockerCommandExpression = `^(?:[A-Za-z_][A-Za-z0-9_]*=\S+\s+)*` +
	`(?:sudo\s+)?(?:\S*/)?(?:docker|dockerd|docker-compose)\b`

var (
	dockerCommandPattern  = regexp.MustCompile(dockerCommandExpression)
	shellSeparatorPattern = regexp.MustCompile(`&&|\|\||;`)
)

func shellUsesDocker(script string) bool {
	for _, line := range scriptLines(script) {
		if strings.HasPrefix(line, "#") {
			continue
		}
		for _, segment := range shellSeparatorPattern.Split(line, -1) {
			trimmed := strings.TrimSpace(segment)
			if strings.HasPrefix(trimmed, "echo ") || strings.HasPrefix(trimmed, "printf ") {
				continue
			}
			if dockerCommandPattern.MatchString(trimmed) || strings.Contains(strings.ToLower(trimmed), "/var/run/docker.sock") {
				return true
			}
		}
	}
	return false
}

func isDockerAction(uses string) bool {
	lower := strings.ToLower(strings.TrimSpace(uses))
	return strings.HasPrefix(lower, "docker://") || strings.HasPrefix(lower, "docker/") ||
		strings.Contains(lower, "/docker/")
}

func hasConfiguredValue(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(typed) != ""
	case map[string]any:
		return len(typed) > 0
	case []any:
		return len(typed) > 0
	default:
		return true
	}
}

func containsDockerDaemonReference(value any) bool {
	switch typed := value.(type) {
	case string:
		return isDockerDaemonMarker(typed)
	case map[string]any:
		for key, item := range typed {
			if isDockerDaemonMarker(key) || containsDockerDaemonReference(item) {
				return true
			}
		}
	case []any:
		for _, item := range typed {
			if containsDockerDaemonReference(item) {
				return true
			}
		}
	}
	return false
}

func isDockerDaemonMarker(value string) bool {
	lower := strings.ToLower(value)
	return lower == "docker" || strings.HasPrefix(lower, "docker:") || strings.Contains(lower, "dockerd") ||
		strings.Contains(lower, "docker.sock") || strings.Contains(lower, "docker_host") ||
		strings.Contains(lower, "docker daemon")
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func containsProblem(problems []string, want string) bool {
	for _, problem := range problems {
		if strings.Contains(problem, want) {
			return true
		}
	}
	return false
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

func removePackageJob(doc *workflowDocument) {
	delete(doc.Jobs, "package")
}

func addDockerCommand(doc *workflowDocument) {
	job := doc.Jobs["package"]
	job.Steps = append(job.Steps, workflowStep{Name: "Build Docker image", Run: "docker build ."})
	doc.Jobs["package"] = job
}

func addDockerAction(doc *workflowDocument) {
	job := doc.Jobs["package"]
	job.Steps = append(job.Steps, workflowStep{Uses: "docker/setup-buildx-action@v3"})
	doc.Jobs["package"] = job
}

func addDockerContainerAction(doc *workflowDocument) {
	job := doc.Jobs["package"]
	job.Steps = append(job.Steps, workflowStep{Uses: "docker://alpine:latest"})
	doc.Jobs["package"] = job
}

func addGenericContainer(doc *workflowDocument) {
	job := doc.Jobs["package"]
	job.Container = "ubuntu:22.04"
	doc.Jobs["package"] = job
}

func addGenericService(doc *workflowDocument) {
	job := doc.Jobs["package"]
	job.Services = map[string]any{"database": map[string]any{"image": "postgres:16"}}
	doc.Jobs["package"] = job
}

func addDockerService(doc *workflowDocument) {
	job := doc.Jobs["package"]
	job.Services = map[string]any{"docker": map[string]any{"image": "docker:27-dind"}}
	doc.Jobs["package"] = job
}

func makeCoverageResolverStatic(doc *workflowDocument) {
	job := doc.Jobs["test"]
	for i := range job.Steps {
		if job.Steps[i].ID == "coverage-artifact" {
			job.Steps[i].Run = "echo \"name=coverage-linux\" >> \"$ATOMGIT_OUTPUT\""
		}
	}
	doc.Jobs["test"] = job
}

func makeCoverageUploadStatic(doc *workflowDocument) {
	job := doc.Jobs["test"]
	for i := range job.Steps {
		if job.Steps[i].Uses == "upload-artifact" {
			job.Steps[i].With["name"] = "coverage-linux"
		}
	}
	doc.Jobs["test"] = job
}

func removePythonSetup(doc *workflowDocument) {
	job := doc.Jobs["package"]
	filtered := make([]workflowStep, 0, len(job.Steps))
	for _, step := range job.Steps {
		if step.Uses != "setup-python" {
			filtered = append(filtered, step)
		}
	}
	job.Steps = filtered
	doc.Jobs["package"] = job
}

func removeWheelInstall(doc *workflowDocument) {
	job := doc.Jobs["package"]
	for i := range job.Steps {
		job.Steps[i].Run = strings.ReplaceAll(job.Steps[i].Run, "python -m pip install dist/*.whl\n", "")
	}
	doc.Jobs["package"] = job
}

func removeEntrypointExecutions(doc *workflowDocument) {
	job := doc.Jobs["package"]
	replacements := map[string]string{
		`echo "=== gc version ===" && gc version`:                             `echo "=== gc version ==="`,
		`echo "=== gitcode version ===" && gitcode version`:                   `echo "=== gitcode version ==="`,
		`echo "=== python -m gc_cli version ===" && python -m gc_cli version`: `echo "=== python -m gc_cli version ==="`,
	}
	for i := range job.Steps {
		for command, titleOnly := range replacements {
			job.Steps[i].Run = strings.ReplaceAll(job.Steps[i].Run, command, titleOnly)
		}
	}
	doc.Jobs["package"] = job
}

func movePythonSetupAfterSmoke(doc *workflowDocument) {
	job := doc.Jobs["package"]
	setupIndex, smokeIndex := -1, -1
	for i, step := range job.Steps {
		if step.Uses == "setup-python" {
			setupIndex = i
		}
		if hasWheelSmoke(step.Run) {
			smokeIndex = i
		}
	}
	if setupIndex < 0 || smokeIndex < 0 {
		return
	}
	setup := job.Steps[setupIndex]
	job.Steps = append(job.Steps[:setupIndex], job.Steps[setupIndex+1:]...)
	if setupIndex < smokeIndex {
		smokeIndex--
	}
	job.Steps = append(job.Steps[:smokeIndex+1], append([]workflowStep{setup}, job.Steps[smokeIndex+1:]...)...)
	doc.Jobs["package"] = job
}
