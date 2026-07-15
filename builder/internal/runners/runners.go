package runners

import "errors"

type Artifact struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

type Runner interface {
	Platform() string
	Run() error
}

type Result struct {
	Platform            string     `json:"platform"`
	SourceDir           string     `json:"source_dir"`
	InjectionDir        string     `json:"injection_dir"`
	ArtifactDir         string     `json:"artifact_dir"`
	StagedFiles         []string   `json:"staged_files"`
	BuildCommand        string     `json:"build_command,omitempty"`
	BuildExecuted       bool       `json:"build_executed"`
	BuildLog            string     `json:"build_log,omitempty"`
	Artifacts           []Artifact `json:"artifacts,omitempty"`
	NotConfigured       bool       `json:"not_configured"`
	NotConfiguredReason string     `json:"not_configured_reason,omitempty"`
}

func DefaultPlatformOrder() []string {
	return []string{"windows_x64", "macos_x64", "macos_arm64", "android_arm64", "ios_arm64"}
}

type NotConfigured struct {
	Name string
}

func (r NotConfigured) Platform() string {
	return r.Name
}

func (r NotConfigured) Run() error {
	return errors.New("runner not configured for platform: " + r.Name)
}
