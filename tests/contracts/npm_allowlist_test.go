package contracts

import (
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var quotedNamePattern = regexp.MustCompile(`"[^"]+"`)

func extractQuotedNames(t *testing.T, content string, pattern *regexp.Regexp, source string) []string {
	t.Helper()
	match := pattern.FindStringSubmatch(content)
	if match == nil {
		t.Fatalf("failed to locate %s in %s", source, content[:0])
	}
	names := quotedNamePattern.FindAllString(match[1], -1)
	if len(names) == 0 {
		t.Fatalf("no quoted names found in %s", source)
	}
	for i, name := range names {
		names[i] = strings.Trim(name, `"`)
	}
	sort.Strings(names)
	return names
}

// TestNPMCoordinateAllowlistsStayInSync asserts that the installer's symlink
// migration allowlist (OWN_NPM_PACKAGES in npm/lib/install.js) and the release
// assembly allowlist (NPM_COORDINATES in scripts/prepare-npm-package.sh)
// carry the same coordinate set. A coordinate shipped through the release
// pipeline but missing from the installer would recreate the hard-refusal
// fixed in issue #591 for leftovers of that coordinate's classic global
// installs.
func TestNPMCoordinateAllowlistsStayInSync(t *testing.T) {
	installer := extractQuotedNames(
		t,
		readRepositoryFile(t, filepath.Join("npm", "lib", "install.js")),
		regexp.MustCompile(`(?s)const OWN_NPM_PACKAGES = \[([^\]]*)\]`),
		"OWN_NPM_PACKAGES in npm/lib/install.js",
	)
	release := extractQuotedNames(
		t,
		readRepositoryFile(t, filepath.Join("scripts", "prepare-npm-package.sh")),
		regexp.MustCompile(`(?s)readonly NPM_COORDINATES=\(([^)]*)\)`),
		"NPM_COORDINATES in scripts/prepare-npm-package.sh",
	)
	if !reflect.DeepEqual(installer, release) {
		t.Fatalf(
			"npm coordinate allowlists drifted: installer OWN_NPM_PACKAGES=%v, release NPM_COORDINATES=%v; keep them in sync to avoid recreating the issue #591 hard refusal",
			installer,
			release,
		)
	}
}
