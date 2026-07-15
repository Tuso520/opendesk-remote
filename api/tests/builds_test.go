package tests

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/opendesk-remote/opendesk-remote/api/internal/app"
	"github.com/opendesk-remote/opendesk-remote/api/internal/auth"
	"github.com/opendesk-remote/opendesk-remote/api/internal/models"
	"github.com/opendesk-remote/opendesk-remote/api/internal/repository"
)

func TestBuildProfilesAndJobsAPI(t *testing.T) {
	server := httptest.NewServer(app.NewRouter(testConfig(t), slog.Default()))
	defer server.Close()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	client := &http.Client{Jar: jar}
	login := postJSON(t, client, server.URL+"/api/v1/auth/login", `{"email":"admin@example.com","password":"admin-password-12345"}`)
	defer login.Body.Close()
	if login.StatusCode != http.StatusOK {
		t.Fatalf("expected login 200, got %d", login.StatusCode)
	}

	createProfile := postJSON(t, client, server.URL+"/api/v1/build-profiles", buildProfileBody())
	defer createProfile.Body.Close()
	if createProfile.StatusCode != http.StatusCreated {
		t.Fatalf("expected profile 201, got %d", createProfile.StatusCode)
	}
	var profileEnvelope struct {
		Data struct {
			ID       int64  `json:"id"`
			AppName  string `json:"app_name"`
			BundleID string `json:"bundle_id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(createProfile.Body).Decode(&profileEnvelope); err != nil {
		t.Fatalf("decode profile: %v", err)
	}
	if profileEnvelope.Data.ID == 0 || profileEnvelope.Data.AppName != "OpenDesk Remote" {
		t.Fatalf("unexpected profile: %+v", profileEnvelope.Data)
	}

	profileDetail, err := client.Get(server.URL + "/api/v1/build-profiles/" + int64String(profileEnvelope.Data.ID))
	if err != nil {
		t.Fatalf("get profile: %v", err)
	}
	defer profileDetail.Body.Close()
	if profileDetail.StatusCode != http.StatusOK {
		t.Fatalf("expected profile detail 200, got %d", profileDetail.StatusCode)
	}

	updateProfile := putJSON(t, client, server.URL+"/api/v1/build-profiles/"+int64String(profileEnvelope.Data.ID), buildProfileBodyNamed("Updated Windows Profile"))
	defer updateProfile.Body.Close()
	if updateProfile.StatusCode != http.StatusOK {
		t.Fatalf("expected profile update 200, got %d", updateProfile.StatusCode)
	}
	var updatedProfileEnvelope struct {
		Data struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.NewDecoder(updateProfile.Body).Decode(&updatedProfileEnvelope); err != nil {
		t.Fatalf("decode updated profile: %v", err)
	}
	if updatedProfileEnvelope.Data.ID != profileEnvelope.Data.ID || updatedProfileEnvelope.Data.Name != "Updated Windows Profile" {
		t.Fatalf("unexpected updated profile: %+v", updatedProfileEnvelope.Data)
	}

	deleteCandidate := postJSON(t, client, server.URL+"/api/v1/client-build-configs", buildProfileBodyNamed("Delete Candidate Profile"))
	defer deleteCandidate.Body.Close()
	if deleteCandidate.StatusCode != http.StatusCreated {
		t.Fatalf("expected delete candidate profile 201, got %d", deleteCandidate.StatusCode)
	}
	var deleteCandidateEnvelope struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(deleteCandidate.Body).Decode(&deleteCandidateEnvelope); err != nil {
		t.Fatalf("decode delete candidate profile: %v", err)
	}
	deleteProfile := deleteJSON(t, client, server.URL+"/api/v1/client-build-configs/"+int64String(deleteCandidateEnvelope.Data.ID))
	defer deleteProfile.Body.Close()
	if deleteProfile.StatusCode != http.StatusOK {
		t.Fatalf("expected delete candidate profile 200, got %d", deleteProfile.StatusCode)
	}

	profiles, err := client.Get(server.URL + "/api/v1/build-profiles")
	if err != nil {
		t.Fatalf("list profiles: %v", err)
	}
	defer profiles.Body.Close()
	if profiles.StatusCode != http.StatusOK {
		t.Fatalf("expected profiles 200, got %d", profiles.StatusCode)
	}

	createJob := postJSON(t, client, server.URL+"/api/v1/build-profiles/"+int64String(profileEnvelope.Data.ID)+"/jobs", `{"platform":"windows_x64"}`)
	defer createJob.Body.Close()
	if createJob.StatusCode != http.StatusCreated {
		t.Fatalf("expected job 201, got %d", createJob.StatusCode)
	}
	var jobEnvelope struct {
		Data struct {
			ID       int64  `json:"id"`
			Platform string `json:"platform"`
			Status   string `json:"status"`
		} `json:"data"`
	}
	if err := json.NewDecoder(createJob.Body).Decode(&jobEnvelope); err != nil {
		t.Fatalf("decode job: %v", err)
	}
	if jobEnvelope.Data.Platform != "windows_x64" || jobEnvelope.Data.Status != "queued" {
		t.Fatalf("unexpected job: %+v", jobEnvelope.Data)
	}

	deleteProfileWithJobs := deleteJSON(t, client, server.URL+"/api/v1/build-profiles/"+int64String(profileEnvelope.Data.ID))
	defer deleteProfileWithJobs.Body.Close()
	if deleteProfileWithJobs.StatusCode != http.StatusConflict {
		t.Fatalf("expected profile with jobs delete 409, got %d", deleteProfileWithJobs.StatusCode)
	}

	jobDetail, err := client.Get(server.URL + "/api/v1/build-jobs/" + int64String(jobEnvelope.Data.ID))
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	defer jobDetail.Body.Close()
	if jobDetail.StatusCode != http.StatusOK {
		t.Fatalf("expected job detail 200, got %d", jobDetail.StatusCode)
	}

	jobLogs, err := client.Get(server.URL + "/api/v1/build-jobs/" + int64String(jobEnvelope.Data.ID) + "/logs")
	if err != nil {
		t.Fatalf("get job logs: %v", err)
	}
	defer jobLogs.Body.Close()
	if jobLogs.StatusCode != http.StatusOK {
		t.Fatalf("expected job logs 200, got %d", jobLogs.StatusCode)
	}

	artifacts, err := client.Get(server.URL + "/api/v1/build-jobs/" + int64String(jobEnvelope.Data.ID) + "/artifacts")
	if err != nil {
		t.Fatalf("list artifacts: %v", err)
	}
	defer artifacts.Body.Close()
	if artifacts.StatusCode != http.StatusOK {
		t.Fatalf("expected artifacts 200, got %d", artifacts.StatusCode)
	}

	retryQueued := postJSON(t, client, server.URL+"/api/v1/build-jobs/"+int64String(jobEnvelope.Data.ID)+"/retry", `{}`)
	defer retryQueued.Body.Close()
	if retryQueued.StatusCode != http.StatusConflict {
		t.Fatalf("expected queued retry 409, got %d", retryQueued.StatusCode)
	}

	cancelJob := postJSON(t, client, server.URL+"/api/v1/build-jobs/"+int64String(jobEnvelope.Data.ID)+"/cancel", `{}`)
	defer cancelJob.Body.Close()
	if cancelJob.StatusCode != http.StatusOK {
		t.Fatalf("expected cancel 200, got %d", cancelJob.StatusCode)
	}
	var cancelEnvelope struct {
		Data struct {
			ID     int64  `json:"id"`
			Status string `json:"status"`
		} `json:"data"`
	}
	if err := json.NewDecoder(cancelJob.Body).Decode(&cancelEnvelope); err != nil {
		t.Fatalf("decode canceled job: %v", err)
	}
	if cancelEnvelope.Data.ID != jobEnvelope.Data.ID || cancelEnvelope.Data.Status != "canceled" {
		t.Fatalf("unexpected canceled job: %+v", cancelEnvelope.Data)
	}

	retryCanceled := postJSON(t, client, server.URL+"/api/v1/build-jobs/"+int64String(jobEnvelope.Data.ID)+"/retry", `{}`)
	defer retryCanceled.Body.Close()
	if retryCanceled.StatusCode != http.StatusCreated {
		t.Fatalf("expected canceled retry 201, got %d", retryCanceled.StatusCode)
	}
	var retryEnvelope struct {
		Data struct {
			ID       int64  `json:"id"`
			Profile  int64  `json:"profile_id"`
			Platform string `json:"platform"`
			Status   string `json:"status"`
		} `json:"data"`
	}
	if err := json.NewDecoder(retryCanceled.Body).Decode(&retryEnvelope); err != nil {
		t.Fatalf("decode retried job: %v", err)
	}
	if retryEnvelope.Data.ID == 0 || retryEnvelope.Data.Profile != profileEnvelope.Data.ID || retryEnvelope.Data.Status != "queued" {
		t.Fatalf("unexpected retried job: %+v", retryEnvelope.Data)
	}

	missingArtifact, err := client.Get(server.URL + "/api/v1/build-artifacts/999/download")
	if err != nil {
		t.Fatalf("download missing artifact: %v", err)
	}
	defer missingArtifact.Body.Close()
	if missingArtifact.StatusCode != http.StatusNotFound {
		t.Fatalf("expected missing artifact 404, got %d", missingArtifact.StatusCode)
	}

	disabledPlatform := postJSON(t, client, server.URL+"/api/v1/build-jobs", `{"profile_id":`+int64String(profileEnvelope.Data.ID)+`,"platform":"macos_x64"}`)
	defer disabledPlatform.Body.Close()
	if disabledPlatform.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected disabled platform 400, got %d", disabledPlatform.StatusCode)
	}
}

func TestBuildArtifactDownloadAPI(t *testing.T) {
	cfg := testConfig(t)
	hash, err := auth.HashPassword(cfg.InitialAdminPassword)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	ctx := context.Background()
	store := repository.NewMemoryWithInitialAdmin(cfg.InitialAdminEmail, hash)
	profile, err := store.CreateBuildProfile(ctx, models.BuildProfile{
		Name:             "Artifact Profile",
		AppName:          "OpenDesk Remote",
		BundleID:         "com.example.opendeskremote",
		ServerConfigJSON: `{}`,
		BrandingJSON:     `{}`,
		PolicyJSON:       `{}`,
		PlatformsJSON:    `{"windows_x64":true}`,
		SigningJSON:      `{}`,
		CreatedBy:        1,
	})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}
	job, err := store.CreateBuildJob(ctx, models.BuildJob{ProfileID: profile.ID, Platform: "windows_x64", Status: models.BuildJobSucceeded})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
	artifactPath := filepath.Join(t.TempDir(), "OpenDeskRemote.exe")
	if err := os.WriteFile(artifactPath, []byte("artifact-bytes"), 0o644); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	artifact, err := store.CreateBuildArtifact(ctx, models.BuildArtifact{
		BuildJobID: job.ID,
		Platform:   "windows_x64",
		Filename:   "OpenDeskRemote.exe",
		LocalPath:  artifactPath,
		SHA256:     "abc123",
		SizeBytes:  14,
	})
	if err != nil {
		t.Fatalf("create artifact: %v", err)
	}

	server := httptest.NewServer(app.NewRouterWithStore(cfg, slog.Default(), store))
	defer server.Close()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	client := &http.Client{Jar: jar}
	login := postJSON(t, client, server.URL+"/api/v1/auth/login", `{"email":"admin@example.com","password":"admin-password-12345"}`)
	defer login.Body.Close()
	if login.StatusCode != http.StatusOK {
		t.Fatalf("expected login 200, got %d", login.StatusCode)
	}
	download, err := client.Get(server.URL + "/api/v1/build-artifacts/" + int64String(artifact.ID) + "/download")
	if err != nil {
		t.Fatalf("download artifact: %v", err)
	}
	defer download.Body.Close()
	if download.StatusCode != http.StatusOK {
		t.Fatalf("expected artifact download 200, got %d", download.StatusCode)
	}
	body, err := io.ReadAll(download.Body)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	if string(body) != "artifact-bytes" {
		t.Fatalf("unexpected artifact body: %q", string(body))
	}
	if download.Header.Get("Content-Disposition") == "" {
		t.Fatalf("expected content disposition header")
	}
}

func buildProfileBody() string {
	return `{
		"name": "Default Windows Profile",
		"build_spec": {
			"app": {
				"name": "OpenDesk Remote",
				"vendor": "OpenDesk",
				"bundle_id": "com.example.opendeskremote",
				"windows_product_name": "OpenDesk Remote",
				"description": "Open-source self-hosted remote desktop"
			},
			"branding": {},
			"server": {
				"id_server": "remote.example.com:21116",
				"relay_server": "remote.example.com:21117",
				"relay_name": "hbbr-relay-a",
				"api_server": "https://remote.example.com",
				"key": "PUBLIC_KEY",
				"websocket": true,
				"relay_grant_required": true
			},
			"policy": {
				"profile": "default-secure",
				"override_settings": {
					"allow-remote-config-modification": "N"
				},
				"default_settings": {
					"verification-method": "use-both-passwords"
				}
			},
			"platforms": {
				"windows_x64": true
			},
			"signing": {
				"windows": "unsigned-dev"
			},
			"source": {
				"rustdesk_ref": "master",
				"opendesk_patchset": "opendesk-remote-m4"
			}
		}
	}`
}

func buildProfileBodyNamed(name string) string {
	return strings.Replace(buildProfileBody(), `"name": "Default Windows Profile"`, `"name": "`+name+`"`, 1)
}

func int64String(value int64) string {
	return strconv.FormatInt(value, 10)
}
