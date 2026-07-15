package runners

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/opendesk-remote/opendesk-remote/builder/internal/buildspec"
	"github.com/opendesk-remote/opendesk-remote/builder/internal/injector"
	"github.com/opendesk-remote/opendesk-remote/builder/internal/platforms"
)

var (
	ErrPlatformDisabled = errors.New("platform is not enabled in BuildSpec")
	ErrRunnerNotReady   = errors.New("windows runner is not configured")
)

type WindowsConfig struct {
	SourceDir       string
	InjectionDir    string
	ArtifactDir     string
	BuildCommand    string
	ArtifactGlob    string
	DryRun          bool
	ToolSearchRoots []string
}

func RunWindows(ctx context.Context, spec buildspec.Spec, cfg WindowsConfig) (Result, error) {
	if !spec.Platforms.WindowsX64 {
		return Result{}, fmt.Errorf("%w: %s", ErrPlatformDisabled, platforms.WindowsX64)
	}
	if strings.TrimSpace(cfg.SourceDir) == "" {
		return Result{}, errors.New("windows runner source directory is required")
	}
	if strings.TrimSpace(cfg.InjectionDir) == "" {
		return Result{}, errors.New("windows runner injection directory is required")
	}
	if strings.TrimSpace(cfg.ArtifactDir) == "" {
		return Result{}, errors.New("windows runner artifact directory is required")
	}
	if err := os.MkdirAll(cfg.ArtifactDir, 0o755); err != nil {
		return Result{}, err
	}
	manifest, err := injector.Generate(spec, cfg.InjectionDir)
	if err != nil {
		return Result{}, err
	}
	result := Result{
		Platform:     platforms.WindowsX64,
		SourceDir:    cfg.SourceDir,
		InjectionDir: cfg.InjectionDir,
		ArtifactDir:  cfg.ArtifactDir,
		BuildCommand: cfg.BuildCommand,
	}
	for _, file := range manifest.Files {
		if strings.HasPrefix(file.Path, "rust/") {
			src := filepath.Join(cfg.InjectionDir, filepath.FromSlash(file.Path))
			dst := filepath.Join(cfg.SourceDir, filepath.FromSlash(strings.TrimPrefix(file.Path, "rust/")))
			if err := copyFile(src, dst); err != nil {
				return Result{}, err
			}
			result.StagedFiles = append(result.StagedFiles, dst)
		}
	}
	if cfg.DryRun {
		return result, nil
	}
	if strings.TrimSpace(cfg.BuildCommand) == "" {
		result.NotConfigured = true
		result.NotConfiguredReason = "set --build-command to compile patched RustDesk Windows client"
		return result, ErrRunnerNotReady
	}
	logPath := filepath.Join(cfg.ArtifactDir, "windows-runner.log")
	if err := runCommand(ctx, cfg.SourceDir, cfg.BuildCommand, logPath); err != nil {
		result.BuildLog = logPath
		return result, err
	}
	result.BuildExecuted = true
	result.BuildLog = logPath
	if strings.TrimSpace(cfg.ArtifactGlob) != "" {
		artifacts, err := collectArtifacts(cfg.SourceDir, cfg.ArtifactDir, cfg.ArtifactGlob)
		if err != nil {
			return result, err
		}
		result.Artifacts = artifacts
	}
	return result, nil
}

func copyFile(src, dst string) error {
	raw, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, raw, 0o644)
}

func runCommand(ctx context.Context, dir, command, logPath string) error {
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", command)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if writeErr := os.WriteFile(logPath, output, 0o644); writeErr != nil {
		return writeErr
	}
	if err != nil {
		return fmt.Errorf("windows build command failed: %w", err)
	}
	return nil
}

func collectArtifacts(sourceDir, artifactDir, pattern string) ([]Artifact, error) {
	glob := pattern
	if !filepath.IsAbs(glob) {
		glob = filepath.Join(sourceDir, filepath.FromSlash(pattern))
	}
	matches, err := filepath.Glob(glob)
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("artifact glob matched no files: %s", pattern)
	}
	var artifacts []Artifact
	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			continue
		}
		raw, err := os.ReadFile(match)
		if err != nil {
			return nil, err
		}
		target := filepath.Join(artifactDir, filepath.Base(match))
		if err := os.WriteFile(target, raw, 0o644); err != nil {
			return nil, err
		}
		sum := sha256.Sum256(raw)
		artifacts = append(artifacts, Artifact{
			Path:   target,
			SHA256: hex.EncodeToString(sum[:]),
			Bytes:  info.Size(),
		})
	}
	if len(artifacts) == 0 {
		return nil, fmt.Errorf("artifact glob matched only directories: %s", pattern)
	}
	return artifacts, nil
}
