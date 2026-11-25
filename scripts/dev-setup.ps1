<#
.SYNOPSIS
    Install and verify the core host toolchain without a dev container.

.DESCRIPTION
    Installs project-managed verification tools, checks Windows prerequisites,
    and runs a smoke build plus a race-enabled test. Packaging and release
    tooling remains in .devcontainer/ and the documented release workflow.
    This script removes GitCode credential variables before launching tools.

.PARAMETER Check
    Run offline checks only. Installs nothing and makes no persistent changes.
#>
[CmdletBinding()]
param([switch]$Check)

$ErrorActionPreference = 'Stop'
Remove-Item Env:GC_TOKEN -ErrorAction SilentlyContinue
Remove-Item Env:GITCODE_TOKEN -ErrorAction SilentlyContinue

$RepoRoot = Split-Path -Parent $PSScriptRoot
Set-Location $RepoRoot

$GoMinMajor = 1
$GoMinMinor = 22
$GolangciVersion = 'v2.12.2'
$GitleaksVersion = 'v8.30.1'
$PreCommitVersion = '4.6.1'
$ToolsRoot = if ($env:GC_DEV_TOOLS_DIR) { $env:GC_DEV_TOOLS_DIR } else {
    Join-Path $env:LOCALAPPDATA 'gitcode-cli\dev-tools'
}
$ManagedBin = Join-Path $ToolsRoot 'bin'
$RawSystemPath = $env:Path
$PreCommitVenv = Join-Path $ToolsRoot "pre-commit-$PreCommitVersion"
$ManagedMarker = 'rem Managed by gitcode-cli scripts/dev-setup.ps1'
$ManagedGoTools = @{
    'golangci-lint' = @{
        Module = 'github.com/golangci/golangci-lint/v2'
        Package = 'github.com/golangci/golangci-lint/v2/cmd/golangci-lint'
        Version = $GolangciVersion
    }
    'gitleaks' = @{
        Module = 'github.com/zricethezav/gitleaks/v8'
        Package = 'github.com/zricethezav/gitleaks/v8'
        Version = $GitleaksVersion
    }
}
$ManagedWrappers = @('make.cmd', 'pre-commit.cmd', 'python3.cmd')
$OldGoCache = $env:GOCACHE
$OldGoTmpDir = $env:GOTMPDIR
$OldGoProxy = $env:GOPROXY
$OldAppData = $env:APPDATA
$OldGoEnv = $env:GOENV
$OldGoPath = $env:GOPATH
$OldGoModCache = $env:GOMODCACHE
$CheckTempRoot = $null
$script:Missing = @()

function Write-Section { param([string]$Name) Write-Host ''; Write-Host "[$Name]" }
function Write-Ok { param([string]$Message) Write-Host "ok   $Message" }
function Write-Info { param([string]$Message) Write-Host "  $Message" }
function Write-Gap { param([string]$Message) Write-Host "MISS $Message"; $script:Missing += $Message }

function Restore-EnvironmentVariable {
    param([string]$Name, [AllowNull()][string]$Value)
    if ($null -eq $Value) {
        Remove-Item "Env:$Name" -ErrorAction SilentlyContinue
    } else {
        Set-Item "Env:$Name" $Value
    }
}

function Invoke-NativeQuietly {
    param([string]$Path, [string[]]$Arguments)
    $oldErrorAction = $ErrorActionPreference
    try {
        $ErrorActionPreference = 'Continue'
        & $Path @Arguments *> $null
        $exitCode = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $oldErrorAction
    }
    return $exitCode
}

function Test-IsManagedPath {
    param([string]$Path)
    if (-not $Path) { return $false }
    try {
        $root = [IO.Path]::GetFullPath($ManagedBin).TrimEnd('\', '/')
        $candidate = [IO.Path]::GetFullPath($Path.Trim('"'))
    } catch {
        return $false
    }
    return $candidate.Equals($root, [StringComparison]::OrdinalIgnoreCase) -or
        $candidate.StartsWith("$root\", [StringComparison]::OrdinalIgnoreCase)
}

function Test-ManagedBinOnPath {
    $root = [IO.Path]::GetFullPath($ManagedBin).TrimEnd('\', '/')
    foreach ($entry in ($RawSystemPath -split ';')) {
        if (-not $entry) { continue }
        try {
            $candidate = [IO.Path]::GetFullPath($entry.Trim('"')).TrimEnd('\', '/')
        } catch {
            continue
        }
        if ($candidate.Equals($root, [StringComparison]::OrdinalIgnoreCase)) { return $true }
    }
    return $false
}

function Get-SystemCommandPath {
    param([string]$Name)
    $commands = Get-Command $Name -All -CommandType Application -ErrorAction SilentlyContinue
    foreach ($command in $commands) {
        if ($command.Source -and -not (Test-IsManagedPath $command.Source)) {
            return $command.Source
        }
    }
    return $null
}

function Test-PathUnderRoot {
    param([string]$Path, [string]$Root)
    try {
        $fullPath = [IO.Path]::GetFullPath($Path)
        $fullRoot = [IO.Path]::GetFullPath($Root).TrimEnd('\', '/')
    } catch {
        return $false
    }
    return $fullPath.Equals($fullRoot, [StringComparison]::OrdinalIgnoreCase) -or
        $fullPath.StartsWith("$fullRoot\", [StringComparison]::OrdinalIgnoreCase)
}

function Test-PathComponentsReparseFree {
    param([string]$Path, [string]$Root)
    if (-not (Test-PathUnderRoot $Path $Root)) { return $false }
    $fullRoot = [IO.Path]::GetFullPath($Root).TrimEnd('\', '/')
    $fullPath = [IO.Path]::GetFullPath($Path)
    $relative = $fullPath.Substring($fullRoot.Length).TrimStart('\', '/')
    $current = $fullRoot
    foreach ($part in $relative -split '[\\/]') {
        if (-not $part) { continue }
        $current = Join-Path $current $part
        if ((Get-PathItem $current) -and (Test-IsReparsePoint $current)) { return $false }
    }
    return $true
}

function Test-SystemPathEntryAllowed {
    param([string]$Path)
    if (-not $Path -or (Test-IsManagedPath $Path)) { return $false }
    try {
        $fullPath = [IO.Path]::GetFullPath($Path.Trim('"'))
        $root = [IO.Path]::GetPathRoot($fullPath)
    } catch {
        return $false
    }
    return $root -and (Test-PathComponentsReparseFree $fullPath $root)
}

function Test-TrustedWingetPath {
    param([string]$Path)
    if (-not $Path -or [IO.Path]::GetFileName($Path) -ne 'winget.exe') { return $false }
    $roots = @(
        [Environment]::GetFolderPath([Environment+SpecialFolder]::Windows),
        [Environment]::GetFolderPath([Environment+SpecialFolder]::ProgramFiles)
    ) | Where-Object { $_ }
    $trustedRoot = $roots | Where-Object { Test-PathUnderRoot $Path $_ } | Select-Object -First 1
    if (-not $trustedRoot -or -not (Test-PathComponentsReparseFree $Path $trustedRoot)) { return $false }
    $item = Get-PathItem $Path
    if (-not $item -or $item.PSIsContainer) { return $false }
    try {
        $signature = Microsoft.PowerShell.Security\Get-AuthenticodeSignature -FilePath $Path
        return $signature.Status -eq 'Valid' -and $signature.SignerCertificate -and
            $signature.SignerCertificate.Subject -match '(^|,\s*)O=Microsoft Corporation(,|$)'
    } catch {
        return $false
    }
}

function Get-TrustedWingetPath {
    if ($env:GC_DEV_WINGET_PATH) {
        if (Test-TrustedWingetPath $env:GC_DEV_WINGET_PATH) {
            return [IO.Path]::GetFullPath($env:GC_DEV_WINGET_PATH)
        }
        Write-Info "ignoring untrusted winget path: $($env:GC_DEV_WINGET_PATH)"
        return $null
    }

    $candidates = @(
        (Join-Path ([Environment]::GetFolderPath([Environment+SpecialFolder]::Windows)) 'System32\winget.exe')
    )
    try {
        $packages = Appx\Get-AppxPackage -Name Microsoft.DesktopAppInstaller -ErrorAction Stop |
            Sort-Object Version -Descending
        foreach ($package in $packages) {
            if ($package.InstallLocation) {
                $candidates += Join-Path $package.InstallLocation 'winget.exe'
            }
        }
    } catch {
        Write-Info 'unable to query the trusted App Installer package'
    }
    foreach ($candidate in $candidates) {
        if (Test-TrustedWingetPath $candidate) { return [IO.Path]::GetFullPath($candidate) }
    }
    return $null
}

function Test-SystemCommandWorks {
    param([string]$Name, [string[]]$Arguments = @('--version'))
    $path = Get-SystemCommandPath $Name
    if (-not $path) { return $false }
    & $path @Arguments > $null 2>&1
    return ($LASTEXITCODE -eq 0)
}

function Refresh-SystemPath {
    param([switch]$IncludePersistent)
    $paths = @($env:Path)
    if ($IncludePersistent) {
        $paths += [Environment]::GetEnvironmentVariable('Path', 'Machine')
        $paths += [Environment]::GetEnvironmentVariable('Path', 'User')
    }
    $entries = $paths -join ';' -split ';' | Where-Object {
        $_ -and (Test-SystemPathEntryAllowed $_)
    }
    $env:Path = @($entries | Select-Object -Unique) -join ';'
}

function Get-PathItem {
    param([string]$Path)
    return Get-Item -Force -LiteralPath $Path -ErrorAction SilentlyContinue
}

function Test-IsReparsePoint {
    param([string]$Path)
    $item = Get-PathItem $Path
    return $item -and (($item.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0)
}

function Test-ManagedLocationSafe {
    param([string]$Path)
    foreach ($candidate in @($ToolsRoot, $ManagedBin, $Path)) {
        if ((Get-PathItem $candidate) -and (Test-IsReparsePoint $candidate)) { return $false }
    }
    return $true
}

function Ensure-SafeManagedDirectory {
    param([string]$Path)
    foreach ($candidate in @($ToolsRoot, $Path)) {
        $item = Get-PathItem $candidate
        if ($item -and (($item.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0)) {
            Write-Gap "refusing to write through reparse point: $candidate"
            return $false
        }
        if ($item -and -not $item.PSIsContainer) {
            Write-Gap "managed directory path is not a directory: $candidate"
            return $false
        }
    }
    New-Item -ItemType Directory -Force -Path $Path | Out-Null
    if (Test-IsReparsePoint $Path) {
        Write-Gap "refusing to write through reparse point: $Path"
        return $false
    }
    return $true
}

function Publish-FileAtomically {
    param([string]$Source, [string]$Destination)
    if (Get-PathItem $Destination) {
        $backup = "$Destination.$([guid]::NewGuid().ToString('N')).bak"
        try {
            [IO.File]::Replace($Source, $Destination, $backup)
        } finally {
            Remove-Item -LiteralPath $backup -Force -ErrorAction SilentlyContinue
        }
    } else {
        [IO.File]::Move($Source, $Destination)
    }
}

function Install-WingetPackage {
    param([string]$Name, [string]$PackageId, [string[]]$VersionArguments = @('--version'))
    if (Test-SystemCommandWorks $Name $VersionArguments) { return }
    if ($Check) { return }
    $winget = Get-TrustedWingetPath
    if (-not $winget) {
        Write-Gap "$Name (winget unavailable; install $PackageId manually)"
        return
    }
    Write-Info "installing $Name with winget ($PackageId)"
    & $winget install --id $PackageId --exact --source winget `
        --accept-package-agreements --accept-source-agreements --disable-interactivity
    if ($LASTEXITCODE -ne 0) { Write-Gap "$Name (winget install failed)"; return }
    Refresh-SystemPath -IncludePersistent
}

function Get-PersistedGoEnvValue {
    param([string]$Name)
    if ($OldGoEnv -eq 'off') { return '' }
    $envFile = if ($OldGoEnv) {
        $OldGoEnv
    } else {
        $configRoot = if ($OldAppData) {
            $OldAppData
        } else {
            [Environment]::GetFolderPath([Environment+SpecialFolder]::ApplicationData)
        }
        Join-Path $configRoot 'go\env'
    }
    if (-not (Test-Path -LiteralPath $envFile -PathType Leaf)) { return '' }
    $value = ''
    foreach ($line in Get-Content -LiteralPath $envFile) {
        if ($line.StartsWith("$Name=", [StringComparison]::Ordinal)) {
            $value = $line.Substring($Name.Length + 1)
        }
    }
    return $value
}

function Initialize-CheckEnvironment {
    if (-not $Check) { return }
    $script:CheckTempRoot = Join-Path ([IO.Path]::GetTempPath()) "gc-dev-check-$([guid]::NewGuid().ToString('N'))"
    $cache = Join-Path $script:CheckTempRoot 'gocache'
    $tmp = Join-Path $script:CheckTempRoot 'gotmp'
    $appData = Join-Path $script:CheckTempRoot 'appdata'
    $goPath = Join-Path $script:CheckTempRoot 'gopath'
    $goModCache = Join-Path $script:CheckTempRoot 'modcache'
    New-Item -ItemType Directory -Force -Path $cache, $tmp, $appData, $goPath, $goModCache | Out-Null
    $persistedGoPath = Get-PersistedGoEnvValue GOPATH
    $persistedGoModCache = Get-PersistedGoEnvValue GOMODCACHE
    $effectiveGoPath = if ($OldGoPath) {
        $OldGoPath
    } elseif ($persistedGoPath) {
        $persistedGoPath
    } else {
        Join-Path $env:USERPROFILE 'go'
    }
    $firstGoPath = ($effectiveGoPath -split ';')[0]
    $effectiveGoModCache = if ($OldGoModCache) {
        $OldGoModCache
    } elseif ($persistedGoModCache) {
        $persistedGoModCache
    } else {
        Join-Path $firstGoPath 'pkg\mod'
    }
    $downloadCache = Join-Path $effectiveGoModCache 'cache\download'
    $localProxy = ([Uri]::new([IO.Path]::GetFullPath($downloadCache))).AbsoluteUri
    $env:GOCACHE = $cache
    $env:GOTMPDIR = $tmp
    $env:GOPROXY = $localProxy
    $env:APPDATA = $appData
    $env:GOENV = 'off'
    $env:GOPATH = $goPath
    $env:GOMODCACHE = $goModCache
}

function Remove-CheckEnvironment {
    Restore-EnvironmentVariable GOCACHE $OldGoCache
    Restore-EnvironmentVariable GOTMPDIR $OldGoTmpDir
    Restore-EnvironmentVariable GOPROXY $OldGoProxy
    Restore-EnvironmentVariable APPDATA $OldAppData
    Restore-EnvironmentVariable GOENV $OldGoEnv
    Restore-EnvironmentVariable GOPATH $OldGoPath
    Restore-EnvironmentVariable GOMODCACHE $OldGoModCache
    if ($script:CheckTempRoot) {
        Remove-Item -LiteralPath $script:CheckTempRoot -Recurse -Force -ErrorAction SilentlyContinue
    }
}

function Test-GoVersion {
    $go = Get-SystemCommandPath go
    if (-not $go) { return $false }
    & $go version > $null 2>&1
    if ($LASTEXITCODE -ne 0) { return $false }
    $raw = (& $go env GOVERSION 2>$null) -replace '^go', ''
    $parts = $raw.Split('.')
    if ($parts.Count -lt 2) { return $false }
    $major = 0; $minor = 0
    if (-not [int]::TryParse($parts[0], [ref]$major)) { return $false }
    if (-not [int]::TryParse($parts[1], [ref]$minor)) { return $false }
    return ($major -gt $GoMinMajor -or ($major -eq $GoMinMajor -and $minor -ge $GoMinMinor))
}

function Get-GoModuleVersion {
    param([string]$Binary, [string]$Module)
    $go = Get-SystemCommandPath go
    if (-not $go -or -not $Binary -or -not (Get-PathItem $Binary)) { return '' }
    if (Test-IsReparsePoint $Binary) { return '' }
    $lines = & $go version -m $Binary 2>$null
    foreach ($line in $lines) {
        $fields = $line.Trim() -split '\s+'
        if ($fields.Count -ge 3 -and $fields[0] -eq 'mod' -and $fields[1] -eq $Module) { return $fields[2] }
    }
    return ''
}

function Ensure-GoTool {
    param([string]$Name, [string]$Module, [string]$Package, [string]$Version)
    $definition = $ManagedGoTools[$Name]
    if (-not $definition -or $definition.Module -ne $Module -or
        $definition.Package -ne $Package -or $definition.Version -ne $Version) {
        Write-Gap "refusing non-whitelisted managed Go tool: $Name"
        return
    }

    $binary = Join-Path $ManagedBin "$Name.exe"
    if (-not (Test-ManagedLocationSafe $binary)) {
        Write-Gap "refusing managed Go tool through reparse point: $binary"
        return
    }
    $item = Get-PathItem $binary
    if ($item -and (Test-IsReparsePoint $binary)) {
        Write-Gap "refusing to replace reparse point: $binary"
        return
    }
    $current = Get-GoModuleVersion $binary $Module
    if ($current -eq $Version) { Write-Ok "$Name $Version ($binary)"; return }
    if ($item -and -not $current) {
        Write-Gap "refusing to overwrite unmanaged Go tool: $binary"
        return
    }
    if ($Check) { Write-Gap "$Name $Version$(if($current){" (found $current)"})"; return }

    $go = Get-SystemCommandPath go
    if (-not $go -or -not (Ensure-SafeManagedDirectory $ToolsRoot)) {
        Write-Gap "$Name $Version (safe Go installation unavailable)"
        return
    }

    $stagingRoot = Join-Path $ToolsRoot ".$Name-install-$([guid]::NewGuid().ToString('N'))"
    $stagingBin = Join-Path $stagingRoot 'bin'
    $oldGoBin = $env:GOBIN
    try {
        New-Item -ItemType Directory -Force -Path $stagingBin | Out-Null
        $env:GOBIN = $stagingBin
        Write-Info "installing $Name $Version into temporary GOBIN"
        & $go install "$Package@$Version"
        if ($LASTEXITCODE -ne 0) {
            Write-Gap "$Name $Version (go install failed)"
            return
        }

        $staged = Join-Path $stagingBin "$Name.exe"
        if ((Get-GoModuleVersion $staged $Module) -ne $Version) {
            Write-Gap "$Name $Version (installed module or version mismatch)"
            return
        }
        if (-not (Ensure-SafeManagedDirectory $ManagedBin)) { return }

        $existing = Get-PathItem $binary
        if ($existing) {
            if (Test-IsReparsePoint $binary) {
                Write-Gap "refusing to replace reparse point: $binary"
                return
            }
            if (-not (Get-GoModuleVersion $binary $Module)) {
                Write-Gap "refusing to overwrite unmanaged Go tool: $binary"
                return
            }
        }

        try {
            Publish-FileAtomically $staged $binary
        } catch {
            Write-Gap "$Name $Version (atomic publish failed: $($_.Exception.Message))"
            return
        }
        if ((Get-GoModuleVersion $binary $Module) -ne $Version) {
            Write-Gap "$Name $Version (published module verification failed)"
            return
        }
        Write-Ok "$Name $Version ($binary)"
    } finally {
        Restore-EnvironmentVariable GOBIN $oldGoBin
        Remove-Item -LiteralPath $stagingRoot -Recurse -Force -ErrorAction SilentlyContinue
    }
}

function Find-GitBash {
    foreach ($candidate in @(
        (Join-Path $env:LOCALAPPDATA 'Programs\Git\bin\bash.exe'),
        'C:\Program Files\Git\bin\bash.exe',
        'C:\Program Files (x86)\Git\bin\bash.exe'
    )) { if (Test-Path $candidate) { return $candidate } }
    return $null
}

function Get-ManagedWrapperText {
    param([string[]]$Lines)
    return [string]::Join([Environment]::NewLine, @($ManagedMarker, '@echo off') + $Lines) +
        [Environment]::NewLine
}

function Test-ManagedWrapperContent {
    param([string]$Path, [string[]]$Lines)
    $leaf = [IO.Path]::GetFileName($Path)
    $parent = [IO.Path]::GetDirectoryName([IO.Path]::GetFullPath($Path))
    if ($ManagedWrappers -notcontains $leaf -or
        -not $parent.Equals([IO.Path]::GetFullPath($ManagedBin), [StringComparison]::OrdinalIgnoreCase)) {
        return $false
    }
    if (-not (Test-ManagedLocationSafe $Path)) { return $false }
    $item = Get-PathItem $Path
    if (-not $item -or $item.PSIsContainer -or (Test-IsReparsePoint $Path)) { return $false }
    return [IO.File]::ReadAllText($Path, [Text.Encoding]::ASCII) -ceq
        (Get-ManagedWrapperText $Lines)
}

function Write-ManagedWrapper {
    param([string]$Path, [string[]]$Lines)
    $leaf = [IO.Path]::GetFileName($Path)
    $parent = [IO.Path]::GetDirectoryName([IO.Path]::GetFullPath($Path))
    if ($ManagedWrappers -notcontains $leaf -or
        -not $parent.Equals([IO.Path]::GetFullPath($ManagedBin), [StringComparison]::OrdinalIgnoreCase)) {
        Write-Gap "refusing non-whitelisted managed wrapper: $Path"
        return $false
    }
    if (-not (Ensure-SafeManagedDirectory $ManagedBin)) { return $false }
    $item = Get-PathItem $Path
    if ($item) {
        if (Test-IsReparsePoint $Path) {
            Write-Gap "refusing to replace reparse point: $Path"
            return $false
        }
        if (-not (Test-ManagedWrapperContent $Path $Lines)) {
            Write-Gap "refusing to overwrite unmanaged wrapper: $Path"
            return $false
        }
    }

    $temporary = Join-Path $ManagedBin ".$leaf-$([guid]::NewGuid().ToString('N')).tmp"
    try {
        [IO.File]::WriteAllText($temporary, (Get-ManagedWrapperText $Lines), [Text.Encoding]::ASCII)
        $current = Get-PathItem $Path
        if ($current -and -not (Test-ManagedWrapperContent $Path $Lines)) {
            Write-Gap "refusing to replace changed or unmanaged wrapper: $Path"
            return $false
        }
        Publish-FileAtomically $temporary $Path
    } catch {
        Write-Gap "failed to publish managed wrapper atomically: $Path ($($_.Exception.Message))"
        return $false
    } finally {
        Remove-Item -LiteralPath $temporary -Force -ErrorAction SilentlyContinue
    }
    return $true
}

function Test-ManagedWrapper {
    param([string]$Path, [string[]]$Lines, [string[]]$Arguments = @('--version'))
    if (-not (Test-ManagedWrapperContent $Path $Lines)) { return $false }
    & $Path @Arguments > $null 2>&1
    return ($LASTEXITCODE -eq 0)
}

function Find-Python {
    foreach ($name in @('python3', 'python')) {
        $command = Get-SystemCommandPath $name
        if ($command) {
            & $command --version > $null 2>&1
            if ($LASTEXITCODE -eq 0) { return $command }
        }
    }
    $launcher = Get-SystemCommandPath py
    if ($launcher) {
        $candidate = & $launcher -3 -c "import sys; print(sys.executable)" 2>$null
        if ($LASTEXITCODE -eq 0 -and $candidate -and (Test-Path $candidate)) { return $candidate }
    }
    return $null
}

function Test-ReparseFreeTree {
    param([string]$Root)
    if (-not (Get-PathItem $Root)) { return $false }
    if (-not (Test-PathComponentsReparseFree $Root $ToolsRoot)) { return $false }
    $queue = New-Object 'Collections.Generic.Queue[string]'
    $queue.Enqueue([IO.Path]::GetFullPath($Root))
    while ($queue.Count -gt 0) {
        $current = $queue.Dequeue()
        $item = Get-PathItem $current
        if (-not $item -or (Test-IsReparsePoint $current)) { return $false }
        if (-not $item.PSIsContainer) { continue }
        try {
            $children = @(Get-ChildItem -Force -LiteralPath $current)
        } catch {
            return $false
        }
        foreach ($child in $children) {
            if (($child.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) { return $false }
            if ($child.PSIsContainer) { $queue.Enqueue($child.FullName) }
        }
    }
    return $true
}

function Get-PreCommitVersion {
    param([string]$Path)
    if (-not (Get-PathItem $Path) -or (Test-IsReparsePoint $Path)) { return '' }
    $oldBytecodeSetting = $env:PYTHONDONTWRITEBYTECODE
    try {
        $env:PYTHONDONTWRITEBYTECODE = '1'
        $output = @(& $Path --version 2>$null)
        $exitCode = $LASTEXITCODE
    } finally {
        Restore-EnvironmentVariable PYTHONDONTWRITEBYTECODE $oldBytecodeSetting
    }
    if ($exitCode -ne 0 -or $output.Count -ne 1) { return '' }
    return [string]$output[0]
}

function Ensure-PreCommit {
    param([string]$Python)
    $wrapper = Join-Path $ManagedBin 'pre-commit.cmd'
    $target = Join-Path $PreCommitVenv 'Scripts\pre-commit.exe'
    $wrapperLines = @("`"$target`" %*")
    $wrapperItem = Get-PathItem $wrapper
    $wrapperManaged = $wrapperItem -and (Test-ManagedWrapperContent $wrapper $wrapperLines)
    if ($wrapperItem -and -not $wrapperManaged) {
        Write-Gap "refusing forged or unmanaged pre-commit wrapper: $wrapper"
        return
    }

    $venvItem = Get-PathItem $PreCommitVenv
    $current = ''
    if ($venvItem) {
        if (-not $wrapperManaged) {
            Write-Gap "refusing to replace unmanaged pre-commit environment: $PreCommitVenv"
            return
        }
        if (-not (Test-ReparseFreeTree $PreCommitVenv)) {
            Write-Gap "refusing pre-commit environment containing a reparse point: $PreCommitVenv"
            return
        }
        $current = Get-PreCommitVersion $target
        if ($current -ceq "pre-commit $PreCommitVersion") {
            Write-Ok "pre-commit $PreCommitVersion ($wrapper)"
            return
        }
        Write-Gap "refusing to overwrite invalid managed pre-commit environment: $PreCommitVenv"
        return
    }
    if ($Check) {
        Write-Gap "pre-commit $PreCommitVersion$(if($current){" (found $current)"})"
        return
    }
    if (-not (Ensure-SafeManagedDirectory $ToolsRoot)) { return }

    $createdVenv = $false
    $completed = $false
    try {
        New-Item -ItemType Directory -Path $PreCommitVenv -ErrorAction Stop | Out-Null
        $createdVenv = $true
        Write-Info "installing pre-commit $PreCommitVersion into $PreCommitVenv"
        & $Python -m venv $PreCommitVenv
        if ($LASTEXITCODE -ne 0 -or -not (Test-ReparseFreeTree $PreCommitVenv)) {
            Write-Gap 'pre-commit venv creation failed or produced a reparse point'
            return
        }
        $venvPython = Join-Path $PreCommitVenv 'Scripts\python.exe'
        & $venvPython -m pip install --disable-pip-version-check "pre-commit==$PreCommitVersion" > $null
        if ($LASTEXITCODE -ne 0 -or -not (Test-ReparseFreeTree $PreCommitVenv)) {
            Write-Gap 'pre-commit venv install failed or produced a reparse point'
            return
        }
        if ((Get-PreCommitVersion $target) -cne "pre-commit $PreCommitVersion") {
            Write-Gap 'pre-commit venv version verification failed'
            return
        }
        if (-not (Write-ManagedWrapper $wrapper $wrapperLines)) { return }
        $completed = $true
        Write-Ok "pre-commit $PreCommitVersion ($wrapper)"
    } finally {
        if (-not $completed -and $createdVenv -and (Test-ReparseFreeTree $PreCommitVenv)) {
            Remove-Item -LiteralPath $PreCommitVenv -Recurse -Force -ErrorAction SilentlyContinue
        }
    }
}

$result = 0
try {
    Initialize-CheckEnvironment
    Refresh-SystemPath
    if (-not $Check) {
        Write-Section 'System dependencies'
        Install-WingetPackage go GoLang.Go @('version')
        Install-WingetPackage git Git.Git
        Install-WingetPackage python Python.Python.3.12
        Install-WingetPackage make ezwinports.make
        Install-WingetPackage gcc BrechtSanders.WinLibs.MCF.UCRT
    }

    $go = Get-SystemCommandPath go
    if (-not $Check -and $go -and -not (Test-GoVersion)) {
        $winget = Get-TrustedWingetPath
        if ($winget) {
            Write-Info 'upgrading Go with winget'
            & $winget upgrade --id GoLang.Go --exact --source winget `
                --accept-package-agreements --accept-source-agreements --disable-interactivity
            if ($LASTEXITCODE -ne 0) { Write-Gap 'Go upgrade failed' }
            Refresh-SystemPath -IncludePersistent
        } else {
            $guidance = "Go $GoMinMajor.$GoMinMinor+ (winget unavailable; download and install from " +
                'https://go.dev/dl/, then rerun this script)'
            Write-Gap $guidance
        }
    }

    Write-Section 'Go toolchain'
    if (-not (Test-GoVersion)) {
        Write-Gap "Go $GoMinMajor.$GoMinMinor+ (install with winget or download from https://go.dev/dl/)"
    } else {
        $go = Get-SystemCommandPath go
        Write-Ok "go $(& $go env GOVERSION) ($go)"
    }
    $goReady = Test-GoVersion

    if ($goReady) {
        Write-Section 'Module dependencies'
        $go = Get-SystemCommandPath go
        $listExit = Invoke-NativeQuietly $go @('list', '-mod=readonly', '-deps', './...')
        if ($listExit -eq 0) { Write-Ok 'module dependencies resolved' }
        elseif ($Check) { Write-Gap 'module dependencies (run without -Check to download)' }
        else {
            & $go mod download
            if ($LASTEXITCODE -eq 0) { Write-Ok 'module dependencies resolved' }
            else {
                Write-Gap 'go mod download failed'
                Write-Info 'hint: check network reachability and go env GOPROXY.'
                Write-Info 'If proxy.golang.org is unreachable, a public module mirror may help, for example:'
                Write-Info '  go env -w GOPROXY=https://goproxy.cn,direct'
                Write-Info '  go env -w GOPROXY=https://mirrors.aliyun.com/goproxy/,direct'
                Write-Info '(examples only; pick one per your own network environment.)'
            }
        }

        Write-Section 'Verification tools'
        $golangci = $ManagedGoTools['golangci-lint']
        Ensure-GoTool golangci-lint $golangci.Module $golangci.Package $golangci.Version
        $gitleaks = $ManagedGoTools['gitleaks']
        Ensure-GoTool gitleaks $gitleaks.Module $gitleaks.Package $gitleaks.Version
    }

    Write-Section 'Git and shell utilities'
    if (Test-SystemCommandWorks git) { Write-Ok "git ($(Get-SystemCommandPath git))" }
    else { Write-Gap 'git' }
    $gitBash = Find-GitBash
    if ($gitBash) { Write-Ok "git bash ($gitBash)" } else { Write-Gap 'git bash' }

    $realMake = $null
    foreach ($name in @('mingw32-make', 'make')) {
        $found = Get-SystemCommandPath $name
        if ($found) { $realMake = $found; break }
    }
    $makeWrapper = Join-Path $ManagedBin 'make.cmd'
    $makeWrapperLines = $null
    if ($realMake -and $gitBash) {
        $fso = New-Object -ComObject Scripting.FileSystemObject
        $shortBash = ($fso.GetFile($gitBash).ShortPath) -replace '\\', '/'
        [void][Runtime.InteropServices.Marshal]::ReleaseComObject($fso)
        $makeWrapperLines = @(
            "`"$realMake`" `"SHELL=$shortBash`" `".SHELLFLAGS=-c`" %*"
        )
    }
    if ($Check) {
        if ($makeWrapperLines -and (Test-ManagedWrapper $makeWrapper $makeWrapperLines)) {
            Write-Ok "managed make wrapper ($makeWrapper)"
        } else { Write-Gap 'managed make wrapper' }
    } elseif ($makeWrapperLines) {
        if (Write-ManagedWrapper $makeWrapper $makeWrapperLines) {
            Write-Ok "managed make wrapper ($makeWrapper)"
        }
    } else { Write-Gap 'GNU make and Git bash' }

    $python = Find-Python
    if ($python) {
        Write-Ok "python ($python)"
        $pythonWrapper = Join-Path $ManagedBin 'python3.cmd'
        $pythonWrapperLines = @("`"$python`" %*")
        if (-not $Check) {
            if (Write-ManagedWrapper $pythonWrapper $pythonWrapperLines) {
                Write-Ok "managed python3 wrapper ($pythonWrapper)"
            }
        } elseif (Test-ManagedWrapper $pythonWrapper $pythonWrapperLines) {
            Write-Ok 'managed python3 wrapper'
        }
        else { Write-Gap 'managed python3 wrapper' }
        Ensure-PreCommit $python
    } else { Write-Gap 'Python 3' }

    $compiler = $null
    foreach ($name in @('gcc', 'clang', 'cc')) {
        $found = Get-SystemCommandPath $name
        if ($found) { $compiler = $found; break }
    }
    if ($compiler) { Write-Ok "C compiler ($compiler)" }
    else { Write-Gap 'C compiler (required by go test -race)' }

    Write-Section 'Verification'
    if ($goReady) {
        $smokeDir = Join-Path ([IO.Path]::GetTempPath()) "gc-dev-setup-$([guid]::NewGuid().ToString('N'))"
        New-Item -ItemType Directory -Force -Path $smokeDir | Out-Null
        try {
            $go = Get-SystemCommandPath go
            $smokeBinary = Join-Path $smokeDir 'gc.exe'
            $buildExit = Invoke-NativeQuietly $go @(
                'build', '-mod=readonly', '-o', $smokeBinary, './cmd/gc'
            )
            $smokeExit = if ($buildExit -eq 0) {
                Invoke-NativeQuietly $smokeBinary @('version')
            } else {
                1
            }
            if ($buildExit -eq 0 -and $smokeExit -eq 0) {
                Write-Ok 'build and gc version'
            } else { Write-Gap 'build or gc version' }
            if ($compiler) {
                $raceExit = Invoke-NativeQuietly $go @(
                    'test', '-mod=readonly', '-race', './pkg/config'
                )
                if ($raceExit -eq 0) { Write-Ok 'race-enabled test' }
                else { Write-Gap 'race-enabled test' }
            }
        } finally {
            Remove-Item -LiteralPath $smokeDir -Recurse -Force -ErrorAction SilentlyContinue
        }
    }

    # In check mode an absent managed bin on PATH is a gap: the pre-commit and
    # pre-push hooks resolve gitleaks through PATH and would fail on commit.
    if ($Check -and -not (Test-ManagedBinOnPath)) {
        Write-Gap 'managed tools on PATH (git hooks depend on it)'
    }

    Write-Host ''
    if ($script:Missing.Count) {
        Write-Host "incomplete: $($script:Missing.Count) dependency gap(s)"
        foreach ($item in $script:Missing) { Write-Host "  - $item" }
        Write-Host "Managed tools directory: $ManagedBin"
        Write-Host 'After resolving gaps, add it to the current shell PATH:'
        Write-Host ('  $env:Path = "{0};$env:Path"' -f $ManagedBin)
        $result = 1
    } else {
        Write-Host 'dev environment ready.'
        Write-Host "Managed tools: $ManagedBin"
        if (Test-ManagedBinOnPath) {
            Write-Host 'No user PATH was changed.'
        } else {
            Write-Host 'WARNING: managed tools are not on PATH. Git hooks (pre-commit / pre-push)'
            Write-Host 'will fail with "gitleaks: command not found" until it is on PATH.'
            Write-Host 'Fix the current shell:'
            Write-Host ('  $env:Path = "{0};$env:Path"' -f $ManagedBin)
            Write-Host 'Persist it (PowerShell profile example; pick your own profile file):'
            $persistExample = "Add-Content -Path `$PROFILE -Value '`$env:Path = `"$ManagedBin;`$env:Path`"'"
            Write-Host "  $persistExample"
            Write-Host 'No user PATH was changed by this script.'
        }
    }
} finally {
    Remove-CheckEnvironment
}
exit $result
