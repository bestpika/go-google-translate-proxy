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

function Assert-SafeBuildPath {
    param([string]$Root, [string]$Path, [bool]$Clean)

    $comparison = [StringComparison]::Ordinal
    if ([IO.Path]::DirectorySeparatorChar -eq '\') {
        $comparison = [StringComparison]::OrdinalIgnoreCase
    }
    $rootPath = [IO.Path]::GetFullPath($Root).TrimEnd([IO.Path]::DirectorySeparatorChar, [IO.Path]::AltDirectorySeparatorChar)
    $outputPath = [IO.Path]::GetFullPath($Path).TrimEnd([IO.Path]::DirectorySeparatorChar, [IO.Path]::AltDirectorySeparatorChar)
    $volumeRoot = [IO.Path]::GetPathRoot([IO.Path]::GetFullPath($Path)).TrimEnd([IO.Path]::DirectorySeparatorChar, [IO.Path]::AltDirectorySeparatorChar)
    $separator = [IO.Path]::DirectorySeparatorChar
    if ($outputPath.Equals($volumeRoot, $comparison) -or $outputPath.Equals($rootPath, $comparison) -or $rootPath.StartsWith($outputPath + $separator, $comparison)) {
        throw '輸出目錄不可為檔案系統根目錄、專案根目錄或其上層目錄。'
    }
    $insideProject = $outputPath.StartsWith($rootPath + $separator, $comparison)
    if ($Clean -and -not $insideProject) {
        throw '-Clean 僅允許清理專案內的輸出子目錄。'
    }
    if ($insideProject) {
        $relative = $outputPath.Substring($rootPath.Length + 1)
        if (($relative -split '[\\/]') | Where-Object { $_.TrimEnd(' ', '.') -in @('.git', '.github') }) {
            throw '輸出目錄不可包含版本庫或工作流程資料。'
        }
    }
    $ancestor = $outputPath
    while ($ancestor) {
        if (Test-Path -LiteralPath $ancestor) {
            $item = Get-Item -LiteralPath $ancestor -Force
            if ($item.Name -in @('.git', '.github')) {
                throw '輸出目錄不可位於版本庫或工作流程資料內。'
            }
            if (($item.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
                throw '輸出目錄及其上層不可包含符號連結或目錄連接。'
            }
            if (-not $item.PSIsContainer) {
                throw '輸出路徑及其上層必須為目錄。'
            }
        }
        $ancestor = [IO.Path]::GetDirectoryName($ancestor)
    }
    if ($Clean -and (Test-Path -LiteralPath $outputPath)) {
        $pending = [Collections.Generic.Stack[string]]::new()
        $pending.Push($outputPath)
        while ($pending.Count -gt 0) {
            foreach ($item in Get-ChildItem -LiteralPath $pending.Pop() -Force) {
                if (($item.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0 -or $item.Name -in @('.git', '.github')) {
                    throw '清理目錄內不可包含連結或版本庫資料。'
                }
                if ($item.PSIsContainer) {
                    $pending.Push($item.FullName)
                }
            }
        }
    }
}

function Assert-BinaryName {
    param([string]$Name)

    if ([string]::IsNullOrWhiteSpace($Name) -or $Name -in @('.', '..') -or $Name -match '[\\/:*?"<>|\x00-\x1f]') {
        throw 'BinaryName 必須是有效的檔名，不可包含路徑或萬用字元。'
    }
}

if ([string]::IsNullOrWhiteSpace($OutputDir)) {
    throw 'OutputDir 不可為空白。'
}
Assert-BinaryName -Name $BinaryName
if ([IO.Path]::IsPathRooted($OutputDir)) {
    $OutputPath = [IO.Path]::GetFullPath($OutputDir)
}
else {
    $OutputPath = [IO.Path]::GetFullPath((Join-Path -Path $ProjectRoot -ChildPath $OutputDir))
}
Assert-SafeBuildPath -Root $ProjectRoot -Path $OutputPath -Clean $Clean.IsPresent

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
    New-BuildTarget -GOOS windows -GOARCH 386
    New-BuildTarget -GOOS windows -GOARCH amd64
    New-BuildTarget -GOOS windows -GOARCH arm -GOARM 7
    New-BuildTarget -GOOS windows -GOARCH arm64
    New-BuildTarget -GOOS linux -GOARCH 386
    New-BuildTarget -GOOS linux -GOARCH amd64
    New-BuildTarget -GOOS linux -GOARCH arm -GOARM 5
    New-BuildTarget -GOOS linux -GOARCH arm -GOARM 6
    New-BuildTarget -GOOS linux -GOARCH arm -GOARM 7
    New-BuildTarget -GOOS linux -GOARCH arm64
    New-BuildTarget -GOOS darwin -GOARCH amd64
    New-BuildTarget -GOOS darwin -GOARCH arm64
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

Push-Location -LiteralPath $ProjectRoot
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
    Pop-Location
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
