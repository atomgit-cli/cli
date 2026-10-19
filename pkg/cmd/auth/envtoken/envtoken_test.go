package envtoken

import (
	"strings"
	"testing"
)

// lookupFrom builds a lookup func from a map of set variables. A key present
// with an empty value models "set but empty", which ActiveToken treats as
// unset, so SetEnvVars must exclude it too.
func lookupFrom(set map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		v, ok := set[name]
		return v, ok
	}
}

func TestSetEnvVars(t *testing.T) {
	tests := []struct {
		name string
		set  map[string]string
		want []string
	}{
		{
			name: "none set",
			set:  map[string]string{},
			want: nil,
		},
		{
			name: "GC_TOKEN only",
			set:  map[string]string{"GC_TOKEN": "x"},
			want: []string{"GC_TOKEN"},
		},
		{
			name: "GITCODE_TOKEN only",
			set:  map[string]string{"GITCODE_TOKEN": "x"},
			want: []string{"GITCODE_TOKEN"},
		},
		{
			name: "both set in priority order",
			set:  map[string]string{"GC_TOKEN": "x", "GITCODE_TOKEN": "x"},
			want: []string{"GC_TOKEN", "GITCODE_TOKEN"},
		},
		{
			name: "set but empty is excluded like ActiveToken",
			set:  map[string]string{"GC_TOKEN": "", "GITCODE_TOKEN": "x"},
			want: []string{"GITCODE_TOKEN"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SetEnvVars(lookupFrom(tt.set))
			if len(got) != len(tt.want) {
				t.Fatalf("SetEnvVars() = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("SetEnvVars()[%d] = %q, want %q (full: %v)", i, got[i], tt.want[i], got)
				}
			}
		})
	}
}

func TestWarning(t *testing.T) {
	tests := []struct {
		name    string
		opts    Options
		wantSub []string
		notSub  []string
	}{
		{
			name: "login saved on linux bash with GC_TOKEN",
			opts: Options{
				EnvVars:    []string{"GC_TOKEN"},
				Shell:      "/bin/bash",
				GOOS:       "linux",
				LoginSaved: true,
			},
			wantSub: []string{
				"GC_TOKEN 已设置",
				"本次登录已保存",
				"sed -i 's/^\\(export \\)\\{0,1\\}GC_TOKEN=/# &/' ~/.bashrc",
				"unset GC_TOKEN",
				`read -rsp "New token: " GC_TOKEN && export GC_TOKEN && echo`,
				"gc auth status",
			},
			notSub: []string{
				"sed -i ''",
				"~/.zshrc",
				"gc auth login",
			},
		},
		{
			name: "login saved on darwin zsh with GITCODE_TOKEN uses BSD sed and zsh rc file",
			opts: Options{
				EnvVars:    []string{"GITCODE_TOKEN"},
				Shell:      "/bin/zsh",
				GOOS:       "darwin",
				LoginSaved: true,
			},
			wantSub: []string{
				"GITCODE_TOKEN 已设置",
				"sed -i '' 's/^\\(export \\)\\{0,1\\}GITCODE_TOKEN=/# &/' ~/.zshrc",
				"unset GITCODE_TOKEN",
				`read -rs "GITCODE_TOKEN?New token: " && export GITCODE_TOKEN && echo`,
			},
			notSub: []string{"~/.bashrc"},
		},
		{
			name: "status failure context includes full login step and no saved note",
			opts: Options{
				EnvVars:    []string{"GC_TOKEN"},
				Shell:      "/bin/bash",
				GOOS:       "linux",
				LoginSaved: false,
			},
			wantSub: []string{
				"仅重新登录不会生效",
				"gc auth login",
				"sed -i 's/^\\(export \\)\\{0,1\\}GC_TOKEN=/# &/' ~/.bashrc",
				"unset GC_TOKEN",
				"gc auth status",
			},
			notSub: []string{"本次登录已保存"},
		},
		{
			name: "windows renders PowerShell commands instead of sed",
			opts: Options{
				EnvVars:    []string{"GC_TOKEN"},
				Shell:      `C:\Program Files\PowerShell\7\pwsh.exe`,
				GOOS:       "windows",
				LoginSaved: true,
			},
			wantSub: []string{
				"[Environment]::SetEnvironmentVariable('GC_TOKEN', $null, 'User')",
				"Remove-Item Env:GC_TOKEN",
				"$env:GC_TOKEN = Read-Host \"New token\"",
				"gc auth status",
			},
			notSub: []string{"sed -i", "unset GC_TOKEN"},
		},
		{
			name: "both vars set disable each and plan B targets the effective one",
			opts: Options{
				EnvVars:    []string{"GC_TOKEN", "GITCODE_TOKEN"},
				Shell:      "/bin/bash",
				GOOS:       "linux",
				LoginSaved: true,
			},
			wantSub: []string{
				"GC_TOKEN、GITCODE_TOKEN 已设置",
				"sed -i 's/^\\(export \\)\\{0,1\\}GC_TOKEN=/# &/' ~/.bashrc",
				"sed -i 's/^\\(export \\)\\{0,1\\}GITCODE_TOKEN=/# &/' ~/.bashrc",
				"unset GC_TOKEN",
				"unset GITCODE_TOKEN",
				`read -rsp "New token: " GC_TOKEN && export GC_TOKEN && echo`,
			},
			notSub: []string{`read -rsp "New token: " GITCODE_TOKEN`},
		},
		{
			name: "unknown env vars are filtered out of the rendered commands",
			opts: Options{
				EnvVars:    []string{"GC_TOKEN", "USER_CONTROLLED; rm -rf ~"},
				Shell:      "/bin/bash",
				GOOS:       "linux",
				LoginSaved: true,
			},
			wantSub: []string{
				"GC_TOKEN",
				"sed -i 's/^\\(export \\)\\{0,1\\}GC_TOKEN=/# &/' ~/.bashrc",
			},
			notSub: []string{"USER_CONTROLLED", "rm -rf"},
		},
		{
			name: "windows status failure context includes a full login step",
			opts: Options{
				EnvVars:    []string{"GC_TOKEN"},
				Shell:      "pwsh.exe",
				GOOS:       "windows",
				LoginSaved: false,
			},
			wantSub: []string{
				"仅重新登录不会生效",
				"gc auth login",
				"[Environment]::SetEnvironmentVariable('GC_TOKEN', $null, 'User')",
				"gc auth status",
			},
			notSub: []string{"本次登录已保存"},
		},
		{
			name: "linux zsh uses the zsh rc file and prompt form",
			opts: Options{
				EnvVars:    []string{"GC_TOKEN"},
				Shell:      "/usr/bin/zsh",
				GOOS:       "linux",
				LoginSaved: true,
			},
			wantSub: []string{
				"sed -i 's/^\\(export \\)\\{0,1\\}GC_TOKEN=/# &/' ~/.zshrc",
				`read -rs "GC_TOKEN?New token: " && export GC_TOKEN && echo`,
			},
			notSub: []string{"~/.bashrc"},
		},
		{
			name: "unrecognized shell falls back to ~/.profile with an adjust note",
			opts: Options{
				EnvVars:    []string{"GC_TOKEN"},
				Shell:      "/usr/bin/fish",
				GOOS:       "linux",
				LoginSaved: true,
			},
			wantSub: []string{
				"~/.profile",
				"请按其语法调整",
			},
			notSub: []string{"~/.bashrc", "~/.zshrc"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Warning(tt.opts)
			if got == "" {
				t.Fatal("Warning() returned empty output")
			}
			for _, sub := range tt.wantSub {
				if !strings.Contains(got, sub) {
					t.Errorf("Warning() output missing %q:\n%s", sub, got)
				}
			}
			for _, sub := range tt.notSub {
				if strings.Contains(got, sub) {
					t.Errorf("Warning() output unexpectedly contains %q:\n%s", sub, got)
				}
			}
		})
	}
}

// TestWarningEndsEveryPlanWithStatusVerify is the acceptance anchor that each
// plan must close the loop with a verifiable end state instead of stopping at
// an intermediate state such as only unsetting the variable.
func TestWarningEndsEveryPlanWithStatusVerify(t *testing.T) {
	got := Warning(Options{
		EnvVars:    []string{"GC_TOKEN"},
		Shell:      "/bin/bash",
		GOOS:       "linux",
		LoginSaved: true,
	})
	plans := strings.Split(got, "方案")
	if len(plans) < 3 { // header + plan A + plan B
		t.Fatalf("expected two plans in output, got %d sections:\n%s", len(plans)-1, got)
	}
	for _, plan := range plans[1:] {
		if !strings.Contains(plan, "gc auth status") {
			t.Errorf("plan does not end with a verification command:\n%s", plan)
		}
	}
}

// TestWarningEmptyEnvVarsReturnsEmpty pins the documented empty contract:
// no known token variable means no guidance at all.
func TestWarningEmptyEnvVarsReturnsEmpty(t *testing.T) {
	tests := []struct {
		name    string
		envVars []string
	}{
		{name: "nil env vars", envVars: nil},
		{name: "empty env vars", envVars: []string{}},
		{name: "only unknown env vars", envVars: []string{"HOME", "PATH"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Warning(Options{EnvVars: tt.envVars, Shell: "/bin/bash", GOOS: "linux", LoginSaved: true}); got != "" {
				t.Fatalf("Warning() = %q, want empty string", got)
			}
		})
	}
}
