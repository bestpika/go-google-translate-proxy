[CmdletBinding()]
param(
    [string]$OutputDir = "dist",
    [string]$BinaryName = "go-google-translate-proxy",
    [switch]$Clean,
    [switch]$Windows,
    [switch]$Linux,
    [switch]$MacOS,
    [switch]$All
)

$ErrorActionPreference = "Stop"

$ProjectRoot = $PSScriptRoot
$OutputPath = Join-Path -Path $ProjectRoot -ChildPath $OutputDir

function Get-TargetLabel {
    param(
        [string]$GOOS,
        [string]$GOARCH,
        [string]$GOARM
    )

    $osLabel = $GOOS
    if ($GOOS -eq "darwin") {
        $osLabel = "macos"
    }

    $archLabel = $GOARCH
    if ($GOARCH -eq "arm" -and $GOARM) {
        $archLabel = "armv$GOARM"
    }

    return "$osLabel-$archLabel"
}

function Get-TargetExtension {
    param([string]$GOOS)

    if ($GOOS -eq "windows") {
        return ".exe"
    }

    return ""
}

function New-BuildTarget {
    param(
        [string]$GOOS,
        [string]$GOARCH,
        [string]$GOARM
    )

    if ($GOARCH -ne "arm") {
        $GOARM = $null
    }

    return [pscustomobject]@{
        GOOS      = $GOOS
        GOARCH    = $GOARCH
        GOARM     = $GOARM
        Label     = Get-TargetLabel -GOOS $GOOS -GOARCH $GOARCH -GOARM $GOARM
        Extension = Get-TargetExtension -GOOS $GOOS
    }
}

function Get-GoEnvValue {
    param([string]$Name)

    $value = (& go env $Name).Trim()
    if ($LASTEXITCODE -ne 0) {
        throw "go env $Name failed"
    }

    return $value
}

function Get-CurrentBuildTarget {
    $currentGOOS = Get-GoEnvValue -Name "GOOS"
    $currentGOARCH = Get-GoEnvValue -Name "GOARCH"
    $currentGOARM = $null

    if ($currentGOARCH -eq "arm") {
        $currentGOARM = Get-GoEnvValue -Name "GOARM"
    }

    return New-BuildTarget -GOOS $currentGOOS -GOARCH $currentGOARCH -GOARM $currentGOARM
}

$AllTargets = @(
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

if ($All) {
    $Targets = @($AllTargets)
}
else {
    $Targets = @()

    if ($Windows) {
        $Targets += @($AllTargets | Where-Object { $_.GOOS -eq "windows" })
    }
    if ($Linux) {
        $Targets += @($AllTargets | Where-Object { $_.GOOS -eq "linux" })
    }
    if ($MacOS) {
        $Targets += @($AllTargets | Where-Object { $_.GOOS -eq "darwin" })
    }

    if ($Targets.Count -eq 0) {
        $Targets = @(Get-CurrentBuildTarget)
    }
}

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
