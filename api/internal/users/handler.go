package users

import (
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/opendesk-remote/opendesk-remote/api/internal/audit"
	apiauth "github.com/opendesk-remote/opendesk-remote/api/internal/auth"
	"github.com/opendesk-remote/opendesk-remote/api/internal/httpx"
	"github.com/opendesk-remote/opendesk-remote/api/internal/models"
	"github.com/opendesk-remote/opendesk-remote/api/internal/repository"
)

type Handler struct {
	repo Repository
}

type Repository interface {
	repository.UserRepository
	audit.Repository
}

func NewHandler(repo Repository) Handler {
	return Handler{repo: repo}
}

type CreateUserGroupRequest struct {
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	MemberUserIDs []int64 `json:"member_user_ids"`
}

type AddUserGroupMemberRequest struct {
	UserID int64 `json:"user_id"`
}

func (h Handler) Item(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUserPath(r.URL.Path)
	if !ok {
		httpx.Error(w, http.StatusNotFound, "NOT_FOUND", "user route not found")
		return
	}
	switch r.Method {
	case http.MethodGet:
		user, err := h.repo.FindUserByID(r.Context(), id)
		if errors.Is(err, repository.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "USER_NOT_FOUND", "user not found")
			return
		}
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, "LOAD_USER_FAILED", "load user failed")
			return
		}
		httpx.JSON(w, http.StatusOK, user)
	case http.MethodPut:
		h.updateUser(w, r, id)
	case http.MethodDelete:
		h.disableUser(w, r, id)
	default:
		httpx.Error(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	}
}

func (h Handler) Collection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		users, err := h.repo.ListUsers(r.Context())
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, "LIST_USERS_FAILED", "list users failed")
			return
		}
		httpx.JSON(w, http.StatusOK, users)
	case http.MethodPost:
		var user models.User
		if err := httpx.DecodeJSON(r, &user); err != nil {
			httpx.Error(w, http.StatusBadRequest, "INVALID_JSON", "invalid JSON body")
			return
		}
		created, err := h.repo.CreateUser(r.Context(), user)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "INVALID_USER", err.Error())
			return
		}
		if err := h.writeAudit(r, audit.Event{
			Action:       "create_user",
			ResourceType: "user",
			ResourceID:   strconv.FormatInt(created.ID, 10),
			Metadata: map[string]any{
				"email":        created.Email,
				"username":     created.Username,
				"display_name": created.DisplayName,
				"status":       created.Status,
				"source":       created.Source,
			},
		}); err != nil {
			httpx.Error(w, http.StatusInternalServerError, "AUDIT_WRITE_FAILED", "failed to record user creation")
			return
		}
		httpx.JSON(w, http.StatusCreated, created)
	default:
		httpx.Error(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	}
}

func (h Handler) updateUser(w http.ResponseWriter, r *http.Request, id int64) {
	var user models.User
	if err := httpx.DecodeJSON(r, &user); err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_JSON", "invalid JSON body")
		return
	}
	if err := validateUser(user); err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_USER", err.Error())
		return
	}
	updated, err := h.repo.UpdateUser(r.Context(), id, user)
	if errors.Is(err, repository.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "USER_NOT_FOUND", "user not found")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "INVALID_USER", err.Error())
		return
	}
	if err := h.writeAudit(r, audit.Event{
		Action:       "update_user",
		ResourceType: "user",
		ResourceID:   strconv.FormatInt(updated.ID, 10),
		Metadata: map[string]any{
			"email":        updated.Email,
			"username":     updated.Username,
			"display_name": updated.DisplayName,
			"status":       updated.Status,
			"source":       updated.Source,
			"mfa_enabled":  updated.MFAEnabled,
		},
	}); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "AUDIT_WRITE_FAILED", "failed to record user update")
		return
	}
	httpx.JSON(w, http.StatusOK, updated)
}

func (h Handler) disableUser(w http.ResponseWriter, r *http.Request, id int64) {
	disabled, err := h.repo.DisableUser(r.Context(), id, time.Now().UTC())
	if errors.Is(err, repository.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "USER_NOT_FOUND", "user not found")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "DISABLE_USER_FAILED", "disable user failed")
		return
	}
	if err := h.writeAudit(r, audit.Event{
		Action:       "disable_user",
		ResourceType: "user",
		ResourceID:   strconv.FormatInt(disabled.ID, 10),
		Metadata: map[string]any{
			"email":    disabled.Email,
			"username": disabled.Username,
			"status":   disabled.Status,
		},
	}); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "AUDIT_WRITE_FAILED", "failed to record user disable")
		return
	}
	httpx.JSON(w, http.StatusOK, disabled)
}

func (h Handler) GroupItem(w http.ResponseWriter, r *http.Request) {
	groupID, userID, hasUserID, ok := parseUserGroupMemberPath(r.URL.Path)
	if !ok {
		httpx.Error(w, http.StatusNotFound, "NOT_FOUND", "user group route not found")
		return
	}

	switch {
	case !hasUserID && r.Method == http.MethodGet:
		members, err := h.repo.ListUserGroupMembers(r.Context(), groupID)
		if errors.Is(err, repository.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "USER_GROUP_NOT_FOUND", "user group not found")
			return
		}
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, "LIST_USER_GROUP_MEMBERS_FAILED", "list user group members failed")
			return
		}
		httpx.JSON(w, http.StatusOK, members)
	case !hasUserID && r.Method == http.MethodPost:
		var req AddUserGroupMemberRequest
		if err := httpx.DecodeJSON(r, &req); err != nil {
			httpx.Error(w, http.StatusBadRequest, "INVALID_JSON", "invalid JSON body")
			return
		}
		if req.UserID <= 0 {
			httpx.Error(w, http.StatusBadRequest, "INVALID_USER_GROUP_MEMBER", "user_id must be positive")
			return
		}
		group, err := h.repo.AddUserGroupMember(r.Context(), groupID, req.UserID)
		if errors.Is(err, repository.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "USER_GROUP_NOT_FOUND", "user group not found")
			return
		}
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "INVALID_USER_GROUP_MEMBER", err.Error())
			return
		}
		if err := h.writeAudit(r, audit.Event{
			Action:       "add_user_group_member",
			ResourceType: "user_group",
			ResourceID:   strconv.FormatInt(group.ID, 10),
			Metadata: map[string]any{
				"group_name": group.Name,
				"user_id":    req.UserID,
			},
		}); err != nil {
			httpx.Error(w, http.StatusInternalServerError, "AUDIT_WRITE_FAILED", "failed to record user group membership change")
			return
		}
		httpx.JSON(w, http.StatusCreated, group)
	case hasUserID && r.Method == http.MethodDelete:
		group, err := h.repo.RemoveUserGroupMember(r.Context(), groupID, userID)
		if errors.Is(err, repository.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "USER_GROUP_NOT_FOUND", "user group not found")
			return
		}
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "INVALID_USER_GROUP_MEMBER", err.Error())
			return
		}
		if err := h.writeAudit(r, audit.Event{
			Action:       "remove_user_group_member",
			ResourceType: "user_group",
			ResourceID:   strconv.FormatInt(group.ID, 10),
			Metadata: map[string]any{
				"group_name": group.Name,
				"user_id":    userID,
			},
		}); err != nil {
			httpx.Error(w, http.StatusInternalServerError, "AUDIT_WRITE_FAILED", "failed to record user group membership change")
			return
		}
		httpx.JSON(w, http.StatusOK, group)
	default:
		httpx.Error(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	}
}

func parseUserGroupMemberPath(path string) (groupID int64, userID int64, hasUserID bool, ok bool) {
	rest := strings.TrimPrefix(path, "/api/v1/user-groups/")
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) != 2 && len(parts) != 3 {
		return 0, 0, false, false
	}
	if parts[1] != "members" {
		return 0, 0, false, false
	}
	groupID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || groupID <= 0 {
		return 0, 0, false, false
	}
	if len(parts) == 2 {
		return groupID, 0, false, true
	}
	userID, err = strconv.ParseInt(parts[2], 10, 64)
	if err != nil || userID <= 0 {
		return 0, 0, false, false
	}
	return groupID, userID, true, true
}

func parseUserPath(path string) (int64, bool) {
	rest := strings.Trim(strings.TrimPrefix(path, "/api/v1/users/"), "/")
	if rest == "" || strings.Contains(rest, "/") {
		return 0, false
	}
	id, err := strconv.ParseInt(rest, 10, 64)
	return id, err == nil && id > 0
}

func validateUser(user models.User) error {
	if strings.TrimSpace(user.Email) == "" || strings.TrimSpace(user.Username) == "" {
		return errors.New("email and username are required")
	}
	if user.Status != "" {
		switch user.Status {
		case models.UserStatusActive, models.UserStatusDisabled, models.UserStatusLocked:
		default:
			return errors.New("status must be active, disabled, or locked")
		}
	}
	if strings.TrimSpace(user.Source) != "" {
		switch strings.TrimSpace(user.Source) {
		case "local", "oidc", "ldap":
		default:
			return errors.New("source must be local, oidc, or ldap")
		}
	}
	return nil
}

func (h Handler) Groups(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		groups, err := h.repo.ListUserGroups(r.Context())
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, "LIST_USER_GROUPS_FAILED", "list user groups failed")
			return
		}
		httpx.JSON(w, http.StatusOK, groups)
	case http.MethodPost:
		var req CreateUserGroupRequest
		if err := httpx.DecodeJSON(r, &req); err != nil {
			httpx.Error(w, http.StatusBadRequest, "INVALID_JSON", "invalid JSON body")
			return
		}
		if strings.TrimSpace(req.Name) == "" {
			httpx.Error(w, http.StatusBadRequest, "INVALID_USER_GROUP", "name is required")
			return
		}
		created, err := h.repo.CreateUserGroup(r.Context(), models.UserGroup{
			Name:          strings.TrimSpace(req.Name),
			Description:   strings.TrimSpace(req.Description),
			MemberUserIDs: req.MemberUserIDs,
		})
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "INVALID_USER_GROUP", err.Error())
			return
		}
		if err := h.writeAudit(r, audit.Event{
			Action:       "create_user_group",
			ResourceType: "user_group",
			ResourceID:   strconv.FormatInt(created.ID, 10),
			Metadata: map[string]any{
				"name":         created.Name,
				"member_count": len(created.MemberUserIDs),
			},
		}); err != nil {
			httpx.Error(w, http.StatusInternalServerError, "AUDIT_WRITE_FAILED", "failed to record user group creation")
			return
		}
		httpx.JSON(w, http.StatusCreated, created)
	default:
		httpx.Error(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	}
}

func (h Handler) writeAudit(r *http.Request, event audit.Event) error {
	event.ActorType = "system"
	if session, ok := apiauth.SessionFromContext(r.Context()); ok {
		event.ActorType = apiauth.ActorType(session)
		userID := session.User.ID
		event.ActorUserID = &userID
	}
	event.IP = requestIP(r)
	event.UserAgent = r.UserAgent()
	return (audit.RepositoryWriter{Repo: h.repo}).Write(r.Context(), event)
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
