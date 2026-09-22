package contracts

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Canonical npm bootstrap commands per distribution coordinate. The bare
// atomgit-cli coordinate is the recommended one; the scoped @gitcode-cli/cli
// coordinate stays valid in parallel. Scoped names additionally pin the
// scope registry; bare names only pin the global registry.
const npmSecurityFlagsScoped = "--yes --ignore-scripts --registry=https://registry.npmjs.org --@gitcode-cli:registry=https://registry.npmjs.org"
const shortNPMBootstrap = "npx -y @gitcode-cli/cli@latest install"
const canonicalNPMBootstrap = "npx " + npmSecurityFlagsScoped + " @gitcode-cli/cli@latest install"
const canonicalNPMGlobal = "npm install -g --ignore-scripts --registry=https://registry.npmjs.org --@gitcode-cli:registry=https://registry.npmjs.org @gitcode-cli/cli@latest"

const npmSecurityFlagsBare = "--yes --ignore-scripts --registry=https://registry.npmjs.org"
const shortBareBootstrap = "npx -y atomgit-cli@latest install"
const canonicalBareBootstrap = "npx " + npmSecurityFlagsBare + " atomgit-cli@latest install"
const canonicalBareGlobal = "npm install -g --ignore-scripts --registry=https://registry.npmjs.org atomgit-cli@latest"

// Equivalent short forms listed when a document recommends one coordinate
// and mentions the others as alternatives.
var equivalentShortBootstraps = []string{
	"npx -y @atomgit-cli/cli@latest install",
	"npx -y @gitcode-cli/cli@latest install",
	"npx -y atomgit-cli@latest install",
}

var npxInstallPattern = regexp.MustCompile(`npx[^` + "`\"\r\n" + `]*(?:@gitcode-cli/cli|atomgit-cli)[^` + "`\"\r\n" + `]*install`)
var npmGlobalPattern = regexp.MustCompile(`npm install -g[^` + "`\"\r\n" + `]*(?:@gitcode-cli/cli|atomgit-cli)[^` + "`\"\r\n" + `]*`)

func TestInstallDocsUseCanonicalNPMBootstrap(t *testing.T) {
	version := strings.TrimSpace(readRepositoryFile(t, "VERSION"))
	currentReleasePath := filepath.Join("docs", "releases", "v"+version+".md")
	userPaths := []string{
		"README.md",
		filepath.Join("npm", "README.md"),
		filepath.Join("docs", "INTRODUCTION.md"),
		filepath.Join("docs", "PACKAGING.md"),
		filepath.Join("docs", "AI-GUIDE.md"),
		currentReleasePath,
	}
	for _, relativePath := range userPaths {
		content := readRepositoryFile(t, relativePath)
		if !showsShortBeforeHardenedBootstrap(content) {
			t.Errorf("%s must show the short npm bootstrap before the hardened form of one coordinate", relativePath)
		}
		assertNPMInstallLinesAreCanonical(t, relativePath, content, true)
	}
	securityPath := filepath.Join("spec", "delivery", "release-process.md")
	content := readRepositoryFile(t, securityPath)
	if !strings.Contains(content, canonicalNPMBootstrap) {
		t.Errorf("%s does not contain canonical npm bootstrap %q", securityPath, canonicalNPMBootstrap)
	}
	assertNPMInstallLinesAreCanonical(t, securityPath, content, false)
	assertNPMInstallLinesAreCanonical(t, filepath.Join("npm", "bin", "gc.js"), readRepositoryFile(t, filepath.Join("npm", "bin", "gc.js")), false)
}

// showsShortBeforeHardenedBootstrap reports whether the content shows the
// short bootstrap before the hardened form for at least one coordinate.
func showsShortBeforeHardenedBootstrap(content string) bool {
	scopedShort, scopedHardened := strings.Index(content, shortNPMBootstrap), strings.Index(content, canonicalNPMBootstrap)
	if scopedShort >= 0 && scopedHardened >= 0 && scopedShort <= scopedHardened {
		return true
	}
	bareShort, bareHardened := strings.Index(content, shortBareBootstrap), strings.Index(content, canonicalBareBootstrap)
	if bareShort >= 0 && bareHardened >= 0 && bareShort <= bareHardened {
		return true
	}
	return false
}

func TestREADMEPrioritizesBootstrapAndExplainsLocalInstall(t *testing.T) {
	content := readRepositoryFile(t, "README.md")
	bootstrap := strings.Index(content, shortNPMBootstrap)
	sourceBuild := strings.Index(content, "### 从源码构建")
	if bootstrap < 0 || sourceBuild < 0 || bootstrap > sourceBuild {
		t.Fatal("README must show the short npm bootstrap before source installation")
	}
	if !strings.Contains(content, "`npm i @gitcode-cli/cli` 或 `npm install @gitcode-cli/cli`") {
		t.Fatal("README must distinguish project-local npm dependencies from CLI installation")
	}
}

func TestCurrentReleaseNotesProvideShortAndHardenedBootstrap(t *testing.T) {
	version := strings.TrimSpace(readRepositoryFile(t, "VERSION"))
	releasePath := filepath.Join("docs", "releases", "v"+version+".md")
	content := readRepositoryFile(t, releasePath)
	pinned := "npx " + npmSecurityFlagsScoped + " @gitcode-cli/cli@" + version + " install"
	shortIndex := strings.Index(content, shortNPMBootstrap)
	hardenedIndex := strings.Index(content, canonicalNPMBootstrap)
	if shortIndex < 0 || hardenedIndex < 0 || shortIndex > hardenedIndex {
		t.Fatalf("%s must show the short bootstrap before the hardened bootstrap", releasePath)
	}
	if strings.Count(content, shortNPMBootstrap) != 1 {
		t.Fatalf("%s must contain the short bootstrap exactly once", releasePath)
	}
	if !strings.Contains(content, "包括 `.npmrc`）可信") {
		t.Fatalf("%s must explain that the short bootstrap requires trusted npm configuration", releasePath)
	}
	if !strings.Contains(content, pinned) {
		t.Fatalf("%s must provide pinned bootstrap command %q", releasePath, pinned)
	}
}

func assertNPMInstallLinesAreCanonical(t *testing.T, path, content string, allowShort bool) {
	t.Helper()
	version := strings.TrimSpace(readRepositoryFile(t, "VERSION"))
	pinnedScoped := "npx " + npmSecurityFlagsScoped + " @gitcode-cli/cli@" + version + " install"
	pinnedBare := "npx " + npmSecurityFlagsBare + " atomgit-cli@" + version + " install"
	for _, command := range npxInstallPattern.FindAllString(content, -1) {
		if command == canonicalNPMBootstrap || command == pinnedScoped || strings.HasPrefix(command, canonicalNPMBootstrap+" --target-dir ") {
			continue
		}
		if command == canonicalBareBootstrap || command == pinnedBare || strings.HasPrefix(command, canonicalBareBootstrap+" --target-dir ") {
			continue
		}
		if allowShort && (command == shortNPMBootstrap || command == shortBareBootstrap) {
			continue
		}
		if allowShort && containsString(equivalentShortBootstraps, command) {
			continue
		}
		t.Errorf("%s contains non-canonical npx install command %q", path, command)
	}
	for _, command := range npmGlobalPattern.FindAllString(content, -1) {
		if command == canonicalNPMGlobal || command == canonicalBareGlobal {
			continue
		}
		t.Errorf("%s contains non-canonical global npm install command %q", path, command)
	}
}

func readRepositoryFile(t *testing.T, relativePath string) string {
	t.Helper()
	path := filepath.Join("..", "..", relativePath)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
