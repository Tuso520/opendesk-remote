package buildworker

import (
	"encoding/json"

	"github.com/opendesk-remote/opendesk-remote/api/internal/models"
)

type cliBuildSpec struct {
	App       cliAppSpec      `json:"app"`
	Branding  json.RawMessage `json:"branding"`
	Server    json.RawMessage `json:"server"`
	Policy    json.RawMessage `json:"policy"`
	Platforms json.RawMessage `json:"platforms"`
	Signing   json.RawMessage `json:"signing"`
	Source    json.RawMessage `json:"source"`
}

type cliAppSpec struct {
	Name               string `json:"name"`
	Vendor             string `json:"vendor"`
	BundleID           string `json:"bundle_id"`
	WindowsProductName string `json:"windows_product_name"`
	Description        string `json:"description"`
}

func buildSpecJSON(profile models.BuildProfile) ([]byte, error) {
	server, err := objectWithDefaults(profile.ServerConfigJSON, map[string]any{
		"relay_name":           "hbbr-relay-a",
		"relay_grant_required": true,
	})
	if err != nil {
		return nil, err
	}
	source, err := objectWithDefaults(profile.SourceJSON, map[string]any{
		"rustdesk_ref":      "master",
		"opendesk_patchset": "opendesk-remote-m4",
	})
	if err != nil {
		return nil, err
	}
	vendor := profile.Vendor
	if vendor == "" {
		vendor = "OpenDesk"
	}
	productName := profile.ProductName
	if productName == "" {
		productName = profile.AppName
	}
	spec := cliBuildSpec{
		App: cliAppSpec{
			Name:               profile.AppName,
			Vendor:             vendor,
			BundleID:           profile.BundleID,
			WindowsProductName: productName,
			Description:        profile.Description,
		},
		Branding:  rawObject(profile.BrandingJSON),
		Server:    server,
		Policy:    rawObject(profile.PolicyJSON),
		Platforms: rawObject(profile.PlatformsJSON),
		Signing:   rawObject(profile.SigningJSON),
		Source:    source,
	}
	return json.MarshalIndent(spec, "", "  ")
}

func objectWithDefaults(raw string, defaults map[string]any) (json.RawMessage, error) {
	values := map[string]any{}
	if raw != "" {
		if err := json.Unmarshal([]byte(raw), &values); err != nil {
			return nil, err
		}
	}
	for key, value := range defaults {
		if _, ok := values[key]; !ok {
			values[key] = value
		}
	}
	return json.Marshal(values)
}

func rawObject(raw string) json.RawMessage {
	if raw == "" {
		return json.RawMessage(`{}`)
	}
	return json.RawMessage(raw)
}
