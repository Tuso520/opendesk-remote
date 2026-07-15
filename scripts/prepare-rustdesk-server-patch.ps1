param(
  [string]$SourceDir = ".upstream/rustdesk-server",
  [string]$RepoUrl = "https://github.com/rustdesk/rustdesk-server.git",
  [string]$Ref = "master"
)

$ErrorActionPreference = "Stop"
$root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$sourcePath = Join-Path $root $SourceDir
$patchPath = Join-Path $root "server/patches/rustdesk-server/0001-opendesk-relay-auth.patch"

if (-not (Test-Path (Join-Path $sourcePath ".git"))) {
  New-Item -ItemType Directory -Force -Path (Split-Path $sourcePath -Parent) | Out-Null
  git clone --depth 1 --recurse-submodules $RepoUrl $sourcePath
}

git -C $sourcePath fetch --depth 1 origin $Ref
git -C $sourcePath checkout $Ref
git -C $sourcePath reset --hard "origin/$Ref"
git -C $sourcePath submodule update --init --depth 1 libs/hbb_common
git -C $sourcePath apply --check $patchPath
git -C $sourcePath apply $patchPath

Write-Host "OpenDesk Relay Auth patch applied to $sourcePath"
