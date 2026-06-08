package valueobjects

import "errors"

var ErrSupportSignalKindInvalid = errors.New("onboarding: support signal kind invalid")

type SupportSignalKind uint8

const (
	SupportSignalKindOrphanExpiredSubscription SupportSignalKind = iota + 1
	SupportSignalKindPaidWithoutToken
	SupportSignalKindTokenReuseAttempt
)

func (k SupportSignalKind) String() string {
	switch k {
	case SupportSignalKindOrphanExpiredSubscription:
		return "orphan_expired_subscription"
	case SupportSignalKindPaidWithoutToken:
		return "paid_without_token"
	case SupportSignalKindTokenReuseAttempt:
		return "token_reuse_attempt"
	default:
		return "unknown"
	}
}

func ParseSupportSignalKind(raw string) (SupportSignalKind, error) {
	switch raw {
	case "orphan_expired_subscription":
		return SupportSignalKindOrphanExpiredSubscription, nil
	case "paid_without_token":
		return SupportSignalKindPaidWithoutToken, nil
	case "token_reuse_attempt":
		return SupportSignalKindTokenReuseAttempt, nil
	default:
		return 0, ErrSupportSignalKindInvalid
	}
}
