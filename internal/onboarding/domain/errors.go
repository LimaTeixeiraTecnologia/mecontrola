package domain

import "errors"

var (
	ErrTokenNotFound             = errors.New("onboarding: token not found")
	ErrTokenExpired              = errors.New("onboarding: token expired")
	ErrTokenNotYetPaid           = errors.New("onboarding: token not yet paid")
	ErrTokenAlreadyConsumedSame  = errors.New("onboarding: token already consumed by same number")
	ErrTokenAlreadyConsumedOther = errors.New("onboarding: token already consumed by different number")
	ErrUnsupportedCountry        = errors.New("onboarding: unsupported country code")
	ErrRateLimited               = errors.New("onboarding: rate limit exceeded")
	ErrTransitionNotAllowed      = errors.New("onboarding: transition not allowed")
)
