package models

import "time"

type UserStatus string

const (
	UserStatusActive   UserStatus = "active"
	UserStatusDisabled UserStatus = "disabled"
	UserStatusLocked   UserStatus = "locked"
)

type DeviceStatus string

const (
	DeviceStatusOnline   DeviceStatus = "online"
	DeviceStatusOffline  DeviceStatus = "offline"
	DeviceStatusDisabled DeviceStatus = "disabled"
)

type User struct {
	ID           int64      `json:"id"`
	Email        string     `json:"email"`
	Username     string     `json:"username"`
	DisplayName  string     `json:"display_name"`
	PasswordHash string     `json:"-"`
	Status       UserStatus `json:"status"`
	MFAEnabled   bool       `json:"mfa_enabled"`
	Source       string     `json:"source"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	LastLoginAt  *time.Time `json:"last_login_at,omitempty"`
}

type APIToken struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	TokenHash  string     `json:"-"`
	ScopesJSON string     `json:"scopes_json"`
	UserID     *int64     `json:"user_id,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
}

type Session struct {
	ID        int64      `json:"id"`
	UserID    int64      `json:"user_id"`
	TokenHash string     `json:"-"`
	IP        string     `json:"ip"`
	UserAgent string     `json:"user_agent"`
	ExpiresAt time.Time  `json:"expires_at"`
	CreatedAt time.Time  `json:"created_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
}

type Group struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type UserGroup struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	MemberUserIDs []int64   `json:"member_user_ids"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type DeviceGroup struct {
	ID              int64     `json:"id"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	MemberDeviceIDs []int64   `json:"member_device_ids"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type Device struct {
	ID                    int64        `json:"id"`
	RustDeskID            string       `json:"rustdesk_id"`
	Name                  string       `json:"name"`
	Alias                 string       `json:"alias"`
	OwnerUserID           *int64       `json:"owner_user_id,omitempty"`
	Status                DeviceStatus `json:"status"`
	Platform              string       `json:"platform"`
	ClientVersion         string       `json:"client_version"`
	OpenDeskClientVersion string       `json:"opendesk_client_version"`
	LastIP                string       `json:"last_ip"`
	LastSeenAt            *time.Time   `json:"last_seen_at,omitempty"`
	RegisteredAt          *time.Time   `json:"registered_at,omitempty"`
	CreatedAt             time.Time    `json:"created_at"`
	UpdatedAt             time.Time    `json:"updated_at"`
}

type AddressBook struct {
	ID          int64              `json:"id"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	OwnerUserID *int64             `json:"owner_user_id,omitempty"`
	Entries     []AddressBookEntry `json:"entries"`
	CreatedAt   time.Time          `json:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at"`
}

type AddressBookEntry struct {
	ID            int64     `json:"id"`
	AddressBookID int64     `json:"address_book_id"`
	DeviceID      int64     `json:"device_id"`
	Alias         string    `json:"alias"`
	CreatedAt     time.Time `json:"created_at"`
}

type Relay struct {
	ID                   int64      `json:"id"`
	Name                 string     `json:"name"`
	Region               string     `json:"region"`
	Host                 string     `json:"host"`
	Port                 int        `json:"port"`
	WSPort               int        `json:"ws_port"`
	PublicKeyFingerprint string     `json:"public_key_fingerprint"`
	Status               string     `json:"status"`
	MaxBandwidthMbps     *int       `json:"max_bandwidth_mbps,omitempty"`
	CurrentSessions      int        `json:"current_sessions"`
	LastHealthAt         *time.Time `json:"last_health_at,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

type RelayGrant struct {
	ID                 int64      `json:"id"`
	GrantID            string     `json:"grant_id"`
	UserID             *int64     `json:"user_id,omitempty"`
	ControllerDeviceID *int64     `json:"controller_device_id,omitempty"`
	TargetDeviceID     *int64     `json:"target_device_id,omitempty"`
	TargetRustDeskID   string     `json:"target_rustdesk_id,omitempty"`
	RelayID            *int64     `json:"relay_id,omitempty"`
	AllowedRelays      []string   `json:"allowed_relays"`
	ExpiresAt          time.Time  `json:"expires_at"`
	Nonce              string     `json:"nonce"`
	Status             string     `json:"status"`
	CreatedAt          time.Time  `json:"created_at"`
	UsedAt             *time.Time `json:"used_at,omitempty"`
}

type AuditEvent struct {
	ID           int64          `json:"id"`
	ActorUserID  *int64         `json:"actor_user_id,omitempty"`
	ActorType    string         `json:"actor_type"`
	Action       string         `json:"action"`
	ResourceType string         `json:"resource_type"`
	ResourceID   *string        `json:"resource_id,omitempty"`
	IP           string         `json:"ip"`
	UserAgent    string         `json:"user_agent"`
	Metadata     map[string]any `json:"metadata_json"`
	CreatedAt    time.Time      `json:"created_at"`
}

type ConnectionLog struct {
	ID                 int64          `json:"id"`
	SessionID          *int64         `json:"session_id,omitempty"`
	ControllerUserID   *int64         `json:"controller_user_id,omitempty"`
	ControllerDeviceID *int64         `json:"controller_device_id,omitempty"`
	TargetDeviceID     *int64         `json:"target_device_id,omitempty"`
	ConnectionType     string         `json:"connection_type"`
	RelayID            *int64         `json:"relay_id,omitempty"`
	StartedAt          time.Time      `json:"started_at"`
	EndedAt            *time.Time     `json:"ended_at,omitempty"`
	Status             string         `json:"status"`
	DenyReason         string         `json:"deny_reason,omitempty"`
	Metadata           map[string]any `json:"metadata_json"`
}

type FileTransferLog struct {
	ID              int64     `json:"id"`
	ConnectionLogID *int64    `json:"connection_log_id,omitempty"`
	Direction       string    `json:"direction"`
	FilenameHash    string    `json:"filename_hash"`
	SizeBytes       int64     `json:"size_bytes"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
}

type LoginLog struct {
	ID            int64      `json:"id"`
	UserID        int64      `json:"user_id,omitempty"`
	Email         string     `json:"email"`
	Username      string     `json:"username,omitempty"`
	DisplayName   string     `json:"display_name,omitempty"`
	Status        string     `json:"status"`
	FailureReason string     `json:"failure_reason,omitempty"`
	IP            string     `json:"ip,omitempty"`
	UserAgent     string     `json:"user_agent,omitempty"`
	LastLoginAt   *time.Time `json:"last_login_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

type SystemSetting struct {
	Key       string    `json:"key"`
	ValueJSON string    `json:"value_json"`
	UpdatedBy *int64    `json:"updated_by,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

type AccessRule struct {
	ID          int64     `json:"id"`
	SubjectType string    `json:"subject_type"`
	SubjectID   int64     `json:"subject_id"`
	TargetType  string    `json:"target_type"`
	TargetID    int64     `json:"target_id"`
	Effect      string    `json:"effect"`
	Priority    int       `json:"priority"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ControlRole struct {
	ID          int64                   `json:"id"`
	Name        string                  `json:"name"`
	Description string                  `json:"description"`
	Enabled     bool                    `json:"enabled"`
	Permissions []ControlRolePermission `json:"permissions"`
	CreatedAt   time.Time               `json:"created_at"`
	UpdatedAt   time.Time               `json:"updated_at"`
}

type ControlRolePermission struct {
	ID            int64  `json:"id"`
	RoleID        int64  `json:"role_id"`
	PermissionKey string `json:"permission_key"`
	Mode          string `json:"mode"`
}

type Strategy struct {
	ID           int64                `json:"id"`
	Name         string               `json:"name"`
	Description  string               `json:"description"`
	Enabled      bool                 `json:"enabled"`
	SettingsJSON string               `json:"settings_json"`
	Assignments  []StrategyAssignment `json:"assignments"`
	CreatedAt    time.Time            `json:"created_at"`
	UpdatedAt    time.Time            `json:"updated_at"`
}

type StrategyAssignment struct {
	ID         int64     `json:"id"`
	StrategyID int64     `json:"strategy_id"`
	TargetType string    `json:"target_type"`
	TargetID   int64     `json:"target_id"`
	CreatedAt  time.Time `json:"created_at"`
}

type BuildProfile struct {
	ID               int64     `json:"id"`
	Name             string    `json:"name"`
	AppName          string    `json:"app_name"`
	Vendor           string    `json:"vendor"`
	BundleID         string    `json:"bundle_id"`
	ProductName      string    `json:"product_name"`
	Description      string    `json:"description"`
	ServerConfigJSON string    `json:"server_config_json"`
	BrandingJSON     string    `json:"branding_json"`
	PolicyJSON       string    `json:"policy_json"`
	PlatformsJSON    string    `json:"platforms_json"`
	SigningJSON      string    `json:"signing_json"`
	SourceJSON       string    `json:"source_json"`
	CreatedBy        int64     `json:"created_by"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type BuildJobStatus string

const (
	BuildJobQueued    BuildJobStatus = "queued"
	BuildJobRunning   BuildJobStatus = "running"
	BuildJobSucceeded BuildJobStatus = "succeeded"
	BuildJobFailed    BuildJobStatus = "failed"
	BuildJobCanceled  BuildJobStatus = "canceled"
)

type BuildJob struct {
	ID           int64          `json:"id"`
	ProfileID    int64          `json:"profile_id"`
	Platform     string         `json:"platform"`
	Status       BuildJobStatus `json:"status"`
	Runner       string         `json:"runner"`
	LogPath      string         `json:"log_path"`
	ErrorMessage string         `json:"error_message,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	StartedAt    *time.Time     `json:"started_at,omitempty"`
	FinishedAt   *time.Time     `json:"finished_at,omitempty"`
}

type BuildArtifact struct {
	ID         int64     `json:"id"`
	BuildJobID int64     `json:"build_job_id"`
	Platform   string    `json:"platform"`
	Filename   string    `json:"filename"`
	LocalPath  string    `json:"local_path"`
	SHA256     string    `json:"sha256"`
	SizeBytes  int64     `json:"size_bytes"`
	CreatedAt  time.Time `json:"created_at"`
}
