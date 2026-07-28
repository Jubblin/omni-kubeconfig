# Install omni-kubeconfig from GitHub releases (checksum-verified).
# Usage: irm .../install.ps1 | iex
# Env: VERSION, INSTALL_DIR, GITHUB_REPO, GITHUB_API_URL, GITHUB_DOWNLOAD_URL, SKIP_SHA256=1

$ErrorActionPreference = "Stop"

$Repo = if ($env:GITHUB_REPO) { $env:GITHUB_REPO } else { "Jubblin/omni-kubeconfig" }
$ApiBase = if ($env:GITHUB_API_URL) { $env:GITHUB_API_URL } else { "https://api.github.com" }
$DownloadBase = if ($env:GITHUB_DOWNLOAD_URL) { $env:GITHUB_DOWNLOAD_URL } else { "https://github.com/$Repo/releases/download" }
$ProjectName = "omni-kubeconfig"

function Get-PlatformAsset {
    if ($env:PROCESSOR_ARCHITECTURE -notmatch "64") {
        throw "unsupported architecture: $env:PROCESSOR_ARCHITECTURE"
    }
    return "$ProjectName-windows-amd64.exe"
}

function Get-InstallDir {
    if ($env:INSTALL_DIR) { return $env:INSTALL_DIR }
    $base = $env:LOCALAPPDATA
    if (-not $base) { throw "LOCALAPPDATA is not set" }
    return Join-Path $base "Programs\omni-kubeconfig"
}

function Resolve-Version {
    if ($env:VERSION) {
        $v = $env:VERSION
        if (-not $v.StartsWith("v")) { $v = "v$v" }
        return $v
    }
    $uri = "$ApiBase/repos/$Repo/releases/latest"
    $resp = Invoke-RestMethod -Uri $uri -Headers @{ Accept = "application/vnd.github+json" }
    if (-not $resp.tag_name) { throw "could not resolve latest release tag" }
    return $resp.tag_name
}

function Get-ExpectedSHA256 {
    param([string]$SumsPath, [string]$AssetName)
    foreach ($line in Get-Content $SumsPath) {
        $parts = $line.Trim() -split "\s+"
        if ($parts.Length -lt 2) { continue }
        $name = $parts[-1].TrimStart("*")
        if ($name -eq $AssetName) { return $parts[0] }
    }
    throw "checksum for $AssetName not found"
}

$asset = Get-PlatformAsset
$tag = Resolve-Version
$installDir = Get-InstallDir
New-Item -ItemType Directory -Force -Path $installDir | Out-Null

$tmp = New-TemporaryFile
$tmpBin = Join-Path ([System.IO.Path]::GetTempPath()) "$ProjectName-$tag.exe"
try {
    $sumsUrl = "$DownloadBase/$tag/sha256sum.txt"
    $binUrl = "$DownloadBase/$tag/$asset"
    $sumsPath = "$tmp.FullName.sums"
    Invoke-WebRequest -Uri $sumsUrl -OutFile $sumsPath
    Invoke-WebRequest -Uri $binUrl -OutFile $tmpBin

    if ($env:SKIP_SHA256 -ne "1") {
        $expected = Get-ExpectedSHA256 -SumsPath $sumsPath -AssetName $asset
        $hash = (Get-FileHash -Path $tmpBin -Algorithm SHA256).Hash.ToLower()
        if ($hash -ne $expected.ToLower()) {
            throw "checksum mismatch for $asset"
        }
    }

    $target = Join-Path $installDir "$ProjectName.exe"
    Copy-Item -Force $tmpBin $target
    Write-Host "Installed omni-kubeconfig $tag to $target"
    $pathParts = $env:PATH -split ";"
    if ($pathParts -notcontains $installDir) {
        Write-Host "Add to PATH: $installDir"
    }
}
finally {
    Remove-Item -Force -ErrorAction SilentlyContinue $tmp, $tmpBin, "$tmp.FullName.sums"
}
