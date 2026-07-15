package controlrole

type Mode string

const (
	UseClientSettings Mode = "use_client_settings"
	Enable            Mode = "enable"
	Disable           Mode = "disable"
)

var PermissionKeys = []string{
	"keyboard_mouse",
	"remote_printer",
	"clipboard",
	"file_transfer",
	"audio",
	"camera",
	"terminal",
	"tcp_tunnel",
	"remote_restart",
	"recording_session",
	"block_user_input",
	"remote_config_modification",
}

type Role struct {
	Name        string
	Enabled     bool
	Permissions map[string]Mode
}

type Evaluator struct{}

func (Evaluator) DefaultPermissions() map[string]Mode {
	out := map[string]Mode{}
	for _, key := range PermissionKeys {
		out[key] = UseClientSettings
	}
	out["terminal"] = Disable
	out["tcp_tunnel"] = Disable
	out["remote_config_modification"] = Disable
	return out
}

func (e Evaluator) Evaluate(roles []Role) map[string]Mode {
	out := e.DefaultPermissions()
	for _, role := range roles {
		if !role.Enabled {
			continue
		}
		for key, mode := range role.Permissions {
			if isKnown(key) {
				out[key] = mode
			}
		}
	}
	return out
}

func isKnown(key string) bool {
	for _, known := range PermissionKeys {
		if known == key {
			return true
		}
	}
	return false
}
