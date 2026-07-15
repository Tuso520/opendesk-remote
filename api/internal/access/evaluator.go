package access

import "sort"

type User struct {
	ID       int64
	Status   string
	GroupIDs []int64
}

type Device struct {
	ID       int64
	Status   string
	GroupIDs []int64
}

type Rule struct {
	ID          int64
	SubjectType string
	SubjectID   int64
	TargetType  string
	TargetID    int64
	Effect      string
	Priority    int
	Enabled     bool
}

type Decision struct {
	Allowed     bool
	Reason      string
	MatchedRule *Rule
}

type Evaluator struct {
	Rules []Rule
}

func (e Evaluator) Evaluate(user User, target Device) Decision {
	if user.Status == "disabled" || user.Status == "locked" {
		return Decision{Allowed: false, Reason: "user_not_active"}
	}
	if target.Status == "disabled" {
		return Decision{Allowed: false, Reason: "target_device_disabled"}
	}
	matches := make([]Rule, 0)
	for _, rule := range e.Rules {
		if rule.Enabled && ruleMatches(rule, user, target) {
			matches = append(matches, rule)
		}
	}
	if len(matches) == 0 {
		return Decision{Allowed: false, Reason: "default_deny"}
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].Effect != matches[j].Effect {
			return matches[i].Effect == "deny"
		}
		if rank(matches[i]) != rank(matches[j]) {
			return rank(matches[i]) > rank(matches[j])
		}
		return matches[i].Priority > matches[j].Priority
	})
	rule := matches[0]
	if rule.Effect == "allow" {
		return Decision{Allowed: true, Reason: "matched_allow_rule", MatchedRule: &rule}
	}
	return Decision{Allowed: false, Reason: "matched_deny_rule", MatchedRule: &rule}
}

func ruleMatches(rule Rule, user User, target Device) bool {
	subject := false
	switch rule.SubjectType {
	case "user":
		subject = rule.SubjectID == user.ID
	case "user_group":
		subject = contains(user.GroupIDs, rule.SubjectID)
	}
	targetMatch := false
	switch rule.TargetType {
	case "device":
		targetMatch = rule.TargetID == target.ID
	case "device_group":
		targetMatch = contains(target.GroupIDs, rule.TargetID)
	}
	return subject && targetMatch
}

func rank(rule Rule) int {
	switch rule.SubjectType + ":" + rule.TargetType {
	case "user:device":
		return 4
	case "user_group:device":
		return 3
	case "user:device_group":
		return 2
	case "user_group:device_group":
		return 1
	default:
		return 0
	}
}

func contains(values []int64, want int64) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
