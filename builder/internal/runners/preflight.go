package runners

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/opendesk-remote/opendesk-remote/builder/internal/platforms"
)

const (
	CheckPass = "pass"
	CheckWarn = "warn"
	CheckFail = "fail"
)

type PreflightCheck struct {
	Name     string `json:"name"`
	Required bool   `json:"required"`
	Status   string `json:"status"`
	Detail   string `json:"detail"`
	Path     string `json:"path,omitempty"`
}

type PreflightReport struct {
	Platform  string           `json:"platform"`
	SourceDir string           `json:"source_dir"`
	Ready     bool             `json:"ready"`
	Checks    []PreflightCheck `json:"checks"`
}

func CheckWindows(ctx context.Context, cfg WindowsConfig) PreflightReport {
	report := PreflightReport{
		Platform:  platforms.WindowsX64,
		SourceDir: cfg.SourceDir,
		Ready:     true,
	}
	add := func(check PreflightCheck) {
		if check.Required && check.Status == CheckFail {
			report.Ready = false
		}
		report.Checks = append(report.Checks, check)
	}

	sourceDir := strings.TrimSpace(cfg.SourceDir)
	if sourceDir == "" {
		add(failCheck("source_dir", true, "set --source to the patched RustDesk client source directory", ""))
	} else if info, err := os.Stat(sourceDir); err != nil || !info.IsDir() {
		add(failCheck("source_dir", true, "source directory does not exist or is not a directory", sourceDir))
	} else {
		add(passCheck("source_dir", true, "source directory exists", sourceDir))
	}

	markers := []string{"Cargo.toml", "vcpkg.json", filepath.FromSlash("flutter/pubspec.yaml"), "src"}
	missingMarkers := missingSourceMarkers(sourceDir, markers)
	if len(missingMarkers) == 0 {
		add(passCheck("rustdesk_source", true, "RustDesk client source markers found", sourceDir))
	} else {
		add(failCheck("rustdesk_source", true, "missing source markers: "+strings.Join(missingMarkers, ", "), sourceDir))
	}

	if cfg.DryRun {
		add(passCheck("build_mode", true, "dry-run only stages OpenDesk generated files", ""))
	} else {
		if strings.TrimSpace(cfg.BuildCommand) == "" {
			add(failCheck("build_command", true, "set --build-command or OPENDESK_BUILDER_WINDOWS_COMMAND", ""))
		} else {
			add(passCheck("build_command", true, "build command is configured", ""))
		}
		if strings.TrimSpace(cfg.ArtifactGlob) == "" {
			add(failCheck("artifact_glob", true, "set --artifact-glob or OPENDESK_BUILDER_WINDOWS_ARTIFACT_GLOB so artifacts can be persisted", ""))
		} else {
			add(passCheck("artifact_glob", true, "artifact glob is configured", cfg.ArtifactGlob))
		}
	}

	toolRequired := !cfg.DryRun
	add(checkTool(ctx, cfg, "powershell", []string{"-NoProfile", "-Command", "$PSVersionTable.PSVersion.ToString()"}, true, "PowerShell is required to execute Windows build commands"))
	add(checkTool(ctx, cfg, "cargo", []string{"--version"}, toolRequired, "Rust cargo is required for real RustDesk Windows builds"))
	add(checkTool(ctx, cfg, "rustc", []string{"--version"}, toolRequired, "Rust compiler is required for real RustDesk Windows builds"))
	add(checkTool(ctx, cfg, "cmake", []string{"--version"}, toolRequired, "CMake is required by native RustDesk dependencies"))
	add(checkTool(ctx, cfg, "flutter", []string{"--version"}, toolRequired, "Flutter is required by the RustDesk desktop UI build"))
	add(checkTool(ctx, cfg, "cl", []string{"/?"}, toolRequired, "MSVC cl.exe is required; install Visual Studio Build Tools C++ workload"))
	add(checkTool(ctx, cfg, "link", []string{"/?"}, toolRequired, "MSVC link.exe is required; install Visual Studio Build Tools C++ workload"))

	return report
}

func missingSourceMarkers(sourceDir string, markers []string) []string {
	if strings.TrimSpace(sourceDir) == "" {
		return markers
	}
	missing := []string{}
	for _, marker := range markers {
		if _, err := os.Stat(filepath.Join(sourceDir, marker)); err != nil {
			missing = append(missing, filepath.ToSlash(marker))
		}
	}
	return missing
}

func checkTool(ctx context.Context, cfg WindowsConfig, name string, args []string, required bool, detail string) PreflightCheck {
	path, ok := findTool(name, cfg)
	if !ok {
		status := CheckWarn
		if required {
			status = CheckFail
		}
		return PreflightCheck{Name: name, Required: required, Status: status, Detail: detail + " (not found)"}
	}
	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(probeCtx, path, args...)
	cmd.Env = toolEnv(os.Environ(), path, cfg)
	raw, err := cmd.CombinedOutput()
	if err != nil {
		if acceptsNonZeroToolOutput(name, raw) {
			return PreflightCheck{Name: name, Required: required, Status: CheckPass, Detail: firstLine(strings.TrimSpace(string(raw))), Path: path}
		}
		status := CheckWarn
		if required {
			status = CheckFail
		}
		return PreflightCheck{Name: name, Required: required, Status: status, Detail: strings.TrimSpace(string(raw) + " " + err.Error()), Path: path}
	}
	output := firstLine(strings.TrimSpace(string(raw)))
	if output == "" {
		output = detail
	}
	return PreflightCheck{Name: name, Required: required, Status: CheckPass, Detail: output, Path: path}
}

func acceptsNonZeroToolOutput(name string, raw []byte) bool {
	output := strings.ToLower(string(raw))
	switch strings.ToLower(name) {
	case "cl":
		return strings.Contains(output, "microsoft") &&
			(strings.Contains(output, "c/c++") || strings.Contains(output, "compiler") || strings.Contains(output, "编译器"))
	case "link":
		return strings.Contains(output, "incremental linker") && strings.Contains(output, "microsoft")
	default:
		return false
	}
}

func findTool(name string, cfg WindowsConfig) (string, bool) {
	if path, err := exec.LookPath(name); err == nil {
		return path, true
	}
	for _, root := range toolSearchRoots(cfg) {
		for _, candidate := range toolNameCandidates(name) {
			path := filepath.Join(root, candidate)
			if info, err := os.Stat(path); err == nil && !info.IsDir() {
				return path, true
			}
		}
	}
	return "", false
}

func toolSearchRoots(cfg WindowsConfig) []string {
	roots := []string{}
	roots = append(roots, cfg.ToolSearchRoots...)
	if cwd, err := os.Getwd(); err == nil {
		roots = appendProjectToolRoots(roots, cwd)
		roots = appendProjectToolRoots(roots, filepath.Join(cwd, ".."))
	}
	if absSource, err := filepath.Abs(cfg.SourceDir); err == nil {
		projectRoot := filepath.Dir(filepath.Dir(absSource))
		roots = appendProjectToolRoots(roots, projectRoot)
	}
	if home, err := os.UserHomeDir(); err == nil {
		roots = append(roots,
			filepath.Join(home, "scoop", "shims"),
			filepath.Join(home, "scoop", "apps", "cmake", "current", "bin"),
			filepath.Join(home, "scoop", "apps", "flutter", "current", "bin"),
		)
	}
	roots = append(roots, visualStudioToolRoots()...)
	return dedupeStrings(roots)
}

func appendProjectToolRoots(roots []string, projectRoot string) []string {
	toolsRoot := filepath.Join(projectRoot, ".tools")
	roots = append(roots,
		filepath.Join(toolsRoot, "cargo", "bin"),
		filepath.Join(toolsRoot, "cmake", "bin"),
		filepath.Join(toolsRoot, "cmake", "current", "bin"),
		filepath.Join(toolsRoot, "flutter", "bin"),
		filepath.Join(toolsRoot, "flutter", "current", "bin"),
	)
	roots = append(roots, globDirs(filepath.Join(toolsRoot, "cmake", "*", "bin"))...)
	roots = append(roots, globDirs(filepath.Join(toolsRoot, "flutter", "*", "bin"))...)
	return roots
}

func visualStudioToolRoots() []string {
	roots := []string{}
	for _, base := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)")} {
		if strings.TrimSpace(base) == "" {
			continue
		}
		for _, year := range []string{"2022", "2019"} {
			for _, edition := range []string{"BuildTools", "Community", "Professional", "Enterprise"} {
				pattern := filepath.Join(base, "Microsoft Visual Studio", year, edition, "VC", "Tools", "MSVC", "*", "bin", "Hostx64", "x64")
				roots = append(roots, globDirs(pattern)...)
			}
		}
	}
	return roots
}

func globDirs(pattern string) []string {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil
	}
	dirs := []string{}
	for _, match := range matches {
		if info, err := os.Stat(match); err == nil && info.IsDir() {
			dirs = append(dirs, match)
		}
	}
	return dirs
}

func toolNameCandidates(name string) []string {
	if runtime.GOOS == "windows" {
		return []string{name + ".exe", name + ".cmd", name + ".bat", name}
	}
	return []string{name, name + ".exe", name + ".cmd", name + ".bat"}
}

func toolEnv(env []string, toolPath string, cfg WindowsConfig) []string {
	normalized := strings.ToLower(filepath.ToSlash(toolPath))
	if !strings.Contains(normalized, "/.tools/cargo/bin/") {
		return env
	}
	projectRoot := localProjectRootFromTool(toolPath, cfg)
	if projectRoot == "" {
		return env
	}
	return append(env,
		"CARGO_HOME="+filepath.Join(projectRoot, ".tools", "cargo"),
		"RUSTUP_HOME="+filepath.Join(projectRoot, ".tools", "rustup"),
	)
}

func localProjectRootFromTool(toolPath string, cfg WindowsConfig) string {
	if absSource, err := filepath.Abs(cfg.SourceDir); err == nil {
		root := filepath.Dir(filepath.Dir(absSource))
		if _, err := os.Stat(filepath.Join(root, ".tools", "rustup")); err == nil {
			return root
		}
	}
	lower := strings.ToLower(filepath.Clean(toolPath))
	needle := string(filepath.Separator) + ".tools" + string(filepath.Separator) + "cargo" + string(filepath.Separator) + "bin"
	if index := strings.LastIndex(lower, strings.ToLower(needle)); index > 0 {
		return toolPath[:index]
	}
	return ""
}

func passCheck(name string, required bool, detail string, path string) PreflightCheck {
	return PreflightCheck{Name: name, Required: required, Status: CheckPass, Detail: detail, Path: path}
}

func failCheck(name string, required bool, detail string, path string) PreflightCheck {
	return PreflightCheck{Name: name, Required: required, Status: CheckFail, Detail: detail, Path: path}
}

func firstLine(value string) string {
	if before, _, ok := strings.Cut(value, "\n"); ok {
		return strings.TrimSpace(before)
	}
	return strings.TrimSpace(value)
}

func dedupeStrings(values []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		cleaned := filepath.Clean(value)
		key := strings.ToLower(cleaned)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, cleaned)
	}
	return out
}
