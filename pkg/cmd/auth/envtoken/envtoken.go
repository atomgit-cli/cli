// Package envtoken renders guidance for the case where GC_TOKEN or
// GITCODE_TOKEN environment variables override a locally stored login.
//
// The guidance is intentionally copy-paste executable: every step is a command
// customized to the variable names in effect, the shell inferred from $SHELL,
// and the sed flavor of the platform. It never embeds token values and never
// modifies any shell configuration file itself.
package envtoken

import (
	"fmt"
	"strings"
)

// tokenEnvVars lists the token environment variables in auth priority order.
var tokenEnvVars = []string{"GC_TOKEN", "GITCODE_TOKEN"}

// SetEnvVars returns the token environment variables that are set and
// non-empty, in priority order (GC_TOKEN before GITCODE_TOKEN). It mirrors
// ActiveToken semantics, where a set-but-empty variable does not override.
// Only existence is reported; values are never returned.
func SetEnvVars(lookup func(string) (string, bool)) []string {
	var set []string
	for _, name := range tokenEnvVars {
		if value, ok := lookup(name); ok && value != "" {
			set = append(set, name)
		}
	}
	return set
}

// Options configures the guidance rendered by Warning.
type Options struct {
	// EnvVars are the set token variables in priority order, from SetEnvVars.
	EnvVars []string
	// Shell is the raw $SHELL value used to infer the configuration file and
	// the read syntax of the session-level commands.
	Shell string
	// GOOS is the runtime GOOS of the current process; "windows" switches the
	// guidance to PowerShell commands.
	GOOS string
	// LoginSaved distinguishes the auth login success context (true, the local
	// login was just stored) from the auth status failure context (false,
	// where a full login step must be part of plan A).
	LoginSaved bool
}

// Warning renders the full guidance block for the given options. It returns
// an empty string when no token environment variable is set. EnvVars are
// filtered to the known token variables so no caller can smuggle arbitrary
// text into the rendered commands.
func Warning(opts Options) string {
	opts.EnvVars = sanitizeEnvVars(opts.EnvVars)
	if len(opts.EnvVars) == 0 {
		return ""
	}
	if opts.GOOS == "windows" {
		return windowsWarning(opts)
	}
	return unixWarning(opts)
}

// ShortWarning renders the compact auth logout reminder. It includes only the
// current-session command and leaves persistent configuration changes to the
// full Warning guidance.
func ShortWarning(opts Options) string {
	opts.EnvVars = sanitizeEnvVars(opts.EnvVars)
	if len(opts.EnvVars) == 0 {
		return ""
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Environment token is still active in this terminal. Clear it for the current session:\n")
	for _, envVar := range opts.EnvVars {
		fmt.Fprintf(&b, "  %s\n", sessionClearCommand(opts.GOOS, envVar))
	}
	fmt.Fprintf(&b, "If a variable is also defined in a shell configuration file, remove or update that definition as well.\n")
	return b.String()
}

// sanitizeEnvVars keeps only the known token environment variables, in
// priority order, dropping anything else a caller might pass in.
func sanitizeEnvVars(envVars []string) []string {
	var kept []string
	for _, name := range tokenEnvVars {
		for _, v := range envVars {
			if v == name {
				kept = append(kept, v)
				break
			}
		}
	}
	return kept
}

// shellKind classifies $SHELL into "bash", "zsh", or "other".
func shellKind(shell string) string {
	base := strings.ToLower(shell)
	base = strings.ReplaceAll(base, "\\", "/")
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	switch {
	case strings.HasPrefix(base, "bash"):
		return "bash"
	case strings.HasPrefix(base, "zsh"):
		return "zsh"
	default:
		return "other"
	}
}

// unixProfile maps a shell kind to its configuration file. Unrecognized
// shells fall back to ~/.profile.
func unixProfile(kind string) string {
	switch kind {
	case "bash":
		return "~/.bashrc"
	case "zsh":
		return "~/.zshrc"
	default:
		return "~/.profile"
	}
}

func unixWarning(opts Options) string {
	kind := shellKind(opts.Shell)
	profile := unixProfile(kind)
	var b strings.Builder
	writeHeader(&b, opts, kind)
	writeUnixPlanA(&b, opts, profile)
	writeUnixPlanB(&b, opts, kind, profile)
	return b.String()
}

// writeHeader prints the warning line. In the status-failure context it names
// the trap explicitly: re-logging in alone cannot take effect.
func writeHeader(b *strings.Builder, opts Options, kind string) {
	names := strings.Join(opts.EnvVars, "、")
	if opts.LoginSaved {
		fmt.Fprintf(b, "⚠ 环境变量 %s 已设置，优先级高于本地登录配置；本次登录在该环境不会生效\n", names)
	} else {
		fmt.Fprintf(b, "⚠ 环境变量 %s 优先级高于本地登录配置；仅重新登录不会生效，需按下面方案处理\n", names)
	}
	if kind == "other" {
		fmt.Fprintf(b, "（命令以 bash 语法给出；如使用其他 shell，请按其语法调整）\n")
	}
	fmt.Fprintf(b, "\n")
}

// writeUnixPlanA prints the switch-to-local-login plan. Commenting out the
// definition is reversible by design; the unset step covers the current
// terminal so the plan closes on a verifiable state.
func writeUnixPlanA(b *strings.Builder, opts Options, profile string) {
	step := 1
	if opts.LoginSaved {
		fmt.Fprintf(b, "  方案 A · 切换为本地登录（推荐，本次登录已保存，完成 1-2 后即生效）：\n")
	} else {
		fmt.Fprintf(b, "  方案 A · 切换为本地登录（推荐）：\n")
		fmt.Fprintf(b, "    1. 登录（在私有、未录制的终端执行）：\n       gc auth login\n")
		step = 2
	}
	fmt.Fprintf(b, "    %d. 停用配置文件中的旧定义（可逆，注释而非删除）：\n", step)
	for _, envVar := range opts.EnvVars {
		fmt.Fprintf(b, "       %s\n", unixSedDisable(opts.GOOS, envVar, profile))
	}
	fmt.Fprintf(b, "    %d. 当前终端立即生效：\n", step+1)
	for _, envVar := range opts.EnvVars {
		fmt.Fprintf(b, "       %s\n", sessionClearCommand(opts.GOOS, envVar))
	}
	// The unset step makes the current terminal pass even when the sed above
	// matched nothing, so verification must run in a fresh terminal to prove
	// the persistent definition is really gone.
	fmt.Fprintf(b, "    %d. 验证（新开一个终端执行）：\n       gc auth status\n", step+2)
	fmt.Fprintf(b, "       （若新终端仍显示环境变量生效，请检索其他 shell 配置文件——如 .bash_profile、.zshenv、/etc/environment——以及带缩进或空格的变体定义）\n\n")
}

// writeUnixPlanB prints the keep-using-the-environment-variable plan. The
// session step uses read + export so the token never lands in shell history.
func writeUnixPlanB(b *strings.Builder, opts Options, kind, profile string) {
	effective := opts.EnvVars[0]
	fmt.Fprintf(b, "  方案 B · 继续使用环境变量认证：\n")
	fmt.Fprintf(b, "    1. 在 %s 中更新 %s 的值（GitCode「访问令牌」页生成）\n", profile, effective)
	fmt.Fprintf(b, "    2. 当前终端立即生效（避免 token 进入 shell history）：\n")
	fmt.Fprintf(b, "       %s\n", unixSessionSet(kind, effective))
	fmt.Fprintf(b, "    3. 验证：\n       gc auth status\n")
}

// unixSedDisable renders the reversible comment-out command. BSD sed (macOS)
// requires an explicit empty suffix argument for in-place editing.
func unixSedDisable(goos, envVar, profile string) string {
	pattern := `s/^\(export \)\{0,1\}` + envVar + `=/# &/`
	if goos == "darwin" {
		return fmt.Sprintf("sed -i '' '%s' %s", pattern, profile)
	}
	return fmt.Sprintf("sed -i '%s' %s", pattern, profile)
}

// unixSessionSet renders the history-safe session update command. zsh's read
// uses the name?prompt form and reserves -p for coprocess input, so the bash
// spelling would silently misbehave there.
func unixSessionSet(kind, envVar string) string {
	if kind == "zsh" {
		return fmt.Sprintf("read -rs %q && export %s && echo", envVar+"?New token: ", envVar)
	}
	return fmt.Sprintf(`read -rsp "New token: " %s && export %s && echo`, envVar, envVar)
}

func windowsWarning(opts Options) string {
	var b strings.Builder
	writeHeader(&b, opts, "")
	writeWindowsPlanA(&b, opts)
	writeWindowsPlanB(&b, opts)
	return b.String()
}

// writeWindowsPlanA prints the PowerShell variant of plan A: user-scope
// removal plus a per-session Remove-Item, since sed does not exist on Windows.
func writeWindowsPlanA(b *strings.Builder, opts Options) {
	step := 1
	if opts.LoginSaved {
		fmt.Fprintf(b, "  方案 A · 切换为本地登录（推荐，本次登录已保存，完成 1-2 后即生效）：\n")
	} else {
		fmt.Fprintf(b, "  方案 A · 切换为本地登录（推荐）：\n")
		fmt.Fprintf(b, "    1. 登录（在 PowerShell 中执行）：\n       gc auth login\n")
		step = 2
	}
	fmt.Fprintf(b, "    %d. 停用用户级环境变量（不可逆；若定义在 $PROFILE 中请注释该行）：\n", step)
	for _, envVar := range opts.EnvVars {
		fmt.Fprintf(b, "       [Environment]::SetEnvironmentVariable('%s', $null, 'User')\n", envVar)
	}
	fmt.Fprintf(b, "    %d. 当前终端立即生效（PowerShell）：\n", step+1)
	for _, envVar := range opts.EnvVars {
		fmt.Fprintf(b, "       %s\n", sessionClearCommand(opts.GOOS, envVar))
	}
	fmt.Fprintf(b, "    %d. 验证（新开一个终端执行）：\n       gc auth status\n", step+2)
	fmt.Fprintf(b, "       （若新终端仍显示环境变量生效，请检查 $PROFILE 与系统级环境变量）\n\n")
}

// writeWindowsPlanB prints the PowerShell variant of plan B. Read-Host keeps
// the token out of shell history; the persistent value is edited by hand in
// step 1, mirroring the Unix plan.
func writeWindowsPlanB(b *strings.Builder, opts Options) {
	effective := opts.EnvVars[0]
	fmt.Fprintf(b, "  方案 B · 继续使用环境变量认证：\n")
	fmt.Fprintf(b, "    1. 在「系统属性 → 环境变量」或 $PROFILE 中更新 %s 的值（GitCode「访问令牌」页生成）\n", effective)
	fmt.Fprintf(b, "    2. 当前终端立即生效（PowerShell，输入会回显，请在私密终端执行）：\n")
	fmt.Fprintf(b, "       $env:%s = Read-Host \"New token\"\n", effective)
	fmt.Fprintf(b, "    3. 验证：\n       gc auth status\n")
}

// sessionClearCommand renders the command that removes a token variable from
// the current terminal session without touching persistent configuration.
func sessionClearCommand(goos, envVar string) string {
	if goos == "windows" {
		return "Remove-Item Env:" + envVar
	}
	return "unset " + envVar
}
