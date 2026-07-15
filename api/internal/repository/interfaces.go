package repository

import (
	"context"
	"time"

	"github.com/opendesk-remote/opendesk-remote/api/internal/models"
)

type UserRepository interface {
	CountUsers(ctx context.Context) (int, error)
	ListUsers(ctx context.Context) ([]models.User, error)
	CreateUser(ctx context.Context, user models.User) (models.User, error)
	FindUserByEmail(ctx context.Context, email string) (models.User, error)
	FindUserByID(ctx context.Context, id int64) (models.User, error)
	UpdateUser(ctx context.Context, id int64, user models.User) (models.User, error)
	DisableUser(ctx context.Context, id int64, at time.Time) (models.User, error)
	RecordUserLogin(ctx context.Context, id int64, at time.Time) (models.User, error)
	ListAPITokens(ctx context.Context) ([]models.APIToken, error)
	CreateAPIToken(ctx context.Context, token models.APIToken) (models.APIToken, error)
	FindAPITokenByHash(ctx context.Context, tokenHash string) (models.APIToken, error)
	RevokeAPIToken(ctx context.Context, id int64, at time.Time) (models.APIToken, error)
	RecordAPITokenUsed(ctx context.Context, id int64, at time.Time) (models.APIToken, error)
	CreateSession(ctx context.Context, session models.Session) (models.Session, error)
	FindSessionByTokenHash(ctx context.Context, tokenHash string) (models.Session, error)
	RevokeSession(ctx context.Context, tokenHash string, at time.Time) (models.Session, error)
	CreateLoginLog(ctx context.Context, log models.LoginLog) (models.LoginLog, error)
	ListUserGroups(ctx context.Context) ([]models.UserGroup, error)
	CreateUserGroup(ctx context.Context, group models.UserGroup) (models.UserGroup, error)
	ListUserGroupMembers(ctx context.Context, groupID int64) ([]int64, error)
	AddUserGroupMember(ctx context.Context, groupID, userID int64) (models.UserGroup, error)
	RemoveUserGroupMember(ctx context.Context, groupID, userID int64) (models.UserGroup, error)
}

type DeviceRepository interface {
	ListDevices(ctx context.Context) ([]models.Device, error)
	CreateDevice(ctx context.Context, device models.Device) (models.Device, error)
	FindDeviceByID(ctx context.Context, id int64) (models.Device, error)
	UpdateDevice(ctx context.Context, id int64, device models.Device) (models.Device, error)
	DisableDevice(ctx context.Context, id int64, at time.Time) (models.Device, error)
	ListDeviceGroups(ctx context.Context) ([]models.DeviceGroup, error)
	CreateDeviceGroup(ctx context.Context, group models.DeviceGroup) (models.DeviceGroup, error)
	ListDeviceGroupMembers(ctx context.Context, groupID int64) ([]int64, error)
	AddDeviceGroupMember(ctx context.Context, groupID, deviceID int64) (models.DeviceGroup, error)
	RemoveDeviceGroupMember(ctx context.Context, groupID, deviceID int64) (models.DeviceGroup, error)
}

type RelayRepository interface {
	ListRelays(ctx context.Context) ([]models.Relay, error)
	CreateRelay(ctx context.Context, relay models.Relay) (models.Relay, error)
	UpdateRelay(ctx context.Context, id int64, relay models.Relay) (models.Relay, error)
	RelayHeartbeat(ctx context.Context, id int64, currentSessions *int, status string) (models.Relay, error)
	DisableRelay(ctx context.Context, id int64, at time.Time) (models.Relay, error)
	CreateRelayGrant(ctx context.Context, grant models.RelayGrant) (models.RelayGrant, error)
	FindRelayGrantByGrantID(ctx context.Context, grantID string) (models.RelayGrant, error)
	UpdateRelayGrantStatus(ctx context.Context, grantID string, status string, usedAt *time.Time) (models.RelayGrant, error)
}

type PolicyRepository interface {
	ListAccessRules(ctx context.Context) ([]models.AccessRule, error)
	CreateAccessRule(ctx context.Context, rule models.AccessRule) (models.AccessRule, error)
	UpdateAccessRule(ctx context.Context, id int64, rule models.AccessRule) (models.AccessRule, error)
	DeleteAccessRule(ctx context.Context, id int64) error
	ListControlRoles(ctx context.Context) ([]models.ControlRole, error)
	CreateControlRole(ctx context.Context, role models.ControlRole) (models.ControlRole, error)
	UpdateControlRole(ctx context.Context, id int64, role models.ControlRole) (models.ControlRole, error)
	DeleteControlRole(ctx context.Context, id int64) error
	ListStrategies(ctx context.Context) ([]models.Strategy, error)
	CreateStrategy(ctx context.Context, strategy models.Strategy) (models.Strategy, error)
	UpdateStrategy(ctx context.Context, id int64, strategy models.Strategy) (models.Strategy, error)
	DeleteStrategy(ctx context.Context, id int64) error
	AddStrategyAssignment(ctx context.Context, strategyID int64, assignment models.StrategyAssignment) (models.Strategy, error)
	RemoveStrategyAssignment(ctx context.Context, strategyID int64, assignmentID int64) (models.Strategy, error)
}

type LogsRepository interface {
	CreateAuditEvent(ctx context.Context, event models.AuditEvent) (models.AuditEvent, error)
	ListAuditEvents(ctx context.Context, filter AuditLogFilter) ([]models.AuditEvent, error)
	CreateConnectionLog(ctx context.Context, log models.ConnectionLog) (models.ConnectionLog, error)
	ListConnectionLogs(ctx context.Context, filter ConnectionLogFilter) ([]models.ConnectionLog, error)
	ListFileTransferLogs(ctx context.Context, filter FileTransferLogFilter) ([]models.FileTransferLog, error)
	ListLoginLogs(ctx context.Context, filter LoginLogFilter) ([]models.LoginLog, error)
}

type AuditLogFilter struct {
	ActorType    string
	Action       string
	ResourceType string
	ResourceID   string
	From         *time.Time
	To           *time.Time
	Limit        int
	Offset       int
}

type ConnectionLogFilter struct {
	Status         string
	ConnectionType string
	From           *time.Time
	To             *time.Time
	Limit          int
	Offset         int
}

type FileTransferLogFilter struct {
	Direction string
	Status    string
	From      *time.Time
	To        *time.Time
	Limit     int
	Offset    int
}

type LoginLogFilter struct {
	Email  string
	Status string
	From   *time.Time
	To     *time.Time
	Limit  int
	Offset int
}

type SettingsRepository interface {
	ListSystemSettings(ctx context.Context) ([]models.SystemSetting, error)
	UpsertSystemSettings(ctx context.Context, settings []models.SystemSetting, updatedBy int64) error
}

type AddressBookRepository interface {
	ListAddressBooks(ctx context.Context) ([]models.AddressBook, error)
	CreateAddressBook(ctx context.Context, book models.AddressBook) (models.AddressBook, error)
	ListAddressBookEntries(ctx context.Context, bookID int64) ([]models.AddressBookEntry, error)
	AddAddressBookEntry(ctx context.Context, bookID int64, entry models.AddressBookEntry) (models.AddressBook, error)
	RemoveAddressBookEntry(ctx context.Context, bookID, entryID int64) (models.AddressBook, error)
}

type BuildRepository interface {
	ListBuildProfiles(ctx context.Context) ([]models.BuildProfile, error)
	CreateBuildProfile(ctx context.Context, profile models.BuildProfile) (models.BuildProfile, error)
	FindBuildProfileByID(ctx context.Context, id int64) (models.BuildProfile, error)
	UpdateBuildProfile(ctx context.Context, id int64, profile models.BuildProfile) (models.BuildProfile, error)
	DeleteBuildProfile(ctx context.Context, id int64) error
	ListBuildJobs(ctx context.Context) ([]models.BuildJob, error)
	CreateBuildJob(ctx context.Context, job models.BuildJob) (models.BuildJob, error)
	FindBuildJobByID(ctx context.Context, id int64) (models.BuildJob, error)
	ClaimNextBuildJob(ctx context.Context, runner string, at time.Time) (models.BuildJob, error)
	CompleteBuildJob(ctx context.Context, id int64, logPath string, at time.Time) (models.BuildJob, error)
	FailBuildJob(ctx context.Context, id int64, logPath string, message string, at time.Time) (models.BuildJob, error)
	CancelBuildJob(ctx context.Context, id int64, message string, at time.Time) (models.BuildJob, error)
	CreateBuildArtifact(ctx context.Context, artifact models.BuildArtifact) (models.BuildArtifact, error)
	ListBuildArtifacts(ctx context.Context, buildJobID int64) ([]models.BuildArtifact, error)
	FindBuildArtifactByID(ctx context.Context, id int64) (models.BuildArtifact, error)
}

type Store interface {
	UserRepository
	DeviceRepository
	RelayRepository
	PolicyRepository
	LogsRepository
	SettingsRepository
	AddressBookRepository
	BuildRepository
}
