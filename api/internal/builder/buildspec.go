package builder

import (
	"errors"
	"regexp"
)

var bundleIDPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*(\.[A-Za-z][A-Za-z0-9-]*)+$`)

type BuildSpec struct {
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
	RustDeskTag      string `json:"rustdesk_tag,omitempty"`
	OpenDeskPatchset string `json:"opendesk_patchset"`
}

func (s BuildSpec) Validate() error {
	if s.App.Name == "" {
		return errors.New("app.name is required")
	}
	if s.App.Name == "RustDesk" {
		return errors.New("app.name must not use RustDesk trademark")
	}
	if !bundleIDPattern.MatchString(s.App.BundleID) {
		return errors.New("app.bundle_id must be a valid reverse-DNS identifier")
	}
	if s.Server.IDServer == "" || s.Server.RelayServer == "" || s.Server.APIServer == "" || s.Server.Key == "" {
		return errors.New("server id_server, relay_server, api_server, and key are required")
	}
	if s.RelayGrantRequired() && s.Server.RelayName == "" {
		return errors.New("server.relay_name is required when relay grants are required")
	}
	if !s.Platforms.Any() {
		return errors.New("at least one platform must be enabled")
	}
	if s.Platforms.IOSARM64 && s.Signing.IOS == "" {
		return errors.New("ios signing mode is required when ios_arm64 is enabled")
	}
	return nil
}

func (s BuildSpec) RelayGrantRequired() bool {
	if s.Server.RelayGrantRequired == nil {
		return true
	}
	return *s.Server.RelayGrantRequired
}

func (s SourceSpec) Ref() string {
	if s.RustDeskRef != "" {
		return s.RustDeskRef
	}
	return s.RustDeskTag
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
