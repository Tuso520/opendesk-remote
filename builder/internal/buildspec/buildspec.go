package buildspec

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
)

var bundleIDPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*(\.[A-Za-z][A-Za-z0-9-]*)+$`)

type Spec struct {
	App       AppSpec      `json:"app"`
	Branding  BrandingSpec `json:"branding"`
	Server    ServerSpec   `json:"server"`
	Policy    PolicySpec   `json:"policy"`
	Platforms PlatformSpec `json:"platforms"`
	Signing   SigningSpec  `json:"signing"`
	Source    SourceSpec   `json:"source"`
}

type AppSpec struct {
	Name               string `json:"name"`
	Vendor             string `json:"vendor"`
	BundleID           string `json:"bundle_id"`
	WindowsProductName string `json:"windows_product_name"`
	Description        string `json:"description"`
}

type BrandingSpec struct {
	LogoPNG             string `json:"logo_png"`
	IconICO             string `json:"icon_ico"`
	IconICNS            string `json:"icon_icns"`
	AndroidAdaptiveIcon string `json:"android_adaptive_icon"`
	IOSAppIcon          string `json:"ios_app_icon"`
	TrayIcon            string `json:"tray_icon"`
	InstallerBanner     string `json:"installer_banner"`
}

type ServerSpec struct {
	IDServer           string `json:"id_server"`
	RelayServer        string `json:"relay_server"`
	RelayName          string `json:"relay_name"`
	APIServer          string `json:"api_server"`
	Key                string `json:"key"`
	WebSocket          bool   `json:"websocket"`
	RelayGrantRequired *bool  `json:"relay_grant_required,omitempty"`
}

type PolicySpec struct {
	Profile          string            `json:"profile"`
	OverrideSettings map[string]string `json:"override_settings"`
	DefaultSettings  map[string]string `json:"default_settings"`
}

type PlatformSpec struct {
	WindowsX64   bool `json:"windows_x64"`
	MacOSX64     bool `json:"macos_x64"`
	MacOSARM64   bool `json:"macos_arm64"`
	AndroidARM64 bool `json:"android_arm64"`
	IOSARM64     bool `json:"ios_arm64"`
}

type SigningSpec struct {
	Windows string `json:"windows"`
	MacOS   string `json:"macos"`
	Android string `json:"android"`
	IOS     string `json:"ios"`
}

type SourceSpec struct {
	RustDeskRef      string `json:"rustdesk_ref"`
	OpenDeskPatchset string `json:"opendesk_patchset"`
}

func Load(path string) (Spec, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Spec{}, err
	}
	var spec Spec
	if err := json.Unmarshal(raw, &spec); err != nil {
		return Spec{}, err
	}
	return spec, spec.Validate()
}

func (s Spec) Validate() error {
	if strings.TrimSpace(s.App.Name) == "" {
		return errors.New("app.name is required")
	}
	if strings.EqualFold(strings.TrimSpace(s.App.Name), "rustdesk") {
		return errors.New("app.name must not use RustDesk trademark")
	}
	if strings.TrimSpace(s.App.Vendor) == "" {
		return errors.New("app.vendor is required")
	}
	if !bundleIDPattern.MatchString(s.App.BundleID) {
		return errors.New("app.bundle_id must be a valid reverse-DNS identifier")
	}
	if s.Server.IDServer == "" || s.Server.RelayServer == "" || s.Server.APIServer == "" || s.Server.Key == "" {
		return errors.New("server id_server, relay_server, api_server, and key are required")
	}
	if s.RelayGrantRequired() && strings.TrimSpace(s.Server.RelayName) == "" {
		return errors.New("server.relay_name is required when relay grants are required")
	}
	if !s.Platforms.Any() {
		return errors.New("at least one platform must be enabled")
	}
	if s.Platforms.IOSARM64 && s.Signing.IOS == "" {
		return errors.New("ios signing mode is required when ios_arm64 is enabled")
	}
	if s.Source.OpenDeskPatchset == "" {
		return errors.New("source.opendesk_patchset is required")
	}
	return nil
}

func (s Spec) RelayGrantRequired() bool {
	if s.Server.RelayGrantRequired == nil {
		return true
	}
	return *s.Server.RelayGrantRequired
}

func (s Spec) NormalizedJSON() ([]byte, error) {
	type normalizedServer struct {
		IDServer           string `json:"id_server"`
		RelayServer        string `json:"relay_server"`
		RelayName          string `json:"relay_name"`
		APIServer          string `json:"api_server"`
		Key                string `json:"key"`
		WebSocket          bool   `json:"websocket"`
		RelayGrantRequired bool   `json:"relay_grant_required"`
	}
	type normalized struct {
		App       AppSpec          `json:"app"`
		Branding  BrandingSpec     `json:"branding"`
		Server    normalizedServer `json:"server"`
		Policy    PolicySpec       `json:"policy"`
		Platforms PlatformSpec     `json:"platforms"`
		Signing   SigningSpec      `json:"signing"`
		Source    SourceSpec       `json:"source"`
	}
	return json.MarshalIndent(normalized{
		App:      s.App,
		Branding: s.Branding,
		Server: normalizedServer{
			IDServer:           s.Server.IDServer,
			RelayServer:        s.Server.RelayServer,
			RelayName:          s.Server.RelayName,
			APIServer:          s.Server.APIServer,
			Key:                s.Server.Key,
			WebSocket:          s.Server.WebSocket,
			RelayGrantRequired: s.RelayGrantRequired(),
		},
		Policy:    s.Policy,
		Platforms: s.Platforms,
		Signing:   s.Signing,
		Source:    s.Source,
	}, "", "  ")
}

func (p PlatformSpec) Any() bool {
	return p.WindowsX64 || p.MacOSX64 || p.MacOSARM64 || p.AndroidARM64 || p.IOSARM64
}

func (p PlatformSpec) Ordered() []string {
	out := []string{}
	if p.WindowsX64 {
		out = append(out, "windows_x64")
	}
	if p.MacOSX64 {
		out = append(out, "macos_x64")
	}
	if p.MacOSARM64 {
		out = append(out, "macos_arm64")
	}
	if p.AndroidARM64 {
		out = append(out, "android_arm64")
	}
	if p.IOSARM64 {
		out = append(out, "ios_arm64")
	}
	return out
}

func (s Spec) Summary() string {
	return fmt.Sprintf("%s (%s), platforms=%v", s.App.Name, s.App.BundleID, s.Platforms.Ordered())
}
