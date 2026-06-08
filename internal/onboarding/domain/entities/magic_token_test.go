package entities_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

type MagicTokenSuite struct {
	suite.Suite
}

func TestMagicTokenSuite(t *testing.T) {
	suite.Run(t, new(MagicTokenSuite))
}

func (s *MagicTokenSuite) TestNewMagicToken_CreatesWithPendingStatus() {
	expires := time.Now().UTC().Add(7 * 24 * time.Hour)
	token, err := entities.NewMagicToken("id-1", []byte("hash"), "plan-1", expires)
	s.Require().NoError(err)
	s.Equal(valueobjects.TokenStatusPending, token.Status())
}

func (s *MagicTokenSuite) TestNewMagicToken_RequiresID() {
	expires := time.Now().UTC().Add(7 * 24 * time.Hour)
	_, err := entities.NewMagicToken("", []byte("hash"), "plan-1", expires)
	s.Error(err)
}

func (s *MagicTokenSuite) TestNewMagicToken_RequiresTokenHash() {
	expires := time.Now().UTC().Add(7 * 24 * time.Hour)
	_, err := entities.NewMagicToken("id-1", nil, "plan-1", expires)
	s.Error(err)
}

func (s *MagicTokenSuite) TestNewMagicToken_RequiresPlanID() {
	expires := time.Now().UTC().Add(7 * 24 * time.Hour)
	_, err := entities.NewMagicToken("id-1", []byte("hash"), "", expires)
	s.Error(err)
}

func (s *MagicTokenSuite) TestMarkPaid_TransitionsPendingToPaid() {
	expires := time.Now().UTC().Add(7 * 24 * time.Hour)
	token, _ := entities.NewMagicToken("id-1", []byte("hash"), "plan-1", expires)
	paidAt := time.Now().UTC()

	paid, err := token.MarkPaid("sub-001", "+5511999990000", "user@test.com", "ext-sale-1", paidAt)

	s.Require().NoError(err)
	s.Equal(valueobjects.TokenStatusPaid, paid.Status())
	s.Equal("sub-001", paid.SubscriptionID())
	s.Equal("+5511999990000", paid.CustomerMobileE164())
	s.Equal("user@test.com", paid.CustomerEmail())
	s.Equal("ext-sale-1", paid.ExternalSaleID())
}

func (s *MagicTokenSuite) TestMarkPaid_IsNoOpWhenAlreadyPaid() {
	expires := time.Now().UTC().Add(7 * 24 * time.Hour)
	token, _ := entities.NewMagicToken("id-1", []byte("hash"), "plan-1", expires)
	paidAt := time.Now().UTC()

	paid, _ := token.MarkPaid("sub-001", "+5511999990000", "user@test.com", "ext-1", paidAt)
	paidAgain, err := paid.MarkPaid("sub-002", "+5511999990001", "other@test.com", "ext-2", paidAt)

	s.Require().NoError(err)
	s.Equal("+5511999990000", paidAgain.CustomerMobileE164())
}

func (s *MagicTokenSuite) TestMarkConsumed_TransitionsPaidToConsumed() {
	expires := time.Now().UTC().Add(7 * 24 * time.Hour)
	token, _ := entities.NewMagicToken("id-1", []byte("hash"), "plan-1", expires)
	token, _ = token.MarkPaid("sub-001", "+5511999990000", "user@test.com", "ext-1", time.Now().UTC())

	consumed, err := token.MarkConsumed("user-1", "+5511999990000", valueobjects.ActivationPathDirect, time.Now().UTC())

	s.Require().NoError(err)
	s.Equal(valueobjects.TokenStatusConsumed, consumed.Status())
	s.Equal("user-1", consumed.ConsumedByUserID())
	s.Equal(valueobjects.ActivationPathDirect, consumed.ActivationPath())
}

func (s *MagicTokenSuite) TestMarkConsumed_FailsFromPending() {
	expires := time.Now().UTC().Add(7 * 24 * time.Hour)
	token, _ := entities.NewMagicToken("id-1", []byte("hash"), "plan-1", expires)

	_, err := token.MarkConsumed("user-1", "+5511999990000", valueobjects.ActivationPathDirect, time.Now().UTC())

	s.ErrorIs(err, domain.ErrTransitionNotAllowed)
}

func (s *MagicTokenSuite) TestMarkExpired_TransitionsPendingToExpired() {
	expires := time.Now().UTC().Add(-1 * time.Hour)
	token, _ := entities.NewMagicToken("id-1", []byte("hash"), "plan-1", expires)

	expired, err := token.MarkExpired()

	s.Require().NoError(err)
	s.Equal(valueobjects.TokenStatusExpired, expired.Status())
}

func (s *MagicTokenSuite) TestMarkExpired_IsNoOpWhenConsumed() {
	expires := time.Now().UTC().Add(7 * 24 * time.Hour)
	token, _ := entities.NewMagicToken("id-1", []byte("hash"), "plan-1", expires)
	token, _ = token.MarkPaid("sub-001", "+5511999990000", "user@test.com", "ext-1", time.Now().UTC())
	token, _ = token.MarkConsumed("user-1", "+5511999990000", valueobjects.ActivationPathDirect, time.Now().UTC())

	expired, err := token.MarkExpired()

	s.Require().NoError(err)
	s.Equal(valueobjects.TokenStatusConsumed, expired.Status())
}

func (s *MagicTokenSuite) TestIsExpiredAt_TrueWhenPastExpiresAt() {
	expires := time.Now().UTC().Add(-1 * time.Minute)
	token, _ := entities.NewMagicToken("id-1", []byte("hash"), "plan-1", expires)

	s.True(token.IsExpiredAt(time.Now().UTC()))
}

func (s *MagicTokenSuite) TestHasOutreach_FalseWhenOutreachNotSent() {
	expires := time.Now().UTC().Add(7 * 24 * time.Hour)
	token, _ := entities.NewMagicToken("id-1", []byte("hash"), "plan-1", expires)

	s.False(token.HasOutreach())
}

func (s *MagicTokenSuite) TestMarkOutreachSent_SetsSentAt() {
	expires := time.Now().UTC().Add(7 * 24 * time.Hour)
	token, _ := entities.NewMagicToken("id-1", []byte("hash"), "plan-1", expires)
	token, _ = token.MarkPaid("sub-001", "+5511999990000", "user@test.com", "ext-1", time.Now().UTC())

	sentAt := time.Now().UTC()
	outreached, err := token.MarkOutreachSent(sentAt)

	s.Require().NoError(err)
	s.True(outreached.HasOutreach())
	s.Equal(sentAt, outreached.OutreachSentAt())
}

func (s *MagicTokenSuite) TestMarkOutreachSent_FailsFromPending() {
	expires := time.Now().UTC().Add(7 * 24 * time.Hour)
	token, _ := entities.NewMagicToken("id-1", []byte("hash"), "plan-1", expires)

	_, err := token.MarkOutreachSent(time.Now().UTC())

	s.ErrorIs(err, domain.ErrTransitionNotAllowed)
}
