package assets

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/opendesk-remote/opendesk-remote/api/internal/audit"
	apiauth "github.com/opendesk-remote/opendesk-remote/api/internal/auth"
	"github.com/opendesk-remote/opendesk-remote/api/internal/httpx"
	"github.com/opendesk-remote/opendesk-remote/api/internal/storage"
)

type Handler struct {
	store       storage.LocalStore
	validator   storage.AssetValidator
	auditWriter audit.Writer
}

type BrandingAssetResponse struct {
	ID       string `json:"id"`
	URI      string `json:"uri"`
	Filename string `json:"filename"`
	SHA256   string `json:"sha256"`
	Size     int64  `json:"size_bytes"`
}

func NewHandler(store storage.LocalStore, validator storage.AssetValidator, auditWriters ...audit.Writer) Handler {
	var writer audit.Writer
	if len(auditWriters) > 0 {
		writer = auditWriters[0]
	}
	return Handler{store: store, validator: validator, auditWriter: writer}
}

func (h Handler) Branding(w http.ResponseWriter, r *http.Request) {
	if !httpx.Method(w, r, http.MethodPost) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, h.validator.MaxBytes+1024*1024)
	if err := r.ParseMultipartForm(h.validator.MaxBytes); err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_BRANDING_ASSET", "multipart asset upload is required")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_BRANDING_ASSET", "file field is required")
		return
	}
	defer file.Close()
	response, err := h.saveBrandingAsset(file, header)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_BRANDING_ASSET", err.Error())
		return
	}
	if h.auditWriter != nil {
		if err := h.auditWriter.Write(r.Context(), brandingUploadAuditEvent(r, response)); err != nil {
			httpx.Error(w, http.StatusInternalServerError, "AUDIT_WRITE_FAILED", "failed to record branding asset upload")
			return
		}
	}
	httpx.JSON(w, http.StatusCreated, response)
}

func (h Handler) saveBrandingAsset(file multipart.File, header *multipart.FileHeader) (BrandingAssetResponse, error) {
	if header == nil {
		return BrandingAssetResponse{}, errors.New("file field is required")
	}
	originalName := filepath.Base(header.Filename)
	if originalName == "." || originalName == string(filepath.Separator) || strings.TrimSpace(originalName) == "" {
		return BrandingAssetResponse{}, errors.New("filename is required")
	}
	if err := h.validator.ValidateBrandingAsset(originalName, header.Size); err != nil {
		return BrandingAssetResponse{}, err
	}
	id, err := randomID()
	if err != nil {
		return BrandingAssetResponse{}, err
	}
	ext := strings.ToLower(filepath.Ext(originalName))
	generatedName := id + ext
	targetPath, err := h.store.SafePath("branding", generatedName)
	if err != nil {
		return BrandingAssetResponse{}, err
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return BrandingAssetResponse{}, err
	}
	tmpPath := targetPath + ".tmp"
	out, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return BrandingAssetResponse{}, err
	}
	sha, size, copyErr := copyAndHash(out, file, h.validator.MaxBytes)
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(tmpPath)
		return BrandingAssetResponse{}, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return BrandingAssetResponse{}, closeErr
	}
	if err := h.validator.ValidateBrandingAsset(originalName, size); err != nil {
		_ = os.Remove(tmpPath)
		return BrandingAssetResponse{}, err
	}
	if err := os.Rename(tmpPath, targetPath); err != nil {
		_ = os.Remove(tmpPath)
		return BrandingAssetResponse{}, err
	}
	return BrandingAssetResponse{
		ID:       id,
		URI:      "asset://" + generatedName,
		Filename: generatedName,
		SHA256:   sha,
		Size:     size,
	}, nil
}

func copyAndHash(dst io.Writer, src io.Reader, maxBytes int64) (string, int64, error) {
	limited := io.LimitReader(src, maxBytes+1)
	reader := &countingReader{reader: limited}
	sha, err := storage.SHA256Hex(io.TeeReader(reader, dst))
	if err != nil {
		return "", reader.n, err
	}
	if reader.n > maxBytes {
		return "", reader.n, errors.New("branding asset size exceeds limit")
	}
	return sha, reader.n, nil
}

type countingReader struct {
	reader io.Reader
	n      int64
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.n += int64(n)
	return n, err
}

func randomID() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}

func brandingUploadAuditEvent(r *http.Request, asset BrandingAssetResponse) audit.Event {
	actorType := "system"
	var actorUserID *int64
	if session, ok := apiauth.SessionFromContext(r.Context()); ok {
		actorType = apiauth.ActorType(session)
		userID := session.User.ID
		actorUserID = &userID
	}
	return audit.Event{
		ActorUserID:  actorUserID,
		ActorType:    actorType,
		Action:       "upload_branding_asset",
		ResourceType: "branding_asset",
		ResourceID:   asset.ID,
		IP:           requestIP(r),
		UserAgent:    r.UserAgent(),
		Metadata: map[string]any{
			"filename":   asset.Filename,
			"sha256":     asset.SHA256,
			"size_bytes": asset.Size,
			"uri":        asset.URI,
		},
	}
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
