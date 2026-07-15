package access

import "testing"

func TestAccessDefaultDeny(t *testing.T) {
	decision := (Evaluator{}).Evaluate(User{ID: 1, Status: "active"}, Device{ID: 2, Status: "online"})
	if decision.Allowed || decision.Reason != "default_deny" {
		t.Fatalf("unexpected decision: %+v", decision)
	}
}

func TestAccessAllowUserToDevice(t *testing.T) {
	evaluator := Evaluator{Rules: []Rule{{ID: 1, SubjectType: "user", SubjectID: 1, TargetType: "device", TargetID: 2, Effect: "allow", Enabled: true}}}
	decision := evaluator.Evaluate(User{ID: 1, Status: "active"}, Device{ID: 2, Status: "online"})
	if !decision.Allowed {
		t.Fatalf("expected allow, got %+v", decision)
	}
}

func TestAccessDenyOverridesAllow(t *testing.T) {
	evaluator := Evaluator{Rules: []Rule{
		{ID: 1, SubjectType: "user", SubjectID: 1, TargetType: "device", TargetID: 2, Effect: "allow", Enabled: true, Priority: 100},
		{ID: 2, SubjectType: "user_group", SubjectID: 9, TargetType: "device", TargetID: 2, Effect: "deny", Enabled: true, Priority: 1},
	}}
	decision := evaluator.Evaluate(User{ID: 1, Status: "active", GroupIDs: []int64{9}}, Device{ID: 2, Status: "online"})
	if decision.Allowed || decision.Reason != "matched_deny_rule" {
		t.Fatalf("expected deny override, got %+v", decision)
	}
}

func TestAccessUserAndDeviceGroupRules(t *testing.T) {
	evaluator := Evaluator{Rules: []Rule{{ID: 1, SubjectType: "user_group", SubjectID: 9, TargetType: "device_group", TargetID: 7, Effect: "allow", Enabled: true}}}
	decision := evaluator.Evaluate(User{ID: 1, Status: "active", GroupIDs: []int64{9}}, Device{ID: 2, Status: "online", GroupIDs: []int64{7}})
	if !decision.Allowed {
		t.Fatalf("expected group allow, got %+v", decision)
	}
}

func TestAccessDisabledUserAndDevice(t *testing.T) {
	evaluator := Evaluator{Rules: []Rule{{ID: 1, SubjectType: "user", SubjectID: 1, TargetType: "device", TargetID: 2, Effect: "allow", Enabled: true}}}
	if evaluator.Evaluate(User{ID: 1, Status: "disabled"}, Device{ID: 2, Status: "online"}).Allowed {
		t.Fatal("disabled user must be denied")
	}
	if evaluator.Evaluate(User{ID: 1, Status: "active"}, Device{ID: 2, Status: "disabled"}).Allowed {
		t.Fatal("disabled device must be denied")
	}
}
