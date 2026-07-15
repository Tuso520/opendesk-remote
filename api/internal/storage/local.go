package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"mime"
	"path/filepath"
	"strings"
)

type LocalStore struct {
	Root string
}

func (s LocalStore) SafePath(parts ...string) (string, error) {
	root, err := filepath.Abs(s.Root)
	if err != nil {
		return "", err
	}
	all := append([]string{root}, parts...)
	clean, err := filepath.Abs(filepath.Join(all...))
	if err != nil {
		return "", err
	}
	if clean != root && !strings.HasPrefix(clean, root+string(filepath.Separator)) {
		return "", errors.New("path traversal rejected")
	}
	return clean, nil
}

type AssetValidator struct {
	MaxBytes int64
}

func (v AssetValidator) ValidateBrandingAsset(filename string, size int64) error {
	if size <= 0 || size > v.MaxBytes {
		return errors.New("branding asset size exceeds limit")
	}
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".png", ".ico", ".icns":
		return nil
	default:
		if mime.TypeByExtension(ext) == "application/x-msdownload" || ext == ".exe" || ext == ".dll" || ext == ".sh" {
			return errors.New("executable upload rejected")
		}
		return errors.New("unsupported branding asset type")
	}
}

func SHA256Hex(r io.Reader) (string, error) {
	hash := sha256.New()
	if _, err := io.Copy(hash, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
