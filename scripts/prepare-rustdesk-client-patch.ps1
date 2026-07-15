param(
  [string]$SourceDir = ".upstream/rustdesk-client",
  [string]$RepoUrl = "https://github.com/rustdesk/rustdesk.git",
  [string]$Ref = "master"
)

$ErrorActionPreference = "Stop"
$root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$sourcePath = Join-Path $root $SourceDir
$patchPath = Join-Path $root "client/patches/rustdesk-client/0001-opendesk-buildspec-relay-grant.patch"

if (-not (Test-Path (Join-Path $sourcePath ".git"))) {
  New-Item -ItemType Directory -Force -Path (Split-Path $sourcePath -Parent) | Out-Null
  git clone --depth 1 $RepoUrl $sourcePath
}

git -C $sourcePath fetch --depth 1 origin $Ref
git -C $sourcePath checkout $Ref
git -C $sourcePath reset --hard "origin/$Ref"
git -C $sourcePath apply --check $patchPath
git -C $sourcePath apply $patchPath

Write-Host "OpenDesk BuildSpec/Relay Grant client patch applied to $sourcePath"
