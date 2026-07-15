package config

import (
	"bufio"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env                   string
	HTTPAddr              string
	PublicURL             string
	APIURL                string
	MySQLDSN              string
	RedisAddr             string
	RedisPassword         string
	StorageDriver         string
	StorageLocalPath      string
	JWTSecret             string
	RelayGrantSigningKey  string
	RelayAuthRequired     bool
	InitialAdminEmail     string
	InitialAdminPassword  string
	SetupToken            string
	BrandDefaultName      string
	AuthTokenTTL          time.Duration
	RelayGrantTTL         time.Duration
	AllowedCORSOrigins    []string
	BrandingAssetMaxBytes int64
	RateLimitEnabled      bool
	RateLimitRequests     int
	RateLimitWindow       time.Duration
	BuilderBinary         string
	BuilderWorkDir        string
	BuilderSourceDir      string
	BuilderWindowsCommand string
	BuilderArtifactGlob   string
	BuilderDryRun         bool
	BuilderTimeout        time.Duration
}

func Load() (Config, error) {
	loadDotEnv(".env")
	cfg := Config{
		Env:                   stringEnv("OPENDESK_ENV", "development"),
		HTTPAddr:              stringEnv("OPENDESK_HTTP_ADDR", ":21114"),
		PublicURL:             stringEnv("OPENDESK_PUBLIC_URL", "http://localhost:21114"),
		APIURL:                stringEnv("OPENDESK_API_URL", "http://localhost:21114"),
		MySQLDSN:              os.Getenv("OPENDESK_MYSQL_DSN"),
		RedisAddr:             stringEnv("OPENDESK_REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword:         os.Getenv("OPENDESK_REDIS_PASSWORD"),
		StorageDriver:         stringEnv("OPENDESK_STORAGE_DRIVER", "local"),
		StorageLocalPath:      stringEnv("OPENDESK_STORAGE_LOCAL_PATH", "./storage"),
		JWTSecret:             stringEnv("OPENDESK_JWT_SECRET", "dev-only-opendesk-session-signing-key-change-me"),
		RelayGrantSigningKey:  os.Getenv("OPENDESK_RELAY_GRANT_SIGNING_KEY"),
		RelayAuthRequired:     boolEnv("OPENDESK_RELAY_AUTH_REQUIRED", true),
		InitialAdminEmail:     os.Getenv("OPENDESK_INITIAL_ADMIN_EMAIL"),
		InitialAdminPassword:  os.Getenv("OPENDESK_INITIAL_ADMIN_PASSWORD"),
		SetupToken:            os.Getenv("OPENDESK_SETUP_TOKEN"),
		BrandDefaultName:      stringEnv("OPENDESK_BRAND_DEFAULT_NAME", "OpenDesk Remote"),
		AuthTokenTTL:          durationEnv("OPENDESK_AUTH_TOKEN_TTL", 15*time.Minute),
		RelayGrantTTL:         durationEnv("OPENDESK_RELAY_GRANT_TTL", 2*time.Minute),
		AllowedCORSOrigins:    csvEnv("OPENDESK_CORS_ORIGINS", "http://localhost:5173,http://127.0.0.1:5173"),
		BrandingAssetMaxBytes: int64Env("OPENDESK_BRANDING_ASSET_MAX_BYTES", 5*1024*1024),
		RateLimitEnabled:      boolEnv("OPENDESK_RATE_LIMIT_ENABLED", true),
		RateLimitRequests:     intEnv("OPENDESK_RATE_LIMIT_REQUESTS", 300),
		RateLimitWindow:       durationEnv("OPENDESK_RATE_LIMIT_WINDOW", time.Minute),
		BuilderBinary:         stringEnv("OPENDESK_BUILDER_BIN", "opendesk-builder"),
		BuilderWorkDir:        stringEnv("OPENDESK_BUILDER_WORK_DIR", ".run/build-worker"),
		BuilderSourceDir:      stringEnv("OPENDESK_BUILDER_SOURCE_DIR", ".upstream/rustdesk-client"),
		BuilderWindowsCommand: os.Getenv("OPENDESK_BUILDER_WINDOWS_COMMAND"),
		BuilderArtifactGlob:   os.Getenv("OPENDESK_BUILDER_WINDOWS_ARTIFACT_GLOB"),
		BuilderDryRun:         boolEnv("OPENDESK_BUILDER_DRY_RUN", true),
		BuilderTimeout:        durationEnv("OPENDESK_BUILDER_TIMEOUT", 2*time.Hour),
	}
	return cfg, cfg.Validate()
}

func loadDotEnv(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(key) == "" {
			continue
		}
		key = strings.TrimSpace(key)
		if os.Getenv(key) != "" {
			continue
		}
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		_ = os.Setenv(key, value)
	}
}

func (c Config) Validate() error {
	if c.StorageDriver != "local" {
		return errors.New("only OPENDESK_STORAGE_DRIVER=local is supported in the first version")
	}
	if c.RelayAuthRequired && strings.TrimSpace(c.RelayGrantSigningKey) == "" {
		return errors.New("OPENDESK_RELAY_GRANT_SIGNING_KEY is required when relay auth is enabled")
	}
	if c.RateLimitEnabled {
		if c.RateLimitRequests <= 0 {
			return errors.New("OPENDESK_RATE_LIMIT_REQUESTS must be positive when rate limit is enabled")
		}
		if c.RateLimitWindow <= 0 {
			return errors.New("OPENDESK_RATE_LIMIT_WINDOW must be positive when rate limit is enabled")
		}
	}
	if strings.TrimSpace(c.InitialAdminEmail) == "" && strings.TrimSpace(c.InitialAdminPassword) != "" {
		return errors.New("OPENDESK_INITIAL_ADMIN_EMAIL is required when OPENDESK_INITIAL_ADMIN_PASSWORD is set")
	}
	if strings.TrimSpace(c.InitialAdminEmail) != "" && strings.TrimSpace(c.InitialAdminPassword) == "" {
		return errors.New("OPENDESK_INITIAL_ADMIN_PASSWORD is required when OPENDESK_INITIAL_ADMIN_EMAIL is set")
	}
	if strings.TrimSpace(c.InitialAdminPassword) != "" && weakPassword(c.InitialAdminPassword) {
		return errors.New("weak initial admin password is not allowed")
	}
	if strings.TrimSpace(c.SetupToken) != "" && weakSecret(c.SetupToken) {
		return errors.New("weak OPENDESK_SETUP_TOKEN is not allowed")
	}
	if c.Env == "production" {
		if strings.TrimSpace(c.MySQLDSN) == "" {
			return errors.New("OPENDESK_MYSQL_DSN is required in production")
		}
		if weakSecret(c.JWTSecret) {
			return errors.New("strong OPENDESK_JWT_SECRET is required in production")
		}
	}
	return nil
}

func weakSecret(value string) bool {
	trimmed := strings.TrimSpace(value)
	return len(trimmed) < 32 || strings.Contains(trimmed, "change-this") || strings.Contains(trimmed, "dev-only")
}

func weakPassword(value string) bool {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	switch trimmed {
	case "", "admin", "password", "test1234", "change-this-in-production":
		return true
	default:
		return len(trimmed) < 12
	}
}

func stringEnv(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func boolEnv(name string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(name)))
	if value == "" {
		return fallback
	}
	return value == "1" || value == "true" || value == "yes" || value == "on"
}

func durationEnv(name string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func int64Env(name string, fallback int64) int64 {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func intEnv(name string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func csvEnv(name, fallback string) []string {
	raw := stringEnv(name, fallback)
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
