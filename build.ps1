[CmdletBinding()]
param(
    [string]$OutputDir = "dist",
    [string]$BinaryName = "go-google-translate-proxy",
    [switch]$Clean
)

$ErrorActionPreference = "Stop"

$ProjectRoot = $PSScriptRoot
$OutputPath = Join-Path -Path $ProjectRoot -ChildPath $OutputDir

$Targets = @(
    [pscustomobject]@{ GOOS = "windows"; GOARCH = "386";   GOARM = $null; Label = "windows-386";   Extension = ".exe" },
    [pscustomobject]@{ GOOS = "windows"; GOARCH = "amd64"; GOARM = $null; Label = "windows-amd64"; Extension = ".exe" },
    [pscustomobject]@{ GOOS = "windows"; GOARCH = "arm";   GOARM = "7";   Label = "windows-armv7"; Extension = ".exe" },
    [pscustomobject]@{ GOOS = "windows"; GOARCH = "arm64"; GOARM = $null; Label = "windows-arm64"; Extension = ".exe" },
    [pscustomobject]@{ GOOS = "linux";   GOARCH = "386";   GOARM = $null; Label = "linux-386";     Extension = "" },
    [pscustomobject]@{ GOOS = "linux";   GOARCH = "amd64"; GOARM = $null; Label = "linux-amd64";   Extension = "" },
    [pscustomobject]@{ GOOS = "linux";   GOARCH = "arm";   GOARM = "5";   Label = "linux-armv5";   Extension = "" },
    [pscustomobject]@{ GOOS = "linux";   GOARCH = "arm";   GOARM = "6";   Label = "linux-armv6";   Extension = "" },
    [pscustomobject]@{ GOOS = "linux";   GOARCH = "arm";   GOARM = "7";   Label = "linux-armv7";   Extension = "" },
    [pscustomobject]@{ GOOS = "linux";   GOARCH = "arm64"; GOARM = $null; Label = "linux-arm64";   Extension = "" },
    [pscustomobject]@{ GOOS = "darwin";  GOARCH = "amd64"; GOARM = $null; Label = "macos-amd64";   Extension = "" },
    [pscustomobject]@{ GOOS = "darwin";  GOARCH = "arm64"; GOARM = $null; Label = "macos-arm64";   Extension = "" }
)

if ($Clean -and (Test-Path -LiteralPath $OutputPath)) {
    Remove-Item -LiteralPath $OutputPath -Recurse -Force
}

if (-not (Test-Path -LiteralPath $OutputPath)) {
    New-Item -ItemType Directory -Path $OutputPath | Out-Null
}

$previousEnv = @{
    CGO_ENABLED = $env:CGO_ENABLED
    GOOS        = $env:GOOS
    GOARCH      = $env:GOARCH
    GOARM       = $env:GOARM
}

try {
    foreach ($Target in $Targets) {
        $env:CGO_ENABLED = "0"
        $env:GOOS = $Target.GOOS
        $env:GOARCH = $Target.GOARCH

        if ($Target.GOARM) {
            $env:GOARM = $Target.GOARM
        }
        else {
            Remove-Item -LiteralPath Env:GOARM -ErrorAction SilentlyContinue
        }

        $OutputFile = "$BinaryName-$($Target.Label)$($Target.Extension)"
        $BinaryPath = Join-Path -Path $OutputPath -ChildPath $OutputFile
        $TargetName = "$($Target.GOOS)/$($Target.GOARCH)"
        if ($Target.GOARM) {
            $TargetName = "$TargetName GOARM=$($Target.GOARM)"
        }

        "Building: $TargetName -> $BinaryPath"
        & go build -trimpath -ldflags "-s -w" -o $BinaryPath .
        if ($LASTEXITCODE -ne 0) {
            throw "go build failed for $TargetName"
        }
    }
}
finally {
    foreach ($Name in $previousEnv.Keys) {
        if ($null -eq $previousEnv[$Name]) {
            Remove-Item -LiteralPath "Env:$Name" -ErrorAction SilentlyContinue
        }
        else {
            Set-Item -LiteralPath "Env:$Name" -Value $previousEnv[$Name]
        }
    }
}

"Built $($Targets.Count) targets in: $OutputPath"
