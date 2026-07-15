package strategy

type TargetType string

const (
	TargetDefault     TargetType = "default"
	TargetDevice      TargetType = "device"
	TargetUser        TargetType = "user"
	TargetDeviceGroup TargetType = "device_group"
)

type Strategy struct {
	ID       int64
	Name     string
	Enabled  bool
	Settings map[string]string
}

type Assignment struct {
	StrategyID int64
	TargetType TargetType
	TargetID   int64
}

type Context struct {
	DeviceID       int64
	UserID         *int64
	DeviceGroupIDs []int64
}

type Result struct {
	Settings        map[string]string
	AppliedStrategy []int64
}

type Resolver struct {
	Strategies  []Strategy
	Assignments []Assignment
	Default     Strategy
}

func (r Resolver) Resolve(ctx Context) Result {
	settings := map[string]string{}
	applied := []int64{}
	if r.Default.Enabled {
		merge(settings, r.Default.Settings)
		applied = append(applied, r.Default.ID)
	}
	for _, target := range []TargetType{TargetDeviceGroup, TargetUser, TargetDevice} {
		for _, assignment := range r.Assignments {
			if assignment.TargetType == target && assignmentMatches(assignment, ctx) {
				if strategy, ok := r.find(assignment.StrategyID); ok && strategy.Enabled {
					merge(settings, strategy.Settings)
					applied = append(applied, strategy.ID)
				}
			}
		}
	}
	return Result{Settings: settings, AppliedStrategy: applied}
}

func (r Resolver) find(id int64) (Strategy, bool) {
	for _, strategy := range r.Strategies {
		if strategy.ID == id {
			return strategy, true
		}
	}
	return Strategy{}, false
}

func assignmentMatches(assignment Assignment, ctx Context) bool {
	switch assignment.TargetType {
	case TargetDevice:
		return assignment.TargetID == ctx.DeviceID
	case TargetUser:
		return ctx.UserID != nil && assignment.TargetID == *ctx.UserID
	case TargetDeviceGroup:
		for _, id := range ctx.DeviceGroupIDs {
			if assignment.TargetID == id {
				return true
			}
		}
	}
	return false
}

func merge(dst map[string]string, src map[string]string) {
	for key, value := range src {
		dst[key] = value
	}
}
