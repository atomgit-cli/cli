package contracts

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestReleaseWorkflowOrdersTagBeforeGoReleaser(t *testing.T) {
	workflow := readReleaseWorkflow(t)
	assertOrdered(t, workflow, "- name: Validate release notes", "- name: Create and push tag")
	assertOrdered(t, workflow, "- name: Validate release tree", "- name: Create and push tag")
	assertOrdered(t, workflow, "- name: Preflight packages and wheel", "- name: Create and push tag")
	assertOrdered(t, workflow, "- name: Create and push tag", "- name: Build GoReleaser assets")
	assertOrdered(t, workflow, "- name: Build GoReleaser assets", "- name: Sync package versions")
	for _, value := range []string{
		`SNAPSHOT_VERSION="$(python -c 'import json; print(json.load(open("dist/metadata.json", encoding="utf-8"))["version"])')"`,
		`prepare-npm-package.sh "${{ steps.version.outputs.VERSION_NUM }}" dist dist "${SNAPSHOT_VERSION}"`,
	} {
		if !strings.Contains(workflow, value) {
			t.Errorf("release workflow missing snapshot asset version handling %q", value)
		}
	}
}

func TestReleaseWorkflowSupportsVerifiedNPMRecovery(t *testing.T) {
	workflow := readReleaseWorkflow(t)
	for _, required := range []string{
		`if: ${{ !inputs.npm_recovery }}`,
		`if: ${{ inputs.npm_recovery }}`,
		`test "$(tr -d '\r\n' < VERSION)" = "${VERSION_NUM}"`,
		`docs/releases/${VERSION_INPUT}.npm-recovery.json`,
		`git fetch --force --tags origin`,
		`test "${TAG_SHA}" = "${TAG_SHA_EXPECTED}"`,
		`test "${TAG_TREE}" = "${TAG_TREE_EXPECTED}"`,
		`git log --format=%T origin/main`,
		`grep -Fxq "${TAG_TREE}" <<< "${MAIN_TREES}"`,
		`test "$(git show "${VERSION_INPUT}:VERSION")" = "${VERSION_NUM}"`,
		`gh release download "${VERSION_INPUT}"`,
		`CHECKSUM_LINE="$(grep -F`,
		`test "${ACTUAL_PACKAGE_SHA}" = "${PACKAGE_SHA_EXPECTED}"`,
		`package/bin/platforms/gc-linux-amd64`,
		`refusing to move npm dist-tag backwards`,
		`npm publish "${PACKAGE_FILE}" --access public --tag "${PUBLISH_TAG}"`,
		`recovery-packages.tsv`,
		`RECOVERY_PACKAGES=${RUNNER_TEMP}/recovery-packages.tsv`,
	} {
		if !strings.Contains(workflow, required) {
			t.Errorf("release workflow missing npm recovery protection %q", required)
		}
	}
}

func TestReleaseWorkflowPublishesAllNPMCoordinates(t *testing.T) {
	workflow := readReleaseWorkflow(t)
	for _, required := range []string{
		`test "$(find . -maxdepth 1 -name '*.tgz' -type f | wc -l)" -eq 3`,
		`test -f "gitcode-cli-cli-${VERSION_NUM}.tgz"`,
		`test -f "atomgit-cli-cli-${VERSION_NUM}.tgz"`,
		`test -f "atomgit-cli-${VERSION_NUM}.tgz"`,
		`"@gitcode-cli/cli") printf 'gitcode-cli-cli-%s.tgz' "${VERSION_NUM}" ;;`,
		`"@atomgit-cli/cli") printf 'atomgit-cli-cli-%s.tgz' "${VERSION_NUM}" ;;`,
		`"atomgit-cli") printf 'atomgit-cli-%s.tgz' "${VERSION_NUM}" ;;`,
		`for COORDINATE in "@gitcode-cli/cli" "@atomgit-cli/cli" "atomgit-cli"; do`,
		`PACKAGE_FILE="$(realpath "release-assets/$(coordinate_tarball "${COORDINATE}")")"`,
		`all three coordinates must publish ${VERSION_NUM}`,
	} {
		if !strings.Contains(workflow, required) {
			t.Errorf("release workflow missing multi-coordinate npm publishing guard %q", required)
		}
	}
	// The per-coordinate publish loop must exist in both the regular npm job
	// (idempotent skip + publish) and the recovery job.
	if got := strings.Count(workflow, `npm view "${COORDINATE}@${VERSION_NUM}" version >/dev/null 2>&1`); got != 2 {
		t.Fatalf("per-coordinate idempotency check appears %d times, want 2 (npm job + recovery)", got)
	}
}

func TestReleaseWorkflowRecoversAllNPMCoordinates(t *testing.T) {
	workflow := readReleaseWorkflow(t)
	for _, required := range []string{
		// v2 manifest schema handling with the coordinate allowlist.
		`m.packages`,
		`unsupported recovery coordinate`,
		`legacy manifest must reference the @gitcode-cli/cli tarball`,
		// Every downloaded tarball must be consumed by exactly one manifest row.
		`DOWNLOADED_TARBALLS`,
		// TSV-driven loops must not let npm commands swallow the manifest stdin.
		`done 3< "${RECOVERY_PACKAGES}"`,
		`done 3< "${RUNNER_TEMP}/recovery-packages.tsv"`,
		// A full three-coordinate manifest must end with all dist-tags aligned.
		`if [[ "$(wc -l < "${RECOVERY_PACKAGES}")" -eq 3 ]]`,
		`expected ${VERSION_NUM}`,
	} {
		if !strings.Contains(workflow, required) {
			t.Errorf("release workflow missing multi-coordinate npm recovery guard %q", required)
		}
	}
	if got := strings.Count(workflow, `done 3< "${RECOVERY_PACKAGES}"`); got != 2 {
		t.Fatalf("recovery loops on fd 3 = %d, want 2 (publish loop + consistency gate)", got)
	}
}

func TestPrepareNPMPackageScriptPublishesAllCoordinates(t *testing.T) {
	path := filepath.Join("..", "..", "scripts", "prepare-npm-package.sh")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	script := string(data)
	for _, required := range []string{
		`readonly NPM_COORDINATES=("@gitcode-cli/cli" "@atomgit-cli/cli" "atomgit-cli")`,
		`refusing npm coordinate outside the reviewed allowlist`,
		`pkg.name = name`,
		`npm pack --pack-destination "${OUTPUT_DIR}"`,
		`trap cleanup_stage EXIT`,
		`for coordinate in "${NPM_COORDINATES[@]}"; do`,
	} {
		if !strings.Contains(script, required) {
			t.Errorf("prepare-npm-package.sh missing multi-coordinate guard %q", required)
		}
	}
	// The per-coordinate staging copies the tree, rewrites the coordinate
	// name on the copy, then exercises and packs it under that name.
	assertOrdered(t, script, `cp -a npm/. "${stage}/"`, `pkg.name = name`)
	assertOrdered(t, script, `pkg.name = name`, `npm pack --pack-destination "${OUTPUT_DIR}"`)
	if !strings.Contains(script, "\n        npm test\n") {
		t.Error("prepare-npm-package.sh must run the npm test suite inside each staged coordinate copy")
	}
}

func TestReleaseWorkflowIsolatesNPMRecovery(t *testing.T) {
	document := parseReleaseWorkflow(t)
	dispatch := requireMapping(t, requireMapping(t, document, "on"), "workflow_dispatch")
	recoveryInput := requireMapping(t, requireMapping(t, dispatch, "inputs"), "npm_recovery")
	if got, ok := recoveryInput["default"].(bool); !ok || got {
		t.Fatalf("npm_recovery default = %#v, want false", recoveryInput["default"])
	}

	jobs := requireMapping(t, document, "jobs")
	preflight := requireMapping(t, jobs, "preflight")
	if got := requireString(t, preflight, "if"); got != "${{ !inputs.npm_recovery }}" {
		t.Fatalf("preflight recovery condition = %q", got)
	}
	recovery := requireMapping(t, jobs, "npm-recovery")
	if got := requireString(t, recovery, "if"); got != "${{ inputs.npm_recovery }}" {
		t.Fatalf("npm recovery condition = %q", got)
	}
	if _, found := recovery["needs"]; found {
		t.Fatal("npm recovery must not depend on the intentionally skipped preflight job")
	}
	permissions := requireMapping(t, recovery, "permissions")
	for key, want := range map[string]string{"contents": "read", "id-token": "write"} {
		if got := requireString(t, permissions, key); got != want {
			t.Fatalf("npm recovery permission %s = %q, want %q", key, got, want)
		}
	}
	for _, jobName := range []string{"tag", "artifacts", "publish", "pypi", "brew", "npm"} {
		job := requireMapping(t, jobs, jobName)
		if !workflowJobNeeds(job, "preflight") {
			t.Fatalf("job %s must depend on skipped preflight during npm recovery", jobName)
		}
	}
}

func TestReleaseWorkflowUsesTrackedNotesAndExactTag(t *testing.T) {
	workflow := readReleaseWorkflow(t)
	required := []string{
		`test "${GITHUB_REF_NAME}" = "main"`,
		`test "${VERSION_INPUT}" = "${VERSION_TAG}"`,
		`test -f "docs/releases/${VERSION_TAG}.md"`,
		`git diff --exit-code`,
		`TAG_SHA="$(git rev-list -n 1 "${VERSION_TAG}")"`,
		`test "${TAG_SHA}" = "${HEAD_SHA}"`,
		`args: release --clean --skip=publish`,
		`dist/gc_linux_amd64`,
		`dist/gc_linux_arm64`,
		`contents: read`,
		`persist-credentials: false`,
		`bash scripts/prepare-package-assets.sh`,
		`name: release-assets`,
		`! -name "gc_${VERSION_NUM}_checksums.txt"`,
		`xargs -0 sha256sum`,
		`sha256sum -c "gc_${VERSION_NUM}_checksums.txt"`,
	}
	for _, value := range required {
		if !strings.Contains(workflow, value) {
			t.Errorf("release workflow missing %q", value)
		}
	}
}

func TestReleaseWorkflowSerializesAndProtectsPublishedAssets(t *testing.T) {
	workflow := readReleaseWorkflow(t)
	required := []string{
		"group: release",
		`gh release download "${VERSION_TAG}" --dir existing-release-assets`,
		"find existing-release-assets -maxdepth 1 -type f",
		`cmp \`,
		`sha256sum -c "gc_${VERSION_NUM}_checksums.txt"`,
		`gh release create "${release_args[@]}" release-assets/*`,
	}
	for _, value := range required {
		if !strings.Contains(workflow, value) {
			t.Errorf("release workflow missing %q", value)
		}
	}
	for _, forbidden := range []string{
		"group: release-${{ inputs.version }}",
		"--clobber",
		"gh release upload",
	} {
		if strings.Contains(workflow, forbidden) {
			t.Errorf("release workflow must not contain %q", forbidden)
		}
	}
}

func TestReleaseWorkflowPinsRunnerAndToolchains(t *testing.T) {
	workflow := readReleaseWorkflow(t)
	for _, forbidden := range []string{"ubuntu-latest", "go-version: '1.22'", "python-version: '3.11'"} {
		if strings.Contains(workflow, forbidden) {
			t.Errorf("release workflow must not contain floating version %q", forbidden)
		}
	}
	for _, required := range []string{"runs-on: ubuntu-24.04", "go-version: '1.22.12'", "python-version: '3.11.9'"} {
		if !strings.Contains(workflow, required) {
			t.Errorf("release workflow missing pinned version %q", required)
		}
	}
}

func TestReleaseWorkflowIsValidYAML(t *testing.T) {
	parseReleaseWorkflow(t)
}

func TestReleaseWorkflowConfiguresPyPIPublishing(t *testing.T) {
	document := parseReleaseWorkflow(t)
	jobs := requireMapping(t, document, "jobs")
	pypi := requireMapping(t, jobs, "pypi")
	for _, dependency := range []string{"preflight", "publish"} {
		if !containsString(requireStringList(t, pypi, "needs"), dependency) {
			t.Fatalf("pypi job must depend on %q", dependency)
		}
	}

	permissions := requireMapping(t, pypi, "permissions")
	for key, want := range map[string]string{"contents": "read", "id-token": "write"} {
		if got := requireString(t, permissions, key); got != want {
			t.Fatalf("pypi permission %s = %q, want %q", key, got, want)
		}
	}

	environment := requireMapping(t, pypi, "environment")
	if got := requireString(t, environment, "name"); got != "pypi" {
		t.Fatalf("pypi environment = %q, want pypi", got)
	}
	pypiYAML, err := yaml.Marshal(pypi)
	if err != nil {
		t.Fatalf("marshal pypi job: %v", err)
	}
	for _, forbidden := range []string{"gh release upload", "pip install", "python -m build"} {
		if strings.Contains(string(pypiYAML), forbidden) {
			t.Fatalf("pypi job must not run %q while holding OIDC permission", forbidden)
		}
	}
	if !strings.Contains(readReleaseWorkflow(t), "pip install build==1.4.1") {
		t.Fatal("pypi build tool must be pinned to build 1.4.1")
	}
	workflow := readReleaseWorkflow(t)
	for _, required := range []string{
		"Verify any existing PyPI distributions",
		`remote[name] != local[name]`,
		"skip-existing: true",
	} {
		if !strings.Contains(workflow, required) {
			t.Errorf("pypi rerun protection missing %q", required)
		}
	}
}

func TestReleaseWorkflowSeparatesWritePermissions(t *testing.T) {
	document := parseReleaseWorkflow(t)
	jobs := requireMapping(t, document, "jobs")
	wants := map[string]string{
		"preflight": "read",
		"tag":       "write",
		"artifacts": "read",
		"publish":   "write",
		"pypi":      "read",
	}
	for jobName, want := range wants {
		job := requireMapping(t, jobs, jobName)
		permissions := requireMapping(t, job, "permissions")
		if got := requireString(t, permissions, "contents"); got != want {
			t.Fatalf("job %s contents permission = %q, want %q", jobName, got, want)
		}
	}
}

func TestPythonBuildBackendIsPinned(t *testing.T) {
	path := filepath.Join("..", "..", "pyproject.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	content := string(data)
	for _, dependency := range []string{"setuptools==80.9.0", "wheel==0.45.1"} {
		if !strings.Contains(content, dependency) {
			t.Fatalf("Python build dependency %q is not pinned", dependency)
		}
	}
}

func TestReleaseWorkflowPinsPublishingDependencies(t *testing.T) {
	workflow := readReleaseWorkflow(t)
	if strings.Contains(workflow, "@latest") {
		t.Fatal("release workflow must not install publishing tools from @latest")
	}
	for _, line := range strings.Split(workflow, "\n") {
		if strings.Contains(line, "uses:") && !pinnedActionPattern.MatchString(line) {
			t.Errorf("release action is not pinned to a commit: %s", strings.TrimSpace(line))
		}
	}
}

func TestReleaseWorkflowKeepsPrereleasesOffNpmLatest(t *testing.T) {
	workflow := readReleaseWorkflow(t)
	for _, required := range []string{
		`PUBLISH_TAG="latest"`, `PUBLISH_TAG="next"`, `--tag "${PUBLISH_TAG}"`,
		`dist-tags.${PUBLISH_TAG}`, `test "${TAG_VERSION}" = "${VERSION_NUM}"`,
	} {
		if !strings.Contains(workflow, required) {
			t.Errorf("release workflow missing npm prerelease tag guard %q", required)
		}
	}
	assertOrdered(t, workflow, `PUBLISH_TAG="latest"`, `npm view "${COORDINATE}@${VERSION_NUM}" version`)
}

func TestPackageScriptPinsNfpm(t *testing.T) {
	path := filepath.Join("..", "..", "scripts", "package.sh")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	script := string(data)
	if !strings.Contains(script, "nfpm/v2/cmd/nfpm@v2.41.3") || strings.Contains(script, "nfpm@latest") {
		t.Fatal("package script must recommend the Go 1.22-compatible pinned nfpm version")
	}
}

var pinnedActionPattern = regexp.MustCompile(`uses:\s+[^@\s]+@[0-9a-f]{40}(?:\s+#.*)?$`)

func readReleaseWorkflow(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", ".github", "workflows", "release.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func parseReleaseWorkflow(t *testing.T) map[string]any {
	t.Helper()
	var document map[string]any
	if err := yaml.Unmarshal([]byte(readReleaseWorkflow(t)), &document); err != nil {
		t.Fatalf("parse release workflow: %v", err)
	}
	return document
}

func requireMapping(t *testing.T, parent map[string]any, key string) map[string]any {
	t.Helper()
	value, ok := parent[key].(map[string]any)
	if !ok {
		t.Fatalf("workflow key %q is not a mapping", key)
	}
	return value
}

func requireString(t *testing.T, parent map[string]any, key string) string {
	t.Helper()
	value, ok := parent[key].(string)
	if !ok {
		t.Fatalf("workflow key %q is not a string", key)
	}
	return value
}

func requireStringList(t *testing.T, parent map[string]any, key string) []string {
	t.Helper()
	values, ok := parent[key].([]any)
	if !ok {
		t.Fatalf("workflow key %q is not a list", key)
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		item, ok := value.(string)
		if !ok {
			t.Fatalf("workflow key %q contains a non-string value", key)
		}
		result = append(result, item)
	}
	return result
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func workflowJobNeeds(job map[string]any, target string) bool {
	switch needs := job["needs"].(type) {
	case string:
		return needs == target
	case []any:
		for _, need := range needs {
			if need == target {
				return true
			}
		}
	}
	return false
}

func assertOrdered(t *testing.T, content, first, second string) {
	t.Helper()
	firstIndex := strings.Index(content, first)
	secondIndex := strings.Index(content, second)
	if firstIndex < 0 || secondIndex < 0 || firstIndex >= secondIndex {
		t.Fatalf("expected %q before %q", first, second)
	}
}
