package builder

import "testing"

func validSpec() BuildSpec {
	relayGrantRequired := true
	return BuildSpec{
		App:       AppSpec{Name: "OpenDesk Remote", BundleID: "com.example.opendeskremote"},
		Server:    ServerSpec{IDServer: "rd.example.com:21116", RelayServer: "rd.example.com:21117", RelayName: "hbbr-relay-a", APIServer: "https://rd.example.com", Key: "PUBLIC_KEY", RelayGrantRequired: &relayGrantRequired},
		Platforms: PlatformSpec{WindowsX64: true, MacOSX64: true, AndroidARM64: true, IOSARM64: true},
		Signing:   SigningSpec{IOS: "bring-your-own-apple-developer"},
		Source:    SourceSpec{RustDeskRef: "master", OpenDeskPatchset: "opendesk-remote-m4"},
	}
}

func TestBuildSpecValidation(t *testing.T) {
	spec := validSpec()
	if err := spec.Validate(); err != nil {
		t.Fatalf("expected valid spec: %v", err)
	}
}

func TestBuildSpecRejectsRustDeskTrademarkAsProductName(t *testing.T) {
	spec := validSpec()
	spec.App.Name = "RustDesk"
	if err := spec.Validate(); err == nil {
		t.Fatal("expected trademark validation error")
	}
}

func TestBuildSpecPlatformOrder(t *testing.T) {
	ordered := validSpec().Platforms.Ordered()
	want := []string{"windows_x64", "macos_x64", "android_arm64", "ios_arm64"}
	for i := range want {
		if ordered[i] != want[i] {
			t.Fatalf("unexpected platform order: got %v want %v", ordered, want)
		}
	}
}

func TestBuildJobFailsClearlyWhenRunnerMissing(t *testing.T) {
	job, artifact := NewRunnerRegistry().Run(validSpec(), BuildJob{Platform: "windows_x64", Status: JobQueued})
	if artifact != nil || job.Status != JobFailed || job.ErrorMessage == "" {
		t.Fatalf("expected clear missing runner failure, job=%+v artifact=%+v", job, artifact)
	}
}
