package buildworker

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/opendesk-remote/opendesk-remote/api/internal/models"
	"github.com/opendesk-remote/opendesk-remote/api/internal/repository"
)

type fakeExecutor struct {
	command Command
	output  string
	stderr  string
	err     error
}

func (f *fakeExecutor) Run(ctx context.Context, command Command) (CommandResult, error) {
	f.command = command
	return CommandResult{Stdout: f.output, Stderr: f.stderr}, f.err
}

func TestWorkerRunOnceCompletesQueuedJob(t *testing.T) {
	store := repository.NewMemory()
	profile, err := store.CreateBuildProfile(context.Background(), models.BuildProfile{
		Name:             "Default Windows Profile",
		AppName:          "OpenDesk Remote",
		Vendor:           "OpenDesk",
		BundleID:         "com.example.opendeskremote",
		ProductName:      "OpenDesk Remote",
		ServerConfigJSON: `{"id_server":"remote.example.com:21116","relay_server":"remote.example.com:21117","relay_name":"hbbr-relay-a","api_server":"https://remote.example.com","key":"PUBLIC_KEY","websocket":true,"relay_grant_required":true}`,
		BrandingJSON:     `{}`,
		PolicyJSON:       `{"profile":"default-secure","override_settings":{},"default_settings":{}}`,
		PlatformsJSON:    `{"windows_x64":true}`,
		SigningJSON:      `{"windows":"unsigned-dev"}`,
		SourceJSON:       `{"rustdesk_ref":"master","opendesk_patchset":"opendesk-remote-m4"}`,
		CreatedBy:        1,
	})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}
	job, err := store.CreateBuildJob(context.Background(), models.BuildJob{ProfileID: profile.ID, Platform: "windows_x64", Status: models.BuildJobQueued})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
	executor := &fakeExecutor{output: `{"build_log":"C:\\build\\windows-runner.log","artifacts":[{"path":"C:\\build\\OpenDeskRemote.exe","sha256":"abc123","bytes":42}]}`}
	worker := New(store, Config{
		RunnerName:    "test-worker",
		BuilderBinary: "opendesk-builder",
		WorkDir:       t.TempDir(),
		SourceDir:     t.TempDir(),
		DryRun:        true,
		Timeout:       time.Minute,
	}, executor)

	result, err := worker.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("run once: %v", err)
	}
	if result.Job.ID != job.ID || result.Job.Status != models.BuildJobSucceeded {
		t.Fatalf("unexpected job: %+v", result.Job)
	}
	if result.Job.Runner != "test-worker" {
		t.Fatalf("expected runner to be recorded, got %+v", result.Job)
	}
	if len(result.Artifacts) != 1 || result.Artifacts[0].SHA256 != "abc123" {
		t.Fatalf("unexpected artifacts: %+v", result.Artifacts)
	}
	if executor.command.Binary != "opendesk-builder" {
		t.Fatalf("unexpected command: %+v", executor.command)
	}
	args := strings.Join(executor.command.Args, " ")
	if !strings.Contains(args, "--platform windows_x64") || !strings.Contains(args, "--dry-run") {
		t.Fatalf("unexpected args: %s", args)
	}
}

func TestWorkerRunOnceReportsNoQueuedJobs(t *testing.T) {
	store := repository.NewMemory()
	worker := New(store, Config{BuilderBinary: "opendesk-builder", WorkDir: t.TempDir(), SourceDir: t.TempDir(), DryRun: true}, &fakeExecutor{})
	result, err := worker.RunOnce(context.Background())
	if err != ErrNoQueuedJobs {
		t.Fatalf("expected no queued jobs, got result=%+v err=%v", result, err)
	}
}

func TestWorkerRunOnceFailsJobWithBuilderStderr(t *testing.T) {
	store := repository.NewMemory()
	profile, err := store.CreateBuildProfile(context.Background(), models.BuildProfile{
		Name:             "Default Windows Profile",
		AppName:          "OpenDesk Remote",
		BundleID:         "com.example.opendeskremote",
		ServerConfigJSON: `{"id_server":"remote.example.com:21116","relay_server":"remote.example.com:21117","relay_name":"hbbr-relay-a","key":"PUBLIC_KEY"}`,
		BrandingJSON:     `{}`,
		PolicyJSON:       `{}`,
		PlatformsJSON:    `{"windows_x64":true}`,
		SigningJSON:      `{}`,
		SourceJSON:       `{"rustdesk_ref":"master"}`,
		CreatedBy:        1,
	})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}
	_, err = store.CreateBuildJob(context.Background(), models.BuildJob{ProfileID: profile.ID, Platform: "windows_x64", Status: models.BuildJobQueued})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
	executor := &fakeExecutor{stderr: "builder exploded", err: errors.New("exit status 1")}
	worker := New(store, Config{
		RunnerName:    "test-worker",
		BuilderBinary: "opendesk-builder",
		WorkDir:       t.TempDir(),
		SourceDir:     t.TempDir(),
		DryRun:        true,
		Timeout:       time.Minute,
	}, executor)

	result, err := worker.RunOnce(context.Background())
	if err == nil {
		t.Fatalf("expected builder error, got result=%+v", result)
	}
	if result.Job.Status != models.BuildJobFailed {
		t.Fatalf("expected failed job, got %+v", result.Job)
	}
	if result.Job.ErrorMessage != "builder exploded" {
		t.Fatalf("expected stderr error message, got %+v", result.Job)
	}
}

func TestWorkerDoctorReturnsParsedNotReadyReport(t *testing.T) {
	store := repository.NewMemory()
	executor := &fakeExecutor{
		output: `{"platform":"windows_x64","source_dir":".upstream/rustdesk-client","ready":false,"checks":[{"name":"cl","required":true,"status":"fail","detail":"MSVC cl.exe is required"}]}`,
		err:    errors.New("exit status 1"),
	}
	worker := New(store, Config{
		BuilderBinary: "opendesk-builder",
		WorkDir:       t.TempDir(),
		SourceDir:     ".upstream/rustdesk-client",
		DryRun:        false,
		Timeout:       time.Minute,
	}, executor)

	report, err := worker.Doctor(context.Background())
	if err != nil {
		t.Fatalf("doctor should return parsed report, got %v", err)
	}
	if report.Ready || len(report.Checks) != 1 || report.Checks[0].Name != "cl" {
		t.Fatalf("unexpected doctor report: %+v", report)
	}
	args := strings.Join(executor.command.Args, " ")
	if !strings.Contains(args, "doctor") || strings.Contains(args, "--dry-run") {
		t.Fatalf("unexpected doctor args: %s", args)
	}
}
