param(
    [string]$SourceDir = "",
    [switch]$NoHwCodec,
    [switch]$SkipCargo,
    [switch]$PortablePack,
    [switch]$ProductionProfile
)

$ErrorActionPreference = "Stop"

function Resolve-RequiredPath {
    param(
        [Parameter(Mandatory = $true)][string]$Path,
        [Parameter(Mandatory = $true)][string]$Name
    )
    if (-not (Test-Path -LiteralPath $Path)) {
        throw "$Name not found: $Path"
    }
    return [System.IO.Path]::GetFullPath($Path)
}

function Repair-MozjpegSysNasmInclude {
    param(
        [Parameter(Mandatory = $true)][string]$CargoHome
    )

    $registrySrc = Join-Path $CargoHome "registry\src"
    if (-not (Test-Path -LiteralPath $registrySrc)) {
        return
    }

    $crates = Get-ChildItem -LiteralPath $registrySrc -Directory -Recurse -Filter "mozjpeg-sys-*" -ErrorAction SilentlyContinue
    foreach ($crate in $crates) {
        $buildRs = Join-Path $crate.FullName "src\build.rs"
        if (-not (Test-Path -LiteralPath $buildRs)) {
            continue
        }

        $text = [System.IO.File]::ReadAllText($buildRs).Replace("`r`n", "`n")
        $original = $text
        $asciiHelper = @'
fn opendesk_ascii_crate_path(root: &Path, relative: &Path) -> Option<PathBuf> {
    let ascii_repo = env::var_os("OPENDESK_ASCII_REPO")?;
    let registry_dir = root.parent()?.file_name()?.to_os_string();
    let crate_dir = root.file_name()?.to_os_string();
    Some(
        PathBuf::from(ascii_repo)
            .join(".tools")
            .join("cargo")
            .join("registry")
            .join("src")
            .join(registry_dir)
            .join(crate_dir)
            .join(relative),
    )
}

'@
        if (-not $text.Contains("fn opendesk_ascii_crate_path(")) {
            $text = $text.Replace("fn main() {", "$asciiHelper`nfn main() {")
        }

        $oldBlock = @'
    let simd_dir_for_nasm = opendesk_prefer_ascii_path(&simd_dir);
    let simd_arch_dir_for_nasm = simd_dir_for_nasm.join(arch_name);
    let vendor_dir_for_nasm = opendesk_prefer_ascii_path(vendor_dir);
'@
        $newBlock = @'
    let simd_relative = Path::new("vendor").join("simd");
    let simd_dir_for_nasm = opendesk_ascii_crate_path(root, &simd_relative)
        .unwrap_or_else(|| opendesk_prefer_ascii_path(&simd_dir));
    let simd_arch_dir_for_nasm = simd_dir_for_nasm.join(arch_name);
    let vendor_dir_for_nasm = opendesk_ascii_crate_path(root, Path::new("vendor"))
        .unwrap_or_else(|| opendesk_prefer_ascii_path(vendor_dir));
'@
        if ($text.Contains($oldBlock)) {
            $text = $text.Replace($oldBlock, $newBlock)
        }

        $needle = "    n.include(&simd_arch_dir);"
        $patch = "    n.include(&simd_dir);"
        if ($text.Contains($needle) -and -not $text.Contains($patch)) {
            $text = $text.Replace($needle, "$needle`n$patch")
        }

        if ($text -ne $original) {
            [System.IO.File]::WriteAllText($buildRs, $text.Replace("`n", "`r`n"), [System.Text.Encoding]::UTF8)
            Write-Host "Patched mozjpeg-sys NASM include path: $buildRs"
        }
    }
}

function Repair-HwcodecVcpkgInstalledRoot {
    param(
        [Parameter(Mandatory = $true)][string]$CargoHome
    )

    $checkoutRoot = Join-Path $CargoHome "git\checkouts"
    if (-not (Test-Path -LiteralPath $checkoutRoot)) {
        return
    }

    $buildScripts = Get-ChildItem -LiteralPath $checkoutRoot -Filter "build.rs" -Recurse -ErrorAction SilentlyContinue
    foreach ($buildRs in $buildScripts) {
        $text = [System.IO.File]::ReadAllText($buildRs.FullName).Replace("`r`n", "`n")
        if (-not ($text.Contains("fn link_vcpkg_installed_root") -and $text.Contains("fn build_ffmpeg"))) {
            continue
        }

        $original = $text
        $oldBlock = @'
        let mut target = if target_os == "windows" {
            format!("{}-windows-static", target_arch)
        } else {
            format!("{}-{}", target_arch, target_os)
        };
'@
        $newBlock = @'
        let mut target = std::env::var("OPENDESK_VCPKG_FFMPEG_TRIPLET").unwrap_or_else(|_| {
            if target_os == "windows" {
                format!("{}-windows-static", target_arch)
            } else {
                format!("{}-{}", target_arch, target_os)
            }
        });
'@
        if ($text.Contains($oldBlock)) {
            $text = $text.Replace($oldBlock, $newBlock)
        }

        if ($text -ne $original) {
            [System.IO.File]::WriteAllText($buildRs.FullName, $text.Replace("`n", "`r`n"), [System.Text.Encoding]::UTF8)
            Write-Host "Patched hwcodec vcpkg triplet selection: $($buildRs.FullName)"
        }
    }
}

function Repair-MagnumOpusVcpkgInstalledRoot {
    param(
        [Parameter(Mandatory = $true)][string]$CargoHome
    )

    $checkoutRoot = Join-Path $CargoHome "git\checkouts"
    if (-not (Test-Path -LiteralPath $checkoutRoot)) {
        return
    }

    $buildScripts = Get-ChildItem -LiteralPath $checkoutRoot -Filter "build.rs" -Recurse -ErrorAction SilentlyContinue
    foreach ($buildRs in $buildScripts) {
        $text = [System.IO.File]::ReadAllText($buildRs.FullName).Replace("`r`n", "`n")
        if (-not ($text.Contains("fn gen_opus") -and $text.Contains("fn link_vcpkg"))) {
            continue
        }

        $original = $text
        $oldTargetBlock = @'
    let mut target = if target_os == "macos" && target_arch == "x64" {
        "x64-osx".to_owned()
    } else if target_os == "macos" && target_arch == "arm64" {
        "arm64-osx".to_owned()
    } else if target_os == "windows" {
        format!("{}-windows-static", target_arch)
    } else {
        format!("{}-{}", target_arch, target_os)
    };
'@
        $newTargetBlock = @'
    let mut target = std::env::var("OPENDESK_VCPKG_OPUS_TRIPLET").unwrap_or_else(|_| {
        if target_os == "macos" && target_arch == "x64" {
            "x64-osx".to_owned()
        } else if target_os == "macos" && target_arch == "arm64" {
            "arm64-osx".to_owned()
        } else if target_os == "windows" {
            format!("{}-windows-static", target_arch)
        } else {
            format!("{}-{}", target_arch, target_os)
        }
    });
'@
        if ($text.Contains($oldTargetBlock)) {
            $text = $text.Replace($oldTargetBlock, $newTargetBlock)
        }

        $oldRootBlock = @'
    path.push("installed");
    path.push(target);
'@
        $newRootBlock = @'
    if let Some(installed_root) = std::env::var_os("VCPKG_INSTALLED_ROOT") {
        path = installed_root.into();
    } else {
        path.push("installed");
    }
    path.push(target);
'@
        if ($text.Contains($oldRootBlock)) {
            $text = $text.Replace($oldRootBlock, $newRootBlock)
        }

        if ($text -ne $original) {
            [System.IO.File]::WriteAllText($buildRs.FullName, $text.Replace("`n", "`r`n"), [System.Text.Encoding]::UTF8)
            Write-Host "Patched magnum-opus vcpkg installed root: $($buildRs.FullName)"
        }
    }
}

function Resolve-VcpkgMediaTriplet {
    param(
        [Parameter(Mandatory = $true)][string]$InstalledRoot
    )

    $candidates = @($env:VCPKG_DEFAULT_TRIPLET, "x64-windows-static", "x64-windows") |
        Where-Object { -not [string]::IsNullOrWhiteSpace($_) } |
        Select-Object -Unique
    foreach ($triplet in $candidates) {
        $include = Join-Path $InstalledRoot "$triplet\include\libavcodec\avcodec.h"
        $libCodec = Join-Path $InstalledRoot "$triplet\lib\avcodec.lib"
        $libFormat = Join-Path $InstalledRoot "$triplet\lib\avformat.lib"
        $libUtil = Join-Path $InstalledRoot "$triplet\lib\avutil.lib"
        if ((Test-Path -LiteralPath $include) -and
            (Test-Path -LiteralPath $libCodec) -and
            (Test-Path -LiteralPath $libFormat) -and
            (Test-Path -LiteralPath $libUtil)) {
            return $triplet
        }
    }

    throw "No usable FFmpeg vcpkg triplet found under $InstalledRoot"
}

function Resolve-VcpkgOpusTriplet {
    param(
        [Parameter(Mandatory = $true)][string]$InstalledRoot
    )

    $candidates = @($env:VCPKG_DEFAULT_TRIPLET, "x64-windows-static", "x64-windows") |
        Where-Object { -not [string]::IsNullOrWhiteSpace($_) } |
        Select-Object -Unique
    foreach ($triplet in $candidates) {
        $include = Join-Path $InstalledRoot "$triplet\include\opus\opus_multistream.h"
        $lib = Join-Path $InstalledRoot "$triplet\lib\opus.lib"
        if ((Test-Path -LiteralPath $include) -and (Test-Path -LiteralPath $lib)) {
            return $triplet
        }
    }

    throw "No usable Opus vcpkg triplet found under $InstalledRoot"
}

$repoRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))
$repoCanonRoot = (Resolve-Path -LiteralPath $repoRoot).Path
if ([string]::IsNullOrWhiteSpace($SourceDir)) {
    $SourceDir = Join-Path $repoRoot ".upstream\rustdesk-client"
}
$SourceDir = Resolve-RequiredPath $SourceDir "RustDesk source"

$toolRoot = Resolve-RequiredPath (Join-Path $repoRoot ".tools") "project tool root"
$env:RUSTUP_HOME = Resolve-RequiredPath (Join-Path $toolRoot "rustup") "RUSTUP_HOME"
$env:CARGO_HOME = Resolve-RequiredPath (Join-Path $toolRoot "cargo") "CARGO_HOME"

$vcvars = Resolve-RequiredPath "C:\Program Files (x86)\Microsoft Visual Studio\2022\BuildTools\VC\Auxiliary\Build\vcvarsall.bat" "vcvarsall.bat"
$pythonRoot = Resolve-RequiredPath "C:\Users\zhou\Tools\python" "Python"
$python = Resolve-RequiredPath (Join-Path $pythonRoot "python.exe") "python.exe"
$flutterBin = Resolve-RequiredPath "C:\Users\zhou\scoop\apps\flutter\current\bin" "Flutter bin"
$llvmBin = Resolve-RequiredPath "C:\Users\zhou\scoop\apps\llvm\current\bin" "LLVM bin"
$llvmLib = Resolve-RequiredPath "C:\Users\zhou\scoop\apps\llvm\current\lib" "LLVM lib"
$cmakeBin = Resolve-RequiredPath "C:\Users\zhou\scoop\apps\cmake\current\bin" "CMake bin"
$ninjaBin = Resolve-RequiredPath "C:\Users\zhou\scoop\apps\ninja\current" "Ninja bin"
$vcpkgRoot = Resolve-RequiredPath "C:\Users\zhou\scoop\apps\vcpkg\current" "vcpkg root"
$vcpkgInstalled = Resolve-RequiredPath "C:\Users\zhou\opendesk-remote-vcpkg" "vcpkg installed root"
$nasmDir = Resolve-RequiredPath "C:\Users\zhou\opendesk-remote-tools\nasm-2.16.03" "NASM"
$gitCmd = Resolve-RequiredPath "C:\Users\zhou\Tools\git\cmd" "Git cmd"

$vcvarsOutput = & cmd.exe /d /s /c "`"$vcvars`" x64 >nul && set"
if ($LASTEXITCODE -ne 0) {
    throw "vcvarsall.bat failed with exit code $LASTEXITCODE"
}
foreach ($line in $vcvarsOutput) {
    if ($line -match "^(.*?)=(.*)$") {
        [Environment]::SetEnvironmentVariable($matches[1], $matches[2], "Process")
    }
}

$pathEntries = @(
    (Join-Path $env:CARGO_HOME "bin"),
    $pythonRoot,
    $flutterBin,
    $llvmBin,
    $cmakeBin,
    $ninjaBin,
    (Resolve-RequiredPath "C:\Users\zhou\scoop\apps\7zip\current" "7zip"),
    $vcpkgRoot,
    $nasmDir,
    $gitCmd
)
$env:Path = (($pathEntries + ($env:Path -split ";")) | Where-Object { -not [string]::IsNullOrWhiteSpace($_) } | Select-Object -Unique) -join ";"

$env:VCPKG_ROOT = $vcpkgRoot
$env:VCPKG_INSTALLED_ROOT = $vcpkgInstalled
$env:VCPKG_DEFAULT_TRIPLET = "x64-windows"
$env:VCPKG_DEFAULT_HOST_TRIPLET = "x64-windows"
$env:OPENDESK_VCPKG_FFMPEG_TRIPLET = Resolve-VcpkgMediaTriplet -InstalledRoot $vcpkgInstalled
$env:OPENDESK_VCPKG_OPUS_TRIPLET = Resolve-VcpkgOpusTriplet -InstalledRoot $vcpkgInstalled
$env:VCPKG_DISABLE_METRICS = "1"
$env:VCPKG_FORCE_SYSTEM_BINARIES = "1"
$env:LIBCLANG_PATH = $llvmBin
$env:LLVM_LIB_DIR = $llvmLib
$env:PUB_CACHE = Join-Path $toolRoot "pub-cache"
$env:OPENDESK_NASM = Join-Path $nasmDir "nasm.exe"
$env:OPENDESK_ASCII_REPO = $repoRoot
$env:OPENDESK_CANON_REPO = $repoCanonRoot
$env:OPENDESK_HWCODEC_STATIC_CRT = "0"
$env:CARGO_BUILD_JOBS = "2"
$env:CARGO_NET_GIT_FETCH_WITH_CLI = "true"
$env:CARGO_NET_RETRY = "10"
$env:CARGO_HTTP_MULTIPLEXING = "false"
$env:CARGO_HTTP_TIMEOUT = "600"
$env:CARGO_HTTP_LOW_SPEED_LIMIT = "1"
$env:PYTHONUTF8 = "1"
if (-not $ProductionProfile) {
    $env:CARGO_PROFILE_RELEASE_LTO = "false"
    $env:CARGO_PROFILE_RELEASE_CODEGEN_UNITS = "16"
    $env:CARGO_PROFILE_RELEASE_INCREMENTAL = "false"
}

Repair-MozjpegSysNasmInclude -CargoHome $env:CARGO_HOME
Repair-HwcodecVcpkgInstalledRoot -CargoHome $env:CARGO_HOME
Repair-MagnumOpusVcpkgInstalledRoot -CargoHome $env:CARGO_HOME

Push-Location $SourceDir
try {
    & rustc --version
    & cargo --version
    & flutter --version
    & clang --version
    & $python --version

    $args = @("build.py", "--flutter")
    if (-not $NoHwCodec) {
        $args += "--hwcodec"
    }
    if ($SkipCargo) {
        $args += "--skip-cargo"
    }
    if (-not $PortablePack) {
        $args += "--skip-portable-pack"
    }

    & $python @args
    if ($LASTEXITCODE -ne 0) {
        throw "RustDesk Windows build failed with exit code $LASTEXITCODE"
    }
}
finally {
    Pop-Location
}
