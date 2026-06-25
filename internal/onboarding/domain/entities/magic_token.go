package entities

import (
	"fmt"
	"time"

	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

type MagicToken struct {
	id                        string
	tokenHash                 []byte
	status                    valueobjects.TokenStatus
	planID                    string
	expiresAt                 time.Time
	createdAt                 time.Time
	paidAt                    time.Time
	consumedAt                time.Time
	outreachSentAt            time.Time
	activationTokenCiphertext string
	subscriptionID            string
	customerMobileE164        string
	customerEmail             string
	externalSaleID            string
	consumedByUserID          string
	consumedByMobileE164      string
	activationPath            valueobjects.ActivationPath
}

func NewMagicToken(id string, tokenHash []byte, planID string, expiresAt time.Time) (MagicToken, error) {
	if id == "" {
		return MagicToken{}, fmt.Errorf("onboarding: magic token id is required")
	}
	if len(tokenHash) == 0 {
		return MagicToken{}, fmt.Errorf("onboarding: token hash is required")
	}
	if planID == "" {
		return MagicToken{}, fmt.Errorf("onboarding: plan id is required")
	}
	if expiresAt.IsZero() {
		return MagicToken{}, fmt.Errorf("onboarding: expires_at is required")
	}
	return MagicToken{
		id:        id,
		tokenHash: tokenHash,
		status:    valueobjects.TokenStatusPending,
		planID:    planID,
		expiresAt: expiresAt,
		createdAt: time.Now().UTC(),
	}, nil
}

func HydrateMagicToken(
	id string,
	tokenHash []byte,
	status valueobjects.TokenStatus,
	planID string,
	expiresAt time.Time,
	createdAt time.Time,
	paidAt time.Time,
	consumedAt time.Time,
	outreachSentAt time.Time,
	activationTokenCiphertext string,
	subscriptionID string,
	customerMobileE164 string,
	customerEmail string,
	externalSaleID string,
	consumedByUserID string,
	consumedByMobileE164 string,
	activationPath valueobjects.ActivationPath,
) MagicToken {
	return MagicToken{
		id:                        id,
		tokenHash:                 tokenHash,
		status:                    status,
		planID:                    planID,
		expiresAt:                 expiresAt,
		createdAt:                 createdAt,
		paidAt:                    paidAt,
		consumedAt:                consumedAt,
		outreachSentAt:            outreachSentAt,
		activationTokenCiphertext: activationTokenCiphertext,
		subscriptionID:            subscriptionID,
		customerMobileE164:        customerMobileE164,
		customerEmail:             customerEmail,
		externalSaleID:            externalSaleID,
		consumedByUserID:          consumedByUserID,
		consumedByMobileE164:      consumedByMobileE164,
		activationPath:            activationPath,
	}
}

func (m MagicToken) ID() string                                  { return m.id }
func (m MagicToken) TokenHash() []byte                           { return m.tokenHash }
func (m MagicToken) Status() valueobjects.TokenStatus            { return m.status }
func (m MagicToken) PlanID() string                              { return m.planID }
func (m MagicToken) ExpiresAt() time.Time                        { return m.expiresAt }
func (m MagicToken) CreatedAt() time.Time                        { return m.createdAt }
func (m MagicToken) PaidAt() time.Time                           { return m.paidAt }
func (m MagicToken) ConsumedAt() time.Time                       { return m.consumedAt }
func (m MagicToken) OutreachSentAt() time.Time                   { return m.outreachSentAt }
func (m MagicToken) ActivationTokenCiphertext() string           { return m.activationTokenCiphertext }
func (m MagicToken) SubscriptionID() string                      { return m.subscriptionID }
func (m MagicToken) CustomerMobileE164() string                  { return m.customerMobileE164 }
func (m MagicToken) CustomerEmail() string                       { return m.customerEmail }
func (m MagicToken) ExternalSaleID() string                      { return m.externalSaleID }
func (m MagicToken) ConsumedByUserID() string                    { return m.consumedByUserID }
func (m MagicToken) ConsumedByMobileE164() string                { return m.consumedByMobileE164 }
func (m MagicToken) ActivationPath() valueobjects.ActivationPath { return m.activationPath }

func (m MagicToken) IsExpiredAt(now time.Time) bool {
	return now.After(m.expiresAt)
}

func (m MagicToken) HasOutreach() bool {
	return !m.outreachSentAt.IsZero()
}

func (m MagicToken) WithActivationTokenCiphertext(ciphertext string) (MagicToken, error) {
	if ciphertext == "" {
		return m, fmt.Errorf("onboarding: activation token ciphertext is required")
	}
	m.activationTokenCiphertext = ciphertext
	return m, nil
}

func (m MagicToken) MarkPaid(subscriptionID, customerMobileE164, customerEmail, externalSaleID string, paidAt time.Time) (MagicToken, error) {
	if m.status != valueobjects.TokenStatusPending {
		return m, nil
	}
	if subscriptionID == "" {
		return m, fmt.Errorf("onboarding: subscription id is required")
	}
	m.status = valueobjects.TokenStatusPaid
	m.subscriptionID = subscriptionID
	m.customerMobileE164 = customerMobileE164
	m.customerEmail = customerEmail
	m.externalSaleID = externalSaleID
	m.paidAt = paidAt
	return m, nil
}

func (m MagicToken) MarkConsumed(userID, mobileE164 string, path valueobjects.ActivationPath, now time.Time) (MagicToken, error) {
	if m.status != valueobjects.TokenStatusPaid {
		return m, domain.ErrTransitionNotAllowed
	}
	m.status = valueobjects.TokenStatusConsumed
	m.consumedByUserID = userID
	m.consumedByMobileE164 = mobileE164
	m.activationPath = path
	m.consumedAt = now
	return m, nil
}

func (m MagicToken) MarkExpired() (MagicToken, error) {
	if m.status == valueobjects.TokenStatusConsumed || m.status == valueobjects.TokenStatusExpired {
		return m, nil
	}
	m.status = valueobjects.TokenStatusExpired
	return m, nil
}

func (m MagicToken) MarkOutreachSent(sentAt time.Time) (MagicToken, error) {
	if m.status != valueobjects.TokenStatusPaid {
		return m, domain.ErrTransitionNotAllowed
	}
	m.outreachSentAt = sentAt
	return m, nil
}
