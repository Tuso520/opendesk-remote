package app

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/opendesk-remote/opendesk-remote/api/internal/addressbooks"
	"github.com/opendesk-remote/opendesk-remote/api/internal/apitokens"
	"github.com/opendesk-remote/opendesk-remote/api/internal/assets"
	"github.com/opendesk-remote/opendesk-remote/api/internal/audit"
	"github.com/opendesk-remote/opendesk-remote/api/internal/auth"
	"github.com/opendesk-remote/opendesk-remote/api/internal/builds"
	"github.com/opendesk-remote/opendesk-remote/api/internal/buildworker"
	"github.com/opendesk-remote/opendesk-remote/api/internal/config"
	"github.com/opendesk-remote/opendesk-remote/api/internal/devices"
	"github.com/opendesk-remote/opendesk-remote/api/internal/health"
	"github.com/opendesk-remote/opendesk-remote/api/internal/middleware"
	"github.com/opendesk-remote/opendesk-remote/api/internal/opslogs"
	"github.com/opendesk-remote/opendesk-remote/api/internal/policies"
	"github.com/opendesk-remote/opendesk-remote/api/internal/redisconn"
	"github.com/opendesk-remote/opendesk-remote/api/internal/relaygrant"
	"github.com/opendesk-remote/opendesk-remote/api/internal/relays"
	"github.com/opendesk-remote/opendesk-remote/api/internal/repository"
	"github.com/opendesk-remote/opendesk-remote/api/internal/settings"
	"github.com/opendesk-remote/opendesk-remote/api/internal/setup"
	"github.com/opendesk-remote/opendesk-remote/api/internal/storage"
	"github.com/opendesk-remote/opendesk-remote/api/internal/users"
)

const version = "0.1.0"

func NewRouter(cfg config.Config, logger *slog.Logger) http.Handler {
	return NewRouterWithStore(cfg, logger, fallbackMemoryStore(cfg, logger))
}

func NewRouterWithStore(cfg config.Config, logger *slog.Logger, repo repository.Store) http.Handler {
	mux := http.NewServeMux()
	healthHandler := health.NewHandlerWithDeps(version, redisconn.New(cfg.RedisAddr, cfg.RedisPassword), cfg.Env == "production", strings.TrimSpace(cfg.MySQLDSN) != "", cfg.StorageDriver)
	relayService := relaygrant.NewService([]byte(cfg.RelayGrantSigningKey), cfg.RelayGrantTTL).WithStore(repo)
	relayHandler := relaygrant.NewHandler(relayService, audit.RepositoryWriter{Repo: repo}).WithConnectionLogWriter(repo)
	authService := auth.NewService(repo, auth.Config{SigningKey: []byte(cfg.JWTSecret), TokenTTL: cfg.AuthTokenTTL})
	authHandler := auth.NewHandler(authService, cfg.Env == "production")
	apiTokenHandler := apitokens.NewHandler(repo, cfg.JWTSecret)
	userHandler := users.NewHandler(repo)
	deviceHandler := devices.NewHandler(repo)
	addressBookHandler := addressbooks.NewHandler(repo)
	relayNodeHandler := relays.NewHandler(repo)
	policyHandler := policies.NewHandler(repo)
	logsHandler := opslogs.NewHandler(repo)
	settingsHandler := settings.NewHandler(repo)
	setupHandler := setup.NewHandler(repo, setup.Config{SetupToken: cfg.SetupToken})
	buildHandler := builds.NewHandler(repo, buildworker.New(repo, buildworker.ConfigFromAPI(cfg), buildworker.CLIExecutor{}))
	assetHandler := assets.NewHandler(storage.LocalStore{Root: cfg.StorageLocalPath}, storage.AssetValidator{MaxBytes: cfg.BrandingAssetMaxBytes}, audit.RepositoryWriter{Repo: repo})

	mux.HandleFunc("/api/v1/health", healthHandler.Health)
	mux.HandleFunc("/api/v1/ready", healthHandler.Ready)
	mux.HandleFunc("/api/v1/version", healthHandler.VersionHandler)
	mux.HandleFunc("/api/v1/auth/login", authHandler.Login)
	mux.HandleFunc("/api/v1/auth/logout", authHandler.Logout)
	mux.HandleFunc("/api/v1/auth/me", authHandler.Me)
	mux.HandleFunc("/api/v1/setup/status", setupHandler.Status)
	mux.HandleFunc("/api/v1/setup/admin", setupHandler.Admin)
	mux.HandleFunc("/api/v1/api-tokens", apiTokenHandler.Collection)
	mux.HandleFunc("/api/v1/api-tokens/", apiTokenHandler.Item)
	mux.HandleFunc("/api/v1/users", userHandler.Collection)
	mux.HandleFunc("/api/v1/users/", userHandler.Item)
	mux.HandleFunc("/api/v1/user-groups", userHandler.Groups)
	mux.HandleFunc("/api/v1/user-groups/", userHandler.GroupItem)
	mux.HandleFunc("/api/v1/devices", deviceHandler.Collection)
	mux.HandleFunc("/api/v1/devices/register", deviceHandler.Register)
	mux.HandleFunc("/api/v1/devices/", deviceHandler.Item)
	mux.HandleFunc("/api/v1/device-groups", deviceHandler.Groups)
	mux.HandleFunc("/api/v1/device-groups/", deviceHandler.GroupItem)
	mux.HandleFunc("/api/v1/address-books", addressBookHandler.Collection)
	mux.HandleFunc("/api/v1/address-books/", addressBookHandler.Item)
	mux.HandleFunc("/api/v1/relays", relayNodeHandler.Collection)
	mux.HandleFunc("/api/v1/relays/", relayNodeHandler.Item)
	mux.HandleFunc("/api/v1/access-rules", policyHandler.AccessRules)
	mux.HandleFunc("/api/v1/access-rules/", policyHandler.AccessRuleItem)
	mux.HandleFunc("/api/v1/access/evaluate", policyHandler.EvaluateAccess)
	mux.HandleFunc("/api/v1/control-roles", policyHandler.ControlRoles)
	mux.HandleFunc("/api/v1/control-roles/", policyHandler.ControlRoleItem)
	mux.HandleFunc("/api/v1/strategies", policyHandler.Strategies)
	mux.HandleFunc("/api/v1/strategies/", policyHandler.StrategyItem)
	mux.HandleFunc("/api/v1/relay-grants", relayHandler.Issue)
	mux.HandleFunc("/api/v1/relay-grants/validate", relayHandler.Validate)
	mux.HandleFunc("/api/v1/relay-grants/revoke", relayHandler.Revoke)
	mux.HandleFunc("/api/v1/relay-grants/", relayHandler.RevokeByPath)
	mux.HandleFunc("/api/v1/build-profiles", buildHandler.Profiles)
	mux.HandleFunc("/api/v1/client-build-configs", buildHandler.Profiles)
	mux.HandleFunc("/api/v1/build-worker/doctor", buildHandler.Doctor)
	mux.HandleFunc("/api/v1/build-jobs", buildHandler.Jobs)
	mux.HandleFunc("/api/v1/build-jobs/run-next", buildHandler.RunNext)
	mux.HandleFunc("/api/v1/build-jobs/", buildHandler.JobItem)
	mux.HandleFunc("/api/v1/build-artifacts/", buildHandler.ArtifactItem)
	mux.HandleFunc("/api/v1/build-profiles/", buildHandler.ProfileItem)
	mux.HandleFunc("/api/v1/client-build-configs/", buildHandler.ProfileItem)
	mux.HandleFunc("/api/v1/logs/audit", logsHandler.Audit)
	mux.HandleFunc("/api/v1/logs/connections", logsHandler.Connections)
	mux.HandleFunc("/api/v1/logs/file-transfers", logsHandler.FileTransfers)
	mux.HandleFunc("/api/v1/logs/logins", logsHandler.Logins)
	mux.HandleFunc("/api/v1/settings", settingsHandler.Collection)
	mux.HandleFunc("/api/v1/settings/oidc", settingsHandler.OIDC)
	mux.HandleFunc("/api/v1/settings/ldap", settingsHandler.LDAP)
	mux.HandleFunc("/api/v1/settings/smtp", settingsHandler.SMTP)
	mux.HandleFunc("/api/v1/assets/branding", assetHandler.Branding)
	var handler http.Handler = mux
	handler = authService.Require(handler, publicRoute)
	handler = middleware.SecurityHeaders(handler)
	handler = middleware.RateLimit(middleware.RateLimitConfig{Enabled: cfg.RateLimitEnabled, Requests: cfg.RateLimitRequests, Window: cfg.RateLimitWindow}, handler)
	handler = middleware.CORS(cfg.AllowedCORSOrigins, handler)
	handler = middleware.Logger(logger, handler)
	handler = middleware.RequestID(handler)
	return handler
}

func publicRoute(r *http.Request) bool {
	switch r.URL.Path {
	case "/api/v1/health", "/api/v1/ready", "/api/v1/version", "/api/v1/auth/login", "/api/v1/auth/logout", "/api/v1/relay-grants/validate", "/api/v1/setup/status", "/api/v1/setup/admin":
		return true
	}
	return strings.HasPrefix(r.URL.Path, "/api/v1/relays/") && strings.HasSuffix(r.URL.Path, "/heartbeat")
}

func initialAdminEmail(cfg config.Config) string {
	if strings.TrimSpace(cfg.InitialAdminEmail) != "" {
		return cfg.InitialAdminEmail
	}
	return "admin@example.com"
}

func initialAdminPassword(cfg config.Config) string {
	if cfg.InitialAdminPassword != "" {
		return cfg.InitialAdminPassword
	}
	return "change-this-in-production"
}
