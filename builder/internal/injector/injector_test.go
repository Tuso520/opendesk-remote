package injector

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opendesk-remote/opendesk-remote/builder/internal/buildspec"
)

func TestGenerateWritesClientInjectionFiles(t *testing.T) {
	outDir := t.TempDir()
	manifest, err := Generate(validSpec(), outDir)
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	if len(manifest.Files) != 4 {
		t.Fatalf("expected 4 generated files, got %+v", manifest.Files)
	}
	raw, err := os.ReadFile(filepath.Join(outDir, "rust", "src", "opendesk_generated.rs"))
	if err != nil {
		t.Fatalf("read generated rust config: %v", err)
	}
	text := string(raw)
	for _, want := range []string{
		"OPENDESK_ID_SERVER",
		"remote.example.com:21116",
		"OPENDESK_RELAY_NAME",
		"hbbr-relay-a",
		"OPENDESK_RELAY_GRANT_REQUIRED: bool = true",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("generated rust config missing %q:\n%s", want, text)
		}
	}
}

func validSpec() buildspec.Spec {
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
			DefaultSettings: map[string]string{
				"verification-method": "use-both-passwords",
			},
			OverrideSettings: map[string]string{
				"enable-terminal": "N",
			},
		},
		Platforms: buildspec.PlatformSpec{WindowsX64: true},
		Source:    buildspec.SourceSpec{OpenDeskPatchset: "opendesk-remote-m4"},
	}
}
