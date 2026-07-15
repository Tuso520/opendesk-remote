package runners

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opendesk-remote/opendesk-remote/builder/internal/buildspec"
)

func TestRunWindowsDryRunStagesGeneratedRustConfig(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "rustdesk")
	injectionDir := filepath.Join(root, "injection")
	artifactDir := filepath.Join(root, "artifacts")
	if err := os.MkdirAll(filepath.Join(sourceDir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}

	result, err := RunWindows(context.Background(), validWindowsSpec(), WindowsConfig{
		SourceDir:    sourceDir,
		InjectionDir: injectionDir,
		ArtifactDir:  artifactDir,
		DryRun:       true,
	})
	if err != nil {
		t.Fatalf("dry-run failed: %v", err)
	}
	if result.BuildExecuted {
		t.Fatal("dry-run should not execute build")
	}
	if len(result.StagedFiles) != 1 {
		t.Fatalf("expected one staged Rust file, got %+v", result.StagedFiles)
	}
	raw, err := os.ReadFile(filepath.Join(sourceDir, "src", "opendesk_generated.rs"))
	if err != nil {
		t.Fatalf("read staged generated config: %v", err)
	}
	if !strings.Contains(string(raw), "OPENDESK_RELAY_GRANT_REQUIRED: bool = true") {
		t.Fatalf("staged config missing relay grant flag:\n%s", string(raw))
	}
}

func TestRunWindowsReturnsClearNotConfiguredError(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "rustdesk")
	if err := os.MkdirAll(filepath.Join(sourceDir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}

	result, err := RunWindows(context.Background(), validWindowsSpec(), WindowsConfig{
		SourceDir:    sourceDir,
		InjectionDir: filepath.Join(root, "injection"),
		ArtifactDir:  filepath.Join(root, "artifacts"),
	})
	if !errors.Is(err, ErrRunnerNotReady) {
		t.Fatalf("expected ErrRunnerNotReady, got result=%+v err=%v", result, err)
	}
	if !result.NotConfigured || result.NotConfiguredReason == "" {
		t.Fatalf("expected clear not configured result, got %+v", result)
	}
}

func TestRunWindowsCollectsArtifactsAfterCommand(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "rustdesk")
	if err := os.MkdirAll(filepath.Join(sourceDir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	result, err := RunWindows(context.Background(), validWindowsSpec(), WindowsConfig{
		SourceDir:    sourceDir,
		InjectionDir: filepath.Join(root, "injection"),
		ArtifactDir:  filepath.Join(root, "artifacts"),
		BuildCommand: "New-Item -ItemType Directory -Force -Path target/release | Out-Null; Set-Content -Path target/release/OpenDeskRemote.exe -Value runner-smoke",
		ArtifactGlob: filepath.FromSlash("target/release/*.exe"),
	})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if !result.BuildExecuted {
		t.Fatal("expected build command to execute")
	}
	if len(result.Artifacts) != 1 {
		t.Fatalf("expected one artifact, got %+v", result.Artifacts)
	}
	if result.Artifacts[0].SHA256 == "" || result.Artifacts[0].Bytes == 0 {
		t.Fatalf("artifact metadata incomplete: %+v", result.Artifacts[0])
	}
}

func TestRunWindowsRejectsDisabledPlatform(t *testing.T) {
	spec := validWindowsSpec()
	spec.Platforms.WindowsX64 = false
	_, err := RunWindows(context.Background(), spec, WindowsConfig{
		SourceDir:    t.TempDir(),
		InjectionDir: filepath.Join(t.TempDir(), "injection"),
		ArtifactDir:  filepath.Join(t.TempDir(), "artifacts"),
		DryRun:       true,
	})
	if !errors.Is(err, ErrPlatformDisabled) {
		t.Fatalf("expected disabled platform error, got %v", err)
	}
}

func validWindowsSpec() buildspec.Spec {
	return buildspec.Spec{
		App: buildspec.AppSpec{
			Name:               "OpenDesk Remote",
			Vendor:             "OpenDesk",
			BundleID:           "com.example.opendeskremote",
			WindowsProductName: "OpenDesk Remote",
		},
		Server: buildspec.ServerSpec{
			IDServer:    "remote.example.com:21116",
			RelayServer: "remote.example.com:21117",
			RelayName:   "hbbr-relay-a",
			APIServer:   "https://remote.example.com",
			Key:         "PUBLIC_KEY",
			WebSocket:   true,
		},
		Policy: buildspec.PolicySpec{
			Profile: "default-secure",
			OverrideSettings: map[string]string{
				"enable-terminal": "N",
			},
		},
		Platforms: buildspec.PlatformSpec{WindowsX64: true},
		Source:    buildspec.SourceSpec{OpenDeskPatchset: "opendesk-remote-m4"},
	}
}
