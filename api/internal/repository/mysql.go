package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/opendesk-remote/opendesk-remote/api/internal/models"
)

const disabledLoginHash = "!disabled-login!"

type MySQL struct {
	db *sql.DB
}

type rowScanner interface {
	Scan(dest ...any) error
}

type queryRower interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func NewMySQL(db *sql.DB) *MySQL {
	return &MySQL{db: db}
}

func (m *MySQL) EnsureInitialAdmin(ctx context.Context, email, passwordHash string) error {
	email = normalizeEmail(email)
	if email == "" {
		return errors.New("initial admin email is required")
	}
	if strings.TrimSpace(passwordHash) == "" {
		return errors.New("initial admin password hash is required")
	}
	username := usernameFromEmail(email)
	_, err := m.db.ExecContext(ctx, `
		INSERT INTO users (email, username, display_name, password_hash, status, source)
		VALUES (?, ?, ?, ?, 'active', 'local')
		ON DUPLICATE KEY UPDATE updated_at = updated_at
	`, email, username, "Initial Admin", passwordHash)
	return err
}

func (m *MySQL) CountUsers(ctx context.Context) (int, error) {
	var count int
	err := m.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count)
	return count, err
}

func (m *MySQL) ListUsers(ctx context.Context) ([]models.User, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, email, username, display_name, password_hash, status, mfa_enabled, source, created_at, updated_at, last_login_at
		FROM users
		ORDER BY id
		LIMIT 500
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := []models.User{}
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (m *MySQL) CreateUser(ctx context.Context, user models.User) (models.User, error) {
	user.Email = normalizeEmail(user.Email)
	user.Username = strings.TrimSpace(user.Username)
	user.DisplayName = strings.TrimSpace(user.DisplayName)
	if user.Email == "" || user.Username == "" {
		return models.User{}, errors.New("email and username are required")
	}
	if user.DisplayName == "" {
		user.DisplayName = user.Username
	}
	if user.PasswordHash == "" {
		user.PasswordHash = disabledLoginHash
	}
	if user.Status == "" {
		user.Status = models.UserStatusActive
	}
	if user.Source == "" {
		user.Source = "local"
	}
	result, err := m.db.ExecContext(ctx, `
		INSERT INTO users (email, username, display_name, password_hash, status, mfa_enabled, source)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, user.Email, user.Username, user.DisplayName, user.PasswordHash, user.Status, user.MFAEnabled, user.Source)
	if err != nil {
		return models.User{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return models.User{}, err
	}
	return m.FindUserByID(ctx, id)
}

func (m *MySQL) ListUserGroups(ctx context.Context) ([]models.UserGroup, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, name, description, created_at, updated_at
		FROM user_groups
		ORDER BY id DESC
		LIMIT 500
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	groups := []models.UserGroup{}
	for rows.Next() {
		group, err := scanUserGroup(rows)
		if err != nil {
			return nil, err
		}
		group.MemberUserIDs, err = m.listUserGroupMembers(ctx, group.ID)
		if err != nil {
			return nil, err
		}
		groups = append(groups, group)
	}
	return groups, rows.Err()
}

func (m *MySQL) CreateUserGroup(ctx context.Context, group models.UserGroup) (models.UserGroup, error) {
	group.Name = strings.TrimSpace(group.Name)
	if group.Name == "" {
		return models.UserGroup{}, errors.New("name is required")
	}
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return models.UserGroup{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
		INSERT INTO user_groups (name, description)
		VALUES (?, ?)
	`, group.Name, nullString(group.Description))
	if err != nil {
		return models.UserGroup{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return models.UserGroup{}, err
	}
	for _, userID := range group.MemberUserIDs {
		if userID <= 0 {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT IGNORE INTO user_group_members (user_id, group_id)
			VALUES (?, ?)
		`, userID, id); err != nil {
			return models.UserGroup{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return models.UserGroup{}, err
	}
	return m.findUserGroupByID(ctx, id)
}

func (m *MySQL) ListUserGroupMembers(ctx context.Context, groupID int64) ([]int64, error) {
	group, err := m.findUserGroupByID(ctx, groupID)
	if err != nil {
		return nil, err
	}
	return group.MemberUserIDs, nil
}

func (m *MySQL) AddUserGroupMember(ctx context.Context, groupID, userID int64) (models.UserGroup, error) {
	if groupID <= 0 || userID <= 0 {
		return models.UserGroup{}, errors.New("group_id and user_id must be positive")
	}
	if _, err := m.findUserGroupByID(ctx, groupID); err != nil {
		return models.UserGroup{}, err
	}
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return models.UserGroup{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
		INSERT IGNORE INTO user_group_members (user_id, group_id)
		VALUES (?, ?)
	`, userID, groupID); err != nil {
		return models.UserGroup{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE user_groups SET updated_at = CURRENT_TIMESTAMP WHERE id = ?`, groupID); err != nil {
		return models.UserGroup{}, err
	}
	if err := tx.Commit(); err != nil {
		return models.UserGroup{}, err
	}
	return m.findUserGroupByID(ctx, groupID)
}

func (m *MySQL) RemoveUserGroupMember(ctx context.Context, groupID, userID int64) (models.UserGroup, error) {
	if groupID <= 0 || userID <= 0 {
		return models.UserGroup{}, errors.New("group_id and user_id must be positive")
	}
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return models.UserGroup{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM user_group_members WHERE group_id = ? AND user_id = ?`, groupID, userID); err != nil {
		return models.UserGroup{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE user_groups SET updated_at = CURRENT_TIMESTAMP WHERE id = ?`, groupID); err != nil {
		return models.UserGroup{}, err
	}
	if err := tx.Commit(); err != nil {
		return models.UserGroup{}, err
	}
	return m.findUserGroupByID(ctx, groupID)
}

func (m *MySQL) FindUserByEmail(ctx context.Context, email string) (models.User, error) {
	return scanUser(m.db.QueryRowContext(ctx, `
		SELECT id, email, username, display_name, password_hash, status, mfa_enabled, source, created_at, updated_at, last_login_at
		FROM users
		WHERE email = ?
	`, normalizeEmail(email)))
}

func (m *MySQL) FindUserByID(ctx context.Context, id int64) (models.User, error) {
	return scanUser(m.db.QueryRowContext(ctx, `
		SELECT id, email, username, display_name, password_hash, status, mfa_enabled, source, created_at, updated_at, last_login_at
		FROM users
		WHERE id = ?
	`, id))
}

func (m *MySQL) UpdateUser(ctx context.Context, id int64, user models.User) (models.User, error) {
	user.Email = normalizeEmail(user.Email)
	user.Username = strings.TrimSpace(user.Username)
	user.DisplayName = strings.TrimSpace(user.DisplayName)
	user.Source = strings.TrimSpace(user.Source)
	if user.Email == "" || user.Username == "" {
		return models.User{}, errors.New("email and username are required")
	}
	if user.DisplayName == "" {
		user.DisplayName = user.Username
	}
	if user.Status == "" {
		user.Status = models.UserStatusActive
	}
	if user.Source == "" {
		user.Source = "local"
	}
	result, err := m.db.ExecContext(ctx, `
		UPDATE users
		SET email = ?, username = ?, display_name = ?, status = ?, mfa_enabled = ?, source = ?, updated_at = ?
		WHERE id = ?
	`, user.Email, user.Username, user.DisplayName, user.Status, user.MFAEnabled, user.Source, time.Now().UTC(), id)
	if err != nil {
		return models.User{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return models.User{}, err
	}
	if affected == 0 {
		return models.User{}, ErrNotFound
	}
	return m.FindUserByID(ctx, id)
}

func (m *MySQL) DisableUser(ctx context.Context, id int64, at time.Time) (models.User, error) {
	result, err := m.db.ExecContext(ctx, `
		UPDATE users SET status = 'disabled', updated_at = ? WHERE id = ?
	`, at.UTC(), id)
	if err != nil {
		return models.User{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return models.User{}, err
	}
	if affected == 0 {
		return models.User{}, ErrNotFound
	}
	return m.FindUserByID(ctx, id)
}

func (m *MySQL) RecordUserLogin(ctx context.Context, id int64, at time.Time) (models.User, error) {
	result, err := m.db.ExecContext(ctx, `UPDATE users SET last_login_at = ?, updated_at = ? WHERE id = ?`, at.UTC(), at.UTC(), id)
	if err != nil {
		return models.User{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return models.User{}, err
	}
	if affected == 0 {
		return models.User{}, ErrNotFound
	}
	return m.FindUserByID(ctx, id)
}

func (m *MySQL) ListAPITokens(ctx context.Context) ([]models.APIToken, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, name, token_hash, CAST(scopes AS CHAR), user_id, expires_at, last_used_at, created_at, revoked_at
		FROM api_tokens
		ORDER BY created_at DESC, id DESC
		LIMIT 500
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tokens := []models.APIToken{}
	for rows.Next() {
		token, err := scanAPIToken(rows)
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
	}
	return tokens, rows.Err()
}

func (m *MySQL) CreateAPIToken(ctx context.Context, token models.APIToken) (models.APIToken, error) {
	token.Name = strings.TrimSpace(token.Name)
	if token.Name == "" || strings.TrimSpace(token.TokenHash) == "" || strings.TrimSpace(token.ScopesJSON) == "" {
		return models.APIToken{}, errors.New("name, token_hash, and scopes_json are required")
	}
	result, err := m.db.ExecContext(ctx, `
		INSERT INTO api_tokens (name, token_hash, scopes, user_id, expires_at)
		VALUES (?, ?, ?, ?, ?)
	`, token.Name, token.TokenHash, token.ScopesJSON, nullInt64(token.UserID), nullTime(token.ExpiresAt))
	if err != nil {
		return models.APIToken{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return models.APIToken{}, err
	}
	return m.findAPITokenByID(ctx, id)
}

func (m *MySQL) FindAPITokenByHash(ctx context.Context, tokenHash string) (models.APIToken, error) {
	return scanAPIToken(m.db.QueryRowContext(ctx, `
		SELECT id, name, token_hash, CAST(scopes AS CHAR), user_id, expires_at, last_used_at, created_at, revoked_at
		FROM api_tokens
		WHERE token_hash = ?
	`, strings.TrimSpace(tokenHash)))
}

func (m *MySQL) RevokeAPIToken(ctx context.Context, id int64, at time.Time) (models.APIToken, error) {
	result, err := m.db.ExecContext(ctx, `UPDATE api_tokens SET revoked_at = ? WHERE id = ?`, at.UTC(), id)
	if err != nil {
		return models.APIToken{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return models.APIToken{}, err
	}
	if affected == 0 {
		return models.APIToken{}, ErrNotFound
	}
	return m.findAPITokenByID(ctx, id)
}

func (m *MySQL) RecordAPITokenUsed(ctx context.Context, id int64, at time.Time) (models.APIToken, error) {
	result, err := m.db.ExecContext(ctx, `UPDATE api_tokens SET last_used_at = ? WHERE id = ?`, at.UTC(), id)
	if err != nil {
		return models.APIToken{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return models.APIToken{}, err
	}
	if affected == 0 {
		return models.APIToken{}, ErrNotFound
	}
	return m.findAPITokenByID(ctx, id)
}

func (m *MySQL) CreateSession(ctx context.Context, session models.Session) (models.Session, error) {
	if session.UserID <= 0 || strings.TrimSpace(session.TokenHash) == "" || session.ExpiresAt.IsZero() {
		return models.Session{}, errors.New("user_id, token_hash, and expires_at are required")
	}
	result, err := m.db.ExecContext(ctx, `
		INSERT INTO sessions (user_id, token_hash, ip, user_agent, expires_at)
		VALUES (?, ?, ?, ?, ?)
	`, session.UserID, strings.TrimSpace(session.TokenHash), nullString(session.IP), nullString(session.UserAgent), session.ExpiresAt.UTC())
	if err != nil {
		return models.Session{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return models.Session{}, err
	}
	return m.findSessionByID(ctx, id)
}

func (m *MySQL) FindSessionByTokenHash(ctx context.Context, tokenHash string) (models.Session, error) {
	return scanSession(m.db.QueryRowContext(ctx, `
		SELECT id, user_id, token_hash, ip, user_agent, expires_at, created_at, revoked_at
		FROM sessions
		WHERE token_hash = ?
	`, strings.TrimSpace(tokenHash)))
}

func (m *MySQL) RevokeSession(ctx context.Context, tokenHash string, at time.Time) (models.Session, error) {
	tokenHash = strings.TrimSpace(tokenHash)
	result, err := m.db.ExecContext(ctx, `UPDATE sessions SET revoked_at = COALESCE(revoked_at, ?) WHERE token_hash = ?`, at.UTC(), tokenHash)
	if err != nil {
		return models.Session{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return models.Session{}, err
	}
	if affected == 0 {
		return models.Session{}, ErrNotFound
	}
	return m.FindSessionByTokenHash(ctx, tokenHash)
}

func (m *MySQL) CreateLoginLog(ctx context.Context, log models.LoginLog) (models.LoginLog, error) {
	if strings.TrimSpace(log.Email) == "" {
		return models.LoginLog{}, errors.New("email is required")
	}
	if !validLoginStatus(log.Status) {
		return models.LoginLog{}, errors.New("login status must be succeeded, failed, or denied")
	}
	createdAt := log.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	result, err := m.db.ExecContext(ctx, `
		INSERT INTO login_logs (user_id, email, username, display_name, status, failure_reason, ip, user_agent, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, nullablePositiveInt64(log.UserID), strings.ToLower(strings.TrimSpace(log.Email)), nullString(log.Username), nullString(log.DisplayName), log.Status, nullString(log.FailureReason), nullString(log.IP), nullString(log.UserAgent), createdAt.UTC())
	if err != nil {
		return models.LoginLog{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return models.LoginLog{}, err
	}
	return m.findLoginLogByID(ctx, id)
}

func (m *MySQL) ListDevices(ctx context.Context) ([]models.Device, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, rustdesk_id, name, alias, owner_user_id, status, platform, client_version, opendesk_client_version, last_ip, last_seen_at, registered_at, created_at, updated_at
		FROM devices
		ORDER BY id
		LIMIT 500
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	devices := []models.Device{}
	for rows.Next() {
		device, err := scanDevice(rows)
		if err != nil {
			return nil, err
		}
		devices = append(devices, device)
	}
	return devices, rows.Err()
}

func (m *MySQL) CreateDevice(ctx context.Context, device models.Device) (models.Device, error) {
	device.RustDeskID = strings.TrimSpace(device.RustDeskID)
	device.Name = strings.TrimSpace(device.Name)
	if device.RustDeskID == "" || device.Name == "" {
		return models.Device{}, errors.New("rustdesk_id and name are required")
	}
	if device.Status == "" {
		device.Status = models.DeviceStatusOffline
	}
	result, err := m.db.ExecContext(ctx, `
		INSERT INTO devices (rustdesk_id, name, alias, owner_user_id, status, platform, client_version, opendesk_client_version, last_ip)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, device.RustDeskID, device.Name, nullString(device.Alias), nullInt64(device.OwnerUserID), device.Status, nullString(device.Platform), nullString(device.ClientVersion), nullString(device.OpenDeskClientVersion), nullString(device.LastIP))
	if err != nil {
		return models.Device{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return models.Device{}, err
	}
	return m.findDeviceByID(ctx, id)
}

func (m *MySQL) FindDeviceByID(ctx context.Context, id int64) (models.Device, error) {
	return m.findDeviceByID(ctx, id)
}

func (m *MySQL) UpdateDevice(ctx context.Context, id int64, device models.Device) (models.Device, error) {
	device.RustDeskID = strings.TrimSpace(device.RustDeskID)
	device.Name = strings.TrimSpace(device.Name)
	if device.RustDeskID == "" || device.Name == "" {
		return models.Device{}, errors.New("rustdesk_id and name are required")
	}
	if device.Status == "" {
		device.Status = models.DeviceStatusOffline
	}
	now := time.Now().UTC()
	result, err := m.db.ExecContext(ctx, `
		UPDATE devices
		SET rustdesk_id = ?, name = ?, alias = ?, owner_user_id = ?, status = ?, platform = ?, client_version = ?, opendesk_client_version = ?, last_ip = ?, updated_at = ?
		WHERE id = ?
	`, device.RustDeskID, device.Name, nullString(device.Alias), nullInt64(device.OwnerUserID), device.Status, nullString(device.Platform), nullString(device.ClientVersion), nullString(device.OpenDeskClientVersion), nullString(device.LastIP), now, id)
	if err != nil {
		return models.Device{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return models.Device{}, err
	}
	if affected == 0 {
		return models.Device{}, ErrNotFound
	}
	return m.findDeviceByID(ctx, id)
}

func (m *MySQL) DisableDevice(ctx context.Context, id int64, at time.Time) (models.Device, error) {
	result, err := m.db.ExecContext(ctx, `
		UPDATE devices SET status = 'disabled', updated_at = ? WHERE id = ?
	`, at.UTC(), id)
	if err != nil {
		return models.Device{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return models.Device{}, err
	}
	if affected == 0 {
		return models.Device{}, ErrNotFound
	}
	return m.findDeviceByID(ctx, id)
}

func (m *MySQL) ListDeviceGroups(ctx context.Context) ([]models.DeviceGroup, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, name, description, created_at, updated_at
		FROM device_groups
		ORDER BY id DESC
		LIMIT 500
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	groups := []models.DeviceGroup{}
	for rows.Next() {
		group, err := scanDeviceGroup(rows)
		if err != nil {
			return nil, err
		}
		group.MemberDeviceIDs, err = m.listDeviceGroupMembers(ctx, group.ID)
		if err != nil {
			return nil, err
		}
		groups = append(groups, group)
	}
	return groups, rows.Err()
}

func (m *MySQL) CreateDeviceGroup(ctx context.Context, group models.DeviceGroup) (models.DeviceGroup, error) {
	group.Name = strings.TrimSpace(group.Name)
	if group.Name == "" {
		return models.DeviceGroup{}, errors.New("name is required")
	}
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return models.DeviceGroup{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
		INSERT INTO device_groups (name, description)
		VALUES (?, ?)
	`, group.Name, nullString(group.Description))
	if err != nil {
		return models.DeviceGroup{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return models.DeviceGroup{}, err
	}
	for _, deviceID := range group.MemberDeviceIDs {
		if deviceID <= 0 {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT IGNORE INTO device_group_members (device_id, group_id)
			VALUES (?, ?)
		`, deviceID, id); err != nil {
			return models.DeviceGroup{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return models.DeviceGroup{}, err
	}
	return m.findDeviceGroupByID(ctx, id)
}

func (m *MySQL) ListDeviceGroupMembers(ctx context.Context, groupID int64) ([]int64, error) {
	group, err := m.findDeviceGroupByID(ctx, groupID)
	if err != nil {
		return nil, err
	}
	return group.MemberDeviceIDs, nil
}

func (m *MySQL) AddDeviceGroupMember(ctx context.Context, groupID, deviceID int64) (models.DeviceGroup, error) {
	if groupID <= 0 || deviceID <= 0 {
		return models.DeviceGroup{}, errors.New("group_id and device_id must be positive")
	}
	if _, err := m.findDeviceGroupByID(ctx, groupID); err != nil {
		return models.DeviceGroup{}, err
	}
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return models.DeviceGroup{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
		INSERT IGNORE INTO device_group_members (device_id, group_id)
		VALUES (?, ?)
	`, deviceID, groupID); err != nil {
		return models.DeviceGroup{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE device_groups SET updated_at = CURRENT_TIMESTAMP WHERE id = ?`, groupID); err != nil {
		return models.DeviceGroup{}, err
	}
	if err := tx.Commit(); err != nil {
		return models.DeviceGroup{}, err
	}
	return m.findDeviceGroupByID(ctx, groupID)
}

func (m *MySQL) RemoveDeviceGroupMember(ctx context.Context, groupID, deviceID int64) (models.DeviceGroup, error) {
	if groupID <= 0 || deviceID <= 0 {
		return models.DeviceGroup{}, errors.New("group_id and device_id must be positive")
	}
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return models.DeviceGroup{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM device_group_members WHERE group_id = ? AND device_id = ?`, groupID, deviceID); err != nil {
		return models.DeviceGroup{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE device_groups SET updated_at = CURRENT_TIMESTAMP WHERE id = ?`, groupID); err != nil {
		return models.DeviceGroup{}, err
	}
	if err := tx.Commit(); err != nil {
		return models.DeviceGroup{}, err
	}
	return m.findDeviceGroupByID(ctx, groupID)
}

func (m *MySQL) ListAddressBooks(ctx context.Context) ([]models.AddressBook, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, name, description, owner_user_id, created_at, updated_at
		FROM address_books
		ORDER BY id DESC
		LIMIT 500
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	books := []models.AddressBook{}
	for rows.Next() {
		book, err := scanAddressBook(rows)
		if err != nil {
			return nil, err
		}
		book.Entries, err = m.listAddressBookEntries(ctx, book.ID)
		if err != nil {
			return nil, err
		}
		books = append(books, book)
	}
	return books, rows.Err()
}

func (m *MySQL) CreateAddressBook(ctx context.Context, book models.AddressBook) (models.AddressBook, error) {
	book.Name = strings.TrimSpace(book.Name)
	if book.Name == "" {
		return models.AddressBook{}, errors.New("name is required")
	}
	for _, entry := range book.Entries {
		if entry.DeviceID <= 0 {
			return models.AddressBook{}, errors.New("entry device_id must be positive")
		}
	}
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return models.AddressBook{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
		INSERT INTO address_books (name, description, owner_user_id)
		VALUES (?, ?, ?)
	`, book.Name, nullString(book.Description), nullInt64(book.OwnerUserID))
	if err != nil {
		return models.AddressBook{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return models.AddressBook{}, err
	}
	for _, entry := range book.Entries {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO address_book_entries (address_book_id, device_id, alias)
			VALUES (?, ?, ?)
		`, id, entry.DeviceID, nullString(entry.Alias)); err != nil {
			return models.AddressBook{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return models.AddressBook{}, err
	}
	return m.findAddressBookByID(ctx, id)
}

func (m *MySQL) ListAddressBookEntries(ctx context.Context, bookID int64) ([]models.AddressBookEntry, error) {
	book, err := m.findAddressBookByID(ctx, bookID)
	if err != nil {
		return nil, err
	}
	return book.Entries, nil
}

func (m *MySQL) AddAddressBookEntry(ctx context.Context, bookID int64, entry models.AddressBookEntry) (models.AddressBook, error) {
	if bookID <= 0 || entry.DeviceID <= 0 {
		return models.AddressBook{}, errors.New("book_id and device_id must be positive")
	}
	if _, err := m.findAddressBookByID(ctx, bookID); err != nil {
		return models.AddressBook{}, err
	}
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return models.AddressBook{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO address_book_entries (address_book_id, device_id, alias)
		VALUES (?, ?, ?)
	`, bookID, entry.DeviceID, nullString(strings.TrimSpace(entry.Alias))); err != nil {
		return models.AddressBook{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE address_books SET updated_at = CURRENT_TIMESTAMP WHERE id = ?`, bookID); err != nil {
		return models.AddressBook{}, err
	}
	if err := tx.Commit(); err != nil {
		return models.AddressBook{}, err
	}
	return m.findAddressBookByID(ctx, bookID)
}

func (m *MySQL) RemoveAddressBookEntry(ctx context.Context, bookID, entryID int64) (models.AddressBook, error) {
	if bookID <= 0 || entryID <= 0 {
		return models.AddressBook{}, errors.New("book_id and entry_id must be positive")
	}
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return models.AddressBook{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `DELETE FROM address_book_entries WHERE address_book_id = ? AND id = ?`, bookID, entryID)
	if err != nil {
		return models.AddressBook{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return models.AddressBook{}, err
	}
	if affected == 0 {
		return models.AddressBook{}, ErrNotFound
	}
	if _, err := tx.ExecContext(ctx, `UPDATE address_books SET updated_at = CURRENT_TIMESTAMP WHERE id = ?`, bookID); err != nil {
		return models.AddressBook{}, err
	}
	if err := tx.Commit(); err != nil {
		return models.AddressBook{}, err
	}
	return m.findAddressBookByID(ctx, bookID)
}

func (m *MySQL) ListRelays(ctx context.Context) ([]models.Relay, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, name, region, host, port, ws_port, public_key_fingerprint, status, max_bandwidth_mbps, current_sessions, last_health_at, created_at, updated_at
		FROM relays
		ORDER BY id
		LIMIT 500
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	relays := []models.Relay{}
	for rows.Next() {
		relay, err := scanRelay(rows)
		if err != nil {
			return nil, err
		}
		relays = append(relays, relay)
	}
	return relays, rows.Err()
}

func (m *MySQL) CreateRelay(ctx context.Context, relay models.Relay) (models.Relay, error) {
	relay.Name = strings.TrimSpace(relay.Name)
	relay.Region = strings.TrimSpace(relay.Region)
	relay.Host = strings.TrimSpace(relay.Host)
	if relay.Name == "" || relay.Region == "" || relay.Host == "" {
		return models.Relay{}, errors.New("name, region, and host are required")
	}
	if relay.Port == 0 {
		relay.Port = 21117
	}
	if relay.WSPort == 0 {
		relay.WSPort = 21119
	}
	if relay.Status == "" {
		relay.Status = "active"
	}
	result, err := m.db.ExecContext(ctx, `
		INSERT INTO relays (name, region, host, port, ws_port, public_key_fingerprint, status, max_bandwidth_mbps)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, relay.Name, relay.Region, relay.Host, relay.Port, relay.WSPort, nullString(relay.PublicKeyFingerprint), relay.Status, nullIntPointer(relay.MaxBandwidthMbps))
	if err != nil {
		return models.Relay{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return models.Relay{}, err
	}
	return m.findRelayByID(ctx, id)
}

func (m *MySQL) UpdateRelay(ctx context.Context, id int64, relay models.Relay) (models.Relay, error) {
	relay.Name = strings.TrimSpace(relay.Name)
	relay.Region = strings.TrimSpace(relay.Region)
	relay.Host = strings.TrimSpace(relay.Host)
	if relay.Name == "" || relay.Region == "" || relay.Host == "" {
		return models.Relay{}, errors.New("name, region, and host are required")
	}
	if relay.Port == 0 {
		relay.Port = 21117
	}
	if relay.WSPort == 0 {
		relay.WSPort = 21119
	}
	if relay.Status == "" {
		relay.Status = "active"
	}
	now := time.Now().UTC()
	result, err := m.db.ExecContext(ctx, `
		UPDATE relays
		SET name = ?, region = ?, host = ?, port = ?, ws_port = ?, public_key_fingerprint = ?, status = ?, max_bandwidth_mbps = ?, updated_at = ?
		WHERE id = ?
	`, relay.Name, relay.Region, relay.Host, relay.Port, relay.WSPort, nullString(relay.PublicKeyFingerprint), relay.Status, nullIntPointer(relay.MaxBandwidthMbps), now, id)
	if err != nil {
		return models.Relay{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return models.Relay{}, err
	}
	if affected == 0 {
		return models.Relay{}, ErrNotFound
	}
	return m.findRelayByID(ctx, id)
}

func (m *MySQL) RelayHeartbeat(ctx context.Context, id int64, currentSessions *int, status string) (models.Relay, error) {
	now := time.Now().UTC()
	var (
		result sql.Result
		err    error
	)
	switch {
	case currentSessions != nil && status != "":
		result, err = m.db.ExecContext(ctx, `UPDATE relays SET current_sessions = ?, status = ?, last_health_at = ?, updated_at = ? WHERE id = ?`, *currentSessions, status, now, now, id)
	case currentSessions != nil:
		result, err = m.db.ExecContext(ctx, `UPDATE relays SET current_sessions = ?, last_health_at = ?, updated_at = ? WHERE id = ?`, *currentSessions, now, now, id)
	case status != "":
		result, err = m.db.ExecContext(ctx, `UPDATE relays SET status = ?, last_health_at = ?, updated_at = ? WHERE id = ?`, status, now, now, id)
	default:
		result, err = m.db.ExecContext(ctx, `UPDATE relays SET last_health_at = ?, updated_at = ? WHERE id = ?`, now, now, id)
	}
	if err != nil {
		return models.Relay{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return models.Relay{}, err
	}
	if affected == 0 {
		return models.Relay{}, ErrNotFound
	}
	return m.findRelayByID(ctx, id)
}

func (m *MySQL) DisableRelay(ctx context.Context, id int64, at time.Time) (models.Relay, error) {
	when := at.UTC()
	result, err := m.db.ExecContext(ctx, `
		UPDATE relays SET status = 'disabled', updated_at = ? WHERE id = ?
	`, when, id)
	if err != nil {
		return models.Relay{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return models.Relay{}, err
	}
	if affected == 0 {
		return models.Relay{}, ErrNotFound
	}
	return m.findRelayByID(ctx, id)
}

func (m *MySQL) CreateRelayGrant(ctx context.Context, grant models.RelayGrant) (models.RelayGrant, error) {
	grant.GrantID = strings.TrimSpace(grant.GrantID)
	grant.TargetRustDeskID = strings.TrimSpace(grant.TargetRustDeskID)
	grant.Nonce = strings.TrimSpace(grant.Nonce)
	if grant.GrantID == "" || grant.Nonce == "" || grant.ExpiresAt.IsZero() || len(grant.AllowedRelays) == 0 {
		return models.RelayGrant{}, errors.New("grant_id, nonce, expires_at, and allowed_relays are required")
	}
	if grant.Status == "" {
		grant.Status = "issued"
	}
	allowedRelaysJSON, err := json.Marshal(grant.AllowedRelays)
	if err != nil {
		return models.RelayGrant{}, err
	}
	result, err := m.db.ExecContext(ctx, `
		INSERT INTO relay_grants (grant_id, user_id, controller_device_id, target_device_id, target_rustdesk_id, relay_id, allowed_relays, expires_at, nonce, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, grant.GrantID, nullInt64(grant.UserID), nullInt64(grant.ControllerDeviceID), nullInt64(grant.TargetDeviceID), nullString(grant.TargetRustDeskID), nullInt64(grant.RelayID), string(allowedRelaysJSON), grant.ExpiresAt.UTC(), grant.Nonce, grant.Status)
	if err != nil {
		return models.RelayGrant{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return models.RelayGrant{}, err
	}
	return m.findRelayGrantByID(ctx, id)
}

func (m *MySQL) FindRelayGrantByGrantID(ctx context.Context, grantID string) (models.RelayGrant, error) {
	return scanRelayGrant(m.db.QueryRowContext(ctx, `
		SELECT id, grant_id, user_id, controller_device_id, target_device_id, target_rustdesk_id, relay_id, CAST(allowed_relays AS CHAR), expires_at, nonce, status, created_at, used_at
		FROM relay_grants
		WHERE grant_id = ?
	`, strings.TrimSpace(grantID)))
}

func (m *MySQL) UpdateRelayGrantStatus(ctx context.Context, grantID string, status string, usedAt *time.Time) (models.RelayGrant, error) {
	if strings.TrimSpace(status) == "" {
		return models.RelayGrant{}, errors.New("status is required")
	}
	result, err := m.db.ExecContext(ctx, `UPDATE relay_grants SET status = ?, used_at = COALESCE(?, used_at) WHERE grant_id = ?`, strings.TrimSpace(status), nullTime(usedAt), strings.TrimSpace(grantID))
	if err != nil {
		return models.RelayGrant{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return models.RelayGrant{}, err
	}
	if affected == 0 {
		return models.RelayGrant{}, ErrNotFound
	}
	return m.FindRelayGrantByGrantID(ctx, grantID)
}

func (m *MySQL) ListAccessRules(ctx context.Context) ([]models.AccessRule, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, subject_type, subject_id, target_type, target_id, effect, priority, enabled, created_at, updated_at
		FROM access_rules
		ORDER BY priority DESC, id DESC
		LIMIT 500
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rules := []models.AccessRule{}
	for rows.Next() {
		rule, err := scanAccessRule(rows)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

func (m *MySQL) CreateAccessRule(ctx context.Context, rule models.AccessRule) (models.AccessRule, error) {
	if rule.SubjectType == "" || rule.SubjectID <= 0 || rule.TargetType == "" || rule.TargetID <= 0 || rule.Effect == "" {
		return models.AccessRule{}, errors.New("subject_type, subject_id, target_type, target_id, and effect are required")
	}
	result, err := m.db.ExecContext(ctx, `
		INSERT INTO access_rules (subject_type, subject_id, target_type, target_id, effect, priority, enabled)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, rule.SubjectType, rule.SubjectID, rule.TargetType, rule.TargetID, rule.Effect, rule.Priority, rule.Enabled)
	if err != nil {
		return models.AccessRule{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return models.AccessRule{}, err
	}
	return m.findAccessRuleByID(ctx, id)
}

func (m *MySQL) UpdateAccessRule(ctx context.Context, id int64, rule models.AccessRule) (models.AccessRule, error) {
	if rule.SubjectType == "" || rule.SubjectID <= 0 || rule.TargetType == "" || rule.TargetID <= 0 || rule.Effect == "" {
		return models.AccessRule{}, errors.New("subject_type, subject_id, target_type, target_id, and effect are required")
	}
	result, err := m.db.ExecContext(ctx, `
		UPDATE access_rules
		SET subject_type = ?, subject_id = ?, target_type = ?, target_id = ?, effect = ?, priority = ?, enabled = ?, updated_at = ?
		WHERE id = ?
	`, rule.SubjectType, rule.SubjectID, rule.TargetType, rule.TargetID, rule.Effect, rule.Priority, rule.Enabled, time.Now().UTC(), id)
	if err != nil {
		return models.AccessRule{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return models.AccessRule{}, err
	}
	if affected == 0 {
		if err := requireRowExists(ctx, m.db, "access_rules", id); err != nil {
			return models.AccessRule{}, err
		}
	}
	return m.findAccessRuleByID(ctx, id)
}

func (m *MySQL) DeleteAccessRule(ctx context.Context, id int64) error {
	result, err := m.db.ExecContext(ctx, `DELETE FROM access_rules WHERE id = ?`, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (m *MySQL) ListControlRoles(ctx context.Context) ([]models.ControlRole, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, name, description, enabled, created_at, updated_at
		FROM control_roles
		ORDER BY id DESC
		LIMIT 500
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	roles := []models.ControlRole{}
	for rows.Next() {
		role, err := scanControlRole(rows)
		if err != nil {
			return nil, err
		}
		role.Permissions, err = m.listControlRolePermissions(ctx, role.ID)
		if err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

func (m *MySQL) CreateControlRole(ctx context.Context, role models.ControlRole) (models.ControlRole, error) {
	role.Name = strings.TrimSpace(role.Name)
	if role.Name == "" {
		return models.ControlRole{}, errors.New("name is required")
	}
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return models.ControlRole{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
		INSERT INTO control_roles (name, description, enabled)
		VALUES (?, ?, ?)
	`, role.Name, nullString(role.Description), role.Enabled)
	if err != nil {
		return models.ControlRole{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return models.ControlRole{}, err
	}
	for _, permission := range role.Permissions {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO control_role_permissions (role_id, permission_key, mode)
			VALUES (?, ?, ?)
		`, id, permission.PermissionKey, permission.Mode); err != nil {
			return models.ControlRole{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return models.ControlRole{}, err
	}
	return m.findControlRoleByID(ctx, id)
}

func (m *MySQL) UpdateControlRole(ctx context.Context, id int64, role models.ControlRole) (models.ControlRole, error) {
	role.Name = strings.TrimSpace(role.Name)
	if role.Name == "" {
		return models.ControlRole{}, errors.New("name is required")
	}
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return models.ControlRole{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
		UPDATE control_roles
		SET name = ?, description = ?, enabled = ?, updated_at = ?
		WHERE id = ?
	`, role.Name, nullString(role.Description), role.Enabled, time.Now().UTC(), id)
	if err != nil {
		return models.ControlRole{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return models.ControlRole{}, err
	}
	if affected == 0 {
		if err := requireRowExists(ctx, tx, "control_roles", id); err != nil {
			return models.ControlRole{}, err
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM control_role_permissions WHERE role_id = ?`, id); err != nil {
		return models.ControlRole{}, err
	}
	for _, permission := range role.Permissions {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO control_role_permissions (role_id, permission_key, mode)
			VALUES (?, ?, ?)
		`, id, permission.PermissionKey, permission.Mode); err != nil {
			return models.ControlRole{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return models.ControlRole{}, err
	}
	return m.findControlRoleByID(ctx, id)
}

func (m *MySQL) DeleteControlRole(ctx context.Context, id int64) error {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM control_role_permissions WHERE role_id = ?`, id); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM control_roles WHERE id = ?`, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return tx.Commit()
}

func (m *MySQL) ListStrategies(ctx context.Context) ([]models.Strategy, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, name, description, enabled, settings_json, created_at, updated_at
		FROM strategies
		ORDER BY id DESC
		LIMIT 500
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	strategies := []models.Strategy{}
	for rows.Next() {
		strategy, err := scanStrategy(rows)
		if err != nil {
			return nil, err
		}
		strategy.Assignments, err = m.listStrategyAssignments(ctx, strategy.ID)
		if err != nil {
			return nil, err
		}
		strategies = append(strategies, strategy)
	}
	return strategies, rows.Err()
}

func (m *MySQL) CreateStrategy(ctx context.Context, strategy models.Strategy) (models.Strategy, error) {
	strategy.Name = strings.TrimSpace(strategy.Name)
	if strategy.Name == "" || strings.TrimSpace(strategy.SettingsJSON) == "" {
		return models.Strategy{}, errors.New("name and settings_json are required")
	}
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return models.Strategy{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
		INSERT INTO strategies (name, description, enabled, settings_json)
		VALUES (?, ?, ?, ?)
	`, strategy.Name, nullString(strategy.Description), strategy.Enabled, strategy.SettingsJSON)
	if err != nil {
		return models.Strategy{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return models.Strategy{}, err
	}
	for _, assignment := range strategy.Assignments {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO strategy_assignments (strategy_id, target_type, target_id)
			VALUES (?, ?, ?)
		`, id, assignment.TargetType, assignment.TargetID); err != nil {
			return models.Strategy{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return models.Strategy{}, err
	}
	return m.findStrategyByID(ctx, id)
}

func (m *MySQL) UpdateStrategy(ctx context.Context, id int64, strategy models.Strategy) (models.Strategy, error) {
	strategy.Name = strings.TrimSpace(strategy.Name)
	if strategy.Name == "" || strings.TrimSpace(strategy.SettingsJSON) == "" {
		return models.Strategy{}, errors.New("name and settings_json are required")
	}
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return models.Strategy{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
		UPDATE strategies
		SET name = ?, description = ?, enabled = ?, settings_json = ?, updated_at = ?
		WHERE id = ?
	`, strategy.Name, nullString(strategy.Description), strategy.Enabled, strategy.SettingsJSON, time.Now().UTC(), id)
	if err != nil {
		return models.Strategy{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return models.Strategy{}, err
	}
	if affected == 0 {
		if err := requireRowExists(ctx, tx, "strategies", id); err != nil {
			return models.Strategy{}, err
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM strategy_assignments WHERE strategy_id = ?`, id); err != nil {
		return models.Strategy{}, err
	}
	for _, assignment := range strategy.Assignments {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO strategy_assignments (strategy_id, target_type, target_id)
			VALUES (?, ?, ?)
		`, id, assignment.TargetType, assignment.TargetID); err != nil {
			return models.Strategy{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return models.Strategy{}, err
	}
	return m.findStrategyByID(ctx, id)
}

func (m *MySQL) DeleteStrategy(ctx context.Context, id int64) error {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM strategy_assignments WHERE strategy_id = ?`, id); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM strategies WHERE id = ?`, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return tx.Commit()
}

func (m *MySQL) AddStrategyAssignment(ctx context.Context, strategyID int64, assignment models.StrategyAssignment) (models.Strategy, error) {
	if assignment.TargetType == "" || assignment.TargetID <= 0 {
		return models.Strategy{}, errors.New("target_type and target_id are required")
	}
	if err := requireRowExists(ctx, m.db, "strategies", strategyID); err != nil {
		return models.Strategy{}, err
	}
	if _, err := m.db.ExecContext(ctx, `
		INSERT INTO strategy_assignments (strategy_id, target_type, target_id)
		VALUES (?, ?, ?)
	`, strategyID, assignment.TargetType, assignment.TargetID); err != nil {
		return models.Strategy{}, err
	}
	return m.findStrategyByID(ctx, strategyID)
}

func (m *MySQL) RemoveStrategyAssignment(ctx context.Context, strategyID int64, assignmentID int64) (models.Strategy, error) {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return models.Strategy{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
		DELETE FROM strategy_assignments
		WHERE strategy_id = ? AND id = ?
	`, strategyID, assignmentID)
	if err != nil {
		return models.Strategy{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return models.Strategy{}, err
	}
	if affected == 0 {
		if err := requireRowExists(ctx, tx, "strategies", strategyID); err != nil {
			return models.Strategy{}, err
		}
		return models.Strategy{}, ErrNotFound
	}
	if err := tx.Commit(); err != nil {
		return models.Strategy{}, err
	}
	return m.findStrategyByID(ctx, strategyID)
}

func (m *MySQL) CreateAuditEvent(ctx context.Context, event models.AuditEvent) (models.AuditEvent, error) {
	metadata := event.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return models.AuditEvent{}, err
	}
	result, err := m.db.ExecContext(ctx, `
		INSERT INTO audit_events (actor_user_id, actor_type, action, resource_type, resource_id, ip, user_agent, metadata_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, nullInt64(event.ActorUserID), event.ActorType, event.Action, event.ResourceType, nullStringPointer(event.ResourceID), nullString(event.IP), nullString(event.UserAgent), string(metadataJSON))
	if err != nil {
		return models.AuditEvent{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return models.AuditEvent{}, err
	}
	return m.findAuditEventByID(ctx, id)
}

func (m *MySQL) ListAuditEvents(ctx context.Context, filter AuditLogFilter) ([]models.AuditEvent, error) {
	query := `
		SELECT id, actor_user_id, actor_type, action, resource_type, resource_id, ip, user_agent, CAST(metadata_json AS CHAR), created_at
		FROM audit_events
		WHERE 1=1
	`
	args := []any{}
	if filter.ActorType != "" {
		query += ` AND actor_type = ?`
		args = append(args, filter.ActorType)
	}
	if filter.Action != "" {
		query += ` AND action = ?`
		args = append(args, filter.Action)
	}
	if filter.ResourceType != "" {
		query += ` AND resource_type = ?`
		args = append(args, filter.ResourceType)
	}
	if filter.ResourceID != "" {
		query += ` AND resource_id = ?`
		args = append(args, filter.ResourceID)
	}
	if filter.From != nil {
		query += ` AND created_at >= ?`
		args = append(args, *filter.From)
	}
	if filter.To != nil {
		query += ` AND created_at <= ?`
		args = append(args, *filter.To)
	}
	query += ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, effectiveLimit(filter.Limit), effectiveOffset(filter.Offset))
	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := []models.AuditEvent{}
	for rows.Next() {
		event, err := scanAuditEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (m *MySQL) CreateConnectionLog(ctx context.Context, log models.ConnectionLog) (models.ConnectionLog, error) {
	if !validConnectionType(log.ConnectionType) {
		return models.ConnectionLog{}, errors.New("connection_type must be direct, relay, or websocket")
	}
	if !validConnectionStatus(log.Status) {
		return models.ConnectionLog{}, errors.New("status must be started, ended, failed, or denied")
	}
	if log.StartedAt.IsZero() {
		log.StartedAt = time.Now().UTC()
	}
	metadata := log.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return models.ConnectionLog{}, err
	}
	result, err := m.db.ExecContext(ctx, `
		INSERT INTO connection_logs (session_id, controller_user_id, controller_device_id, target_device_id, connection_type, relay_id, started_at, ended_at, status, deny_reason, metadata_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, nullInt64(log.SessionID), nullInt64(log.ControllerUserID), nullInt64(log.ControllerDeviceID), nullInt64(log.TargetDeviceID), log.ConnectionType, nullInt64(log.RelayID), log.StartedAt.UTC(), nullTime(log.EndedAt), log.Status, nullString(log.DenyReason), string(metadataJSON))
	if err != nil {
		return models.ConnectionLog{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return models.ConnectionLog{}, err
	}
	return m.findConnectionLogByID(ctx, id)
}

func (m *MySQL) ListConnectionLogs(ctx context.Context, filter ConnectionLogFilter) ([]models.ConnectionLog, error) {
	query := `
		SELECT id, session_id, controller_user_id, controller_device_id, target_device_id, connection_type, relay_id, started_at, ended_at, status, deny_reason, CAST(metadata_json AS CHAR)
		FROM connection_logs
		WHERE 1=1
	`
	args := []any{}
	if filter.Status != "" {
		query += ` AND status = ?`
		args = append(args, filter.Status)
	}
	if filter.ConnectionType != "" {
		query += ` AND connection_type = ?`
		args = append(args, filter.ConnectionType)
	}
	if filter.From != nil {
		query += ` AND started_at >= ?`
		args = append(args, *filter.From)
	}
	if filter.To != nil {
		query += ` AND started_at <= ?`
		args = append(args, *filter.To)
	}
	query += ` ORDER BY started_at DESC LIMIT ? OFFSET ?`
	args = append(args, effectiveLimit(filter.Limit), effectiveOffset(filter.Offset))
	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	logs := []models.ConnectionLog{}
	for rows.Next() {
		log, err := scanConnectionLog(rows)
		if err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}
	return logs, rows.Err()
}

func (m *MySQL) ListFileTransferLogs(ctx context.Context, filter FileTransferLogFilter) ([]models.FileTransferLog, error) {
	query := `
		SELECT id, connection_log_id, direction, filename_hash, size_bytes, status, created_at
		FROM file_transfer_logs
		WHERE 1=1
	`
	args := []any{}
	if filter.Direction != "" {
		query += ` AND direction = ?`
		args = append(args, filter.Direction)
	}
	if filter.Status != "" {
		query += ` AND status = ?`
		args = append(args, filter.Status)
	}
	if filter.From != nil {
		query += ` AND created_at >= ?`
		args = append(args, *filter.From)
	}
	if filter.To != nil {
		query += ` AND created_at <= ?`
		args = append(args, *filter.To)
	}
	query += ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, effectiveLimit(filter.Limit), effectiveOffset(filter.Offset))
	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	logs := []models.FileTransferLog{}
	for rows.Next() {
		log, err := scanFileTransferLog(rows)
		if err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}
	return logs, rows.Err()
}

func (m *MySQL) ListLoginLogs(ctx context.Context, filter LoginLogFilter) ([]models.LoginLog, error) {
	query := `
		SELECT id, user_id, email, username, display_name, status, failure_reason, ip, user_agent, created_at
		FROM login_logs
		WHERE 1=1
	`
	args := []any{}
	if filter.Email != "" {
		query += ` AND LOWER(email) = ?`
		args = append(args, strings.ToLower(filter.Email))
	}
	if filter.Status != "" {
		query += ` AND status = ?`
		args = append(args, filter.Status)
	}
	if filter.From != nil {
		query += ` AND created_at >= ?`
		args = append(args, *filter.From)
	}
	if filter.To != nil {
		query += ` AND created_at <= ?`
		args = append(args, *filter.To)
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`
	args = append(args, effectiveLimit(filter.Limit), effectiveOffset(filter.Offset))
	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	logs := []models.LoginLog{}
	for rows.Next() {
		log, err := scanLoginLog(rows)
		if err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}
	return logs, rows.Err()
}

func (m *MySQL) ListSystemSettings(ctx context.Context) ([]models.SystemSetting, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT setting_key, CAST(setting_value AS CHAR), updated_by, updated_at
		FROM system_settings
		ORDER BY setting_key
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	settings := []models.SystemSetting{}
	for rows.Next() {
		setting, err := scanSystemSetting(rows)
		if err != nil {
			return nil, err
		}
		settings = append(settings, setting)
	}
	return settings, rows.Err()
}

func (m *MySQL) UpsertSystemSettings(ctx context.Context, settings []models.SystemSetting, updatedBy int64) error {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, setting := range settings {
		key := strings.TrimSpace(setting.Key)
		if key == "" || strings.TrimSpace(setting.ValueJSON) == "" {
			return errors.New("setting key and value_json are required")
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO system_settings (setting_key, setting_value, updated_by)
			VALUES (?, ?, ?)
			ON DUPLICATE KEY UPDATE
				setting_value = VALUES(setting_value),
				updated_by = VALUES(updated_by)
		`, key, setting.ValueJSON, nullablePositiveInt64(updatedBy)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (m *MySQL) ListBuildProfiles(ctx context.Context) ([]models.BuildProfile, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, name, app_name, vendor, bundle_id, product_name, description, server_config_json, branding_json, policy_json, platforms_json, signing_json, source_json, created_by, created_at, updated_at
		FROM build_profiles
		ORDER BY id DESC
		LIMIT 500
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	profiles := []models.BuildProfile{}
	for rows.Next() {
		profile, err := scanBuildProfile(rows)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, profile)
	}
	return profiles, rows.Err()
}

func (m *MySQL) CreateBuildProfile(ctx context.Context, profile models.BuildProfile) (models.BuildProfile, error) {
	if profile.Name == "" || profile.AppName == "" || profile.BundleID == "" {
		return models.BuildProfile{}, errors.New("name, app_name, and bundle_id are required")
	}
	result, err := m.db.ExecContext(ctx, `
		INSERT INTO build_profiles (name, app_name, vendor, bundle_id, product_name, description, server_config_json, branding_json, policy_json, platforms_json, signing_json, source_json, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, profile.Name, profile.AppName, nullString(profile.Vendor), profile.BundleID, nullString(profile.ProductName), nullString(profile.Description), profile.ServerConfigJSON, profile.BrandingJSON, profile.PolicyJSON, profile.PlatformsJSON, profile.SigningJSON, profile.SourceJSON, profile.CreatedBy)
	if err != nil {
		return models.BuildProfile{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return models.BuildProfile{}, err
	}
	return m.FindBuildProfileByID(ctx, id)
}

func (m *MySQL) FindBuildProfileByID(ctx context.Context, id int64) (models.BuildProfile, error) {
	return scanBuildProfile(m.db.QueryRowContext(ctx, `
		SELECT id, name, app_name, vendor, bundle_id, product_name, description, server_config_json, branding_json, policy_json, platforms_json, signing_json, source_json, created_by, created_at, updated_at
		FROM build_profiles
		WHERE id = ?
	`, id))
}

func (m *MySQL) UpdateBuildProfile(ctx context.Context, id int64, profile models.BuildProfile) (models.BuildProfile, error) {
	if profile.Name == "" || profile.AppName == "" || profile.BundleID == "" {
		return models.BuildProfile{}, errors.New("name, app_name, and bundle_id are required")
	}
	result, err := m.db.ExecContext(ctx, `
		UPDATE build_profiles
		SET name = ?, app_name = ?, vendor = ?, bundle_id = ?, product_name = ?, description = ?, server_config_json = ?, branding_json = ?, policy_json = ?, platforms_json = ?, signing_json = ?, source_json = ?, updated_at = ?
		WHERE id = ?
	`, profile.Name, profile.AppName, nullString(profile.Vendor), profile.BundleID, nullString(profile.ProductName), nullString(profile.Description), profile.ServerConfigJSON, profile.BrandingJSON, profile.PolicyJSON, profile.PlatformsJSON, profile.SigningJSON, profile.SourceJSON, time.Now().UTC(), id)
	if err != nil {
		return models.BuildProfile{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return models.BuildProfile{}, err
	}
	if affected == 0 {
		if err := requireRowExists(ctx, m.db, "build_profiles", id); err != nil {
			return models.BuildProfile{}, err
		}
	}
	return m.FindBuildProfileByID(ctx, id)
}

func (m *MySQL) DeleteBuildProfile(ctx context.Context, id int64) error {
	var jobCount int
	if err := m.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM build_jobs WHERE profile_id = ?`, id).Scan(&jobCount); err != nil {
		return err
	}
	if jobCount > 0 {
		return ErrConflict
	}
	result, err := m.db.ExecContext(ctx, `DELETE FROM build_profiles WHERE id = ?`, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (m *MySQL) ListBuildJobs(ctx context.Context) ([]models.BuildJob, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, profile_id, platform, status, runner, log_path, error_message, created_at, started_at, finished_at
		FROM build_jobs
		ORDER BY id DESC
		LIMIT 500
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	jobs := []models.BuildJob{}
	for rows.Next() {
		job, err := scanBuildJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (m *MySQL) CreateBuildJob(ctx context.Context, job models.BuildJob) (models.BuildJob, error) {
	if job.ProfileID <= 0 || job.Platform == "" {
		return models.BuildJob{}, errors.New("profile_id and platform are required")
	}
	if job.Status == "" {
		job.Status = models.BuildJobQueued
	}
	result, err := m.db.ExecContext(ctx, `
		INSERT INTO build_jobs (profile_id, platform, status, runner, log_path, error_message)
		VALUES (?, ?, ?, ?, ?, ?)
	`, job.ProfileID, job.Platform, job.Status, nullString(job.Runner), nullString(job.LogPath), nullString(job.ErrorMessage))
	if err != nil {
		return models.BuildJob{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return models.BuildJob{}, err
	}
	return m.findBuildJobByID(ctx, id)
}

func (m *MySQL) FindBuildJobByID(ctx context.Context, id int64) (models.BuildJob, error) {
	return m.findBuildJobByID(ctx, id)
}

func (m *MySQL) ClaimNextBuildJob(ctx context.Context, runner string, at time.Time) (models.BuildJob, error) {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return models.BuildJob{}, err
	}
	defer tx.Rollback()
	var id int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM build_jobs WHERE status = 'queued' ORDER BY id LIMIT 1 FOR UPDATE`).Scan(&id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.BuildJob{}, ErrNotFound
		}
		return models.BuildJob{}, err
	}
	when := at.UTC()
	if _, err := tx.ExecContext(ctx, `UPDATE build_jobs SET status = 'running', runner = ?, started_at = ? WHERE id = ?`, runner, when, id); err != nil {
		return models.BuildJob{}, err
	}
	if err := tx.Commit(); err != nil {
		return models.BuildJob{}, err
	}
	return m.findBuildJobByID(ctx, id)
}

func (m *MySQL) CompleteBuildJob(ctx context.Context, id int64, logPath string, at time.Time) (models.BuildJob, error) {
	return m.finishBuildJob(ctx, id, models.BuildJobSucceeded, logPath, "", at)
}

func (m *MySQL) FailBuildJob(ctx context.Context, id int64, logPath string, message string, at time.Time) (models.BuildJob, error) {
	return m.finishBuildJob(ctx, id, models.BuildJobFailed, logPath, message, at)
}

func (m *MySQL) CancelBuildJob(ctx context.Context, id int64, message string, at time.Time) (models.BuildJob, error) {
	return m.finishBuildJob(ctx, id, models.BuildJobCanceled, "", message, at)
}

func (m *MySQL) finishBuildJob(ctx context.Context, id int64, status models.BuildJobStatus, logPath string, message string, at time.Time) (models.BuildJob, error) {
	result, err := m.db.ExecContext(ctx, `UPDATE build_jobs SET status = ?, log_path = ?, error_message = ?, finished_at = ? WHERE id = ?`, status, nullString(logPath), nullString(message), at.UTC(), id)
	if err != nil {
		return models.BuildJob{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return models.BuildJob{}, err
	}
	if affected == 0 {
		return models.BuildJob{}, ErrNotFound
	}
	return m.findBuildJobByID(ctx, id)
}

func (m *MySQL) CreateBuildArtifact(ctx context.Context, artifact models.BuildArtifact) (models.BuildArtifact, error) {
	if artifact.BuildJobID <= 0 || artifact.Platform == "" || artifact.Filename == "" {
		return models.BuildArtifact{}, errors.New("build_job_id, platform, and filename are required")
	}
	result, err := m.db.ExecContext(ctx, `
		INSERT INTO build_artifacts (build_job_id, platform, filename, local_path, sha256, size_bytes)
		VALUES (?, ?, ?, ?, ?, ?)
	`, artifact.BuildJobID, artifact.Platform, artifact.Filename, artifact.LocalPath, artifact.SHA256, artifact.SizeBytes)
	if err != nil {
		return models.BuildArtifact{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return models.BuildArtifact{}, err
	}
	return m.findBuildArtifactByID(ctx, id)
}

func (m *MySQL) ListBuildArtifacts(ctx context.Context, buildJobID int64) ([]models.BuildArtifact, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, build_job_id, platform, filename, local_path, sha256, size_bytes, created_at
		FROM build_artifacts
		WHERE build_job_id = ?
		ORDER BY id
	`, buildJobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	artifacts := []models.BuildArtifact{}
	for rows.Next() {
		artifact, err := scanBuildArtifact(rows)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, artifact)
	}
	return artifacts, rows.Err()
}

func (m *MySQL) FindBuildArtifactByID(ctx context.Context, id int64) (models.BuildArtifact, error) {
	return m.findBuildArtifactByID(ctx, id)
}

func (m *MySQL) findDeviceByID(ctx context.Context, id int64) (models.Device, error) {
	return scanDevice(m.db.QueryRowContext(ctx, `
		SELECT id, rustdesk_id, name, alias, owner_user_id, status, platform, client_version, opendesk_client_version, last_ip, last_seen_at, registered_at, created_at, updated_at
		FROM devices
		WHERE id = ?
	`, id))
}

func (m *MySQL) findBuildJobByID(ctx context.Context, id int64) (models.BuildJob, error) {
	return scanBuildJob(m.db.QueryRowContext(ctx, `
		SELECT id, profile_id, platform, status, runner, log_path, error_message, created_at, started_at, finished_at
		FROM build_jobs
		WHERE id = ?
	`, id))
}

func (m *MySQL) findBuildArtifactByID(ctx context.Context, id int64) (models.BuildArtifact, error) {
	return scanBuildArtifact(m.db.QueryRowContext(ctx, `
		SELECT id, build_job_id, platform, filename, local_path, sha256, size_bytes, created_at
		FROM build_artifacts
		WHERE id = ?
	`, id))
}

func (m *MySQL) findAPITokenByID(ctx context.Context, id int64) (models.APIToken, error) {
	return scanAPIToken(m.db.QueryRowContext(ctx, `
		SELECT id, name, token_hash, CAST(scopes AS CHAR), user_id, expires_at, last_used_at, created_at, revoked_at
		FROM api_tokens
		WHERE id = ?
	`, id))
}

func (m *MySQL) findSessionByID(ctx context.Context, id int64) (models.Session, error) {
	return scanSession(m.db.QueryRowContext(ctx, `
		SELECT id, user_id, token_hash, ip, user_agent, expires_at, created_at, revoked_at
		FROM sessions
		WHERE id = ?
	`, id))
}

func (m *MySQL) findLoginLogByID(ctx context.Context, id int64) (models.LoginLog, error) {
	return scanLoginLog(m.db.QueryRowContext(ctx, `
		SELECT id, user_id, email, username, display_name, status, failure_reason, ip, user_agent, created_at
		FROM login_logs
		WHERE id = ?
	`, id))
}

func (m *MySQL) findAuditEventByID(ctx context.Context, id int64) (models.AuditEvent, error) {
	return scanAuditEvent(m.db.QueryRowContext(ctx, `
		SELECT id, actor_user_id, actor_type, action, resource_type, resource_id, ip, user_agent, CAST(metadata_json AS CHAR), created_at
		FROM audit_events
		WHERE id = ?
	`, id))
}

func (m *MySQL) findConnectionLogByID(ctx context.Context, id int64) (models.ConnectionLog, error) {
	return scanConnectionLog(m.db.QueryRowContext(ctx, `
		SELECT id, session_id, controller_user_id, controller_device_id, target_device_id, connection_type, relay_id, started_at, ended_at, status, deny_reason, CAST(metadata_json AS CHAR)
		FROM connection_logs
		WHERE id = ?
	`, id))
}

func requireRowExists(ctx context.Context, q queryRower, table string, id int64) error {
	var query string
	switch table {
	case "access_rules":
		query = "SELECT id FROM access_rules WHERE id = ?"
	case "control_roles":
		query = "SELECT id FROM control_roles WHERE id = ?"
	case "strategies":
		query = "SELECT id FROM strategies WHERE id = ?"
	case "build_profiles":
		query = "SELECT id FROM build_profiles WHERE id = ?"
	default:
		return errors.New("unknown table for existence check")
	}
	var existingID int64
	if err := q.QueryRowContext(ctx, query, id).Scan(&existingID); errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	return nil
}

func (m *MySQL) findAccessRuleByID(ctx context.Context, id int64) (models.AccessRule, error) {
	return scanAccessRule(m.db.QueryRowContext(ctx, `
		SELECT id, subject_type, subject_id, target_type, target_id, effect, priority, enabled, created_at, updated_at
		FROM access_rules
		WHERE id = ?
	`, id))
}

func (m *MySQL) findControlRoleByID(ctx context.Context, id int64) (models.ControlRole, error) {
	role, err := scanControlRole(m.db.QueryRowContext(ctx, `
		SELECT id, name, description, enabled, created_at, updated_at
		FROM control_roles
		WHERE id = ?
	`, id))
	if err != nil {
		return models.ControlRole{}, err
	}
	role.Permissions, err = m.listControlRolePermissions(ctx, id)
	if err != nil {
		return models.ControlRole{}, err
	}
	return role, nil
}

func (m *MySQL) listControlRolePermissions(ctx context.Context, roleID int64) ([]models.ControlRolePermission, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, role_id, permission_key, mode
		FROM control_role_permissions
		WHERE role_id = ?
		ORDER BY id
	`, roleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	permissions := []models.ControlRolePermission{}
	for rows.Next() {
		permission, err := scanControlRolePermission(rows)
		if err != nil {
			return nil, err
		}
		permissions = append(permissions, permission)
	}
	return permissions, rows.Err()
}

func (m *MySQL) findStrategyByID(ctx context.Context, id int64) (models.Strategy, error) {
	strategy, err := scanStrategy(m.db.QueryRowContext(ctx, `
		SELECT id, name, description, enabled, settings_json, created_at, updated_at
		FROM strategies
		WHERE id = ?
	`, id))
	if err != nil {
		return models.Strategy{}, err
	}
	strategy.Assignments, err = m.listStrategyAssignments(ctx, id)
	if err != nil {
		return models.Strategy{}, err
	}
	return strategy, nil
}

func (m *MySQL) listStrategyAssignments(ctx context.Context, strategyID int64) ([]models.StrategyAssignment, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, strategy_id, target_type, target_id, created_at
		FROM strategy_assignments
		WHERE strategy_id = ?
		ORDER BY id
	`, strategyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	assignments := []models.StrategyAssignment{}
	for rows.Next() {
		assignment, err := scanStrategyAssignment(rows)
		if err != nil {
			return nil, err
		}
		assignments = append(assignments, assignment)
	}
	return assignments, rows.Err()
}

func (m *MySQL) findRelayByID(ctx context.Context, id int64) (models.Relay, error) {
	return scanRelay(m.db.QueryRowContext(ctx, `
		SELECT id, name, region, host, port, ws_port, public_key_fingerprint, status, max_bandwidth_mbps, current_sessions, last_health_at, created_at, updated_at
		FROM relays
		WHERE id = ?
	`, id))
}

func (m *MySQL) findRelayGrantByID(ctx context.Context, id int64) (models.RelayGrant, error) {
	return scanRelayGrant(m.db.QueryRowContext(ctx, `
		SELECT id, grant_id, user_id, controller_device_id, target_device_id, target_rustdesk_id, relay_id, CAST(allowed_relays AS CHAR), expires_at, nonce, status, created_at, used_at
		FROM relay_grants
		WHERE id = ?
	`, id))
}

func (m *MySQL) findUserGroupByID(ctx context.Context, id int64) (models.UserGroup, error) {
	group, err := scanUserGroup(m.db.QueryRowContext(ctx, `
		SELECT id, name, description, created_at, updated_at
		FROM user_groups
		WHERE id = ?
	`, id))
	if err != nil {
		return models.UserGroup{}, err
	}
	group.MemberUserIDs, err = m.listUserGroupMembers(ctx, id)
	if err != nil {
		return models.UserGroup{}, err
	}
	return group, nil
}

func (m *MySQL) listUserGroupMembers(ctx context.Context, groupID int64) ([]int64, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT user_id
		FROM user_group_members
		WHERE group_id = ?
		ORDER BY user_id
	`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (m *MySQL) findDeviceGroupByID(ctx context.Context, id int64) (models.DeviceGroup, error) {
	group, err := scanDeviceGroup(m.db.QueryRowContext(ctx, `
		SELECT id, name, description, created_at, updated_at
		FROM device_groups
		WHERE id = ?
	`, id))
	if err != nil {
		return models.DeviceGroup{}, err
	}
	group.MemberDeviceIDs, err = m.listDeviceGroupMembers(ctx, id)
	if err != nil {
		return models.DeviceGroup{}, err
	}
	return group, nil
}

func (m *MySQL) listDeviceGroupMembers(ctx context.Context, groupID int64) ([]int64, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT device_id
		FROM device_group_members
		WHERE group_id = ?
		ORDER BY device_id
	`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (m *MySQL) findAddressBookByID(ctx context.Context, id int64) (models.AddressBook, error) {
	book, err := scanAddressBook(m.db.QueryRowContext(ctx, `
		SELECT id, name, description, owner_user_id, created_at, updated_at
		FROM address_books
		WHERE id = ?
	`, id))
	if err != nil {
		return models.AddressBook{}, err
	}
	book.Entries, err = m.listAddressBookEntries(ctx, id)
	if err != nil {
		return models.AddressBook{}, err
	}
	return book, nil
}

func (m *MySQL) listAddressBookEntries(ctx context.Context, bookID int64) ([]models.AddressBookEntry, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, address_book_id, device_id, alias, created_at
		FROM address_book_entries
		WHERE address_book_id = ?
		ORDER BY id
	`, bookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := []models.AddressBookEntry{}
	for rows.Next() {
		entry, err := scanAddressBookEntry(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func scanUser(row rowScanner) (models.User, error) {
	var user models.User
	var status string
	var lastLogin sql.NullTime
	err := row.Scan(&user.ID, &user.Email, &user.Username, &user.DisplayName, &user.PasswordHash, &status, &user.MFAEnabled, &user.Source, &user.CreatedAt, &user.UpdatedAt, &lastLogin)
	if errors.Is(err, sql.ErrNoRows) {
		return models.User{}, ErrNotFound
	}
	if err != nil {
		return models.User{}, err
	}
	user.Status = models.UserStatus(status)
	if lastLogin.Valid {
		user.LastLoginAt = &lastLogin.Time
	}
	return user, nil
}

func scanAPIToken(row rowScanner) (models.APIToken, error) {
	var token models.APIToken
	var userID sql.NullInt64
	var expiresAt, lastUsedAt, revokedAt sql.NullTime
	err := row.Scan(&token.ID, &token.Name, &token.TokenHash, &token.ScopesJSON, &userID, &expiresAt, &lastUsedAt, &token.CreatedAt, &revokedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return models.APIToken{}, ErrNotFound
	}
	if err != nil {
		return models.APIToken{}, err
	}
	token.UserID = int64FromNull(userID)
	token.ExpiresAt = timeFromNull(expiresAt)
	token.LastUsedAt = timeFromNull(lastUsedAt)
	token.RevokedAt = timeFromNull(revokedAt)
	return token, nil
}

func scanSession(row rowScanner) (models.Session, error) {
	var session models.Session
	var ip, userAgent sql.NullString
	var revokedAt sql.NullTime
	err := row.Scan(&session.ID, &session.UserID, &session.TokenHash, &ip, &userAgent, &session.ExpiresAt, &session.CreatedAt, &revokedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Session{}, ErrNotFound
	}
	if err != nil {
		return models.Session{}, err
	}
	session.IP = ip.String
	session.UserAgent = userAgent.String
	session.RevokedAt = timeFromNull(revokedAt)
	return session, nil
}

func scanUserGroup(row rowScanner) (models.UserGroup, error) {
	var group models.UserGroup
	var description sql.NullString
	err := row.Scan(&group.ID, &group.Name, &description, &group.CreatedAt, &group.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return models.UserGroup{}, ErrNotFound
	}
	if err != nil {
		return models.UserGroup{}, err
	}
	group.Description = description.String
	group.MemberUserIDs = []int64{}
	return group, nil
}

func scanDevice(row rowScanner) (models.Device, error) {
	var device models.Device
	var alias, platform, clientVersion, openDeskVersion, lastIP sql.NullString
	var ownerUserID sql.NullInt64
	var status string
	var lastSeenAt, registeredAt sql.NullTime
	err := row.Scan(&device.ID, &device.RustDeskID, &device.Name, &alias, &ownerUserID, &status, &platform, &clientVersion, &openDeskVersion, &lastIP, &lastSeenAt, &registeredAt, &device.CreatedAt, &device.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Device{}, ErrNotFound
	}
	if err != nil {
		return models.Device{}, err
	}
	device.Alias = alias.String
	if ownerUserID.Valid {
		device.OwnerUserID = &ownerUserID.Int64
	}
	device.Status = models.DeviceStatus(status)
	device.Platform = platform.String
	device.ClientVersion = clientVersion.String
	device.OpenDeskClientVersion = openDeskVersion.String
	device.LastIP = lastIP.String
	if lastSeenAt.Valid {
		device.LastSeenAt = &lastSeenAt.Time
	}
	if registeredAt.Valid {
		device.RegisteredAt = &registeredAt.Time
	}
	return device, nil
}

func scanDeviceGroup(row rowScanner) (models.DeviceGroup, error) {
	var group models.DeviceGroup
	var description sql.NullString
	err := row.Scan(&group.ID, &group.Name, &description, &group.CreatedAt, &group.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return models.DeviceGroup{}, ErrNotFound
	}
	if err != nil {
		return models.DeviceGroup{}, err
	}
	group.Description = description.String
	group.MemberDeviceIDs = []int64{}
	return group, nil
}

func scanAddressBook(row rowScanner) (models.AddressBook, error) {
	var book models.AddressBook
	var description sql.NullString
	var ownerUserID sql.NullInt64
	err := row.Scan(&book.ID, &book.Name, &description, &ownerUserID, &book.CreatedAt, &book.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return models.AddressBook{}, ErrNotFound
	}
	if err != nil {
		return models.AddressBook{}, err
	}
	book.Description = description.String
	book.OwnerUserID = int64FromNull(ownerUserID)
	book.Entries = []models.AddressBookEntry{}
	return book, nil
}

func scanAddressBookEntry(row rowScanner) (models.AddressBookEntry, error) {
	var entry models.AddressBookEntry
	var alias sql.NullString
	err := row.Scan(&entry.ID, &entry.AddressBookID, &entry.DeviceID, &alias, &entry.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return models.AddressBookEntry{}, ErrNotFound
	}
	if err != nil {
		return models.AddressBookEntry{}, err
	}
	entry.Alias = alias.String
	return entry, nil
}

func scanRelay(row rowScanner) (models.Relay, error) {
	var relay models.Relay
	var fingerprint sql.NullString
	var maxBandwidth sql.NullInt64
	var lastHealth sql.NullTime
	err := row.Scan(&relay.ID, &relay.Name, &relay.Region, &relay.Host, &relay.Port, &relay.WSPort, &fingerprint, &relay.Status, &maxBandwidth, &relay.CurrentSessions, &lastHealth, &relay.CreatedAt, &relay.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Relay{}, ErrNotFound
	}
	if err != nil {
		return models.Relay{}, err
	}
	relay.PublicKeyFingerprint = fingerprint.String
	if maxBandwidth.Valid {
		value := int(maxBandwidth.Int64)
		relay.MaxBandwidthMbps = &value
	}
	if lastHealth.Valid {
		relay.LastHealthAt = &lastHealth.Time
	}
	return relay, nil
}

func scanRelayGrant(row rowScanner) (models.RelayGrant, error) {
	var grant models.RelayGrant
	var userID, controllerDeviceID, targetDeviceID, relayID sql.NullInt64
	var targetRustDeskID, allowedRelaysJSON sql.NullString
	var usedAt sql.NullTime
	err := row.Scan(&grant.ID, &grant.GrantID, &userID, &controllerDeviceID, &targetDeviceID, &targetRustDeskID, &relayID, &allowedRelaysJSON, &grant.ExpiresAt, &grant.Nonce, &grant.Status, &grant.CreatedAt, &usedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return models.RelayGrant{}, ErrNotFound
	}
	if err != nil {
		return models.RelayGrant{}, err
	}
	grant.UserID = int64FromNull(userID)
	grant.ControllerDeviceID = int64FromNull(controllerDeviceID)
	grant.TargetDeviceID = int64FromNull(targetDeviceID)
	grant.RelayID = int64FromNull(relayID)
	grant.TargetRustDeskID = targetRustDeskID.String
	grant.UsedAt = timeFromNull(usedAt)
	if allowedRelaysJSON.Valid && strings.TrimSpace(allowedRelaysJSON.String) != "" {
		if err := json.Unmarshal([]byte(allowedRelaysJSON.String), &grant.AllowedRelays); err != nil {
			return models.RelayGrant{}, err
		}
	}
	if grant.AllowedRelays == nil {
		grant.AllowedRelays = []string{}
	}
	return grant, nil
}

func scanAccessRule(row rowScanner) (models.AccessRule, error) {
	var rule models.AccessRule
	err := row.Scan(&rule.ID, &rule.SubjectType, &rule.SubjectID, &rule.TargetType, &rule.TargetID, &rule.Effect, &rule.Priority, &rule.Enabled, &rule.CreatedAt, &rule.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return models.AccessRule{}, ErrNotFound
	}
	if err != nil {
		return models.AccessRule{}, err
	}
	return rule, nil
}

func scanControlRole(row rowScanner) (models.ControlRole, error) {
	var role models.ControlRole
	var description sql.NullString
	err := row.Scan(&role.ID, &role.Name, &description, &role.Enabled, &role.CreatedAt, &role.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return models.ControlRole{}, ErrNotFound
	}
	if err != nil {
		return models.ControlRole{}, err
	}
	role.Description = description.String
	return role, nil
}

func scanControlRolePermission(row rowScanner) (models.ControlRolePermission, error) {
	var permission models.ControlRolePermission
	err := row.Scan(&permission.ID, &permission.RoleID, &permission.PermissionKey, &permission.Mode)
	if errors.Is(err, sql.ErrNoRows) {
		return models.ControlRolePermission{}, ErrNotFound
	}
	if err != nil {
		return models.ControlRolePermission{}, err
	}
	return permission, nil
}

func scanStrategy(row rowScanner) (models.Strategy, error) {
	var strategy models.Strategy
	var description sql.NullString
	err := row.Scan(&strategy.ID, &strategy.Name, &description, &strategy.Enabled, &strategy.SettingsJSON, &strategy.CreatedAt, &strategy.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Strategy{}, ErrNotFound
	}
	if err != nil {
		return models.Strategy{}, err
	}
	strategy.Description = description.String
	return strategy, nil
}

func scanStrategyAssignment(row rowScanner) (models.StrategyAssignment, error) {
	var assignment models.StrategyAssignment
	err := row.Scan(&assignment.ID, &assignment.StrategyID, &assignment.TargetType, &assignment.TargetID, &assignment.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return models.StrategyAssignment{}, ErrNotFound
	}
	if err != nil {
		return models.StrategyAssignment{}, err
	}
	return assignment, nil
}

func scanAuditEvent(row rowScanner) (models.AuditEvent, error) {
	var event models.AuditEvent
	var actorUserID sql.NullInt64
	var resourceID, ip, userAgent, metadata sql.NullString
	err := row.Scan(&event.ID, &actorUserID, &event.ActorType, &event.Action, &event.ResourceType, &resourceID, &ip, &userAgent, &metadata, &event.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return models.AuditEvent{}, ErrNotFound
	}
	if err != nil {
		return models.AuditEvent{}, err
	}
	event.ActorUserID = int64FromNull(actorUserID)
	if resourceID.Valid {
		event.ResourceID = &resourceID.String
	}
	event.IP = ip.String
	event.UserAgent = userAgent.String
	parsed, err := parseJSONObject(metadata)
	if err != nil {
		return models.AuditEvent{}, err
	}
	event.Metadata = parsed
	return event, nil
}

func scanConnectionLog(row rowScanner) (models.ConnectionLog, error) {
	var log models.ConnectionLog
	var sessionID, controllerUserID, controllerDeviceID, targetDeviceID, relayID sql.NullInt64
	var endedAt sql.NullTime
	var denyReason, metadata sql.NullString
	err := row.Scan(&log.ID, &sessionID, &controllerUserID, &controllerDeviceID, &targetDeviceID, &log.ConnectionType, &relayID, &log.StartedAt, &endedAt, &log.Status, &denyReason, &metadata)
	if errors.Is(err, sql.ErrNoRows) {
		return models.ConnectionLog{}, ErrNotFound
	}
	if err != nil {
		return models.ConnectionLog{}, err
	}
	log.SessionID = int64FromNull(sessionID)
	log.ControllerUserID = int64FromNull(controllerUserID)
	log.ControllerDeviceID = int64FromNull(controllerDeviceID)
	log.TargetDeviceID = int64FromNull(targetDeviceID)
	log.RelayID = int64FromNull(relayID)
	if endedAt.Valid {
		log.EndedAt = &endedAt.Time
	}
	log.DenyReason = denyReason.String
	parsed, err := parseJSONObject(metadata)
	if err != nil {
		return models.ConnectionLog{}, err
	}
	log.Metadata = parsed
	return log, nil
}

func scanFileTransferLog(row rowScanner) (models.FileTransferLog, error) {
	var log models.FileTransferLog
	var connectionLogID sql.NullInt64
	err := row.Scan(&log.ID, &connectionLogID, &log.Direction, &log.FilenameHash, &log.SizeBytes, &log.Status, &log.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return models.FileTransferLog{}, ErrNotFound
	}
	if err != nil {
		return models.FileTransferLog{}, err
	}
	log.ConnectionLogID = int64FromNull(connectionLogID)
	return log, nil
}

func scanLoginLog(row rowScanner) (models.LoginLog, error) {
	var log models.LoginLog
	var userID sql.NullInt64
	var username, displayName, failureReason, ip, userAgent sql.NullString
	err := row.Scan(&log.ID, &userID, &log.Email, &username, &displayName, &log.Status, &failureReason, &ip, &userAgent, &log.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return models.LoginLog{}, ErrNotFound
	}
	if err != nil {
		return models.LoginLog{}, err
	}
	if userID.Valid {
		log.UserID = userID.Int64
	}
	log.Username = username.String
	log.DisplayName = displayName.String
	log.FailureReason = failureReason.String
	log.IP = ip.String
	log.UserAgent = userAgent.String
	if log.Status == "succeeded" {
		when := log.CreatedAt
		log.LastLoginAt = &when
	}
	return log, nil
}

func scanSystemSetting(row rowScanner) (models.SystemSetting, error) {
	var setting models.SystemSetting
	var updatedBy sql.NullInt64
	err := row.Scan(&setting.Key, &setting.ValueJSON, &updatedBy, &setting.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return models.SystemSetting{}, ErrNotFound
	}
	if err != nil {
		return models.SystemSetting{}, err
	}
	setting.UpdatedBy = int64FromNull(updatedBy)
	return setting, nil
}

func scanBuildProfile(row rowScanner) (models.BuildProfile, error) {
	var profile models.BuildProfile
	var vendor, productName, description sql.NullString
	var sourceJSON sql.NullString
	err := row.Scan(&profile.ID, &profile.Name, &profile.AppName, &vendor, &profile.BundleID, &productName, &description, &profile.ServerConfigJSON, &profile.BrandingJSON, &profile.PolicyJSON, &profile.PlatformsJSON, &profile.SigningJSON, &sourceJSON, &profile.CreatedBy, &profile.CreatedAt, &profile.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return models.BuildProfile{}, ErrNotFound
	}
	if err != nil {
		return models.BuildProfile{}, err
	}
	profile.Vendor = vendor.String
	profile.ProductName = productName.String
	profile.Description = description.String
	profile.SourceJSON = sourceJSON.String
	return profile, nil
}

func scanBuildJob(row rowScanner) (models.BuildJob, error) {
	var job models.BuildJob
	var status string
	var runner, logPath, errorMessage sql.NullString
	var startedAt, finishedAt sql.NullTime
	err := row.Scan(&job.ID, &job.ProfileID, &job.Platform, &status, &runner, &logPath, &errorMessage, &job.CreatedAt, &startedAt, &finishedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return models.BuildJob{}, ErrNotFound
	}
	if err != nil {
		return models.BuildJob{}, err
	}
	job.Status = models.BuildJobStatus(status)
	job.Runner = runner.String
	job.LogPath = logPath.String
	job.ErrorMessage = errorMessage.String
	if startedAt.Valid {
		job.StartedAt = &startedAt.Time
	}
	if finishedAt.Valid {
		job.FinishedAt = &finishedAt.Time
	}
	return job, nil
}

func scanBuildArtifact(row rowScanner) (models.BuildArtifact, error) {
	var artifact models.BuildArtifact
	err := row.Scan(&artifact.ID, &artifact.BuildJobID, &artifact.Platform, &artifact.Filename, &artifact.LocalPath, &artifact.SHA256, &artifact.SizeBytes, &artifact.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return models.BuildArtifact{}, ErrNotFound
	}
	if err != nil {
		return models.BuildArtifact{}, err
	}
	return artifact, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func usernameFromEmail(email string) string {
	username, _, _ := strings.Cut(email, "@")
	if strings.TrimSpace(username) == "" {
		return "admin"
	}
	return strings.TrimSpace(username)
}

func nullString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func nullStringPointer(value *string) any {
	if value == nil {
		return nil
	}
	return nullString(*value)
}

func nullInt64(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullIntPointer(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC()
}

func nullablePositiveInt64(value int64) any {
	if value <= 0 {
		return nil
	}
	return value
}

func int64FromNull(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	out := value.Int64
	return &out
}

func timeFromNull(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	out := value.Time
	return &out
}

func parseJSONObject(value sql.NullString) (map[string]any, error) {
	if !value.Valid || strings.TrimSpace(value.String) == "" {
		return map[string]any{}, nil
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(value.String), &out); err != nil {
		return nil, err
	}
	if out == nil {
		return map[string]any{}, nil
	}
	return out, nil
}
