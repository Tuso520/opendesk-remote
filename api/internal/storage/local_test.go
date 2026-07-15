package storage

import (
	"strings"
	"testing"
)

func TestSafePathRejectsTraversal(t *testing.T) {
	store := LocalStore{Root: "/tmp/opendesk"}
	if _, err := store.SafePath("..", "secret"); err == nil {
		t.Fatal("expected path traversal rejection")
	}
}

func TestAssetUploadValidation(t *testing.T) {
	validator := AssetValidator{MaxBytes: 1024}
	if err := validator.ValidateBrandingAsset("logo.png", 100); err != nil {
		t.Fatalf("expected png accepted: %v", err)
	}
	if err := validator.ValidateBrandingAsset("payload.exe", 100); err == nil {
		t.Fatal("expected executable rejected")
	}
	if err := validator.ValidateBrandingAsset("logo.svg", 100); err == nil {
		t.Fatal("expected unsupported asset rejected")
	}
}

func TestSHA256Hex(t *testing.T) {
	sum, err := SHA256Hex(strings.NewReader("opendesk"))
	if err != nil {
		t.Fatalf("hash failed: %v", err)
	}
	if len(sum) != 64 {
		t.Fatalf("expected sha256 hex, got %q", sum)
	}
}
