// Package main is the entry point for gitcode-cli
package main

import (
	"os"
	"runtime/debug"

	"gitcode.com/gitcode-cli/cli/pkg/cmd/root"
	cmdutil "gitcode.com/gitcode-cli/cli/pkg/cmdutil"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func init() {
	// When version info is not injected via -ldflags, try to get it from debug.ReadBuildInfo
	if version == "dev" {
		info, ok := debug.ReadBuildInfo()
		if ok {
			// Get VCS info from build settings
			for _, setting := range info.Settings {
				switch setting.Key {
				case "vcs.revision":
					if commit == "none" {
						commit = setting.Value
					}
				case "vcs.time":
					if date == "unknown" {
						date = setting.Value
					}
				case "vcs.modified":
					if setting.Value == "true" && commit != "none" {
						commit += "-modified"
					}
				}
			}
		}
	}
}

func main() {
	// Execute the root command
	if err := root.Execute(version, commit, date); err != nil {
		// Silent protocol errors (credential helper "no credential")
		// keep their exit code but print nothing, so git can continue
		// with the next helper without stderr noise.
		if !cmdutil.IsSilent(err) {
			os.Stderr.WriteString("Error: " + err.Error() + "\n")
		}
		os.Exit(cmdutil.ExitCode(err))
	}
}
