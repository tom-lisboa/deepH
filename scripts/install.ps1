param(
  [string]$Version = "latest",
  [string]$Owner = $(if ($env:DEEPH_GITHUB_OWNER) { $env:DEEPH_GITHUB_OWNER } else { "tom-lisboa" }),
  [string]$Repo = $(if ($env:DEEPH_GITHUB_REPO) { $env:DEEPH_GITHUB_REPO } else { "deepH" }),
  [string]$InstallDir = $(if ($env:DEEPH_INSTALL_DIR) { $env:DEEPH_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA "Programs\deeph" })
)

$ErrorActionPreference = "Stop"

function Get-AssetName {
  $arch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLowerInvariant()
  switch ($arch) {
    "x64" { return "deeph-windows-amd64.exe" }
    "arm64" { return "deeph-windows-arm64.exe" }
    default { throw "Unsupported Windows architecture: $arch" }
  }
}

function Add-PathIfMissing([string]$PathToAdd) {
  $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
  $parts = @()
  if ($userPath) {
    $parts = $userPath.Split(";") | ForEach-Object { $_.Trim() } | Where-Object { $_ -ne "" }
  }
  if ($parts -contains $PathToAdd) {
    return
  }
  $newPath = if ($userPath) { "$userPath;$PathToAdd" } else { $PathToAdd }
  [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
  Write-Host "Added to user PATH: $PathToAdd"
}

$asset = Get-AssetName
$baseUrl = if ($Version -eq "latest") {
  "https://github.com/$Owner/$Repo/releases/latest/download"
} else {
  "https://github.com/$Owner/$Repo/releases/download/$Version"
}

$assetUrl = "$baseUrl/$asset"
$checksumsUrl = "$baseUrl/checksums.txt"

New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
$tmpDir = Join-Path ([System.IO.Path]::GetTempPath()) ("deeph-install-" + [Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $tmpDir -Force | Out-Null
$tmpAsset = Join-Path $tmpDir $asset
$tmpChecksums = Join-Path $tmpDir "checksums.txt"
$target = Join-Path $InstallDir "deeph.exe"

try {
  Write-Host "Downloading $assetUrl"
  Invoke-WebRequest -Uri $assetUrl -OutFile $tmpAsset -UseBasicParsing

  try {
    Invoke-WebRequest -Uri $checksumsUrl -OutFile $tmpChecksums -UseBasicParsing
    $checksumLine = Get-Content $tmpChecksums | Where-Object { $_ -match (" " + [Regex]::Escape($asset) + "$") } | Select-Object -First 1
    if ($checksumLine) {
      $expected = ($checksumLine -split "\s+")[0]
      $actual = (Get-FileHash -Algorithm SHA256 -Path $tmpAsset).Hash.ToLowerInvariant()
      if ($expected.ToLowerInvariant() -ne $actual) {
        throw "Checksum mismatch for $asset"
      }
    }
  } catch {
    Write-Host "Warning: checksum verification skipped ($($_.Exception.Message))"
  }

  Copy-Item -Path $tmpAsset -Destination $target -Force
  Add-PathIfMissing $InstallDir

  Write-Host ""
  Write-Host "Installed deeph at: $target"
  Write-Host "Open a new terminal and run: deeph"
} finally {
  if (Test-Path $tmpDir) {
    Remove-Item -Path $tmpDir -Recurse -Force
  }
}
