package runners

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckWindowsDryRunAcceptsValidSourceTree(t *testing.T) {
	sourceDir := windowsSourceFixture(t)
	report := CheckWindows(context.Background(), WindowsConfig{
		SourceDir: sourceDir,
		DryRun:    true,
	})
	if !report.Ready {
		t.Fatalf("expected dry-run preflight ready, got %+v", report.Checks)
	}
	if checkStatus(report, "rustdesk_source") != CheckPass {
		t.Fatalf("expected source marker pass, got %+v", report.Checks)
	}
	if checkRequired(report, "cargo") {
		t.Fatalf("cargo should be advisory for dry-run preflight: %+v", report.Checks)
	}
}

func TestCheckWindowsRealBuildReportsMissingConfiguration(t *testing.T) {
	sourceDir := windowsSourceFixture(t)
	report := CheckWindows(context.Background(), WindowsConfig{SourceDir: sourceDir})
	if report.Ready {
		t.Fatalf("expected real build preflight to fail without build config/tools: %+v", report.Checks)
	}
	if checkStatus(report, "build_command") != CheckFail {
		t.Fatalf("expected missing build command failure: %+v", report.Checks)
	}
	if checkStatus(report, "artifact_glob") != CheckFail {
		t.Fatalf("expected missing artifact glob failure: %+v", report.Checks)
	}
	if !checkRequired(report, "cargo") {
		t.Fatalf("cargo should be required for real build preflight: %+v", report.Checks)
	}
}

func TestCheckWindowsReportsMissingSourceMarkers(t *testing.T) {
	sourceDir := t.TempDir()
	report := CheckWindows(context.Background(), WindowsConfig{
		SourceDir: sourceDir,
		DryRun:    true,
	})
	if report.Ready {
		t.Fatalf("expected preflight to fail for incomplete source tree: %+v", report.Checks)
	}
	if checkStatus(report, "rustdesk_source") != CheckFail {
		t.Fatalf("expected source marker failure: %+v", report.Checks)
	}
}

func TestFindToolUsesProjectLocalTools(t *testing.T) {
	projectRoot := t.TempDir()
	sourceDir := filepath.Join(projectRoot, ".upstream", "rustdesk-client")
	toolDir := filepath.Join(projectRoot, ".tools", "cmake", "bin")
	if err := os.MkdirAll(toolDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(toolDir, "opendesk-test-tool.exe")
	if err := os.WriteFile(want, []byte(""), 0o755); err != nil {
		t.Fatal(err)
	}
	got, ok := findTool("opendesk-test-tool", WindowsConfig{SourceDir: sourceDir})
	if !ok {
		t.Fatalf("expected project-local tool to be found")
	}
	if filepath.Clean(got) != filepath.Clean(want) {
		t.Fatalf("expected %s, got %s", want, got)
	}
}

func TestAcceptsMSVCLinkHelpOutput(t *testing.T) {
	raw := []byte("Microsoft (R) Incremental Linker Version 14.44.35228.0\r\nCopyright (C) Microsoft Corporation.")
	if !acceptsNonZeroToolOutput("link", raw) {
		t.Fatalf("expected MSVC link help output to be accepted")
	}
	if acceptsNonZeroToolOutput("cmake", raw) {
		t.Fatalf("did not expect unrelated tool to accept MSVC link output")
	}
}

func TestAcceptsMSVCClHelpOutput(t *testing.T) {
	raw := []byte("Microsoft (R) C/C++ Optimizing Compiler Version 19.44.35228 for x64")
	if !acceptsNonZeroToolOutput("cl", raw) {
		t.Fatalf("expected MSVC cl help output to be accepted")
	}
	if acceptsNonZeroToolOutput("link", raw) {
		t.Fatalf("did not expect link to accept MSVC cl output")
	}
}

func windowsSourceFixture(t *testing.T) string {
	t.Helper()
	sourceDir := t.TempDir()
	for _, dir := range []string{
		"src",
		filepath.FromSlash("flutter"),
	} {
		if err := os.MkdirAll(filepath.Join(sourceDir, dir), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	for _, file := range []string{
		"Cargo.toml",
		"vcpkg.json",
		filepath.FromSlash("flutter/pubspec.yaml"),
	} {
		if err := os.WriteFile(filepath.Join(sourceDir, file), []byte("{}"), 0o644); err != nil {
			t.Fatalf("write %s: %v", file, err)
		}
	}
	return sourceDir
}

func checkStatus(report PreflightReport, name string) string {
	for _, check := range report.Checks {
		if check.Name == name {
			return check.Status
		}
	}
	return ""
}

func checkRequired(report PreflightReport, name string) bool {
	for _, check := range report.Checks {
		if check.Name == name {
			return check.Required
		}
	}
	return false
}
