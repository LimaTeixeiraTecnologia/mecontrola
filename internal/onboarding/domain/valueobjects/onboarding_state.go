package valueobjects

import "errors"

var ErrOnboardingStateInvalid = errors.New("onboarding: onboarding state invalid")

type OnboardingState uint8

const (
	OnboardingStateAwaitingToken OnboardingState = iota + 1
	OnboardingStateAwaitingIncome
	OnboardingStateAwaitingCardDecision
	OnboardingStateAwaitingCardName
	OnboardingStateAwaitingCardLimit
	OnboardingStateAwaitingCardClosingDay
	OnboardingStateAwaitingCardDueDay
	OnboardingStateAwaitingMoreCards
	OnboardingStateAwaitingSplitConfirm
	OnboardingStateAwaitingFirstTransaction
	OnboardingStateActive
)

func (s OnboardingState) String() string {
	switch s {
	case OnboardingStateAwaitingToken:
		return "awaiting_token"
	case OnboardingStateAwaitingIncome:
		return "awaiting_income"
	case OnboardingStateAwaitingCardDecision:
		return "awaiting_card_decision"
	case OnboardingStateAwaitingCardName:
		return "awaiting_card_name"
	case OnboardingStateAwaitingCardLimit:
		return "awaiting_card_limit"
	case OnboardingStateAwaitingCardClosingDay:
		return "awaiting_card_closing_day"
	case OnboardingStateAwaitingCardDueDay:
		return "awaiting_card_due_day"
	case OnboardingStateAwaitingMoreCards:
		return "awaiting_more_cards"
	case OnboardingStateAwaitingSplitConfirm:
		return "awaiting_split_confirm"
	case OnboardingStateAwaitingFirstTransaction:
		return "awaiting_first_transaction"
	case OnboardingStateActive:
		return "active"
	default:
		return "unknown"
	}
}

func (s OnboardingState) IsTerminal() bool {
	return s == OnboardingStateActive
}

func ParseOnboardingState(raw string) (OnboardingState, error) {
	switch raw {
	case "awaiting_token":
		return OnboardingStateAwaitingToken, nil
	case "awaiting_income":
		return OnboardingStateAwaitingIncome, nil
	case "awaiting_card_decision":
		return OnboardingStateAwaitingCardDecision, nil
	case "awaiting_card_name":
		return OnboardingStateAwaitingCardName, nil
	case "awaiting_card_limit":
		return OnboardingStateAwaitingCardLimit, nil
	case "awaiting_card_closing_day":
		return OnboardingStateAwaitingCardClosingDay, nil
	case "awaiting_card_due_day":
		return OnboardingStateAwaitingCardDueDay, nil
	case "awaiting_more_cards":
		return OnboardingStateAwaitingMoreCards, nil
	case "awaiting_split_confirm":
		return OnboardingStateAwaitingSplitConfirm, nil
	case "awaiting_first_transaction":
		return OnboardingStateAwaitingFirstTransaction, nil
	case "active":
		return OnboardingStateActive, nil
	default:
		return 0, ErrOnboardingStateInvalid
	}
}
