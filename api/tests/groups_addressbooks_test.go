package tests

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/opendesk-remote/opendesk-remote/api/internal/app"
)

func TestGroupsAndAddressBooksAPI(t *testing.T) {
	server := httptest.NewServer(app.NewRouter(testConfig(t), slog.Default()))
	defer server.Close()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	client := &http.Client{Jar: jar}
	login := postJSON(t, client, server.URL+"/api/v1/auth/login", `{"email":"admin@example.com","password":"admin-password-12345"}`)
	defer login.Body.Close()
	if login.StatusCode != http.StatusOK {
		t.Fatalf("expected login 200, got %d", login.StatusCode)
	}

	userGroup := postJSON(t, client, server.URL+"/api/v1/user-groups", `{"name":"Operators","description":"Remote operators","member_user_ids":[1]}`)
	defer userGroup.Body.Close()
	if userGroup.StatusCode != http.StatusCreated {
		t.Fatalf("expected user group 201, got %d", userGroup.StatusCode)
	}
	var userGroupEnvelope struct {
		Data struct {
			ID            int64   `json:"id"`
			Name          string  `json:"name"`
			MemberUserIDs []int64 `json:"member_user_ids"`
		} `json:"data"`
	}
	if err := json.NewDecoder(userGroup.Body).Decode(&userGroupEnvelope); err != nil {
		t.Fatalf("decode user group: %v", err)
	}
	if userGroupEnvelope.Data.ID == 0 || userGroupEnvelope.Data.Name != "Operators" || len(userGroupEnvelope.Data.MemberUserIDs) != 1 {
		t.Fatalf("unexpected user group: %+v", userGroupEnvelope.Data)
	}
	userGroupMembers, err := client.Get(server.URL + "/api/v1/user-groups/" + idString(userGroupEnvelope.Data.ID) + "/members")
	if err != nil {
		t.Fatalf("GET user group members failed: %v", err)
	}
	defer userGroupMembers.Body.Close()
	if userGroupMembers.StatusCode != http.StatusOK {
		t.Fatalf("expected user group members 200, got %d", userGroupMembers.StatusCode)
	}
	var userGroupMembersEnvelope struct {
		Data []int64 `json:"data"`
	}
	if err := json.NewDecoder(userGroupMembers.Body).Decode(&userGroupMembersEnvelope); err != nil {
		t.Fatalf("decode user group members: %v", err)
	}
	if len(userGroupMembersEnvelope.Data) != 1 || userGroupMembersEnvelope.Data[0] != 1 {
		t.Fatalf("unexpected user group members: %+v", userGroupMembersEnvelope.Data)
	}
	removeUserMember := deleteURL(t, client, server.URL+"/api/v1/user-groups/"+idString(userGroupEnvelope.Data.ID)+"/members/1")
	defer removeUserMember.Body.Close()
	if removeUserMember.StatusCode != http.StatusOK {
		t.Fatalf("expected remove user group member 200, got %d", removeUserMember.StatusCode)
	}
	addUserMember := postJSON(t, client, server.URL+"/api/v1/user-groups/"+idString(userGroupEnvelope.Data.ID)+"/members", `{"user_id":1}`)
	defer addUserMember.Body.Close()
	if addUserMember.StatusCode != http.StatusCreated {
		t.Fatalf("expected add user group member 201, got %d", addUserMember.StatusCode)
	}

	deviceGroup := postJSON(t, client, server.URL+"/api/v1/device-groups", `{"name":"Workstations","description":"Managed workstations","member_device_ids":[1]}`)
	defer deviceGroup.Body.Close()
	if deviceGroup.StatusCode != http.StatusCreated {
		t.Fatalf("expected device group 201, got %d", deviceGroup.StatusCode)
	}
	var deviceGroupEnvelope struct {
		Data struct {
			ID              int64   `json:"id"`
			Name            string  `json:"name"`
			MemberDeviceIDs []int64 `json:"member_device_ids"`
		} `json:"data"`
	}
	if err := json.NewDecoder(deviceGroup.Body).Decode(&deviceGroupEnvelope); err != nil {
		t.Fatalf("decode device group: %v", err)
	}
	if deviceGroupEnvelope.Data.ID == 0 || deviceGroupEnvelope.Data.Name != "Workstations" || len(deviceGroupEnvelope.Data.MemberDeviceIDs) != 1 {
		t.Fatalf("unexpected device group: %+v", deviceGroupEnvelope.Data)
	}
	deviceGroupMembers, err := client.Get(server.URL + "/api/v1/device-groups/" + idString(deviceGroupEnvelope.Data.ID) + "/members")
	if err != nil {
		t.Fatalf("GET device group members failed: %v", err)
	}
	defer deviceGroupMembers.Body.Close()
	if deviceGroupMembers.StatusCode != http.StatusOK {
		t.Fatalf("expected device group members 200, got %d", deviceGroupMembers.StatusCode)
	}
	var deviceGroupMembersEnvelope struct {
		Data []int64 `json:"data"`
	}
	if err := json.NewDecoder(deviceGroupMembers.Body).Decode(&deviceGroupMembersEnvelope); err != nil {
		t.Fatalf("decode device group members: %v", err)
	}
	if len(deviceGroupMembersEnvelope.Data) != 1 || deviceGroupMembersEnvelope.Data[0] != 1 {
		t.Fatalf("unexpected device group members: %+v", deviceGroupMembersEnvelope.Data)
	}
	removeDeviceMember := deleteURL(t, client, server.URL+"/api/v1/device-groups/"+idString(deviceGroupEnvelope.Data.ID)+"/members/1")
	defer removeDeviceMember.Body.Close()
	if removeDeviceMember.StatusCode != http.StatusOK {
		t.Fatalf("expected remove device group member 200, got %d", removeDeviceMember.StatusCode)
	}
	addDeviceMember := postJSON(t, client, server.URL+"/api/v1/device-groups/"+idString(deviceGroupEnvelope.Data.ID)+"/members", `{"device_id":1}`)
	defer addDeviceMember.Body.Close()
	if addDeviceMember.StatusCode != http.StatusCreated {
		t.Fatalf("expected add device group member 201, got %d", addDeviceMember.StatusCode)
	}

	addressBook := postJSON(t, client, server.URL+"/api/v1/address-books", `{"name":"Team Devices","description":"Shared managed devices","owner_user_id":1,"entries":[{"device_id":1,"alias":"Demo Workstation"}]}`)
	defer addressBook.Body.Close()
	if addressBook.StatusCode != http.StatusCreated {
		t.Fatalf("expected address book 201, got %d", addressBook.StatusCode)
	}
	var addressBookEnvelope struct {
		Data struct {
			ID      int64 `json:"id"`
			Entries []struct {
				ID       int64  `json:"id"`
				DeviceID int64  `json:"device_id"`
				Alias    string `json:"alias"`
			} `json:"entries"`
		} `json:"data"`
	}
	if err := json.NewDecoder(addressBook.Body).Decode(&addressBookEnvelope); err != nil {
		t.Fatalf("decode address book: %v", err)
	}
	if addressBookEnvelope.Data.ID == 0 || len(addressBookEnvelope.Data.Entries) != 1 || addressBookEnvelope.Data.Entries[0].Alias != "Demo Workstation" {
		t.Fatalf("unexpected address book: %+v", addressBookEnvelope.Data)
	}
	addressBookEntries, err := client.Get(server.URL + "/api/v1/address-books/" + idString(addressBookEnvelope.Data.ID) + "/entries")
	if err != nil {
		t.Fatalf("GET address book entries failed: %v", err)
	}
	defer addressBookEntries.Body.Close()
	if addressBookEntries.StatusCode != http.StatusOK {
		t.Fatalf("expected address book entries 200, got %d", addressBookEntries.StatusCode)
	}
	var addressBookEntriesEnvelope struct {
		Data []struct {
			ID       int64  `json:"id"`
			DeviceID int64  `json:"device_id"`
			Alias    string `json:"alias"`
		} `json:"data"`
	}
	if err := json.NewDecoder(addressBookEntries.Body).Decode(&addressBookEntriesEnvelope); err != nil {
		t.Fatalf("decode address book entries: %v", err)
	}
	if len(addressBookEntriesEnvelope.Data) != 1 || addressBookEntriesEnvelope.Data[0].DeviceID != 1 {
		t.Fatalf("unexpected address book entries: %+v", addressBookEntriesEnvelope.Data)
	}
	removeEntry := deleteURL(t, client, server.URL+"/api/v1/address-books/"+idString(addressBookEnvelope.Data.ID)+"/entries/"+idString(addressBookEnvelope.Data.Entries[0].ID))
	defer removeEntry.Body.Close()
	if removeEntry.StatusCode != http.StatusOK {
		t.Fatalf("expected remove address book entry 200, got %d", removeEntry.StatusCode)
	}
	addEntry := postJSON(t, client, server.URL+"/api/v1/address-books/"+idString(addressBookEnvelope.Data.ID)+"/entries", `{"device_id":1,"alias":"Demo Workstation Restored"}`)
	defer addEntry.Body.Close()
	if addEntry.StatusCode != http.StatusCreated {
		t.Fatalf("expected add address book entry 201, got %d", addEntry.StatusCode)
	}

	for _, endpoint := range []string{"/api/v1/user-groups", "/api/v1/device-groups", "/api/v1/address-books"} {
		resp, err := client.Get(server.URL + endpoint)
		if err != nil {
			t.Fatalf("GET %s failed: %v", endpoint, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected %s 200, got %d", endpoint, resp.StatusCode)
		}
	}
}

func TestGroupsAndAddressBooksRejectInvalidInput(t *testing.T) {
	server := httptest.NewServer(app.NewRouter(testConfig(t), slog.Default()))
	defer server.Close()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	client := &http.Client{Jar: jar}
	login := postJSON(t, client, server.URL+"/api/v1/auth/login", `{"email":"admin@example.com","password":"admin-password-12345"}`)
	defer login.Body.Close()
	if login.StatusCode != http.StatusOK {
		t.Fatalf("expected login 200, got %d", login.StatusCode)
	}

	invalidUserGroup := postJSON(t, client, server.URL+"/api/v1/user-groups", `{"description":"missing name"}`)
	defer invalidUserGroup.Body.Close()
	if invalidUserGroup.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected invalid user group 400, got %d", invalidUserGroup.StatusCode)
	}

	invalidDeviceGroup := postJSON(t, client, server.URL+"/api/v1/device-groups", `{"name":""}`)
	defer invalidDeviceGroup.Body.Close()
	if invalidDeviceGroup.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected invalid device group 400, got %d", invalidDeviceGroup.StatusCode)
	}

	invalidAddressBook := postJSON(t, client, server.URL+"/api/v1/address-books", `{"name":"Bad Book","entries":[{"device_id":0}]}`)
	defer invalidAddressBook.Body.Close()
	if invalidAddressBook.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected invalid address book 400, got %d", invalidAddressBook.StatusCode)
	}

	invalidUserMember := postJSON(t, client, server.URL+"/api/v1/user-groups/1/members", `{"user_id":0}`)
	defer invalidUserMember.Body.Close()
	if invalidUserMember.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected invalid user group member 400, got %d", invalidUserMember.StatusCode)
	}

	missingDeviceGroup, err := client.Get(server.URL + "/api/v1/device-groups/999999/members")
	if err != nil {
		t.Fatalf("missing device group members failed: %v", err)
	}
	defer missingDeviceGroup.Body.Close()
	if missingDeviceGroup.StatusCode != http.StatusNotFound {
		t.Fatalf("expected missing device group 404, got %d", missingDeviceGroup.StatusCode)
	}
}

func deleteURL(t *testing.T, client *http.Client, url string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		t.Fatalf("create DELETE request: %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("DELETE %s failed: %v", url, err)
	}
	return resp
}

func idString(id int64) string {
	return strconv.FormatInt(id, 10)
}
