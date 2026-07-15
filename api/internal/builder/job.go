package builder

import (
	"errors"
	"time"
)

type JobStatus string

const (
	JobQueued    JobStatus = "queued"
	JobRunning   JobStatus = "running"
	JobSucceeded JobStatus = "succeeded"
	JobFailed    JobStatus = "failed"
	JobCanceled  JobStatus = "canceled"
)

type BuildJob struct {
	ID           int64      `json:"id"`
	ProfileID    int64      `json:"profile_id"`
	Platform     string     `json:"platform"`
	Status       JobStatus  `json:"status"`
	Runner       string     `json:"runner"`
	LogPath      string     `json:"log_path"`
	ErrorMessage string     `json:"error_message,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	FinishedAt   *time.Time `json:"finished_at,omitempty"`
}

type Runner interface {
	Platform() string
	Run(spec BuildSpec, job BuildJob) (Artifact, error)
}

type Artifact struct {
	Platform  string `json:"platform"`
	Filename  string `json:"filename"`
	LocalPath string `json:"local_path"`
	SHA256    string `json:"sha256"`
	SizeBytes int64  `json:"size_bytes"`
}

type RunnerRegistry struct {
	runners map[string]Runner
}

func NewRunnerRegistry(runners ...Runner) RunnerRegistry {
	registry := RunnerRegistry{runners: map[string]Runner{}}
	for _, runner := range runners {
		registry.runners[runner.Platform()] = runner
	}
	return registry
}

func (r RunnerRegistry) Run(spec BuildSpec, job BuildJob) (BuildJob, *Artifact) {
	now := time.Now().UTC()
	job.StartedAt = &now
	job.Status = JobRunning
	runner, ok := r.runners[job.Platform]
	if !ok {
		finished := time.Now().UTC()
		job.Status = JobFailed
		job.FinishedAt = &finished
		job.ErrorMessage = "runner not configured for platform: " + job.Platform
		return job, nil
	}
	artifact, err := runner.Run(spec, job)
	finished := time.Now().UTC()
	job.FinishedAt = &finished
	if err != nil {
		job.Status = JobFailed
		job.ErrorMessage = err.Error()
		return job, nil
	}
	job.Status = JobSucceeded
	return job, &artifact
}

type NotConfiguredRunner struct {
	Name string
}

func (r NotConfiguredRunner) Platform() string {
	return r.Name
}

func (r NotConfiguredRunner) Run(BuildSpec, BuildJob) (Artifact, error) {
	return Artifact{}, errors.New("runner not configured for platform: " + r.Name)
}
