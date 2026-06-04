package valueobjects

type PlanCode uint8

const (
	PlanCodeUnknown PlanCode = iota
	PlanCodeMonthly
	PlanCodeQuarterly
	PlanCodeAnnual
)

func (p PlanCode) String() string {
	switch p {
	case PlanCodeMonthly:
		return "MONTHLY"
	case PlanCodeQuarterly:
		return "QUARTERLY"
	case PlanCodeAnnual:
		return "ANNUAL"
	default:
		return "UNKNOWN"
	}
}

func ParsePlanCode(s string) (PlanCode, error) {
	switch s {
	case "MONTHLY":
		return PlanCodeMonthly, nil
	case "QUARTERLY":
		return PlanCodeQuarterly, nil
	case "ANNUAL":
		return PlanCodeAnnual, nil
	default:
		return PlanCodeUnknown, ErrUnknownPlanCode
	}
}
