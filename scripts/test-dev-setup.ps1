# Exercise the Windows setup workflow with a fully stubbed local toolchain.
[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$scriptPath = Join-Path $PSScriptRoot 'dev-setup.ps1'
$powerShell = Join-Path $PSHOME 'powershell.exe'
$tempRoot = Join-Path ([IO.Path]::GetTempPath()) "gc-dev-setup-test-$([guid]::NewGuid().ToString('N'))"
$stubBin = Join-Path $tempRoot 'system-bin'
$emptyBin = Join-Path $tempRoot 'empty-bin'
$fakeLocalAppData = Join-Path $tempRoot 'local-app-data'
$dispatcher = Join-Path $tempRoot 'stub-command.ps1'
$stubExecutable = Join-Path $tempRoot 'stub.exe'
$tokenSentinel = 'gc-token-must-not-reach-child'
$legacyTokenSentinel = 'legacy-token-must-not-reach-child'
$userPathBefore = [Environment]::GetEnvironmentVariable('Path', 'User')

function Assert-True {
    param([bool]$Condition, [string]$Message)
    if (-not $Condition) { throw $Message }
}

function Assert-Equal {
    param($Expected, $Actual, [string]$Message)
    if ($Expected -ne $Actual) {
        throw "$Message (expected=$Expected actual=$Actual)"
    }
}

function Set-TestFile {
    param([string]$Path, [string[]]$Lines)
    $parent = Split-Path -Parent $Path
    if ($parent) { New-Item -ItemType Directory -Force -Path $parent | Out-Null }
    Set-Content -LiteralPath $Path -Value $Lines -Encoding ASCII
}

function Restore-TestEnvironment {
    param([hashtable]$Saved)
    foreach ($name in $Saved.Keys) {
        if ($null -eq $Saved[$name]) {
            Remove-Item "Env:$name" -ErrorAction SilentlyContinue
        } else {
            Set-Item "Env:$name" $Saved[$name]
        }
    }
}

function Invoke-Setup {
    param(
        [string]$ToolsRoot,
        [string]$LogPath,
        [string]$SystemPath = $stubBin,
        [string]$GoVersion = 'go1.22.5',
        [hashtable]$ExtraEnvironment = @{},
        [switch]$Check
    )

    $variables = @{
        GC_DEV_TOOLS_DIR = $ToolsRoot
        GC_TOKEN = $tokenSentinel
        GITCODE_TOKEN = $legacyTokenSentinel
        GC_TEST_CHILD_LOG = $LogPath
        GC_TEST_STUB_EXE = $stubExecutable
        GC_TEST_GO_VERSION = $GoVersion
        GC_TEST_GO_LIST_EXIT = '0'
        GOCACHE = 'parent-gocache-sentinel'
        GOTMPDIR = 'parent-gotmp-sentinel'
        GOPROXY = 'http://127.0.0.1:1'
        APPDATA = (Join-Path $tempRoot 'default-outside-appdata')
        GOENV = 'outside-goenv-sentinel'
        GOPATH = (Join-Path $tempRoot 'module-gopath')
        GOMODCACHE = (Join-Path $tempRoot 'module-gopath\pkg\mod')
        LOCALAPPDATA = $fakeLocalAppData
        Path = "$SystemPath;$env:SystemRoot\System32;$env:SystemRoot"
    }
    foreach ($name in $ExtraEnvironment.Keys) { $variables[$name] = $ExtraEnvironment[$name] }
    $saved = @{}
    foreach ($name in $variables.Keys) {
        $saved[$name] = [Environment]::GetEnvironmentVariable($name, 'Process')
        if ($null -eq $variables[$name]) {
            Remove-Item "Env:$name" -ErrorAction SilentlyContinue
        } else {
            Set-Item "Env:$name" $variables[$name]
        }
    }

    try {
        $arguments = @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', $scriptPath)
        if ($Check) { $arguments += '-Check' }
        $oldErrorAction = $ErrorActionPreference
        $ErrorActionPreference = 'Continue'
        $output = & $powerShell @arguments 2>&1 | Out-String
        $status = $LASTEXITCODE
        $ErrorActionPreference = $oldErrorAction
        return [pscustomobject]@{ Status = $status; Output = $output }
    } finally {
        $ErrorActionPreference = 'Stop'
        Restore-TestEnvironment $saved
    }
}

function Get-NewLogLines {
    param([string]$Path, [int]$Start)
    if (-not (Test-Path $Path)) { return @() }
    return @(Get-Content -LiteralPath $Path | Select-Object -Skip $Start)
}

try {
    New-Item -ItemType Directory -Force -Path $tempRoot, $stubBin, $emptyBin | Out-Null

    $executableSource = @'
using System;
using System.IO;

public static class Program
{
    public static int Main(string[] args)
    {
        string log = Environment.GetEnvironmentVariable("GC_TEST_CHILD_LOG");
        if (!String.IsNullOrEmpty(log))
        {
            string token = Environment.GetEnvironmentVariable("GC_TOKEN") ?? "";
            string legacy = Environment.GetEnvironmentVariable("GITCODE_TOKEN") ?? "";
            string cache = Environment.GetEnvironmentVariable("GOCACHE") ?? "";
            string tmp = Environment.GetEnvironmentVariable("GOTMPDIR") ?? "";
            string appData = Environment.GetEnvironmentVariable("APPDATA") ?? "";
            string goEnv = Environment.GetEnvironmentVariable("GOENV") ?? "";
            File.AppendAllText(log, "exe|GC=" + token + "|LEGACY=" + legacy +
                "|CACHE=" + cache + "|TMP=" + tmp + "|APPDATA=" + appData +
                "|GOENV=" + goEnv + Environment.NewLine);
        }
        string executable = Path.GetFileName(Environment.GetCommandLineArgs()[0]);
        string wingetMarker = Environment.GetEnvironmentVariable("GC_TEST_WINGET_MARKER");
        if (String.Equals(executable, "winget.exe", StringComparison.OrdinalIgnoreCase) &&
            !String.IsNullOrEmpty(wingetMarker))
        {
            File.WriteAllText(wingetMarker, "executed");
        }
        if (args.Length > 0 && args[0] == "--version")
        {
            if (executable.StartsWith("pre-commit", StringComparison.OrdinalIgnoreCase) &&
                Environment.GetEnvironmentVariable("PYTHONDONTWRITEBYTECODE") != "1")
            {
                return 91;
            }
            string version = Environment.GetEnvironmentVariable("GC_TEST_PRECOMMIT_VERSION");
            Console.WriteLine(String.IsNullOrEmpty(version) ? "pre-commit 4.6.1" : version);
        }
        if (args.Length > 1 && args[0] == "-m" && args[1] == "pip" &&
            Environment.GetEnvironmentVariable("GC_TEST_PRECOMMIT_PIP_FAIL") == "1")
        {
            return 23;
        }
        return 0;
    }
}
'@
    Add-Type -TypeDefinition $executableSource -Language CSharp `
        -OutputAssembly $stubExecutable -OutputType ConsoleApplication

    $dispatcherSource = @'
param(
    [Parameter(Position = 0)]
    [string]$Tool,
    [Parameter(Position = 1, ValueFromRemainingArguments = $true)]
    [string[]]$Arguments,
    [Parameter()]
    [Alias('o')]
    [string]$OutputPath
)

$ErrorActionPreference = 'Stop'
$record = "$Tool|$($Arguments -join ' ')|GC=$env:GC_TOKEN|LEGACY=$env:GITCODE_TOKEN" +
    "|CACHE=$env:GOCACHE|TMP=$env:GOTMPDIR|APPDATA=$env:APPDATA|GOENV=$env:GOENV" +
    "|GOPATH=$env:GOPATH|GOMODCACHE=$env:GOMODCACHE|GOPROXY=$env:GOPROXY|GOBIN=$env:GOBIN"
Add-Content -LiteralPath $env:GC_TEST_CHILD_LOG -Value $record

if ($Tool -eq 'go') {
    if ($env:GC_TEST_WRITE_GO_CONFIG -eq '1') {
        $telemetry = Join-Path $env:APPDATA 'go\telemetry'
        New-Item -ItemType Directory -Force -Path $telemetry | Out-Null
        Set-Content -LiteralPath (Join-Path $telemetry 'stub-write') -Value 'isolated'
    }
    if ($Arguments[0] -eq 'version' -and $Arguments.Count -eq 1) {
        Write-Output "go version $env:GC_TEST_GO_VERSION windows/amd64"
        exit 0
    }
    if ($Arguments[0] -eq 'env' -and $Arguments[1] -eq 'GOVERSION') {
        Write-Output $env:GC_TEST_GO_VERSION
        exit 0
    }
    if ($Arguments[0] -eq 'version' -and $Arguments[1] -eq '-m') {
        $binary = $Arguments[2]
        if (-not (Test-Path -LiteralPath $binary)) { exit 1 }
        $content = Get-Content -Raw -LiteralPath $binary
        $module = [regex]::Match($content, '(?m)^module=(.+)$').Groups[1].Value.Trim()
        $version = [regex]::Match($content, '(?m)^version=(.+)$').Groups[1].Value.Trim()
        if (-not $module -or -not $version) { exit 1 }
        Write-Output $binary
        Write-Output "`tmod`t$module`t$version"
        exit 0
    }
    if ($Arguments[0] -eq 'list') {
        if ($env:GC_TEST_GO_LIST_STDERR) {
            [Console]::Error.WriteLine($env:GC_TEST_GO_LIST_STDERR)
        }
        exit [int]$env:GC_TEST_GO_LIST_EXIT
    }
    if ($Arguments[0] -eq 'mod' -and $Arguments[1] -eq 'download') {
        if ($env:GC_TEST_GO_DOWNLOAD_EXIT) { exit [int]$env:GC_TEST_GO_DOWNLOAD_EXIT }
        exit 0
    }
    if ($Arguments[0] -eq 'install') {
        $spec = $Arguments[1]
        $at = $spec.LastIndexOf('@')
        $package = $spec.Substring(0, $at)
        $version = $spec.Substring($at + 1)
        if ($package -eq 'github.com/golangci/golangci-lint/v2/cmd/golangci-lint') {
            $name = 'golangci-lint'
            $module = 'github.com/golangci/golangci-lint/v2'
        } elseif ($package -eq 'github.com/zricethezav/gitleaks/v8') {
            $name = 'gitleaks'
            $module = $package
        } else {
            exit 12
        }
        New-Item -ItemType Directory -Force -Path $env:GOBIN | Out-Null
        Set-Content -LiteralPath (Join-Path $env:GOBIN "$name.exe") -Encoding ASCII -Value @(
            'GC_STUB'
            "module=$module"
            "version=$version"
        )
        exit 0
    }
    if ($Arguments[0] -eq 'build') {
        if (-not $OutputPath) { exit 13 }
        Copy-Item -LiteralPath $env:GC_TEST_STUB_EXE -Destination $OutputPath -Force
        exit 0
    }
    if ($Arguments[0] -eq 'test') {
        if ($env:GC_TEST_GO_TEST_STDERR) {
            [Console]::Error.WriteLine($env:GC_TEST_GO_TEST_STDERR)
        }
        if ($env:GC_TEST_GO_TEST_EXIT) { exit [int]$env:GC_TEST_GO_TEST_EXIT }
        exit 0
    }
    exit 14
}

if ($Tool -in @('python', 'python3')) {
    if ($Arguments[0] -eq '--version') {
        Write-Output 'Python 3.12.4'
        exit 0
    }
    if ($Arguments[0] -eq '-m' -and $Arguments[1] -eq 'venv') {
        $scripts = Join-Path $Arguments[2] 'Scripts'
        New-Item -ItemType Directory -Force -Path $scripts | Out-Null
        Copy-Item -LiteralPath $env:GC_TEST_STUB_EXE -Destination (Join-Path $scripts 'python.exe')
        Copy-Item -LiteralPath $env:GC_TEST_STUB_EXE -Destination (Join-Path $scripts 'pre-commit.exe')
        exit 0
    }
    exit 0
}

if ($Arguments[0] -eq '--version') { Write-Output "$Tool stub 1.0" }
exit 0
'@
    Set-Content -LiteralPath $dispatcher -Value $dispatcherSource -Encoding UTF8
    foreach ($tool in @('go', 'git', 'python', 'python3', 'make', 'mingw32-make', 'gcc')) {
        Set-TestFile (Join-Path $stubBin "$tool.cmd") @(
            '@echo off'
            "`"$powerShell`" -NoProfile -ExecutionPolicy Bypass -File `"$dispatcher`" $tool %*"
            'exit /b %ERRORLEVEL%'
        )
    }

    $gitBash = Join-Path $fakeLocalAppData 'Programs\Git\bin\bash.exe'
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $gitBash) | Out-Null
    Copy-Item -LiteralPath $stubExecutable -Destination $gitBash

    Write-Host '[test] installation and idempotency'
    $installTools = Join-Path $tempRoot 'install-tools'
    $installLog = Join-Path $tempRoot 'install.log'
    $first = Invoke-Setup -ToolsRoot $installTools -LogPath $installLog
    Assert-Equal 0 $first.Status "first installation failed`n$($first.Output)"
    Assert-True ([bool]($first.Output -match [regex]::Escape(
        "`$env:Path = `"$installTools\bin;`$env:Path`""))) `
        'setup output did not provide the current-shell PATH command'
    Assert-True ($first.Output -match 'WARNING: managed tools are not on PATH') 'install output missing PATH warning'
    Assert-True ($first.Output -match 'pre-commit / pre-push') 'install warning missing hook consequence'
    Assert-True ($first.Output -match 'gitleaks: command not found') 'install warning missing gitleaks failure text'
    Assert-True ($first.Output -match [regex]::Escape('Add-Content -Path $PROFILE')) 'install warning missing persist example'
    $installLines = @(Get-Content -LiteralPath $installLog)
    Assert-True ([bool]($installLines -match
        '^go\|install github\.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2\.12\.2\|')) `
        'golangci-lint was not installed from its cmd package'
    Assert-True ([bool]($installLines -match
        '^go\|install github\.com/zricethezav/gitleaks/v8@v8\.30\.1\|')) `
        'gitleaks install package was not used'
    Assert-True ([bool]($installLines -match
        ('^python3\|-m venv {0}\\pre-commit-4\.6\.1\|' -f [regex]::Escape($installTools)))) `
        'pre-commit was not created in its versioned final venv'
    foreach ($name in @('golangci-lint.exe', 'gitleaks.exe', 'make.cmd', 'python3.cmd', 'pre-commit.cmd')) {
        Assert-True (Test-Path (Join-Path $installTools "bin\$name")) "missing managed artifact: $name"
    }
    $goInstallCount = @($installLines -match '^go\|install ').Count
    $venvInstallCount = @($installLines -match '^python3\|-m venv ').Count
    $second = Invoke-Setup -ToolsRoot $installTools -LogPath $installLog
    Assert-Equal 0 $second.Status "second installation failed`n$($second.Output)"
    $secondInstallCount = @((Get-Content -LiteralPath $installLog) -match '^go\|install ').Count
    Assert-Equal $goInstallCount $secondInstallCount 'second installation was not idempotent'
    $secondVenvCount = @((Get-Content -LiteralPath $installLog) -match '^python3\|-m venv ').Count
    Assert-Equal $venvInstallCount $secondVenvCount 'second installation rebuilt the pre-commit venv'
    $leftovers = @(Get-ChildItem -Force -Recurse -LiteralPath $installTools |
        Where-Object {
            $_.Name -like '.*-install-*' -or $_.Name -like '.pre-commit-backup-*' -or
            $_.Name -like '*.tmp'
        })
    Assert-Equal 0 $leftovers.Count 'temporary publish files were left behind'

    Write-Host '[test] complete and incomplete check mode'
    $beforeCheck = @(Get-Content -LiteralPath $installLog).Count
    $outsideAppData = Join-Path $tempRoot 'outside-check-appdata'
    $checkGoPath = Join-Path $tempRoot 'check-gopath'
    $checkModCache = Join-Path $tempRoot 'check-modcache'
    $checkDownloadCache = Join-Path $checkModCache 'cache\download'
    New-Item -ItemType Directory -Force -Path $outsideAppData, $checkDownloadCache | Out-Null
    Set-TestFile (Join-Path $checkDownloadCache 'sentinel') @('read-only-source')
    $checkModCacheEntriesBefore = @(Get-ChildItem -Force -Recurse -LiteralPath $checkModCache).Count
    $expectedLocalProxy = ([Uri]::new([IO.Path]::GetFullPath($checkDownloadCache))).AbsoluteUri
    $onPathBin = Join-Path $installTools 'bin'
    $complete = Invoke-Setup -ToolsRoot $installTools -LogPath $installLog -Check -SystemPath "$onPathBin;$stubBin" -ExtraEnvironment @{
        APPDATA = $outsideAppData
        GOENV = 'outside-goenv-sentinel'
        GOPATH = $checkGoPath
        GOMODCACHE = $checkModCache
        GC_TEST_WRITE_GO_CONFIG = '1'
    }
    Assert-Equal 0 $complete.Status "complete check mode failed`n$($complete.Output)"
    $checkLines = Get-NewLogLines $installLog $beforeCheck
    $checkChildLines = @($checkLines | Where-Object { $_ -match '\|APPDATA=' })
    Assert-True ($checkChildLines.Count -gt 0) 'check mode did not launch any observable child process'
    foreach ($line in $checkChildLines) {
        Assert-True ($line -match '\|APPDATA=([^|]*gc-dev-check-[^|]*\\appdata)\|GOENV=off') `
            "check child did not observe isolated APPDATA/GOENV: $line"
    }
    $goCheckLines = @($checkLines | Where-Object { $_ -like 'go|*' })
    Assert-True ($goCheckLines.Count -gt 0) 'check mode did not invoke the Go stub'
    foreach ($line in $goCheckLines) {
        Assert-True ($line -match '\|CACHE=([^|]*gc-dev-check-[^|]*\\gocache)\|TMP=([^|]*gc-dev-check-[^|]*\\gotmp)') `
            "check mode did not isolate Go cache/temp: $line"
        Assert-True (-not (Test-Path $Matches[1])) "isolated GOCACHE was not removed: $($Matches[1])"
        Assert-True (-not (Test-Path $Matches[2])) "isolated GOTMPDIR was not removed: $($Matches[2])"
        Assert-True ($line -match '\|APPDATA=([^|]*gc-dev-check-[^|]*\\appdata)\|GOENV=off\|') `
            "check mode did not isolate APPDATA/GOENV: $line"
        Assert-True (-not (Test-Path $Matches[1])) "isolated APPDATA was not removed: $($Matches[1])"
        Assert-True ($line -match
            '\|GOPATH=([^|]*gc-dev-check-[^|]*\\gopath)\|GOMODCACHE=([^|]*gc-dev-check-[^|]*\\modcache)\|') `
            "check mode did not isolate GOPATH/GOMODCACHE: $line"
        Assert-True (-not (Test-Path $Matches[1])) "isolated GOPATH was not removed: $($Matches[1])"
        Assert-True (-not (Test-Path $Matches[2])) "isolated GOMODCACHE was not removed: $($Matches[2])"
        Assert-True ($line -match ("\|GOPROXY={0}\|" -f [regex]::Escape($expectedLocalProxy))) `
            "check mode did not use the original download cache as a local proxy: $line"
    }
    Assert-Equal 0 @(Get-ChildItem -Force -LiteralPath $outsideAppData).Count `
        'check mode wrote Go user config outside the isolated APPDATA'
    Assert-Equal $checkModCacheEntriesBefore `
        @(Get-ChildItem -Force -Recurse -LiteralPath $checkModCache).Count `
        'check mode mutated the original module cache'

    $persistedGoPath = Join-Path $tempRoot 'persisted go#path'
    $persistedModCache = Join-Path $tempRoot 'persisted mod#cache'
    $persistedDownloadCache = Join-Path $persistedModCache 'cache\download'
    $persistedGoEnv = Join-Path $tempRoot 'persisted-go-env'
    New-Item -ItemType Directory -Force -Path $persistedDownloadCache | Out-Null
    Set-TestFile $persistedGoEnv @(
        "GOPATH=$persistedGoPath"
        "GOMODCACHE=$persistedModCache"
    )
    $persistedGoEnvBefore = Get-Content -Raw -LiteralPath $persistedGoEnv
    $beforePersistedCheck = @(Get-Content -LiteralPath $installLog).Count
    $persistedCheck = Invoke-Setup -ToolsRoot $installTools -LogPath $installLog -Check `
        -SystemPath "$onPathBin;$stubBin" -ExtraEnvironment @{
            GOENV = $persistedGoEnv
            GOPATH = $null
            GOMODCACHE = $null
        }
    Assert-Equal 0 $persistedCheck.Status "persisted Go env check failed`n$($persistedCheck.Output)"
    $persistedLines = Get-NewLogLines $installLog $beforePersistedCheck
    $expectedPersistedProxy = ([Uri]::new([IO.Path]::GetFullPath($persistedDownloadCache))).AbsoluteUri
    Assert-True ([bool]($persistedLines -match
        ("\|GOPROXY={0}\|" -f [regex]::Escape($expectedPersistedProxy)))) `
        'check mode ignored persisted GOMODCACHE'
    Assert-Equal $persistedGoEnvBefore (Get-Content -Raw -LiteralPath $persistedGoEnv) `
        'check mode modified the persisted Go env file'

    $moduleGap = Invoke-Setup -ToolsRoot $installTools -LogPath $installLog -Check `
        -SystemPath "$onPathBin;$stubBin" -ExtraEnvironment @{
            GC_TEST_GO_LIST_EXIT = '17'
            GC_TEST_GO_LIST_STDERR = 'module cache miss'
        }
    Assert-Equal 1 $moduleGap.Status "module cache miss did not produce a gap`n$($moduleGap.Output)"
    Assert-True ($moduleGap.Output -match 'module dependencies \(run without -Check to download\)') `
        'native stderr aborted check mode before the module gap was recorded'
    Assert-True ($moduleGap.Output -match 'incomplete: 1 dependency gap') `
        'module cache miss did not reach the final gap summary'

    $raceGap = Invoke-Setup -ToolsRoot $installTools -LogPath $installLog -Check `
        -SystemPath "$onPathBin;$stubBin" -ExtraEnvironment @{
            GC_TEST_GO_TEST_EXIT = '18'
            GC_TEST_GO_TEST_STDERR = 'race module cache miss'
        }
    Assert-Equal 1 $raceGap.Status "race failure did not produce a gap`n$($raceGap.Output)"
    Assert-True ($raceGap.Output -match 'race-enabled test') `
        'native stderr aborted check mode before the race gap was recorded'
    Assert-True ($raceGap.Output -match 'incomplete: 1 dependency gap') `
        'race failure did not reach the final gap summary'

    Write-Host '[test] PATH gap in check mode'
    $pathGap = Invoke-Setup -ToolsRoot $installTools -LogPath $installLog -Check
    Assert-Equal 1 $pathGap.Status "off-PATH check mode unexpectedly passed`n$($pathGap.Output)"
    Assert-True ($pathGap.Output -match 'MISS managed tools on PATH') 'off-PATH check missing PATH gap'
    Assert-True ($pathGap.Output -match 'incomplete: 1 dependency gap') 'off-PATH check gap count wrong'
    Assert-True ($pathGap.Output -notmatch 'dev environment ready') 'off-PATH check still reported ready'

    Write-Host '[test] go mod download failure guidance'
    # The PowerShell flow lists modules first and only downloads when the
    # list fails (unlike bash, which downloads directly), so fail both stub
    # exits to reach the download branch.
    $downloadFail = Invoke-Setup -ToolsRoot $installTools -LogPath $installLog -ExtraEnvironment @{
        GC_TEST_GO_LIST_EXIT = '17'
        GC_TEST_GO_DOWNLOAD_EXIT = '21'
    }
    Assert-Equal 1 $downloadFail.Status "download failure scenario unexpectedly passed`n$($downloadFail.Output)"
    Assert-True ($downloadFail.Output -match 'MISS go mod download failed') 'download failure missing gap'
    Assert-True ($downloadFail.Output -match 'go env GOPROXY') 'download failure missing GOPROXY hint'
    Assert-True ($downloadFail.Output -match [regex]::Escape('goproxy.cn,direct')) 'download failure missing goproxy.cn example'
    Assert-True ($downloadFail.Output -match [regex]::Escape('mirrors.aliyun.com/goproxy/')) 'download failure missing aliyun mirror example'
    Assert-True ($downloadFail.Output -match 'examples only') 'download failure missing examples-only wording'

    $missingTools = Join-Path $tempRoot 'missing-tools'
    $missing = Invoke-Setup -ToolsRoot $missingTools -LogPath (Join-Path $tempRoot 'missing.log') `
        -SystemPath $emptyBin -Check
    Assert-Equal 1 $missing.Status "missing dependency check did not fail`n$($missing.Output)"
    Assert-True (-not (Test-Path $missingTools)) 'check mode created the managed tools directory'

    Write-Host '[test] child credential isolation'
    $allLog = Get-Content -Raw -LiteralPath $installLog
    Assert-True ($allLog -notmatch [regex]::Escape($tokenSentinel)) 'GC_TOKEN reached a child process'
    Assert-True ($allLog -notmatch [regex]::Escape($legacyTokenSentinel)) `
        'GITCODE_TOKEN reached a child process'

    Write-Host '[test] forged managed wrapper content'
    $managedMarker = 'rem Managed by gitcode-cli scripts/dev-setup.ps1'
    $makeWrapper = Join-Path $installTools 'bin\make.cmd'
    $savedMakeWrapper = [IO.File]::ReadAllText($makeWrapper, [Text.Encoding]::ASCII)
    $forgedWrapperMarker = Join-Path $tempRoot 'forged-wrapper-was-run'
    [IO.File]::WriteAllText($makeWrapper, [string]::Join([Environment]::NewLine, @(
        $managedMarker
        '@echo off'
        "echo forged>>`"$forgedWrapperMarker`""
    )) + [Environment]::NewLine, [Text.Encoding]::ASCII)
    try {
        $forged = Invoke-Setup -ToolsRoot $installTools -LogPath (Join-Path $tempRoot 'forged.log')
        Assert-Equal 1 $forged.Status "forged wrapper scenario unexpectedly passed`n$($forged.Output)"
        Assert-True (-not (Test-Path $forgedWrapperMarker)) 'forged marker wrapper was executed'
        Assert-True ($forged.Output -match 'refusing to overwrite unmanaged wrapper') `
            'forged wrapper content was not rejected'
    } finally {
        [IO.File]::WriteAllText($makeWrapper, $savedMakeWrapper, [Text.Encoding]::ASCII)
    }

    $preCommitWrapper = Join-Path $installTools 'bin\pre-commit.cmd'
    $savedPreCommitWrapper = [IO.File]::ReadAllText($preCommitWrapper, [Text.Encoding]::ASCII)
    $forgedPreCommitMarker = Join-Path $tempRoot 'forged-precommit-was-run'
    $forgedPreCommitTarget = Join-Path $tempRoot 'forged-precommit.cmd'
    Set-TestFile $forgedPreCommitTarget @(
        '@echo off'
        "echo bad>>`"$forgedPreCommitMarker`""
        '@echo pre-commit 4.6.1'
        'exit /b 0'
    )
    [IO.File]::WriteAllText($preCommitWrapper, [string]::Join([Environment]::NewLine, @(
        $managedMarker
        '@echo off'
        "`"$forgedPreCommitTarget`" %*"
    )) + [Environment]::NewLine, [Text.Encoding]::ASCII)
    try {
        $forgedPreCommit = Invoke-Setup -ToolsRoot $installTools `
            -LogPath (Join-Path $tempRoot 'forged-precommit.log')
        Assert-Equal 1 $forgedPreCommit.Status `
            "forged pre-commit wrapper unexpectedly passed`n$($forgedPreCommit.Output)"
        Assert-True (-not (Test-Path $forgedPreCommitMarker)) 'forged pre-commit target was executed'
        Assert-True ($forgedPreCommit.Output -match 'refusing forged or unmanaged pre-commit wrapper') `
            'forged pre-commit target was not rejected'
    } finally {
        [IO.File]::WriteAllText($preCommitWrapper, $savedPreCommitWrapper, [Text.Encoding]::ASCII)
    }

    Write-Host '[test] unmanaged and failed pre-commit installation'
    $unmanagedPreCommitTools = Join-Path $tempRoot 'unmanaged-precommit-tools'
    $unmanagedVenv = Join-Path $unmanagedPreCommitTools 'pre-commit-4.6.1'
    New-Item -ItemType Directory -Force -Path $unmanagedVenv | Out-Null
    Set-TestFile (Join-Path $unmanagedVenv 'keep.txt') @('do-not-modify')
    $unmanagedPreCommit = Invoke-Setup -ToolsRoot $unmanagedPreCommitTools `
        -LogPath (Join-Path $tempRoot 'unmanaged-precommit.log')
    Assert-Equal 1 $unmanagedPreCommit.Status `
        "unmanaged pre-commit scenario unexpectedly passed`n$($unmanagedPreCommit.Output)"
    Assert-Equal 'do-not-modify' ((Get-Content -Raw -LiteralPath `
        (Join-Path $unmanagedVenv 'keep.txt')).Trim()) 'unmanaged pre-commit content was modified'
    Assert-True ($unmanagedPreCommit.Output -match 'refusing to replace unmanaged pre-commit environment') `
        'unmanaged pre-commit environment was not rejected'

    $failedPreCommitTools = Join-Path $tempRoot 'failed-precommit-tools'
    $failedPreCommit = Invoke-Setup -ToolsRoot $failedPreCommitTools `
        -LogPath (Join-Path $tempRoot 'failed-precommit.log') `
        -ExtraEnvironment @{ GC_TEST_PRECOMMIT_PIP_FAIL = '1' }
    Assert-Equal 1 $failedPreCommit.Status `
        "failed pre-commit installation unexpectedly passed`n$($failedPreCommit.Output)"
    Assert-True (-not (Test-Path (Join-Path $failedPreCommitTools 'pre-commit-4.6.1'))) `
        'failed pre-commit install left a final venv'
    Assert-True (-not (Test-Path (Join-Path $failedPreCommitTools 'bin\pre-commit.cmd'))) `
        'failed pre-commit install left a wrapper'
    $failedPreCommitLeftovers = @(Get-ChildItem -Force -LiteralPath $failedPreCommitTools |
        Where-Object { $_.Name -like '.pre-commit-*' })
    Assert-Equal 0 $failedPreCommitLeftovers.Count 'failed pre-commit install left staging or backup state'

    Write-Host '[test] nested pre-commit reparse point'
    $preCommitScripts = Join-Path $installTools 'pre-commit-4.6.1\Scripts'
    $nestedPreCommitTarget = Join-Path $tempRoot 'nested-precommit-target'
    [IO.Directory]::Move($preCommitScripts, $nestedPreCommitTarget)
    New-Item -ItemType Junction -Path $preCommitScripts -Target $nestedPreCommitTarget | Out-Null
    try {
        $nestedPreCommit = Invoke-Setup -ToolsRoot $installTools `
            -LogPath (Join-Path $tempRoot 'nested-precommit.log')
        Assert-Equal 1 $nestedPreCommit.Status `
            "nested pre-commit reparse scenario unexpectedly passed`n$($nestedPreCommit.Output)"
        Assert-True ($nestedPreCommit.Output -match 'pre-commit environment containing a reparse point') `
            'nested pre-commit reparse point was not rejected'
    } finally {
        [IO.Directory]::Delete($preCommitScripts)
        [IO.Directory]::Move($nestedPreCommitTarget, $preCommitScripts)
    }

    Write-Host '[test] unmanaged artifact protection and system PATH filtering'
    $unmanagedTools = Join-Path $tempRoot 'unmanaged-tools'
    $unmanagedBin = Join-Path $unmanagedTools 'bin'
    New-Item -ItemType Directory -Force -Path $unmanagedBin | Out-Null
    Set-TestFile (Join-Path $unmanagedBin 'golangci-lint.exe') @('do-not-overwrite')
    Set-TestFile (Join-Path $unmanagedBin 'make.cmd') @('@echo unmanaged-wrapper')
    $hijackMarker = Join-Path $tempRoot 'managed-go-was-run'
    Set-TestFile (Join-Path $unmanagedBin 'go.cmd') @(
        '@echo off'
        "echo bad>>`"$hijackMarker`""
        'exit /b 77'
    )
    $unmanagedBinAlias = Join-Path $tempRoot 'unmanaged-bin-alias'
    New-Item -ItemType Junction -Path $unmanagedBinAlias -Target $unmanagedBin | Out-Null
    $unmanaged = Invoke-Setup -ToolsRoot $unmanagedTools -LogPath (Join-Path $tempRoot 'unmanaged.log') `
        -SystemPath "$unmanagedBinAlias;$stubBin"
    Assert-Equal 1 $unmanaged.Status "unmanaged overwrite scenario unexpectedly passed`n$($unmanaged.Output)"
    Assert-Equal 'do-not-overwrite' ((Get-Content -Raw -LiteralPath `
        (Join-Path $unmanagedBin 'golangci-lint.exe')).Trim()) 'unmanaged Go tool was overwritten'
    Assert-Equal '@echo unmanaged-wrapper' ((Get-Content -Raw -LiteralPath `
        (Join-Path $unmanagedBin 'make.cmd')).Trim()) 'unmanaged wrapper was overwritten'
    Assert-True (-not (Test-Path $hijackMarker)) 'ManagedBin hijacked system Go resolution'
    Assert-True ($unmanaged.Output -match 'refusing to overwrite unmanaged Go tool') `
        'unmanaged Go tool refusal was not reported'
    Assert-True ($unmanaged.Output -match 'refusing to overwrite unmanaged wrapper') `
        'unmanaged wrapper refusal was not reported'

    Write-Host '[test] reparse-point protection'
    $reparseTools = Join-Path $tempRoot 'reparse-tools'
    $reparseTarget = Join-Path $tempRoot 'reparse-target'
    New-Item -ItemType Directory -Force -Path $reparseTools, $reparseTarget | Out-Null
    New-Item -ItemType Junction -Path (Join-Path $reparseTools 'bin') -Target $reparseTarget | Out-Null
    $reparse = Invoke-Setup -ToolsRoot $reparseTools -LogPath (Join-Path $tempRoot 'reparse.log')
    Assert-Equal 1 $reparse.Status "reparse-point scenario unexpectedly passed`n$($reparse.Output)"
    Assert-Equal 0 @(Get-ChildItem -Force -LiteralPath $reparseTarget).Count `
        'script wrote through the ManagedBin reparse point'
    Assert-True ($reparse.Output -match 'refusing to write through reparse point') `
        'reparse-point refusal was not reported'

    Write-Host '[test] malicious winget path and old Go guidance'
    $maliciousWingetMarker = Join-Path $tempRoot 'malicious-winget-was-run'
    $maliciousWinget = Join-Path $stubBin 'winget.exe'
    Copy-Item -LiteralPath $stubExecutable -Destination $maliciousWinget
    $oldGoTools = Join-Path $tempRoot 'old-go-tools'
    $oldGo = Invoke-Setup -ToolsRoot $oldGoTools -LogPath (Join-Path $tempRoot 'old-go.log') `
        -GoVersion 'go1.21.13' -ExtraEnvironment @{
            GC_DEV_WINGET_PATH = $maliciousWinget
            GC_TEST_WINGET_MARKER = $maliciousWingetMarker
        }
    Assert-Equal 1 $oldGo.Status "old Go scenario unexpectedly passed`n$($oldGo.Output)"
    Assert-True (-not (Test-Path $maliciousWingetMarker)) 'untrusted winget from PATH was executed'
    Assert-True ($oldGo.Output -match 'ignoring untrusted winget path') `
        'untrusted winget path was not reported'
    Assert-True ($oldGo.Output -match 'winget unavailable; download and install from https://go.dev/dl/') `
        'old Go without winget did not provide actionable guidance'

    Assert-Equal $userPathBefore ([Environment]::GetEnvironmentVariable('Path', 'User')) `
        'tests or setup script changed the user PATH'
    Write-Host 'Windows dev setup tests passed.'
} finally {
    if (Test-Path $tempRoot) {
        Remove-Item -LiteralPath $tempRoot -Recurse -Force -ErrorAction SilentlyContinue
    }
}
