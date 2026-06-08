package valueobjects

import "errors"

var ErrActivationPathInvalid = errors.New("onboarding: activation path invalid")

type ActivationPath uint8

const (
	ActivationPathDirect ActivationPath = iota + 1
	ActivationPathFallbackE164
	ActivationPathOutreach
	ActivationPathAdmin
)

func (p ActivationPath) String() string {
	switch p {
	case ActivationPathDirect:
		return "direct"
	case ActivationPathFallbackE164:
		return "fallback_e164"
	case ActivationPathOutreach:
		return "outreach"
	case ActivationPathAdmin:
		return "admin"
	default:
		return "unknown"
	}
}

func ParseActivationPath(raw string) (ActivationPath, error) {
	switch raw {
	case "direct":
		return ActivationPathDirect, nil
	case "fallback_e164":
		return ActivationPathFallbackE164, nil
	case "outreach":
		return ActivationPathOutreach, nil
	case "admin":
		return ActivationPathAdmin, nil
	default:
		return 0, ErrActivationPathInvalid
	}
}
