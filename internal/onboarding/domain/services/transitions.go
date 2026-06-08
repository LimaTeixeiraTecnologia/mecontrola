package services

import (
	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

type TransitionService struct{}

func NewTransitionService() TransitionService {
	return TransitionService{}
}

func (s TransitionService) CanMarkPaid(status valueobjects.TokenStatus) bool {
	return status == valueobjects.TokenStatusPending
}

func (s TransitionService) CanConsume(status valueobjects.TokenStatus) bool {
	return status == valueobjects.TokenStatusPaid
}

func (s TransitionService) CanMarkOutreach(status valueobjects.TokenStatus) bool {
	return status == valueobjects.TokenStatusPaid
}

func (s TransitionService) CanExpire(status valueobjects.TokenStatus) bool {
	return status == valueobjects.TokenStatusPending || status == valueobjects.TokenStatusPaid
}

func (s TransitionService) ValidateConsume(status valueobjects.TokenStatus) error {
	switch status {
	case valueobjects.TokenStatusPaid:
		return nil
	case valueobjects.TokenStatusPending:
		return domain.ErrTokenNotYetPaid
	case valueobjects.TokenStatusExpired:
		return domain.ErrTokenExpired
	case valueobjects.TokenStatusConsumed:
		return domain.ErrTokenAlreadyConsumedSame
	default:
		return domain.ErrTransitionNotAllowed
	}
}

func (s TransitionService) ValidateMarkPaid(status valueobjects.TokenStatus) error {
	if s.CanMarkPaid(status) {
		return nil
	}
	return nil
}
