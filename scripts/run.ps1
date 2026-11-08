<#
.SYNOPSIS
    Downloads (once), verifies, caches, and runs Wintonicz without an installer.

.DESCRIPTION
    Quiet bootstrapper for Windows PowerShell 5.1+ and PowerShell 7.

      irm https://raw.githubusercontent.com/MatheusCampagnolo/wintonicz/main/scripts/run.ps1 | iex

    Behavior:
      * The first run downloads the release executable for this CPU, verifies its SHA-256
        digest against the release's checksums.txt, and stores it in
        %LOCALAPPDATA%\Wintonicz\cache\<version>\wintonicz.exe. Every run first asks GitHub for the newest release (one small request); a newer one is
        downloaded and verified once, otherwise the cached copy starts. Offline, the newest cached copy runs.
      * Nothing is installed: no PATH change, no profile edit, no service, no registry write.
      * A half-downloaded or hash-mismatched file is never executed.
      * Pin a version (-Version) or use -NoUpdate to skip the lookup.

    Passing arguments through `irm | iex` is not possible. Use the script-block form instead:

      & ([scriptblock]::Create((irm <url>))) system info --json
      & ([scriptblock]::Create((irm <url>))) -Admin

    Environment overrides (useful for pipelines and mirrors):
      WTZ_VERSION   pin a release, for example 0.2.0-alpha (same as -Version)
      WTZ_ADMIN     set to 1 to start elevated (same as -Admin)
      WTZ_BASE_URL  release download base URL (mirrors and tests); must serve
                    <base>/v<version>/<asset> and <base>/v<version>/checksums.txt

.PARAMETER Version
    Release version to run, without the leading "v". Default: newest release.

.PARAMETER Admin
    Start Wintonicz in an elevated window (one UAC prompt for the whole session).

.PARAMETER NoCache
    Download into a temporary folder and delete it afterwards.

.PARAMETER ClearCache
    Delete the Wintonicz cache and exit.

.PARAMETER NoUpdate
    Do not look for a newer release; run the newest cached version (falls back to a lookup when nothing is cached).

.PARAMETER Refresh
    Obsolete: the newest release is checked on every run.

.PARAMETER WtzArgs
    Everything else is passed to wintonicz.exe unchanged.
#>
[CmdletBinding(PositionalBinding = $false)]
param(
    [string]$Version = $env:WTZ_VERSION,
    [switch]$Admin,
    [switch]$NoCache,
    [switch]$ClearCache,
    [switch]$Refresh,   # kept for compatibility; the newest release is now checked on every run
    [switch]$NoUpdate,
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$WtzArgs
)

# Through `irm | iex` this script runs inside the caller's session, so remember what we change and put it back.
$script:WtzSavedErrorAction = $ErrorActionPreference
$script:WtzSavedProgress = $ProgressPreference
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'   # hides the progress bar; also makes Windows PowerShell 5.1 downloads much faster

$WtzOwner = 'MatheusCampagnolo'
$WtzRepo = 'wintonicz'


function Write-Wtz([string]$Message) {
    Write-Host "wintonicz: $Message" -ForegroundColor DarkGray
}

function Stop-Wtz([string]$Message) {
    throw "wintonicz: $Message"
}

function Get-WtzCacheRoot {
    $base = $env:LOCALAPPDATA
    if ([string]::IsNullOrWhiteSpace($base)) { $base = [IO.Path]::GetTempPath() }
    Join-Path $base 'Wintonicz\cache'
}

function Get-WtzArchitecture {
    # PROCESSOR_ARCHITEW6432 is set when a 32-bit PowerShell runs on 64-bit Windows.
    $arch = $env:PROCESSOR_ARCHITEW6432
    if ([string]::IsNullOrEmpty($arch)) { $arch = $env:PROCESSOR_ARCHITECTURE }
    switch ($arch) {
        'AMD64' { 'amd64' }
        'ARM64' { 'amd64' }   # Windows on ARM runs the x64 build under emulation until a native build is validated
        default { Stop-Wtz "unsupported CPU architecture '$arch'. Wintonicz currently supports 64-bit Windows." }
    }
}

function Test-WtzAdministrator {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    ([Security.Principal.WindowsPrincipal]$identity).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Get-WtzDownloadBase {
    if ($env:WTZ_BASE_URL) { return $env:WTZ_BASE_URL.TrimEnd('/') }
    "https://github.com/$WtzOwner/$WtzRepo/releases/download"
}

function Invoke-WtzDownload([string]$Uri, [string]$OutFile) {
    $params = @{ Uri = $Uri; OutFile = $OutFile; UseBasicParsing = $true }
    Invoke-WebRequest @params | Out-Null
}

function Get-WtzCachedVersion([string]$CacheRoot) {
    if (-not (Test-Path -LiteralPath $CacheRoot)) { return $null }
    $local = Get-ChildItem -LiteralPath $CacheRoot -Directory -ErrorAction SilentlyContinue |
        Where-Object { Test-Path -LiteralPath (Join-Path $_.FullName 'wintonicz.exe') } |
        Sort-Object LastWriteTime -Descending | Select-Object -First 1
    if ($local) { $local.Name } else { $null }
}

function Get-WtzLatestVersion([string]$CacheRoot) {
    $cached = Get-WtzCachedVersion $CacheRoot
    if ($NoUpdate -and $cached) { return $cached }
    try {
        # Checked on every run (one small request, short timeout). The releases list includes
        # prereleases, unlike /releases/latest.
        $headers = @{ 'User-Agent' = 'wintonicz-launcher'; 'Accept' = 'application/vnd.github+json' }
        $releases = Invoke-RestMethod -Uri "https://api.github.com/repos/$WtzOwner/$WtzRepo/releases?per_page=5" -Headers $headers -TimeoutSec 8
        $release = $releases | Where-Object { -not $_.draft } | Select-Object -First 1
        if (-not $release) { Stop-Wtz 'no published release was found.' }
        return ([string]$release.tag_name).TrimStart('v')
    } catch {
        # Offline or rate limited: keep working with the newest cached version.
        if ($cached) { return $cached }
        Stop-Wtz "could not look up the newest release ($($_.Exception.Message)). Check your connection or pass -Version."
    }
}

function Install-WtzRelease([string]$Ver, [string]$Arch, [string]$TargetDirectory) {
    $asset = "wintonicz_${Ver}_windows_$Arch.exe"
    $base = "$(Get-WtzDownloadBase)/v$Ver"
    $work = Join-Path ([IO.Path]::GetTempPath()) ("wintonicz-" + [Guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path $work -Force | Out-Null
    try {
        $sums = Join-Path $work 'checksums.txt'
        $part = Join-Path $work $asset
        try {
            Invoke-WtzDownload "$base/checksums.txt" $sums
            Invoke-WtzDownload "$base/$asset" $part
        } catch {
            Stop-Wtz "download failed for v$Ver ($($_.Exception.Message))."
        }
        $expected = $null
        foreach ($line in Get-Content -LiteralPath $sums) {
            if ($line -match '^\s*([0-9a-fA-F]{64})\s+\*?(.+?)\s*$' -and $Matches[2] -eq $asset) { $expected = $Matches[1]; break }
        }
        if (-not $expected) { Stop-Wtz "checksums.txt has no entry for $asset." }
        $actual = (Get-FileHash -LiteralPath $part -Algorithm SHA256).Hash
        if ($actual -ne $expected.ToUpperInvariant()) {
            Stop-Wtz "SHA-256 mismatch for $asset. Expected $expected but got $actual. The file was not run."
        }
        New-Item -ItemType Directory -Path $TargetDirectory -Force | Out-Null
        $final = Join-Path $TargetDirectory 'wintonicz.exe'
        $staged = Join-Path $TargetDirectory ('wintonicz.exe.' + [Guid]::NewGuid().ToString('N') + '.tmp')
        Copy-Item -LiteralPath $part -Destination $staged -Force
        Move-Item -LiteralPath $staged -Destination $final -Force   # same-volume rename, so a partial file is never visible as wintonicz.exe
        return $final
    } finally {
        Remove-Item -LiteralPath $work -Recurse -Force -ErrorAction SilentlyContinue
    }
}

function Invoke-Wintonicz {
    [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

    $cacheRoot = Get-WtzCacheRoot
    if ($ClearCache) {
        if (Test-Path -LiteralPath $cacheRoot) { Remove-Item -LiteralPath $cacheRoot -Recurse -Force }
        Write-Wtz "cache cleared ($cacheRoot)."
        $script:WtzExitCode = 0
        return
    }

    $arch = Get-WtzArchitecture
    $ver = $Version
    if ($ver) { $ver = $ver.TrimStart('v') } else { $ver = Get-WtzLatestVersion $cacheRoot }
    if ($ver -notmatch '^[0-9A-Za-z][0-9A-Za-z.\-+]*$') { Stop-Wtz "'$ver' is not a valid version." }

    $temporary = $null
    if ($NoCache) {
        $temporary = Join-Path ([IO.Path]::GetTempPath()) ("wintonicz-run-" + [Guid]::NewGuid().ToString('N'))
        $targetDirectory = $temporary
    } else {
        $targetDirectory = Join-Path $cacheRoot $ver
    }

    $exe = Join-Path $targetDirectory 'wintonicz.exe'
    try {
        if (-not (Test-Path -LiteralPath $exe)) {
            $have = Get-WtzCachedVersion $cacheRoot
            if ($have -and -not $NoCache) { Write-Wtz "updating $have -> $ver..." } else { Write-Wtz "preparing $ver (first run only)..." }
            $exe = Install-WtzRelease $ver $arch $targetDirectory
        }

        $wantAdmin = $Admin -or $env:WTZ_ADMIN -eq '1'
        if ($wantAdmin -and -not (Test-WtzAdministrator)) {
            # One UAC prompt for the whole session. The elevated window stays open so output remains readable.
            $quoted = ($WtzArgs | ForEach-Object { "'" + ($_ -replace "'", "''") + "'" }) -join ' '
            $command = "& '$($exe -replace "'", "''")' $quoted"
            $shell = (Get-Process -Id $PID).Path
            $process = Start-Process -FilePath $shell -Verb RunAs -PassThru -Wait `
                -ArgumentList @('-NoProfile', '-NoLogo', '-Command', $command)
            $script:WtzExitCode = $process.ExitCode
            return
        }

        & $exe @WtzArgs
        $script:WtzExitCode = $LASTEXITCODE
        return
    } finally {
        if ($temporary) { Remove-Item -LiteralPath $temporary -Recurse -Force -ErrorAction SilentlyContinue }
    }
}

$script:WtzExitCode = 0
try {
    Invoke-Wintonicz   # must not be captured: the child process needs the real console, and its output is not ours to collect
} finally {
    $ErrorActionPreference = $script:WtzSavedErrorAction
    $ProgressPreference = $script:WtzSavedProgress
    foreach ($name in 'Write-Wtz', 'Stop-Wtz', 'Get-WtzCacheRoot', 'Get-WtzArchitecture', 'Test-WtzAdministrator',
                      'Get-WtzDownloadBase', 'Invoke-WtzDownload', 'Get-WtzCachedVersion', 'Get-WtzLatestVersion', 'Install-WtzRelease', 'Invoke-Wintonicz') {
        Remove-Item -LiteralPath "Function:\$name" -ErrorAction SilentlyContinue
    }
}
$exitCode = $script:WtzExitCode
$ranAsFile = $MyInvocation.MyCommand.Path -and ((Split-Path -Leaf $MyInvocation.MyCommand.Path) -ieq 'run.ps1')
if ($ranAsFile) {
    exit $exitCode                     # launched as a file (CI, agents): return the real exit code
}
# Launched through iex or a script block: `exit` would close the user's terminal, so expose the code instead.
$global:LASTEXITCODE = $exitCode
