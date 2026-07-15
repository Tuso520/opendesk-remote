package buildworker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/opendesk-remote/opendesk-remote/api/internal/config"
	"github.com/opendesk-remote/opendesk-remote/api/internal/models"
	"github.com/opendesk-remote/opendesk-remote/api/internal/repository"
)

var ErrNoQueuedJobs = errors.New("no queued build jobs")

type Store interface {
	repository.BuildRepository
}

type Config struct {
	RunnerName    string
	BuilderBinary string
	WorkDir       string
	SourceDir     string
	BuildCommand  string
	ArtifactGlob  string
	DryRun        bool
	Timeout       time.Duration
}

type Worker struct {
	store    Store
	cfg      Config
	executor Executor
	now      func() time.Time
}

type Executor interface {
	Run(ctx context.Context, command Command) (CommandResult, error)
}

type Command struct {
	Binary string
	Args   []string
}

type CommandResult struct {
	Stdout string
	Stderr string
}

type RunResult struct {
	Job       models.BuildJob        `json:"job"`
	Artifacts []models.BuildArtifact `json:"artifacts"`
	Message   string                 `json:"message,omitempty"`
}

type DoctorResult struct {
	Platform  string        `json:"platform"`
	SourceDir string        `json:"source_dir"`
	Ready     bool          `json:"ready"`
	Checks    []DoctorCheck `json:"checks"`
}

type DoctorCheck struct {
	Name     string `json:"name"`
	Required bool   `json:"required"`
	Status   string `json:"status"`
	Detail   string `json:"detail"`
	Path     string `json:"path,omitempty"`
}

type CLIExecutor struct{}

func New(store Store, cfg Config, executor Executor) *Worker {
	if strings.TrimSpace(cfg.RunnerName) == "" {
		cfg.RunnerName = "local-worker"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 2 * time.Hour
	}
	if executor == nil {
		executor = CLIExecutor{}
	}
	return &Worker{
		store:    store,
		cfg:      cfg,
		executor: executor,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func ConfigFromAPI(cfg config.Config) Config {
	return Config{
		RunnerName:    "local-worker",
		BuilderBinary: cfg.BuilderBinary,
		WorkDir:       cfg.BuilderWorkDir,
		SourceDir:     cfg.BuilderSourceDir,
		BuildCommand:  cfg.BuilderWindowsCommand,
		ArtifactGlob:  cfg.BuilderArtifactGlob,
		DryRun:        cfg.BuilderDryRun,
		Timeout:       cfg.BuilderTimeout,
	}
}

func (e CLIExecutor) Run(ctx context.Context, command Command) (CommandResult, error) {
	cmd := exec.CommandContext(ctx, command.Binary, command.Args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return CommandResult{Stdout: stdout.String(), Stderr: stderr.String()}, err
}

func (w *Worker) RunOnce(ctx context.Context) (RunResult, error) {
	job, err := w.store.ClaimNextBuildJob(ctx, w.cfg.RunnerName, w.now())
	if errors.Is(err, repository.ErrNotFound) {
		return RunResult{Message: ErrNoQueuedJobs.Error()}, ErrNoQueuedJobs
	}
	if err != nil {
		return RunResult{}, err
	}
	profile, err := w.store.FindBuildProfileByID(ctx, job.ProfileID)
	if err != nil {
		failed, _ := w.store.FailBuildJob(ctx, job.ID, "", "build profile not found", w.now())
		return RunResult{Job: failed}, err
	}
	result, err := w.runClaimed(ctx, job, profile)
	if err != nil {
		return result, err
	}
	return result, nil
}

func (w *Worker) Doctor(ctx context.Context) (DoctorResult, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, w.cfg.Timeout)
	defer cancel()
	command := Command{Binary: w.cfg.BuilderBinary, Args: w.doctorArgs()}
	output, runErr := w.executor.Run(timeoutCtx, command)
	result, parseErr := parseDoctorResult(output.Stdout)
	if parseErr == nil {
		return result, nil
	}
	if runErr != nil {
		message := strings.TrimSpace(output.Stderr)
		if message == "" {
			message = runErr.Error()
		}
		return DoctorResult{}, errors.New(message)
	}
	return DoctorResult{}, parseErr
}

func (w *Worker) runClaimed(ctx context.Context, job models.BuildJob, profile models.BuildProfile) (RunResult, error) {
	jobDir := filepath.Join(w.cfg.WorkDir, fmt.Sprintf("job-%d", job.ID))
	injectionDir := filepath.Join(jobDir, "injection")
	artifactDir := filepath.Join(jobDir, "artifacts")
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		return w.fail(ctx, job.ID, "", err)
	}
	specPath := filepath.Join(jobDir, "buildspec.json")
	spec, err := buildSpecJSON(profile)
	if err != nil {
		return w.fail(ctx, job.ID, "", err)
	}
	if err := os.WriteFile(specPath, spec, 0o644); err != nil {
		return w.fail(ctx, job.ID, "", err)
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, w.cfg.Timeout)
	defer cancel()
	command := Command{Binary: w.cfg.BuilderBinary, Args: w.args(job, specPath, injectionDir, artifactDir)}
	output, runErr := w.executor.Run(timeoutCtx, command)
	parsed, parseErr := parseRunnerResult(output.Stdout)
	if runErr != nil {
		message := strings.TrimSpace(output.Stderr)
		if message == "" {
			message = runErr.Error()
		}
		failed, _ := w.store.FailBuildJob(ctx, job.ID, parsed.BuildLog, message, w.now())
		return RunResult{Job: failed}, runErr
	}
	if parseErr != nil {
		return w.fail(ctx, job.ID, "", parseErr)
	}
	artifacts := []models.BuildArtifact{}
	for _, artifact := range parsed.Artifacts {
		created, err := w.store.CreateBuildArtifact(ctx, models.BuildArtifact{
			BuildJobID: job.ID,
			Platform:   job.Platform,
			Filename:   filepath.Base(artifact.Path),
			LocalPath:  artifact.Path,
			SHA256:     artifact.SHA256,
			SizeBytes:  artifact.Bytes,
		})
		if err != nil {
			return w.fail(ctx, job.ID, parsed.BuildLog, err)
		}
		artifacts = append(artifacts, created)
	}
	completed, err := w.store.CompleteBuildJob(ctx, job.ID, parsed.BuildLog, w.now())
	if err != nil {
		return RunResult{}, err
	}
	return RunResult{Job: completed, Artifacts: artifacts}, nil
}

func (w *Worker) doctorArgs() []string {
	args := []string{
		"doctor",
		"--platform", "windows_x64",
		"--source", w.cfg.SourceDir,
		"--timeout", w.cfg.Timeout.String(),
	}
	if w.cfg.DryRun {
		args = append(args, "--dry-run")
	}
	if strings.TrimSpace(w.cfg.BuildCommand) != "" {
		args = append(args, "--build-command", w.cfg.BuildCommand)
	}
	if strings.TrimSpace(w.cfg.ArtifactGlob) != "" {
		args = append(args, "--artifact-glob", w.cfg.ArtifactGlob)
	}
	return args
}

func (w *Worker) args(job models.BuildJob, specPath string, injectionDir string, artifactDir string) []string {
	args := []string{
		"run",
		"--platform", job.Platform,
		"--spec", specPath,
		"--source", w.cfg.SourceDir,
		"--injection", injectionDir,
		"--artifacts", artifactDir,
	}
	if w.cfg.DryRun {
		args = append(args, "--dry-run")
	}
	if strings.TrimSpace(w.cfg.BuildCommand) != "" {
		args = append(args, "--build-command", w.cfg.BuildCommand)
	}
	if strings.TrimSpace(w.cfg.ArtifactGlob) != "" {
		args = append(args, "--artifact-glob", w.cfg.ArtifactGlob)
	}
	args = append(args, "--timeout", w.cfg.Timeout.String())
	return args
}

func (w *Worker) fail(ctx context.Context, id int64, logPath string, err error) (RunResult, error) {
	failed, failErr := w.store.FailBuildJob(ctx, id, logPath, err.Error(), w.now())
	if failErr != nil {
		return RunResult{}, failErr
	}
	return RunResult{Job: failed}, err
}

type runnerResult struct {
	BuildLog  string `json:"build_log"`
	Artifacts []struct {
		Path   string `json:"path"`
		SHA256 string `json:"sha256"`
		Bytes  int64  `json:"bytes"`
	} `json:"artifacts"`
}

func parseRunnerResult(raw string) (runnerResult, error) {
	var result runnerResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return runnerResult{}, fmt.Errorf("parse builder output: %w", err)
	}
	return result, nil
}

func parseDoctorResult(raw string) (DoctorResult, error) {
	var result DoctorResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return DoctorResult{}, fmt.Errorf("parse builder doctor output: %w", err)
	}
	return result, nil
}
