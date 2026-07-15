package strategy

import "testing"

func TestStrategyPriorityDeviceOverridesUserAndGroup(t *testing.T) {
	userID := int64(5)
	resolver := Resolver{
		Default: Strategy{ID: 1, Enabled: true, Settings: map[string]string{"file_transfer": "Y", "terminal": "N"}},
		Strategies: []Strategy{
			{ID: 2, Enabled: true, Settings: map[string]string{"file_transfer": "N"}},
			{ID: 3, Enabled: true, Settings: map[string]string{"clipboard": "N"}},
			{ID: 4, Enabled: true, Settings: map[string]string{"terminal": "Y"}},
		},
		Assignments: []Assignment{
			{StrategyID: 2, TargetType: TargetDeviceGroup, TargetID: 9},
			{StrategyID: 3, TargetType: TargetUser, TargetID: userID},
			{StrategyID: 4, TargetType: TargetDevice, TargetID: 10},
		},
	}
	result := resolver.Resolve(Context{DeviceID: 10, UserID: &userID, DeviceGroupIDs: []int64{9}})
	if result.Settings["file_transfer"] != "N" || result.Settings["clipboard"] != "N" || result.Settings["terminal"] != "Y" {
		t.Fatalf("unexpected effective strategy: %+v", result.Settings)
	}
}

func TestStrategyDefaultOnly(t *testing.T) {
	result := (Resolver{Default: Strategy{ID: 1, Enabled: true, Settings: map[string]string{"terminal": "N"}}}).Resolve(Context{DeviceID: 1})
	if result.Settings["terminal"] != "N" {
		t.Fatalf("expected default strategy, got %+v", result.Settings)
	}
}
