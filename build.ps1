[CmdletBinding()]
param(
    [string]$OutputDir = "dist",
    [string]$OutputName = "go-google-translate-proxy.exe",
    [switch]$Clean
)

$ErrorActionPreference = "Stop"

$ProjectRoot = $PSScriptRoot
$OutputPath = Join-Path -Path $ProjectRoot -ChildPath $OutputDir
$BinaryPath = Join-Path -Path $OutputPath -ChildPath $OutputName

if ($Clean -and (Test-Path -LiteralPath $OutputPath)) {
    Remove-Item -LiteralPath $OutputPath -Recurse -Force
}

if (-not (Test-Path -LiteralPath $OutputPath)) {
    New-Item -ItemType Directory -Path $OutputPath | Out-Null
}

$previousCGOEnabled = $env:CGO_ENABLED
$env:CGO_ENABLED = "0"

try {
    & go build -trimpath -ldflags "-s -w" -o $BinaryPath .
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }
}
finally {
    $env:CGO_ENABLED = $previousCGOEnabled
}

"Built: $BinaryPath"
