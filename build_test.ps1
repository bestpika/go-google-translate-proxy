[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$buildScript = Join-Path $PSScriptRoot 'build.ps1'
$tokens = $null
$parseErrors = $null
$ast = [Management.Automation.Language.Parser]::ParseFile($buildScript, [ref]$tokens, [ref]$parseErrors)
if ($parseErrors.Count -gt 0) {
    throw "建置腳本語法錯誤：$parseErrors"
}
$functions = $ast.FindAll({ param($node) $node -is [Management.Automation.Language.FunctionDefinitionAst] }, $false)
foreach ($function in $functions) {
    Invoke-Expression $function.Extent.Text
}

function Assert-Rejected {
    param([scriptblock]$Action)
    $rejected = $false
    try { & $Action | Out-Null } catch { $rejected = $true }
    if (-not $rejected) {
        throw "應拒絕危險操作：$Action"
    }
}

$tempBase = [IO.Path]::GetTempPath()
if ($env:OPENCODE_TEST_TEMP) { $tempBase = $env:OPENCODE_TEST_TEMP }
$tempRoot = Join-Path $tempBase ("translate-build-test-" + [guid]::NewGuid().ToString('N'))
$root = Join-Path $tempRoot 'project'
$outside = Join-Path $tempRoot 'outside'
$links = [Collections.Generic.List[string]]::new()
New-Item -ItemType Directory -Path $root, $outside | Out-Null
try {
    foreach ($path in @($root, $tempRoot, [IO.Path]::GetPathRoot($root), $outside, "$root-extra", (Join-Path $root '.git'), (Join-Path $root '.github/workflows'), (Join-Path $root '.git.'), (Join-Path $root '.github /workflows'))) {
        Assert-Rejected { Assert-SafeBuildPath -Root $root -Path $path -Clean $true }
    }
    Assert-SafeBuildPath -Root $root -Path (Join-Path $root 'dist') -Clean $true
    Assert-Rejected { Assert-SafeBuildPath -Root $root -Path ([IO.Path]::GetPathRoot($root)) -Clean $false }
    Assert-SafeBuildPath -Root $root -Path (Join-Path $root 'dist[1]') -Clean $true
    Assert-SafeBuildPath -Root $root -Path $outside -Clean $false
    foreach ($name in @('', ' ', '.', '..', '../escape', 'escape\name', 'drive:name', 'wild*card')) {
        Assert-Rejected { Assert-BinaryName -Name $name }
    }
    Assert-BinaryName -Name 'go-google-translate-proxy'
    Assert-BinaryName -Name '翻譯服務'

    $link = Join-Path $root 'linked'
    $linkType = 'SymbolicLink'
    if ([IO.Path]::DirectorySeparatorChar -eq '\') { $linkType = 'Junction' }
    New-Item -ItemType $linkType -Path $link -Target $outside | Out-Null
    $links.Add($link)
    Assert-Rejected { Assert-SafeBuildPath -Root $root -Path (Join-Path $link 'child') -Clean $true }
    $dist = Join-Path $root 'dist'
    New-Item -ItemType Directory -Path $dist | Out-Null
    $nestedLink = Join-Path $dist 'nested-link'
    New-Item -ItemType $linkType -Path $nestedLink -Target $outside | Out-Null
    $links.Add($nestedLink)
    Assert-Rejected { Assert-SafeBuildPath -Root $root -Path $dist -Clean $true }

    $target = New-BuildTarget -GOOS darwin -GOARCH arm64 -GOARM 7
    if ($target.Label -ne 'macos-arm64' -or $target.GOARM -or $target.Extension) { throw 'macOS 目標錯誤。' }
    $target = New-BuildTarget -GOOS windows -GOARCH arm -GOARM 7
    if ($target.Label -ne 'windows-armv7' -or $target.GOARM -ne '7' -or $target.Extension -ne '.exe') { throw 'Windows ARM 目標錯誤。' }
    Assert-Rejected { & $buildScript -Clean -All -OutputDir '.' }
    Assert-Rejected { & $buildScript -All -BinaryName '../escape' }
    '建置路徑、連結、檔名與平台標籤測試通過。'
}
finally {
    foreach ($link in $links) { Remove-Item -LiteralPath $link -Force }
    Remove-Item -LiteralPath $tempRoot -Recurse -Force
}
