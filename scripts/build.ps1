[CmdletBinding()]
param(
    [string]$Version = "dev",
    [string]$OutputDirectory = "bin"
)

$ErrorActionPreference = "Stop"
$repositoryRoot = Split-Path -Parent $PSScriptRoot
$outputPath = Join-Path $repositoryRoot $OutputDirectory
$executablePath = Join-Path $outputPath "wintonicz.exe"

New-Item -ItemType Directory -Path $outputPath -Force | Out-Null

Push-Location $repositoryRoot
try {
    & go build -trimpath -ldflags "-X main.version=$Version" -o $executablePath ./cmd/wintonicz
    if ($LASTEXITCODE -ne 0) {
        throw "Go build failed with exit code $LASTEXITCODE."
    }
} finally {
    Pop-Location
}

Write-Host "Built $executablePath"
