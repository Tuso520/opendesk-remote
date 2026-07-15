package buildspec

import "testing"

func TestSpecValidateAcceptsRelayGrantBuildSpec(t *testing.T) {
	required := true
	spec := validSpec()
	spec.Server.RelayGrantRequired = &required

	if err := spec.Validate(); err != nil {
		t.Fatalf("expected valid spec, got %v", err)
	}
	if !spec.RelayGrantRequired() {
		t.Fatal("expected relay grant to be required")
	}
}

func TestSpecValidateRequiresRelayNameWhenRelayGrantRequired(t *testing.T) {
	spec := validSpec()
	spec.Server.RelayName = ""

	if err := spec.Validate(); err == nil {
		t.Fatal("expected missing relay_name error")
	}
}

func TestSpecValidateRejectsRustDeskTrademark(t *testing.T) {
	spec := validSpec()
	spec.App.Name = "RustDesk"

	if err := spec.Validate(); err == nil {
		t.Fatal("expected trademark validation error")
	}
}

func validSpec() Spec {
	return Spec{
		App: AppSpec{
			Name:     "OpenDesk Remote",
			Vendor:   "OpenDesk",
			BundleID: "com.example.opendeskremote",
		},
		Server: ServerSpec{
			IDServer:    "remote.example.com:21116",
			RelayServer: "remote.example.com:21117",
			RelayName:   "hbbr-relay-a",
			APIServer:   "https://remote.example.com",
			Key:         "PUBLIC_KEY",
			WebSocket:   true,
		},
		Platforms: PlatformSpec{WindowsX64: true},
		Signing:   SigningSpec{},
		Source:    SourceSpec{OpenDeskPatchset: "opendesk-remote-m4"},
	}
}
