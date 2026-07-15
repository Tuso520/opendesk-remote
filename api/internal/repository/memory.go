package repository

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/opendesk-remote/opendesk-remote/api/internal/models"
)

var ErrNotFound = errors.New("not found")
var ErrConflict = errors.New("conflict")

type Memory struct {
	mu             sync.Mutex
	nextID         int64
	users          []models.User
	apiTokens      []models.APIToken
	sessions       []models.Session
	userGroups     []models.UserGroup
	devices        []models.Device
	deviceGroups   []models.DeviceGroup
	addressBooks   []models.AddressBook
	relays         []models.Relay
	relayGrants    []models.RelayGrant
	accessRules    []models.AccessRule
	controlRoles   []models.ControlRole
	strategies     []models.Strategy
	auditEvents    []models.AuditEvent
	loginLogs      []models.LoginLog
	connectionLogs []models.ConnectionLog
	fileTransfers  []models.FileTransferLog
	settings       map[string]models.SystemSetting
	buildProfiles  []models.BuildProfile
	buildJobs      []models.BuildJob
	buildArtifacts []models.BuildArtifact
}

func NewMemory() *Memory {
	return NewMemoryWithInitialAdmin("admin@example.com", "")
}

func NewMemoryWithoutInitialAdmin() *Memory {
	return newMemoryWithUsersAndNext(nil, 0)
}

func NewMemoryWithInitialAdmin(email, passwordHash string) *Memory {
	now := time.Now().UTC()
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		email = "admin@example.com"
	}
	username, _, _ := strings.Cut(email, "@")
	if username == "" {
		username = "admin"
	}
	return newMemoryWithUsersAndNext([]models.User{
		{ID: 1, Email: email, Username: username, DisplayName: "Initial Admin", PasswordHash: passwordHash, Status: models.UserStatusActive, Source: "local", CreatedAt: now, UpdatedAt: now},
	}, 100)
}

func newMemoryWithUsersAndNext(users []models.User, nextID int64) *Memory {
	now := time.Now().UTC()
	return &Memory{
		nextID:     nextID,
		users:      users,
		apiTokens:  []models.APIToken{},
		sessions:   []models.Session{},
		userGroups: []models.UserGroup{},
		devices: []models.Device{
			{ID: 1, RustDeskID: "100000001", Name: "Demo Windows Workstation", Status: models.DeviceStatusOffline, Platform: "windows", OpenDeskClientVersion: "pending", CreatedAt: now, UpdatedAt: now},
		},
		deviceGroups: []models.DeviceGroup{},
		addressBooks: []models.AddressBook{},
		relays: []models.Relay{
			{ID: 1, Name: "hbbr-relay-a", Region: "region-a", Host: "remote.example.com", Port: 21117, WSPort: 21119, Status: "active", CreatedAt: now, UpdatedAt: now},
		},
		relayGrants:    []models.RelayGrant{},
		accessRules:    []models.AccessRule{},
		controlRoles:   []models.ControlRole{},
		strategies:     []models.Strategy{},
		auditEvents:    []models.AuditEvent{},
		loginLogs:      []models.LoginLog{},
		connectionLogs: []models.ConnectionLog{},
		fileTransfers:  []models.FileTransferLog{},
		settings:       map[string]models.SystemSetting{},
		buildProfiles:  []models.BuildProfile{},
		buildJobs:      []models.BuildJob{},
		buildArtifacts: []models.BuildArtifact{},
	}
}

func (m *Memory) CountUsers(ctx context.Context) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.users), nil
}

func (m *Memory) FindUserByEmail(ctx context.Context, email string) (models.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, user := range m.users {
		if strings.EqualFold(user.Email, email) {
			return user, nil
		}
	}
	return models.User{}, ErrNotFound
}

func (m *Memory) FindUserByID(ctx context.Context, id int64) (models.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, user := range m.users {
		if user.ID == id {
			return user, nil
		}
	}
	return models.User{}, ErrNotFound
}

func (m *Memory) UpdateUser(ctx context.Context, id int64, user models.User) (models.User, error) {
	if strings.TrimSpace(user.Email) == "" || strings.TrimSpace(user.Username) == "" {
		return models.User{}, errors.New("email and username are required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	for index := range m.users {
		if m.users[index].ID != id {
			continue
		}
		m.users[index].Email = strings.ToLower(strings.TrimSpace(user.Email))
		m.users[index].Username = strings.TrimSpace(user.Username)
		m.users[index].DisplayName = strings.TrimSpace(user.DisplayName)
		if m.users[index].DisplayName == "" {
			m.users[index].DisplayName = m.users[index].Username
		}
		if user.Status != "" {
			m.users[index].Status = user.Status
		}
		m.users[index].MFAEnabled = user.MFAEnabled
		if strings.TrimSpace(user.Source) != "" {
			m.users[index].Source = strings.TrimSpace(user.Source)
		}
		m.users[index].UpdatedAt = now
		return m.users[index], nil
	}
	return models.User{}, ErrNotFound
}

func (m *Memory) DisableUser(ctx context.Context, id int64, at time.Time) (models.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	when := at.UTC()
	for index := range m.users {
		if m.users[index].ID != id {
			continue
		}
		m.users[index].Status = models.UserStatusDisabled
		m.users[index].UpdatedAt = when
		return m.users[index], nil
	}
	return models.User{}, ErrNotFound
}

func (m *Memory) RecordUserLogin(ctx context.Context, id int64, at time.Time) (models.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for index := range m.users {
		if m.users[index].ID != id {
			continue
		}
		when := at.UTC()
		m.users[index].LastLoginAt = &when
		m.users[index].UpdatedAt = when
		return m.users[index], nil
	}
	return models.User{}, ErrNotFound
}

func (m *Memory) ListAPITokens(ctx context.Context) ([]models.APIToken, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]models.APIToken, 0, len(m.apiTokens))
	for _, token := range m.apiTokens {
		out = append(out, cloneAPIToken(token))
	}
	return out, nil
}

func (m *Memory) CreateAPIToken(ctx context.Context, token models.APIToken) (models.APIToken, error) {
	if strings.TrimSpace(token.Name) == "" || strings.TrimSpace(token.TokenHash) == "" || strings.TrimSpace(token.ScopesJSON) == "" {
		return models.APIToken{}, errors.New("name, token_hash, and scopes_json are required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	m.nextID++
	token.ID = m.nextID
	token.CreatedAt = now
	m.apiTokens = append([]models.APIToken{cloneAPIToken(token)}, m.apiTokens...)
	return cloneAPIToken(token), nil
}

func (m *Memory) FindAPITokenByHash(ctx context.Context, tokenHash string) (models.APIToken, error) {
	tokenHash = strings.TrimSpace(tokenHash)
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, token := range m.apiTokens {
		if token.TokenHash == tokenHash {
			return cloneAPIToken(token), nil
		}
	}
	return models.APIToken{}, ErrNotFound
}

func (m *Memory) RevokeAPIToken(ctx context.Context, id int64, at time.Time) (models.APIToken, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	when := at.UTC()
	for index := range m.apiTokens {
		if m.apiTokens[index].ID != id {
			continue
		}
		m.apiTokens[index].RevokedAt = &when
		return cloneAPIToken(m.apiTokens[index]), nil
	}
	return models.APIToken{}, ErrNotFound
}

func (m *Memory) RecordAPITokenUsed(ctx context.Context, id int64, at time.Time) (models.APIToken, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	when := at.UTC()
	for index := range m.apiTokens {
		if m.apiTokens[index].ID != id {
			continue
		}
		m.apiTokens[index].LastUsedAt = &when
		return cloneAPIToken(m.apiTokens[index]), nil
	}
	return models.APIToken{}, ErrNotFound
}

func (m *Memory) CreateSession(ctx context.Context, session models.Session) (models.Session, error) {
	if session.UserID <= 0 || strings.TrimSpace(session.TokenHash) == "" || session.ExpiresAt.IsZero() {
		return models.Session{}, errors.New("user_id, token_hash, and expires_at are required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	m.nextID++
	session.ID = m.nextID
	session.TokenHash = strings.TrimSpace(session.TokenHash)
	session.IP = strings.TrimSpace(session.IP)
	session.UserAgent = strings.TrimSpace(session.UserAgent)
	session.ExpiresAt = session.ExpiresAt.UTC()
	session.CreatedAt = now
	m.sessions = append([]models.Session{cloneSession(session)}, m.sessions...)
	return cloneSession(session), nil
}

func (m *Memory) FindSessionByTokenHash(ctx context.Context, tokenHash string) (models.Session, error) {
	tokenHash = strings.TrimSpace(tokenHash)
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, session := range m.sessions {
		if session.TokenHash == tokenHash {
			return cloneSession(session), nil
		}
	}
	return models.Session{}, ErrNotFound
}

func (m *Memory) RevokeSession(ctx context.Context, tokenHash string, at time.Time) (models.Session, error) {
	tokenHash = strings.TrimSpace(tokenHash)
	m.mu.Lock()
	defer m.mu.Unlock()
	when := at.UTC()
	for index := range m.sessions {
		if m.sessions[index].TokenHash != tokenHash {
			continue
		}
		if m.sessions[index].RevokedAt == nil {
			m.sessions[index].RevokedAt = &when
		}
		return cloneSession(m.sessions[index]), nil
	}
	return models.Session{}, ErrNotFound
}

func (m *Memory) CreateLoginLog(ctx context.Context, log models.LoginLog) (models.LoginLog, error) {
	if strings.TrimSpace(log.Email) == "" {
		return models.LoginLog{}, errors.New("email is required")
	}
	if !validLoginStatus(log.Status) {
		return models.LoginLog{}, errors.New("login status must be succeeded, failed, or denied")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	m.nextID++
	log.ID = m.nextID
	log.Email = strings.ToLower(strings.TrimSpace(log.Email))
	log.Username = strings.TrimSpace(log.Username)
	log.DisplayName = strings.TrimSpace(log.DisplayName)
	log.FailureReason = strings.TrimSpace(log.FailureReason)
	log.IP = strings.TrimSpace(log.IP)
	log.UserAgent = strings.TrimSpace(log.UserAgent)
	if log.CreatedAt.IsZero() {
		log.CreatedAt = now
	}
	log.CreatedAt = log.CreatedAt.UTC()
	if log.Status == "succeeded" && log.LastLoginAt == nil {
		when := log.CreatedAt
		log.LastLoginAt = &when
	}
	m.loginLogs = append([]models.LoginLog{cloneLoginLog(log)}, m.loginLogs...)
	return cloneLoginLog(log), nil
}

func (m *Memory) ListUsers(ctx context.Context) ([]models.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]models.User(nil), m.users...), nil
}

func (m *Memory) CreateUser(ctx context.Context, user models.User) (models.User, error) {
	if user.Email == "" || user.Username == "" {
		return models.User{}, errors.New("email and username are required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	m.nextID++
	user.ID = m.nextID
	user.Status = models.UserStatusActive
	user.Source = "local"
	user.CreatedAt = now
	user.UpdatedAt = now
	m.users = append(m.users, user)
	return user, nil
}

func (m *Memory) ListUserGroups(ctx context.Context) ([]models.UserGroup, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	groups := make([]models.UserGroup, 0, len(m.userGroups))
	for _, group := range m.userGroups {
		groups = append(groups, cloneUserGroup(group))
	}
	return groups, nil
}

func (m *Memory) CreateUserGroup(ctx context.Context, group models.UserGroup) (models.UserGroup, error) {
	if strings.TrimSpace(group.Name) == "" {
		return models.UserGroup{}, errors.New("name is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	m.nextID++
	group.ID = m.nextID
	group.Name = strings.TrimSpace(group.Name)
	group.Description = strings.TrimSpace(group.Description)
	group.MemberUserIDs = positiveIDs(group.MemberUserIDs)
	group.CreatedAt = now
	group.UpdatedAt = now
	m.userGroups = append(m.userGroups, cloneUserGroup(group))
	return cloneUserGroup(group), nil
}

func (m *Memory) ListUserGroupMembers(ctx context.Context, groupID int64) ([]int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, group := range m.userGroups {
		if group.ID == groupID {
			return append([]int64(nil), group.MemberUserIDs...), nil
		}
	}
	return nil, ErrNotFound
}

func (m *Memory) AddUserGroupMember(ctx context.Context, groupID, userID int64) (models.UserGroup, error) {
	if groupID <= 0 || userID <= 0 {
		return models.UserGroup{}, errors.New("group_id and user_id must be positive")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for index := range m.userGroups {
		if m.userGroups[index].ID != groupID {
			continue
		}
		if !containsInt64(m.userGroups[index].MemberUserIDs, userID) {
			m.userGroups[index].MemberUserIDs = append(m.userGroups[index].MemberUserIDs, userID)
			sort.Slice(m.userGroups[index].MemberUserIDs, func(i, j int) bool {
				return m.userGroups[index].MemberUserIDs[i] < m.userGroups[index].MemberUserIDs[j]
			})
			m.userGroups[index].UpdatedAt = time.Now().UTC()
		}
		return cloneUserGroup(m.userGroups[index]), nil
	}
	return models.UserGroup{}, ErrNotFound
}

func (m *Memory) RemoveUserGroupMember(ctx context.Context, groupID, userID int64) (models.UserGroup, error) {
	if groupID <= 0 || userID <= 0 {
		return models.UserGroup{}, errors.New("group_id and user_id must be positive")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for index := range m.userGroups {
		if m.userGroups[index].ID != groupID {
			continue
		}
		m.userGroups[index].MemberUserIDs = removeInt64(m.userGroups[index].MemberUserIDs, userID)
		m.userGroups[index].UpdatedAt = time.Now().UTC()
		return cloneUserGroup(m.userGroups[index]), nil
	}
	return models.UserGroup{}, ErrNotFound
}

func (m *Memory) ListDevices(ctx context.Context) ([]models.Device, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]models.Device(nil), m.devices...), nil
}

func (m *Memory) CreateDevice(ctx context.Context, device models.Device) (models.Device, error) {
	if device.RustDeskID == "" || device.Name == "" {
		return models.Device{}, errors.New("rustdesk_id and name are required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	m.nextID++
	device.ID = m.nextID
	if device.Status == "" {
		device.Status = models.DeviceStatusOffline
	}
	device.CreatedAt = now
	device.UpdatedAt = now
	m.devices = append(m.devices, device)
	return device, nil
}

func (m *Memory) FindDeviceByID(ctx context.Context, id int64) (models.Device, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, device := range m.devices {
		if device.ID == id {
			return device, nil
		}
	}
	return models.Device{}, ErrNotFound
}

func (m *Memory) UpdateDevice(ctx context.Context, id int64, device models.Device) (models.Device, error) {
	if strings.TrimSpace(device.RustDeskID) == "" || strings.TrimSpace(device.Name) == "" {
		return models.Device{}, errors.New("rustdesk_id and name are required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	for index := range m.devices {
		if m.devices[index].ID != id {
			continue
		}
		m.devices[index].RustDeskID = strings.TrimSpace(device.RustDeskID)
		m.devices[index].Name = strings.TrimSpace(device.Name)
		m.devices[index].Alias = strings.TrimSpace(device.Alias)
		m.devices[index].OwnerUserID = device.OwnerUserID
		if device.Status != "" {
			m.devices[index].Status = device.Status
		}
		m.devices[index].Platform = strings.TrimSpace(device.Platform)
		m.devices[index].ClientVersion = strings.TrimSpace(device.ClientVersion)
		m.devices[index].OpenDeskClientVersion = strings.TrimSpace(device.OpenDeskClientVersion)
		m.devices[index].LastIP = strings.TrimSpace(device.LastIP)
		m.devices[index].UpdatedAt = now
		return m.devices[index], nil
	}
	return models.Device{}, ErrNotFound
}

func (m *Memory) DisableDevice(ctx context.Context, id int64, at time.Time) (models.Device, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	when := at.UTC()
	for index := range m.devices {
		if m.devices[index].ID != id {
			continue
		}
		m.devices[index].Status = models.DeviceStatusDisabled
		m.devices[index].UpdatedAt = when
		return m.devices[index], nil
	}
	return models.Device{}, ErrNotFound
}

func (m *Memory) ListDeviceGroups(ctx context.Context) ([]models.DeviceGroup, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	groups := make([]models.DeviceGroup, 0, len(m.deviceGroups))
	for _, group := range m.deviceGroups {
		groups = append(groups, cloneDeviceGroup(group))
	}
	return groups, nil
}

func (m *Memory) CreateDeviceGroup(ctx context.Context, group models.DeviceGroup) (models.DeviceGroup, error) {
	if strings.TrimSpace(group.Name) == "" {
		return models.DeviceGroup{}, errors.New("name is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	m.nextID++
	group.ID = m.nextID
	group.Name = strings.TrimSpace(group.Name)
	group.Description = strings.TrimSpace(group.Description)
	group.MemberDeviceIDs = positiveIDs(group.MemberDeviceIDs)
	group.CreatedAt = now
	group.UpdatedAt = now
	m.deviceGroups = append(m.deviceGroups, cloneDeviceGroup(group))
	return cloneDeviceGroup(group), nil
}

func (m *Memory) ListDeviceGroupMembers(ctx context.Context, groupID int64) ([]int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, group := range m.deviceGroups {
		if group.ID == groupID {
			return append([]int64(nil), group.MemberDeviceIDs...), nil
		}
	}
	return nil, ErrNotFound
}

func (m *Memory) AddDeviceGroupMember(ctx context.Context, groupID, deviceID int64) (models.DeviceGroup, error) {
	if groupID <= 0 || deviceID <= 0 {
		return models.DeviceGroup{}, errors.New("group_id and device_id must be positive")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for index := range m.deviceGroups {
		if m.deviceGroups[index].ID != groupID {
			continue
		}
		if !containsInt64(m.deviceGroups[index].MemberDeviceIDs, deviceID) {
			m.deviceGroups[index].MemberDeviceIDs = append(m.deviceGroups[index].MemberDeviceIDs, deviceID)
			sort.Slice(m.deviceGroups[index].MemberDeviceIDs, func(i, j int) bool {
				return m.deviceGroups[index].MemberDeviceIDs[i] < m.deviceGroups[index].MemberDeviceIDs[j]
			})
			m.deviceGroups[index].UpdatedAt = time.Now().UTC()
		}
		return cloneDeviceGroup(m.deviceGroups[index]), nil
	}
	return models.DeviceGroup{}, ErrNotFound
}

func (m *Memory) RemoveDeviceGroupMember(ctx context.Context, groupID, deviceID int64) (models.DeviceGroup, error) {
	if groupID <= 0 || deviceID <= 0 {
		return models.DeviceGroup{}, errors.New("group_id and device_id must be positive")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for index := range m.deviceGroups {
		if m.deviceGroups[index].ID != groupID {
			continue
		}
		m.deviceGroups[index].MemberDeviceIDs = removeInt64(m.deviceGroups[index].MemberDeviceIDs, deviceID)
		m.deviceGroups[index].UpdatedAt = time.Now().UTC()
		return cloneDeviceGroup(m.deviceGroups[index]), nil
	}
	return models.DeviceGroup{}, ErrNotFound
}

func (m *Memory) ListAddressBooks(ctx context.Context) ([]models.AddressBook, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	books := make([]models.AddressBook, 0, len(m.addressBooks))
	for _, book := range m.addressBooks {
		books = append(books, cloneAddressBook(book))
	}
	return books, nil
}

func (m *Memory) CreateAddressBook(ctx context.Context, book models.AddressBook) (models.AddressBook, error) {
	if strings.TrimSpace(book.Name) == "" {
		return models.AddressBook{}, errors.New("name is required")
	}
	for _, entry := range book.Entries {
		if entry.DeviceID <= 0 {
			return models.AddressBook{}, errors.New("entry device_id must be positive")
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	m.nextID++
	book.ID = m.nextID
	book.Name = strings.TrimSpace(book.Name)
	book.Description = strings.TrimSpace(book.Description)
	book.CreatedAt = now
	book.UpdatedAt = now
	for index := range book.Entries {
		m.nextID++
		book.Entries[index].ID = m.nextID
		book.Entries[index].AddressBookID = book.ID
		book.Entries[index].Alias = strings.TrimSpace(book.Entries[index].Alias)
		book.Entries[index].CreatedAt = now
	}
	m.addressBooks = append(m.addressBooks, cloneAddressBook(book))
	return cloneAddressBook(book), nil
}

func (m *Memory) ListAddressBookEntries(ctx context.Context, bookID int64) ([]models.AddressBookEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, book := range m.addressBooks {
		if book.ID == bookID {
			return append([]models.AddressBookEntry(nil), book.Entries...), nil
		}
	}
	return nil, ErrNotFound
}

func (m *Memory) AddAddressBookEntry(ctx context.Context, bookID int64, entry models.AddressBookEntry) (models.AddressBook, error) {
	if bookID <= 0 || entry.DeviceID <= 0 {
		return models.AddressBook{}, errors.New("book_id and device_id must be positive")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for index := range m.addressBooks {
		if m.addressBooks[index].ID != bookID {
			continue
		}
		for _, existing := range m.addressBooks[index].Entries {
			if existing.DeviceID == entry.DeviceID {
				return models.AddressBook{}, errors.New("address book entry already exists")
			}
		}
		now := time.Now().UTC()
		m.nextID++
		entry.ID = m.nextID
		entry.AddressBookID = bookID
		entry.Alias = strings.TrimSpace(entry.Alias)
		entry.CreatedAt = now
		m.addressBooks[index].Entries = append(m.addressBooks[index].Entries, entry)
		m.addressBooks[index].UpdatedAt = now
		return cloneAddressBook(m.addressBooks[index]), nil
	}
	return models.AddressBook{}, ErrNotFound
}

func (m *Memory) RemoveAddressBookEntry(ctx context.Context, bookID, entryID int64) (models.AddressBook, error) {
	if bookID <= 0 || entryID <= 0 {
		return models.AddressBook{}, errors.New("book_id and entry_id must be positive")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for index := range m.addressBooks {
		if m.addressBooks[index].ID != bookID {
			continue
		}
		entries := m.addressBooks[index].Entries[:0]
		removed := false
		for _, entry := range m.addressBooks[index].Entries {
			if entry.ID == entryID {
				removed = true
				continue
			}
			entries = append(entries, entry)
		}
		if !removed {
			return models.AddressBook{}, ErrNotFound
		}
		m.addressBooks[index].Entries = entries
		m.addressBooks[index].UpdatedAt = time.Now().UTC()
		return cloneAddressBook(m.addressBooks[index]), nil
	}
	return models.AddressBook{}, ErrNotFound
}

func (m *Memory) ListRelays(ctx context.Context) ([]models.Relay, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]models.Relay(nil), m.relays...), nil
}

func (m *Memory) CreateRelay(ctx context.Context, relay models.Relay) (models.Relay, error) {
	if relay.Name == "" || relay.Region == "" || relay.Host == "" {
		return models.Relay{}, errors.New("name, region, and host are required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	m.nextID++
	relay.ID = m.nextID
	if relay.Port == 0 {
		relay.Port = 21117
	}
	if relay.WSPort == 0 {
		relay.WSPort = 21119
	}
	if relay.Status == "" {
		relay.Status = "active"
	}
	relay.CreatedAt = now
	relay.UpdatedAt = now
	m.relays = append(m.relays, relay)
	return relay, nil
}

func (m *Memory) UpdateRelay(ctx context.Context, id int64, relay models.Relay) (models.Relay, error) {
	if strings.TrimSpace(relay.Name) == "" || strings.TrimSpace(relay.Region) == "" || strings.TrimSpace(relay.Host) == "" {
		return models.Relay{}, errors.New("name, region, and host are required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	for index := range m.relays {
		if m.relays[index].ID != id {
			continue
		}
		if relay.Port == 0 {
			relay.Port = 21117
		}
		if relay.WSPort == 0 {
			relay.WSPort = 21119
		}
		if relay.Status == "" {
			relay.Status = m.relays[index].Status
		}
		m.relays[index].Name = strings.TrimSpace(relay.Name)
		m.relays[index].Region = strings.TrimSpace(relay.Region)
		m.relays[index].Host = strings.TrimSpace(relay.Host)
		m.relays[index].Port = relay.Port
		m.relays[index].WSPort = relay.WSPort
		m.relays[index].PublicKeyFingerprint = strings.TrimSpace(relay.PublicKeyFingerprint)
		m.relays[index].Status = relay.Status
		m.relays[index].MaxBandwidthMbps = relay.MaxBandwidthMbps
		m.relays[index].UpdatedAt = now
		return m.relays[index], nil
	}
	return models.Relay{}, ErrNotFound
}

func (m *Memory) RelayHeartbeat(ctx context.Context, id int64, currentSessions *int, status string) (models.Relay, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	for index := range m.relays {
		if m.relays[index].ID != id {
			continue
		}
		if currentSessions != nil {
			m.relays[index].CurrentSessions = *currentSessions
		}
		if status != "" {
			m.relays[index].Status = status
		}
		m.relays[index].LastHealthAt = &now
		m.relays[index].UpdatedAt = now
		return m.relays[index], nil
	}
	return models.Relay{}, ErrNotFound
}

func (m *Memory) DisableRelay(ctx context.Context, id int64, at time.Time) (models.Relay, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	when := at.UTC()
	for index := range m.relays {
		if m.relays[index].ID != id {
			continue
		}
		m.relays[index].Status = "disabled"
		m.relays[index].UpdatedAt = when
		return m.relays[index], nil
	}
	return models.Relay{}, ErrNotFound
}

func (m *Memory) CreateRelayGrant(ctx context.Context, grant models.RelayGrant) (models.RelayGrant, error) {
	if strings.TrimSpace(grant.GrantID) == "" || strings.TrimSpace(grant.Nonce) == "" || grant.ExpiresAt.IsZero() || len(grant.AllowedRelays) == 0 {
		return models.RelayGrant{}, errors.New("grant_id, nonce, expires_at, and allowed_relays are required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	m.nextID++
	grant.ID = m.nextID
	grant.GrantID = strings.TrimSpace(grant.GrantID)
	grant.TargetRustDeskID = strings.TrimSpace(grant.TargetRustDeskID)
	grant.Nonce = strings.TrimSpace(grant.Nonce)
	if grant.Status == "" {
		grant.Status = "issued"
	}
	grant.ExpiresAt = grant.ExpiresAt.UTC()
	grant.CreatedAt = now
	grant.AllowedRelays = append([]string(nil), grant.AllowedRelays...)
	m.relayGrants = append([]models.RelayGrant{cloneRelayGrant(grant)}, m.relayGrants...)
	return cloneRelayGrant(grant), nil
}

func (m *Memory) FindRelayGrantByGrantID(ctx context.Context, grantID string) (models.RelayGrant, error) {
	grantID = strings.TrimSpace(grantID)
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, grant := range m.relayGrants {
		if grant.GrantID == grantID {
			return cloneRelayGrant(grant), nil
		}
	}
	return models.RelayGrant{}, ErrNotFound
}

func (m *Memory) UpdateRelayGrantStatus(ctx context.Context, grantID string, status string, usedAt *time.Time) (models.RelayGrant, error) {
	if strings.TrimSpace(status) == "" {
		return models.RelayGrant{}, errors.New("status is required")
	}
	grantID = strings.TrimSpace(grantID)
	m.mu.Lock()
	defer m.mu.Unlock()
	for index := range m.relayGrants {
		if m.relayGrants[index].GrantID != grantID {
			continue
		}
		m.relayGrants[index].Status = strings.TrimSpace(status)
		if usedAt != nil {
			when := usedAt.UTC()
			m.relayGrants[index].UsedAt = &when
		}
		return cloneRelayGrant(m.relayGrants[index]), nil
	}
	return models.RelayGrant{}, ErrNotFound
}

func (m *Memory) ListAccessRules(ctx context.Context) ([]models.AccessRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]models.AccessRule(nil), m.accessRules...), nil
}

func (m *Memory) CreateAccessRule(ctx context.Context, rule models.AccessRule) (models.AccessRule, error) {
	if rule.SubjectType == "" || rule.SubjectID <= 0 || rule.TargetType == "" || rule.TargetID <= 0 || rule.Effect == "" {
		return models.AccessRule{}, errors.New("subject_type, subject_id, target_type, target_id, and effect are required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	m.nextID++
	rule.ID = m.nextID
	rule.CreatedAt = now
	rule.UpdatedAt = now
	m.accessRules = append(m.accessRules, rule)
	return rule, nil
}

func (m *Memory) UpdateAccessRule(ctx context.Context, id int64, rule models.AccessRule) (models.AccessRule, error) {
	if rule.SubjectType == "" || rule.SubjectID <= 0 || rule.TargetType == "" || rule.TargetID <= 0 || rule.Effect == "" {
		return models.AccessRule{}, errors.New("subject_type, subject_id, target_type, target_id, and effect are required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for index := range m.accessRules {
		if m.accessRules[index].ID != id {
			continue
		}
		rule.ID = id
		rule.CreatedAt = m.accessRules[index].CreatedAt
		rule.UpdatedAt = time.Now().UTC()
		m.accessRules[index] = rule
		return rule, nil
	}
	return models.AccessRule{}, ErrNotFound
}

func (m *Memory) DeleteAccessRule(ctx context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for index := range m.accessRules {
		if m.accessRules[index].ID != id {
			continue
		}
		m.accessRules = append(m.accessRules[:index], m.accessRules[index+1:]...)
		return nil
	}
	return ErrNotFound
}

func (m *Memory) ListControlRoles(ctx context.Context) ([]models.ControlRole, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]models.ControlRole(nil), m.controlRoles...), nil
}

func (m *Memory) CreateControlRole(ctx context.Context, role models.ControlRole) (models.ControlRole, error) {
	if strings.TrimSpace(role.Name) == "" {
		return models.ControlRole{}, errors.New("name is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	m.nextID++
	role.ID = m.nextID
	role.CreatedAt = now
	role.UpdatedAt = now
	for index := range role.Permissions {
		m.nextID++
		role.Permissions[index].ID = m.nextID
		role.Permissions[index].RoleID = role.ID
	}
	m.controlRoles = append(m.controlRoles, role)
	return role, nil
}

func (m *Memory) UpdateControlRole(ctx context.Context, id int64, role models.ControlRole) (models.ControlRole, error) {
	if strings.TrimSpace(role.Name) == "" {
		return models.ControlRole{}, errors.New("name is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for index := range m.controlRoles {
		if m.controlRoles[index].ID != id {
			continue
		}
		now := time.Now().UTC()
		role.ID = id
		role.CreatedAt = m.controlRoles[index].CreatedAt
		role.UpdatedAt = now
		for permissionIndex := range role.Permissions {
			m.nextID++
			role.Permissions[permissionIndex].ID = m.nextID
			role.Permissions[permissionIndex].RoleID = id
		}
		m.controlRoles[index] = role
		return role, nil
	}
	return models.ControlRole{}, ErrNotFound
}

func (m *Memory) DeleteControlRole(ctx context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for index := range m.controlRoles {
		if m.controlRoles[index].ID != id {
			continue
		}
		m.controlRoles = append(m.controlRoles[:index], m.controlRoles[index+1:]...)
		return nil
	}
	return ErrNotFound
}

func (m *Memory) ListStrategies(ctx context.Context) ([]models.Strategy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]models.Strategy(nil), m.strategies...), nil
}

func (m *Memory) CreateStrategy(ctx context.Context, strategy models.Strategy) (models.Strategy, error) {
	if strings.TrimSpace(strategy.Name) == "" || strings.TrimSpace(strategy.SettingsJSON) == "" {
		return models.Strategy{}, errors.New("name and settings_json are required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	m.nextID++
	strategy.ID = m.nextID
	strategy.CreatedAt = now
	strategy.UpdatedAt = now
	for index := range strategy.Assignments {
		m.nextID++
		strategy.Assignments[index].ID = m.nextID
		strategy.Assignments[index].StrategyID = strategy.ID
		strategy.Assignments[index].CreatedAt = now
	}
	m.strategies = append(m.strategies, strategy)
	return strategy, nil
}

func (m *Memory) UpdateStrategy(ctx context.Context, id int64, strategy models.Strategy) (models.Strategy, error) {
	if strings.TrimSpace(strategy.Name) == "" || strings.TrimSpace(strategy.SettingsJSON) == "" {
		return models.Strategy{}, errors.New("name and settings_json are required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for index := range m.strategies {
		if m.strategies[index].ID != id {
			continue
		}
		now := time.Now().UTC()
		strategy.ID = id
		strategy.CreatedAt = m.strategies[index].CreatedAt
		strategy.UpdatedAt = now
		for assignmentIndex := range strategy.Assignments {
			m.nextID++
			strategy.Assignments[assignmentIndex].ID = m.nextID
			strategy.Assignments[assignmentIndex].StrategyID = id
			strategy.Assignments[assignmentIndex].CreatedAt = now
		}
		m.strategies[index] = strategy
		return strategy, nil
	}
	return models.Strategy{}, ErrNotFound
}

func (m *Memory) DeleteStrategy(ctx context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for index := range m.strategies {
		if m.strategies[index].ID != id {
			continue
		}
		m.strategies = append(m.strategies[:index], m.strategies[index+1:]...)
		return nil
	}
	return ErrNotFound
}

func (m *Memory) AddStrategyAssignment(ctx context.Context, strategyID int64, assignment models.StrategyAssignment) (models.Strategy, error) {
	if assignment.TargetType == "" || assignment.TargetID <= 0 {
		return models.Strategy{}, errors.New("target_type and target_id are required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for index := range m.strategies {
		if m.strategies[index].ID != strategyID {
			continue
		}
		m.nextID++
		assignment.ID = m.nextID
		assignment.StrategyID = strategyID
		assignment.CreatedAt = time.Now().UTC()
		m.strategies[index].Assignments = append(m.strategies[index].Assignments, assignment)
		return m.strategies[index], nil
	}
	return models.Strategy{}, ErrNotFound
}

func (m *Memory) RemoveStrategyAssignment(ctx context.Context, strategyID int64, assignmentID int64) (models.Strategy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for strategyIndex := range m.strategies {
		if m.strategies[strategyIndex].ID != strategyID {
			continue
		}
		for assignmentIndex := range m.strategies[strategyIndex].Assignments {
			if m.strategies[strategyIndex].Assignments[assignmentIndex].ID != assignmentID {
				continue
			}
			assignments := m.strategies[strategyIndex].Assignments
			m.strategies[strategyIndex].Assignments = append(assignments[:assignmentIndex], assignments[assignmentIndex+1:]...)
			return m.strategies[strategyIndex], nil
		}
		return models.Strategy{}, ErrNotFound
	}
	return models.Strategy{}, ErrNotFound
}

func (m *Memory) CreateAuditEvent(ctx context.Context, event models.AuditEvent) (models.AuditEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	m.nextID++
	event.ID = m.nextID
	if event.CreatedAt.IsZero() {
		event.CreatedAt = now
	}
	event.Metadata = cloneMetadata(event.Metadata)
	m.auditEvents = append([]models.AuditEvent{cloneAuditEvent(event)}, m.auditEvents...)
	return cloneAuditEvent(event), nil
}

func (m *Memory) ListAuditEvents(ctx context.Context, filter AuditLogFilter) ([]models.AuditEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	events := []models.AuditEvent{}
	for _, event := range m.auditEvents {
		if filter.ActorType != "" && event.ActorType != filter.ActorType {
			continue
		}
		if filter.Action != "" && event.Action != filter.Action {
			continue
		}
		if filter.ResourceType != "" && event.ResourceType != filter.ResourceType {
			continue
		}
		if filter.ResourceID != "" && (event.ResourceID == nil || *event.ResourceID != filter.ResourceID) {
			continue
		}
		if !timeInRange(event.CreatedAt, filter.From, filter.To) {
			continue
		}
		events = append(events, cloneAuditEvent(event))
	}
	return paginate(events, filter.Offset, filter.Limit), nil
}

func (m *Memory) CreateConnectionLog(ctx context.Context, log models.ConnectionLog) (models.ConnectionLog, error) {
	if !validConnectionType(log.ConnectionType) {
		return models.ConnectionLog{}, errors.New("connection_type must be direct, relay, or websocket")
	}
	if !validConnectionStatus(log.Status) {
		return models.ConnectionLog{}, errors.New("status must be started, ended, failed, or denied")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	m.nextID++
	log.ID = m.nextID
	if log.StartedAt.IsZero() {
		log.StartedAt = now
	}
	log.StartedAt = log.StartedAt.UTC()
	log.Metadata = cloneMetadata(log.Metadata)
	m.connectionLogs = append([]models.ConnectionLog{cloneConnectionLog(log)}, m.connectionLogs...)
	return cloneConnectionLog(log), nil
}

func (m *Memory) ListConnectionLogs(ctx context.Context, filter ConnectionLogFilter) ([]models.ConnectionLog, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	logs := []models.ConnectionLog{}
	for _, log := range m.connectionLogs {
		if filter.Status != "" && log.Status != filter.Status {
			continue
		}
		if filter.ConnectionType != "" && log.ConnectionType != filter.ConnectionType {
			continue
		}
		if !timeInRange(log.StartedAt, filter.From, filter.To) {
			continue
		}
		logs = append(logs, cloneConnectionLog(log))
	}
	return paginate(logs, filter.Offset, filter.Limit), nil
}

func (m *Memory) ListFileTransferLogs(ctx context.Context, filter FileTransferLogFilter) ([]models.FileTransferLog, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	logs := []models.FileTransferLog{}
	for _, log := range m.fileTransfers {
		if filter.Direction != "" && log.Direction != filter.Direction {
			continue
		}
		if filter.Status != "" && log.Status != filter.Status {
			continue
		}
		if !timeInRange(log.CreatedAt, filter.From, filter.To) {
			continue
		}
		logs = append(logs, log)
	}
	return paginate(logs, filter.Offset, filter.Limit), nil
}

func (m *Memory) ListLoginLogs(ctx context.Context, filter LoginLogFilter) ([]models.LoginLog, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	logs := []models.LoginLog{}
	for _, log := range m.loginLogs {
		if filter.Email != "" && !strings.EqualFold(log.Email, filter.Email) {
			continue
		}
		if filter.Status != "" && log.Status != filter.Status {
			continue
		}
		if !timeInRange(log.CreatedAt, filter.From, filter.To) {
			continue
		}
		logs = append(logs, cloneLoginLog(log))
	}
	sort.Slice(logs, func(i, j int) bool {
		return logs[i].CreatedAt.After(logs[j].CreatedAt)
	})
	return paginate(logs, filter.Offset, filter.Limit), nil
}

func (m *Memory) ListSystemSettings(ctx context.Context) ([]models.SystemSetting, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	settings := make([]models.SystemSetting, 0, len(m.settings))
	for _, setting := range m.settings {
		settings = append(settings, setting)
	}
	sort.Slice(settings, func(i, j int) bool {
		return settings[i].Key < settings[j].Key
	})
	return settings, nil
}

func (m *Memory) UpsertSystemSettings(ctx context.Context, settings []models.SystemSetting, updatedBy int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.settings == nil {
		m.settings = map[string]models.SystemSetting{}
	}
	now := time.Now().UTC()
	for _, setting := range settings {
		key := strings.TrimSpace(setting.Key)
		if key == "" || strings.TrimSpace(setting.ValueJSON) == "" {
			return errors.New("setting key and value_json are required")
		}
		setting.Key = key
		setting.UpdatedAt = now
		if updatedBy > 0 {
			setting.UpdatedBy = &updatedBy
		}
		m.settings[key] = setting
	}
	return nil
}

func (m *Memory) ListBuildProfiles(ctx context.Context) ([]models.BuildProfile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]models.BuildProfile(nil), m.buildProfiles...), nil
}

func (m *Memory) CreateBuildProfile(ctx context.Context, profile models.BuildProfile) (models.BuildProfile, error) {
	if profile.Name == "" || profile.AppName == "" || profile.BundleID == "" {
		return models.BuildProfile{}, errors.New("name, app_name, and bundle_id are required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	m.nextID++
	profile.ID = m.nextID
	profile.CreatedAt = now
	profile.UpdatedAt = now
	m.buildProfiles = append(m.buildProfiles, profile)
	return profile, nil
}

func (m *Memory) FindBuildProfileByID(ctx context.Context, id int64) (models.BuildProfile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, profile := range m.buildProfiles {
		if profile.ID == id {
			return profile, nil
		}
	}
	return models.BuildProfile{}, ErrNotFound
}

func (m *Memory) UpdateBuildProfile(ctx context.Context, id int64, profile models.BuildProfile) (models.BuildProfile, error) {
	if profile.Name == "" || profile.AppName == "" || profile.BundleID == "" {
		return models.BuildProfile{}, errors.New("name, app_name, and bundle_id are required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for index := range m.buildProfiles {
		if m.buildProfiles[index].ID != id {
			continue
		}
		profile.ID = id
		profile.CreatedBy = m.buildProfiles[index].CreatedBy
		profile.CreatedAt = m.buildProfiles[index].CreatedAt
		profile.UpdatedAt = time.Now().UTC()
		m.buildProfiles[index] = profile
		return profile, nil
	}
	return models.BuildProfile{}, ErrNotFound
}

func (m *Memory) DeleteBuildProfile(ctx context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, job := range m.buildJobs {
		if job.ProfileID == id {
			return ErrConflict
		}
	}
	for index := range m.buildProfiles {
		if m.buildProfiles[index].ID != id {
			continue
		}
		m.buildProfiles = append(m.buildProfiles[:index], m.buildProfiles[index+1:]...)
		return nil
	}
	return ErrNotFound
}

func (m *Memory) ListBuildJobs(ctx context.Context) ([]models.BuildJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]models.BuildJob(nil), m.buildJobs...), nil
}

func (m *Memory) CreateBuildJob(ctx context.Context, job models.BuildJob) (models.BuildJob, error) {
	if job.ProfileID <= 0 || job.Platform == "" {
		return models.BuildJob{}, errors.New("profile_id and platform are required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	m.nextID++
	job.ID = m.nextID
	if job.Status == "" {
		job.Status = models.BuildJobQueued
	}
	job.CreatedAt = now
	m.buildJobs = append(m.buildJobs, job)
	return job, nil
}

func (m *Memory) FindBuildJobByID(ctx context.Context, id int64) (models.BuildJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, job := range m.buildJobs {
		if job.ID == id {
			return job, nil
		}
	}
	return models.BuildJob{}, ErrNotFound
}

func (m *Memory) ClaimNextBuildJob(ctx context.Context, runner string, at time.Time) (models.BuildJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for index := range m.buildJobs {
		if m.buildJobs[index].Status != models.BuildJobQueued {
			continue
		}
		when := at.UTC()
		m.buildJobs[index].Status = models.BuildJobRunning
		m.buildJobs[index].Runner = runner
		m.buildJobs[index].StartedAt = &when
		return m.buildJobs[index], nil
	}
	return models.BuildJob{}, ErrNotFound
}

func (m *Memory) CompleteBuildJob(ctx context.Context, id int64, logPath string, at time.Time) (models.BuildJob, error) {
	return m.finishBuildJob(id, models.BuildJobSucceeded, logPath, "", at)
}

func (m *Memory) FailBuildJob(ctx context.Context, id int64, logPath string, message string, at time.Time) (models.BuildJob, error) {
	return m.finishBuildJob(id, models.BuildJobFailed, logPath, message, at)
}

func (m *Memory) CancelBuildJob(ctx context.Context, id int64, message string, at time.Time) (models.BuildJob, error) {
	return m.finishBuildJob(id, models.BuildJobCanceled, "", message, at)
}

func (m *Memory) finishBuildJob(id int64, status models.BuildJobStatus, logPath string, message string, at time.Time) (models.BuildJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for index := range m.buildJobs {
		if m.buildJobs[index].ID != id {
			continue
		}
		when := at.UTC()
		m.buildJobs[index].Status = status
		m.buildJobs[index].LogPath = logPath
		m.buildJobs[index].ErrorMessage = message
		m.buildJobs[index].FinishedAt = &when
		return m.buildJobs[index], nil
	}
	return models.BuildJob{}, ErrNotFound
}

func (m *Memory) CreateBuildArtifact(ctx context.Context, artifact models.BuildArtifact) (models.BuildArtifact, error) {
	if artifact.BuildJobID <= 0 || artifact.Platform == "" || artifact.Filename == "" {
		return models.BuildArtifact{}, errors.New("build_job_id, platform, and filename are required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	m.nextID++
	artifact.ID = m.nextID
	artifact.CreatedAt = now
	m.buildArtifacts = append(m.buildArtifacts, artifact)
	return artifact, nil
}

func (m *Memory) ListBuildArtifacts(ctx context.Context, buildJobID int64) ([]models.BuildArtifact, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	artifacts := []models.BuildArtifact{}
	for _, artifact := range m.buildArtifacts {
		if artifact.BuildJobID == buildJobID {
			artifacts = append(artifacts, artifact)
		}
	}
	return artifacts, nil
}

func (m *Memory) FindBuildArtifactByID(ctx context.Context, id int64) (models.BuildArtifact, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, artifact := range m.buildArtifacts {
		if artifact.ID == id {
			return artifact, nil
		}
	}
	return models.BuildArtifact{}, ErrNotFound
}

func cloneUserGroup(group models.UserGroup) models.UserGroup {
	group.MemberUserIDs = append([]int64(nil), group.MemberUserIDs...)
	return group
}

func cloneAPIToken(token models.APIToken) models.APIToken {
	if token.UserID != nil {
		userID := *token.UserID
		token.UserID = &userID
	}
	if token.ExpiresAt != nil {
		expiresAt := *token.ExpiresAt
		token.ExpiresAt = &expiresAt
	}
	if token.LastUsedAt != nil {
		lastUsedAt := *token.LastUsedAt
		token.LastUsedAt = &lastUsedAt
	}
	if token.RevokedAt != nil {
		revokedAt := *token.RevokedAt
		token.RevokedAt = &revokedAt
	}
	return token
}

func cloneSession(session models.Session) models.Session {
	if session.RevokedAt != nil {
		revokedAt := *session.RevokedAt
		session.RevokedAt = &revokedAt
	}
	return session
}

func cloneLoginLog(log models.LoginLog) models.LoginLog {
	if log.LastLoginAt != nil {
		lastLoginAt := *log.LastLoginAt
		log.LastLoginAt = &lastLoginAt
	}
	return log
}

func cloneConnectionLog(log models.ConnectionLog) models.ConnectionLog {
	if log.SessionID != nil {
		sessionID := *log.SessionID
		log.SessionID = &sessionID
	}
	if log.ControllerUserID != nil {
		controllerUserID := *log.ControllerUserID
		log.ControllerUserID = &controllerUserID
	}
	if log.ControllerDeviceID != nil {
		controllerDeviceID := *log.ControllerDeviceID
		log.ControllerDeviceID = &controllerDeviceID
	}
	if log.TargetDeviceID != nil {
		targetDeviceID := *log.TargetDeviceID
		log.TargetDeviceID = &targetDeviceID
	}
	if log.RelayID != nil {
		relayID := *log.RelayID
		log.RelayID = &relayID
	}
	if log.EndedAt != nil {
		endedAt := *log.EndedAt
		log.EndedAt = &endedAt
	}
	log.Metadata = cloneMetadata(log.Metadata)
	return log
}

func cloneRelayGrant(grant models.RelayGrant) models.RelayGrant {
	if grant.UserID != nil {
		userID := *grant.UserID
		grant.UserID = &userID
	}
	if grant.ControllerDeviceID != nil {
		controllerDeviceID := *grant.ControllerDeviceID
		grant.ControllerDeviceID = &controllerDeviceID
	}
	if grant.TargetDeviceID != nil {
		targetDeviceID := *grant.TargetDeviceID
		grant.TargetDeviceID = &targetDeviceID
	}
	if grant.RelayID != nil {
		relayID := *grant.RelayID
		grant.RelayID = &relayID
	}
	if grant.UsedAt != nil {
		usedAt := *grant.UsedAt
		grant.UsedAt = &usedAt
	}
	grant.AllowedRelays = append([]string(nil), grant.AllowedRelays...)
	return grant
}

func cloneDeviceGroup(group models.DeviceGroup) models.DeviceGroup {
	group.MemberDeviceIDs = append([]int64(nil), group.MemberDeviceIDs...)
	return group
}

func cloneAddressBook(book models.AddressBook) models.AddressBook {
	book.Entries = append([]models.AddressBookEntry(nil), book.Entries...)
	return book
}

func validLoginStatus(status string) bool {
	return status == "succeeded" || status == "failed" || status == "denied"
}

func validConnectionType(connectionType string) bool {
	return connectionType == "direct" || connectionType == "relay" || connectionType == "websocket"
}

func validConnectionStatus(status string) bool {
	return status == "started" || status == "ended" || status == "failed" || status == "denied"
}

func cloneAuditEvent(event models.AuditEvent) models.AuditEvent {
	if event.ResourceID != nil {
		resourceID := *event.ResourceID
		event.ResourceID = &resourceID
	}
	event.Metadata = cloneMetadata(event.Metadata)
	return event
}

func cloneMetadata(metadata map[string]any) map[string]any {
	if metadata == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(metadata))
	for key, value := range metadata {
		out[key] = value
	}
	return out
}

func positiveIDs(ids []int64) []int64 {
	out := []int64{}
	seen := map[int64]bool{}
	for _, id := range ids {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

func containsInt64(ids []int64, want int64) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

func removeInt64(ids []int64, remove int64) []int64 {
	out := ids[:0]
	for _, id := range ids {
		if id != remove {
			out = append(out, id)
		}
	}
	return out
}

func effectiveLimit(limit int) int {
	if limit <= 0 || limit > 500 {
		return 500
	}
	return limit
}

func effectiveOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}

func paginate[T any](items []T, offset int, limit int) []T {
	start := effectiveOffset(offset)
	if start >= len(items) {
		return []T{}
	}
	end := start + effectiveLimit(limit)
	if end > len(items) {
		end = len(items)
	}
	return items[start:end]
}

func timeInRange(value time.Time, from *time.Time, to *time.Time) bool {
	if from != nil && value.Before(*from) {
		return false
	}
	if to != nil && value.After(*to) {
		return false
	}
	return true
}
