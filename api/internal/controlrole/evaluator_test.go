package controlrole

import "testing"

func TestControlRoleDefaultsSecurePermissions(t *testing.T) {
	perms := (Evaluator{}).DefaultPermissions()
	if perms["terminal"] != Disable || perms["tcp_tunnel"] != Disable || perms["remote_config_modification"] != Disable {
		t.Fatalf("unsafe default permissions: %+v", perms)
	}
	if perms["clipboard"] != UseClientSettings || perms["keyboard_mouse"] != UseClientSettings {
		t.Fatalf("unexpected client setting defaults: %+v", perms)
	}
}

func TestControlRoleAppliesEnabledRole(t *testing.T) {
	perms := (Evaluator{}).Evaluate([]Role{{Enabled: true, Permissions: map[string]Mode{"file_transfer": Disable, "audio": Enable}}})
	if perms["file_transfer"] != Disable || perms["audio"] != Enable {
		t.Fatalf("role not applied: %+v", perms)
	}
}

func TestControlRoleIgnoresDisabledRoleAndUnknownKeys(t *testing.T) {
	perms := (Evaluator{}).Evaluate([]Role{{Enabled: false, Permissions: map[string]Mode{"clipboard": Disable}}, {Enabled: true, Permissions: map[string]Mode{"unknown": Enable}}})
	if perms["clipboard"] != UseClientSettings {
		t.Fatalf("disabled role should not change clipboard: %+v", perms)
	}
	if _, ok := perms["unknown"]; ok {
		t.Fatal("unknown permission must not be added")
	}
}
