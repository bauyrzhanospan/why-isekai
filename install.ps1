# Install isekai on Windows from GitHub Releases.
#   irm https://raw.githubusercontent.com/bauyrzhanospan/why-isekai/main/install.ps1 | iex
# Options: $env:ISEKAI_VERSION = "0.1.0"; $env:ISEKAI_INSTALL_DIR = "$HOME\.local\bin"
$ErrorActionPreference = "Stop"
$repo = "bauyrzhanospan/why-isekai"
$version = $env:ISEKAI_VERSION
$dir = if ($env:ISEKAI_INSTALL_DIR) { $env:ISEKAI_INSTALL_DIR } else { Join-Path $HOME ".local\bin" }
$arch = if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }

if (-not $version) {
  $version = (Invoke-RestMethod "https://api.github.com/repos/$repo/releases/latest").tag_name
}
$version = $version.TrimStart("v")
$name = "isekai_${version}_windows_${arch}.zip"
$base = "https://github.com/$repo/releases/download/v$version"
$tmp = Join-Path ([IO.Path]::GetTempPath()) ([Guid]::NewGuid())
New-Item -ItemType Directory -Path $tmp | Out-Null
try {
  Write-Host "downloading $name"
  Invoke-WebRequest "$base/$name" -OutFile (Join-Path $tmp $name)
  Invoke-WebRequest "$base/checksums.txt" -OutFile (Join-Path $tmp "checksums.txt")
  $line = Get-Content (Join-Path $tmp "checksums.txt") | Where-Object { $_ -match " $([regex]::Escape($name))$" }
  if (-not $line) { throw "$name not listed in checksums.txt" }
  $want = ($line -split " ")[0]
  $got = (Get-FileHash (Join-Path $tmp $name) -Algorithm SHA256).Hash.ToLower()
  if ($want -ne $got) { throw "checksum mismatch for $name" }
  Expand-Archive (Join-Path $tmp $name) -DestinationPath (Join-Path $tmp "x")
  New-Item -ItemType Directory -Force -Path $dir | Out-Null
  Copy-Item (Join-Path $tmp "x\isekai.exe") (Join-Path $dir "isekai.exe") -Force
} finally {
  Remove-Item -Recurse -Force $tmp
}
Write-Host "installed $dir\isekai.exe"
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if (-not ($userPath -split ";" | Where-Object { $_ -eq $dir })) {
  [Environment]::SetEnvironmentVariable("Path", "$userPath;$dir", "User")
  Write-Host "added $dir to your user PATH (open a new terminal)"
}
Write-Host "next: cd <your repo>; isekai init"
