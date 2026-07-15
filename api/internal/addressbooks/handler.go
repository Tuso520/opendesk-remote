package addressbooks

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/opendesk-remote/opendesk-remote/api/internal/httpx"
	"github.com/opendesk-remote/opendesk-remote/api/internal/models"
	"github.com/opendesk-remote/opendesk-remote/api/internal/repository"
)

type Handler struct {
	repo repository.AddressBookRepository
}

type CreateAddressBookRequest struct {
	Name        string                    `json:"name"`
	Description string                    `json:"description"`
	OwnerUserID *int64                    `json:"owner_user_id"`
	Entries     []models.AddressBookEntry `json:"entries"`
}

type AddAddressBookEntryRequest struct {
	DeviceID int64  `json:"device_id"`
	Alias    string `json:"alias"`
}

func NewHandler(repo repository.AddressBookRepository) Handler {
	return Handler{repo: repo}
}

func (h Handler) Collection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		books, err := h.repo.ListAddressBooks(r.Context())
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, "LIST_ADDRESS_BOOKS_FAILED", "list address books failed")
			return
		}
		httpx.JSON(w, http.StatusOK, books)
	case http.MethodPost:
		var req CreateAddressBookRequest
		if err := httpx.DecodeJSON(r, &req); err != nil {
			httpx.Error(w, http.StatusBadRequest, "INVALID_JSON", "invalid JSON body")
			return
		}
		if strings.TrimSpace(req.Name) == "" {
			httpx.Error(w, http.StatusBadRequest, "INVALID_ADDRESS_BOOK", "name is required")
			return
		}
		for _, entry := range req.Entries {
			if entry.DeviceID <= 0 {
				httpx.Error(w, http.StatusBadRequest, "INVALID_ADDRESS_BOOK", "entry device_id must be positive")
				return
			}
		}
		created, err := h.repo.CreateAddressBook(r.Context(), models.AddressBook{
			Name:        strings.TrimSpace(req.Name),
			Description: strings.TrimSpace(req.Description),
			OwnerUserID: req.OwnerUserID,
			Entries:     normalizedEntries(req.Entries),
		})
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "INVALID_ADDRESS_BOOK", err.Error())
			return
		}
		httpx.JSON(w, http.StatusCreated, created)
	default:
		httpx.Error(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	}
}

func (h Handler) Item(w http.ResponseWriter, r *http.Request) {
	bookID, entryID, hasEntryID, ok := parseAddressBookEntryPath(r.URL.Path)
	if !ok {
		httpx.Error(w, http.StatusNotFound, "NOT_FOUND", "address book route not found")
		return
	}

	switch {
	case !hasEntryID && r.Method == http.MethodGet:
		entries, err := h.repo.ListAddressBookEntries(r.Context(), bookID)
		if errors.Is(err, repository.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "ADDRESS_BOOK_NOT_FOUND", "address book not found")
			return
		}
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, "LIST_ADDRESS_BOOK_ENTRIES_FAILED", "list address book entries failed")
			return
		}
		httpx.JSON(w, http.StatusOK, entries)
	case !hasEntryID && r.Method == http.MethodPost:
		var req AddAddressBookEntryRequest
		if err := httpx.DecodeJSON(r, &req); err != nil {
			httpx.Error(w, http.StatusBadRequest, "INVALID_JSON", "invalid JSON body")
			return
		}
		if req.DeviceID <= 0 {
			httpx.Error(w, http.StatusBadRequest, "INVALID_ADDRESS_BOOK_ENTRY", "device_id must be positive")
			return
		}
		book, err := h.repo.AddAddressBookEntry(r.Context(), bookID, models.AddressBookEntry{
			DeviceID: req.DeviceID,
			Alias:    strings.TrimSpace(req.Alias),
		})
		if errors.Is(err, repository.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "ADDRESS_BOOK_NOT_FOUND", "address book not found")
			return
		}
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "INVALID_ADDRESS_BOOK_ENTRY", err.Error())
			return
		}
		httpx.JSON(w, http.StatusCreated, book)
	case hasEntryID && r.Method == http.MethodDelete:
		book, err := h.repo.RemoveAddressBookEntry(r.Context(), bookID, entryID)
		if errors.Is(err, repository.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "ADDRESS_BOOK_ENTRY_NOT_FOUND", "address book entry not found")
			return
		}
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "INVALID_ADDRESS_BOOK_ENTRY", err.Error())
			return
		}
		httpx.JSON(w, http.StatusOK, book)
	default:
		httpx.Error(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	}
}

func normalizedEntries(in []models.AddressBookEntry) []models.AddressBookEntry {
	out := make([]models.AddressBookEntry, 0, len(in))
	for _, entry := range in {
		out = append(out, models.AddressBookEntry{
			DeviceID: entry.DeviceID,
			Alias:    strings.TrimSpace(entry.Alias),
		})
	}
	return out
}

func parseAddressBookEntryPath(path string) (bookID int64, entryID int64, hasEntryID bool, ok bool) {
	rest := strings.TrimPrefix(path, "/api/v1/address-books/")
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) != 2 && len(parts) != 3 {
		return 0, 0, false, false
	}
	if parts[1] != "entries" {
		return 0, 0, false, false
	}
	bookID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || bookID <= 0 {
		return 0, 0, false, false
	}
	if len(parts) == 2 {
		return bookID, 0, false, true
	}
	entryID, err = strconv.ParseInt(parts[2], 10, 64)
	if err != nil || entryID <= 0 {
		return 0, 0, false, false
	}
	return bookID, entryID, true, true
}
