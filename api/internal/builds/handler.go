package builds

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/opendesk-remote/opendesk-remote/api/internal/audit"
	apiauth "github.com/opendesk-remote/opendesk-remote/api/internal/auth"
	apibuilder "github.com/opendesk-remote/opendesk-remote/api/internal/builder"
	"github.com/opendesk-remote/opendesk-remote/api/internal/buildworker"
	"github.com/opendesk-remote/opendesk-remote/api/internal/httpx"
	"github.com/opendesk-remote/opendesk-remote/api/internal/models"
	"github.com/opendesk-remote/opendesk-remote/api/internal/repository"
)

var supportedPlatforms = []string{"windows_x64", "macos_x64", "macos_arm64", "android_arm64", "ios_arm64"}

type Handler struct {
	repo   Repository
	worker *buildworker.Worker
}

type Repository interface {
	repository.BuildRepository
	audit.Repository
}

type CreateProfileRequest struct {
	Name      string               `json:"name"`
	BuildSpec apibuilder.BuildSpec `json:"build_spec"`
}

type CreateJobRequest struct {
	ProfileID int64  `json:"profile_id"`
	Platform  string `json:"platform"`
}

func NewHandler(repo Repository, worker *buildworker.Worker) Handler {
	return Handler{repo: repo, worker: worker}
}

func (h Handler) Profiles(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		profiles, err := h.repo.ListBuildProfiles(r.Context())
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, "LIST_BUILD_PROFILES_FAILED", "list build profiles failed")
			return
		}
		if profiles == nil {
			profiles = []models.BuildProfile{}
		}
		httpx.JSON(w, http.StatusOK, profiles)
	case http.MethodPost:
		var req CreateProfileRequest
		if err := httpx.DecodeJSON(r, &req); err != nil {
			httpx.Error(w, http.StatusBadRequest, "INVALID_JSON", "invalid JSON body")
			return
		}
		if strings.TrimSpace(req.Name) == "" {
			req.Name = req.BuildSpec.App.Name
		}
		if err := req.BuildSpec.Validate(); err != nil {
			httpx.Error(w, http.StatusBadRequest, "INVALID_BUILD_SPEC", err.Error())
			return
		}
		session, ok := apiauth.SessionFromContext(r.Context())
		if !ok {
			httpx.Error(w, http.StatusUnauthorized, "UNAUTHENTICATED", "authentication is required")
			return
		}
		profile, err := profileFromSpec(req.Name, req.BuildSpec, session.User.ID)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "INVALID_BUILD_SPEC", err.Error())
			return
		}
		created, err := h.repo.CreateBuildProfile(r.Context(), profile)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "INVALID_BUILD_PROFILE", err.Error())
			return
		}
		if err := h.writeAudit(r, audit.Event{
			Action:       "create_build_profile",
			ResourceType: "build_profile",
			ResourceID:   strconv.FormatInt(created.ID, 10),
			Metadata: map[string]any{
				"name":     created.Name,
				"app_name": created.AppName,
				"vendor":   created.Vendor,
			},
		}); err != nil {
			httpx.Error(w, http.StatusInternalServerError, "AUDIT_WRITE_FAILED", "failed to record build profile creation")
			return
		}
		httpx.JSON(w, http.StatusCreated, created)
	default:
		httpx.Error(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	}
}

func (h Handler) ProfileItem(w http.ResponseWriter, r *http.Request) {
	prefix := "/api/v1/build-profiles/"
	if strings.HasPrefix(r.URL.Path, "/api/v1/client-build-configs/") {
		prefix = "/api/v1/client-build-configs/"
	}
	parts, ok := pathParts(r.URL.Path, prefix)
	if !ok {
		httpx.Error(w, http.StatusNotFound, "NOT_FOUND", "build profile endpoint not found")
		return
	}
	id, ok := parsePositiveID(parts[0])
	if !ok {
		httpx.Error(w, http.StatusBadRequest, "INVALID_BUILD_PROFILE", "invalid build profile id")
		return
	}
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			h.getProfile(w, r, id)
		case http.MethodPut:
			h.updateProfile(w, r, id)
		case http.MethodDelete:
			h.deleteProfile(w, r, id)
		default:
			httpx.Error(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		}
		return
	}
	if len(parts) == 2 && parts[1] == "jobs" {
		if !httpx.Method(w, r, http.MethodPost) {
			return
		}
		h.createProfileJob(w, r, id)
		return
	}
	httpx.Error(w, http.StatusNotFound, "NOT_FOUND", "build profile endpoint not found")
}

func (h Handler) getProfile(w http.ResponseWriter, r *http.Request, id int64) {
	profile, err := h.repo.FindBuildProfileByID(r.Context(), id)
	if errors.Is(err, repository.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "BUILD_PROFILE_NOT_FOUND", "build profile not found")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "LOAD_BUILD_PROFILE_FAILED", "load build profile failed")
		return
	}
	httpx.JSON(w, http.StatusOK, profile)
}

func (h Handler) updateProfile(w http.ResponseWriter, r *http.Request, id int64) {
	var req CreateProfileRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_JSON", "invalid JSON body")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		req.Name = req.BuildSpec.App.Name
	}
	if err := req.BuildSpec.Validate(); err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_BUILD_SPEC", err.Error())
		return
	}
	session, ok := apiauth.SessionFromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "UNAUTHENTICATED", "authentication is required")
		return
	}
	profile, err := profileFromSpec(req.Name, req.BuildSpec, session.User.ID)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_BUILD_SPEC", err.Error())
		return
	}
	updated, err := h.repo.UpdateBuildProfile(r.Context(), id, profile)
	if errors.Is(err, repository.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "BUILD_PROFILE_NOT_FOUND", "build profile not found")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_BUILD_PROFILE", err.Error())
		return
	}
	if err := h.writeAudit(r, audit.Event{
		Action:       "update_build_profile",
		ResourceType: "build_profile",
		ResourceID:   strconv.FormatInt(updated.ID, 10),
		Metadata: map[string]any{
			"name":     updated.Name,
			"app_name": updated.AppName,
			"vendor":   updated.Vendor,
		},
	}); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "AUDIT_WRITE_FAILED", "failed to record build profile update")
		return
	}
	httpx.JSON(w, http.StatusOK, updated)
}

func (h Handler) deleteProfile(w http.ResponseWriter, r *http.Request, id int64) {
	if err := h.repo.DeleteBuildProfile(r.Context(), id); errors.Is(err, repository.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "BUILD_PROFILE_NOT_FOUND", "build profile not found")
		return
	} else if errors.Is(err, repository.ErrConflict) {
		httpx.Error(w, http.StatusConflict, "BUILD_PROFILE_HAS_JOBS", "build profile has build jobs and cannot be deleted")
		return
	} else if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "DELETE_BUILD_PROFILE_FAILED", "delete build profile failed")
		return
	}
	if err := h.writeAudit(r, audit.Event{
		Action:       "delete_build_profile",
		ResourceType: "build_profile",
		ResourceID:   strconv.FormatInt(id, 10),
	}); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "AUDIT_WRITE_FAILED", "failed to record build profile deletion")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (h Handler) createProfileJob(w http.ResponseWriter, r *http.Request, id int64) {
	var req CreateJobRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_JSON", "invalid JSON body")
		return
	}
	req.Platform = strings.TrimSpace(req.Platform)
	if req.Platform == "" {
		httpx.Error(w, http.StatusBadRequest, "INVALID_BUILD_JOB", "platform is required")
		return
	}
	h.createJobForProfile(w, r, id, req.Platform, "profile_item")
}

func (h Handler) RunNext(w http.ResponseWriter, r *http.Request) {
	if !httpx.Method(w, r, http.MethodPost) {
		return
	}
	if h.worker == nil {
		httpx.Error(w, http.StatusServiceUnavailable, "BUILD_WORKER_NOT_CONFIGURED", "build worker is not configured")
		return
	}
	result, err := h.worker.RunOnce(r.Context())
	if errors.Is(err, buildworker.ErrNoQueuedJobs) {
		httpx.JSON(w, http.StatusOK, result)
		return
	}
	if err != nil {
		if result.Job.ID != 0 {
			httpx.JSON(w, http.StatusOK, result)
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "BUILD_WORKER_FAILED", "build worker failed")
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

func (h Handler) Doctor(w http.ResponseWriter, r *http.Request) {
	if !httpx.Method(w, r, http.MethodGet) {
		return
	}
	if h.worker == nil {
		httpx.Error(w, http.StatusServiceUnavailable, "BUILD_WORKER_NOT_CONFIGURED", "build worker is not configured")
		return
	}
	result, err := h.worker.Doctor(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "BUILD_WORKER_DOCTOR_FAILED", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

func (h Handler) JobItem(w http.ResponseWriter, r *http.Request) {
	parts, ok := pathParts(r.URL.Path, "/api/v1/build-jobs/")
	if !ok {
		httpx.Error(w, http.StatusNotFound, "NOT_FOUND", "build job endpoint not found")
		return
	}
	id, ok := parsePositiveID(parts[0])
	if !ok {
		httpx.Error(w, http.StatusBadRequest, "INVALID_BUILD_JOB", "invalid build job id")
		return
	}
	if len(parts) == 1 {
		if !httpx.Method(w, r, http.MethodGet) {
			return
		}
		h.getJob(w, r, id)
		return
	}
	if len(parts) != 2 {
		httpx.Error(w, http.StatusNotFound, "NOT_FOUND", "build job endpoint not found")
		return
	}
	switch parts[1] {
	case "cancel":
		if !httpx.Method(w, r, http.MethodPost) {
			return
		}
		h.cancelJob(w, r, id)
	case "retry":
		if !httpx.Method(w, r, http.MethodPost) {
			return
		}
		h.retryJob(w, r, id)
	case "artifacts":
		if !httpx.Method(w, r, http.MethodGet) {
			return
		}
		h.listArtifacts(w, r, id)
	case "logs":
		if !httpx.Method(w, r, http.MethodGet) {
			return
		}
		h.getJobLogs(w, r, id)
	default:
		httpx.Error(w, http.StatusNotFound, "NOT_FOUND", "build job endpoint not found")
	}
}

func (h Handler) Jobs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		jobs, err := h.repo.ListBuildJobs(r.Context())
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, "LIST_BUILD_JOBS_FAILED", "list build jobs failed")
			return
		}
		if jobs == nil {
			jobs = []models.BuildJob{}
		}
		httpx.JSON(w, http.StatusOK, jobs)
	case http.MethodPost:
		var req CreateJobRequest
		if err := httpx.DecodeJSON(r, &req); err != nil {
			httpx.Error(w, http.StatusBadRequest, "INVALID_JSON", "invalid JSON body")
			return
		}
		req.Platform = strings.TrimSpace(req.Platform)
		if req.ProfileID <= 0 || req.Platform == "" {
			httpx.Error(w, http.StatusBadRequest, "INVALID_BUILD_JOB", "profile_id and platform are required")
			return
		}
		if !slices.Contains(supportedPlatforms, req.Platform) {
			httpx.Error(w, http.StatusBadRequest, "INVALID_BUILD_JOB", "unsupported platform")
			return
		}
		h.createJobForProfile(w, r, req.ProfileID, req.Platform, "build_jobs")
	default:
		httpx.Error(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	}
}

func (h Handler) createJobForProfile(w http.ResponseWriter, r *http.Request, profileID int64, platform string, creationMode string) {
	if !slices.Contains(supportedPlatforms, platform) {
		httpx.Error(w, http.StatusBadRequest, "INVALID_BUILD_JOB", "unsupported platform")
		return
	}
	profile, err := h.repo.FindBuildProfileByID(r.Context(), profileID)
	if errors.Is(err, repository.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "BUILD_PROFILE_NOT_FOUND", "build profile not found")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "LOAD_BUILD_PROFILE_FAILED", "load build profile failed")
		return
	}
	if !profileEnablesPlatform(profile, platform) {
		httpx.Error(w, http.StatusBadRequest, "INVALID_BUILD_JOB", "profile does not enable platform")
		return
	}
	job, err := h.repo.CreateBuildJob(r.Context(), models.BuildJob{
		ProfileID: profileID,
		Platform:  platform,
		Status:    models.BuildJobQueued,
		Runner:    "pending",
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_BUILD_JOB", err.Error())
		return
	}
	if err := h.writeAudit(r, audit.Event{
		Action:       "create_build_job",
		ResourceType: "build_job",
		ResourceID:   strconv.FormatInt(job.ID, 10),
		Metadata: map[string]any{
			"profile_id":    job.ProfileID,
			"platform":      job.Platform,
			"status":        job.Status,
			"creation_mode": creationMode,
		},
	}); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "AUDIT_WRITE_FAILED", "failed to record build job creation")
		return
	}
	httpx.JSON(w, http.StatusCreated, job)
}

func (h Handler) ArtifactItem(w http.ResponseWriter, r *http.Request) {
	parts, ok := pathParts(r.URL.Path, "/api/v1/build-artifacts/")
	if !ok || len(parts) != 2 || parts[1] != "download" {
		httpx.Error(w, http.StatusNotFound, "NOT_FOUND", "build artifact endpoint not found")
		return
	}
	if !httpx.Method(w, r, http.MethodGet) {
		return
	}
	id, ok := parsePositiveID(parts[0])
	if !ok {
		httpx.Error(w, http.StatusBadRequest, "INVALID_BUILD_ARTIFACT", "invalid build artifact id")
		return
	}
	artifact, err := h.repo.FindBuildArtifactByID(r.Context(), id)
	if errors.Is(err, repository.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "BUILD_ARTIFACT_NOT_FOUND", "build artifact not found")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "LOAD_BUILD_ARTIFACT_FAILED", "load build artifact failed")
		return
	}
	if strings.TrimSpace(artifact.LocalPath) == "" {
		httpx.Error(w, http.StatusNotFound, "BUILD_ARTIFACT_NOT_FOUND", "build artifact file not found")
		return
	}
	file, err := os.Open(artifact.LocalPath)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "BUILD_ARTIFACT_NOT_FOUND", "build artifact file not found")
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.IsDir() {
		httpx.Error(w, http.StatusNotFound, "BUILD_ARTIFACT_NOT_FOUND", "build artifact file not found")
		return
	}
	if err := h.writeAudit(r, audit.Event{
		Action:       "download_build_artifact",
		ResourceType: "build_artifact",
		ResourceID:   strconv.FormatInt(artifact.ID, 10),
		Metadata: map[string]any{
			"build_job_id": artifact.BuildJobID,
			"platform":     artifact.Platform,
			"filename":     artifact.Filename,
			"sha256":       artifact.SHA256,
			"size_bytes":   artifact.SizeBytes,
		},
	}); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "AUDIT_WRITE_FAILED", "failed to record build artifact download")
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": artifact.Filename}))
	http.ServeContent(w, r, artifact.Filename, info.ModTime(), file)
}

func (h Handler) getJob(w http.ResponseWriter, r *http.Request, id int64) {
	job, err := h.repo.FindBuildJobByID(r.Context(), id)
	if errors.Is(err, repository.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "BUILD_JOB_NOT_FOUND", "build job not found")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "LOAD_BUILD_JOB_FAILED", "load build job failed")
		return
	}
	httpx.JSON(w, http.StatusOK, job)
}

func (h Handler) cancelJob(w http.ResponseWriter, r *http.Request, id int64) {
	job, err := h.repo.FindBuildJobByID(r.Context(), id)
	if errors.Is(err, repository.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "BUILD_JOB_NOT_FOUND", "build job not found")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "LOAD_BUILD_JOB_FAILED", "load build job failed")
		return
	}
	if job.Status != models.BuildJobQueued {
		httpx.Error(w, http.StatusConflict, "BUILD_JOB_NOT_CANCELABLE", "only queued build jobs can be canceled")
		return
	}
	canceled, err := h.repo.CancelBuildJob(r.Context(), id, "canceled by administrator", time.Now().UTC())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "CANCEL_BUILD_JOB_FAILED", "cancel build job failed")
		return
	}
	httpx.JSON(w, http.StatusOK, canceled)
}

func (h Handler) retryJob(w http.ResponseWriter, r *http.Request, id int64) {
	job, err := h.repo.FindBuildJobByID(r.Context(), id)
	if errors.Is(err, repository.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "BUILD_JOB_NOT_FOUND", "build job not found")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "LOAD_BUILD_JOB_FAILED", "load build job failed")
		return
	}
	if job.Status == models.BuildJobQueued || job.Status == models.BuildJobRunning {
		httpx.Error(w, http.StatusConflict, "BUILD_JOB_NOT_RETRYABLE", "queued or running build jobs cannot be retried")
		return
	}
	if _, err := h.repo.FindBuildProfileByID(r.Context(), job.ProfileID); errors.Is(err, repository.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "BUILD_PROFILE_NOT_FOUND", "build profile not found")
		return
	} else if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "LOAD_BUILD_PROFILE_FAILED", "load build profile failed")
		return
	}
	created, err := h.repo.CreateBuildJob(r.Context(), models.BuildJob{
		ProfileID: job.ProfileID,
		Platform:  job.Platform,
		Status:    models.BuildJobQueued,
		Runner:    "pending",
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_BUILD_JOB", err.Error())
		return
	}
	if err := h.writeAudit(r, audit.Event{
		Action:       "create_build_job",
		ResourceType: "build_job",
		ResourceID:   strconv.FormatInt(created.ID, 10),
		Metadata: map[string]any{
			"profile_id":    created.ProfileID,
			"platform":      created.Platform,
			"status":        created.Status,
			"source_job_id": job.ID,
			"creation_mode": "retry",
		},
	}); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "AUDIT_WRITE_FAILED", "failed to record build job creation")
		return
	}
	httpx.JSON(w, http.StatusCreated, created)
}

func (h Handler) listArtifacts(w http.ResponseWriter, r *http.Request, id int64) {
	if _, err := h.repo.FindBuildJobByID(r.Context(), id); errors.Is(err, repository.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "BUILD_JOB_NOT_FOUND", "build job not found")
		return
	} else if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "LOAD_BUILD_JOB_FAILED", "load build job failed")
		return
	}
	artifacts, err := h.repo.ListBuildArtifacts(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "LIST_BUILD_ARTIFACTS_FAILED", "list build artifacts failed")
		return
	}
	if artifacts == nil {
		artifacts = []models.BuildArtifact{}
	}
	httpx.JSON(w, http.StatusOK, artifacts)
}

type BuildJobLogsResponse struct {
	JobID     int64  `json:"job_id"`
	LogPath   string `json:"log_path"`
	Content   string `json:"content"`
	Truncated bool   `json:"truncated"`
}

func (h Handler) getJobLogs(w http.ResponseWriter, r *http.Request, id int64) {
	job, err := h.repo.FindBuildJobByID(r.Context(), id)
	if errors.Is(err, repository.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "BUILD_JOB_NOT_FOUND", "build job not found")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "LOAD_BUILD_JOB_FAILED", "load build job failed")
		return
	}
	if strings.TrimSpace(job.LogPath) == "" {
		httpx.JSON(w, http.StatusOK, BuildJobLogsResponse{JobID: job.ID})
		return
	}
	file, err := os.Open(job.LogPath)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "BUILD_JOB_LOG_NOT_FOUND", "build job log not found")
		return
	}
	defer file.Close()
	const maxLogBytes int64 = 1024 * 1024
	limited := io.LimitReader(file, maxLogBytes+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "READ_BUILD_JOB_LOG_FAILED", "read build job log failed")
		return
	}
	truncated := int64(len(raw)) > maxLogBytes
	if truncated {
		raw = raw[:maxLogBytes]
	}
	httpx.JSON(w, http.StatusOK, BuildJobLogsResponse{
		JobID:     job.ID,
		LogPath:   job.LogPath,
		Content:   string(raw),
		Truncated: truncated,
	})
}

func profileFromSpec(name string, spec apibuilder.BuildSpec, createdBy int64) (models.BuildProfile, error) {
	server, err := marshalObject(spec.Server)
	if err != nil {
		return models.BuildProfile{}, err
	}
	branding, err := marshalObject(spec.Branding)
	if err != nil {
		return models.BuildProfile{}, err
	}
	policy, err := marshalObject(spec.Policy)
	if err != nil {
		return models.BuildProfile{}, err
	}
	platforms, err := marshalObject(spec.Platforms)
	if err != nil {
		return models.BuildProfile{}, err
	}
	signing, err := marshalObject(spec.Signing)
	if err != nil {
		return models.BuildProfile{}, err
	}
	source, err := marshalObject(normalizedSource(spec.Source))
	if err != nil {
		return models.BuildProfile{}, err
	}
	return models.BuildProfile{
		Name:             strings.TrimSpace(name),
		AppName:          spec.App.Name,
		Vendor:           spec.App.Vendor,
		BundleID:         spec.App.BundleID,
		ProductName:      spec.App.WindowsProductName,
		Description:      spec.App.Description,
		ServerConfigJSON: server,
		BrandingJSON:     branding,
		PolicyJSON:       policy,
		PlatformsJSON:    platforms,
		SigningJSON:      signing,
		SourceJSON:       source,
		CreatedBy:        createdBy,
	}, nil
}

type sourceForStorage struct {
	RustDeskRef      string `json:"rustdesk_ref"`
	OpenDeskPatchset string `json:"opendesk_patchset"`
}

func normalizedSource(source apibuilder.SourceSpec) sourceForStorage {
	ref := source.Ref()
	if strings.TrimSpace(ref) == "" {
		ref = "master"
	}
	patchset := source.OpenDeskPatchset
	if strings.TrimSpace(patchset) == "" {
		patchset = "opendesk-remote-m4"
	}
	return sourceForStorage{RustDeskRef: ref, OpenDeskPatchset: patchset}
}

func profileEnablesPlatform(profile models.BuildProfile, platform string) bool {
	var platforms apibuilder.PlatformSpec
	if err := json.Unmarshal([]byte(profile.PlatformsJSON), &platforms); err != nil {
		return false
	}
	switch platform {
	case "windows_x64":
		return platforms.WindowsX64
	case "macos_x64":
		return platforms.MacOSX64
	case "macos_arm64":
		return platforms.MacOSARM64
	case "android_arm64":
		return platforms.AndroidARM64
	case "ios_arm64":
		return platforms.IOSARM64
	default:
		return false
	}
}

func marshalObject(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func pathParts(path string, prefix string) ([]string, bool) {
	if !strings.HasPrefix(path, prefix) {
		return nil, false
	}
	trimmed := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	if trimmed == "" {
		return nil, false
	}
	parts := strings.Split(trimmed, "/")
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			return nil, false
		}
	}
	return parts, true
}

func parsePositiveID(value string) (int64, bool) {
	id, err := strconv.ParseInt(value, 10, 64)
	return id, err == nil && id > 0
}

func (h Handler) writeAudit(r *http.Request, event audit.Event) error {
	event.ActorType = "system"
	if session, ok := apiauth.SessionFromContext(r.Context()); ok {
		event.ActorType = apiauth.ActorType(session)
		userID := session.User.ID
		event.ActorUserID = &userID
	}
	event.IP = requestIP(r)
	event.UserAgent = r.UserAgent()
	return (audit.RepositoryWriter{Repo: h.repo}).Write(r.Context(), event)
}

func requestIP(r *http.Request) string {
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		ip, _, _ := strings.Cut(forwarded, ",")
		return strings.TrimSpace(ip)
	}
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
