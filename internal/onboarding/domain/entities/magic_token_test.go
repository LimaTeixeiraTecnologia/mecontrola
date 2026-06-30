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
	now time.Time
}

func TestMagicTokenSuite(t *testing.T) {
	suite.Run(t, new(MagicTokenSuite))
}

func (s *MagicTokenSuite) SetupTest() {
	s.now = time.Now().UTC().Truncate(time.Second)
}

func (s *MagicTokenSuite) newPendingToken(expiresAt time.Time) entities.MagicToken {
	token, err := entities.NewMagicToken("id-1", []byte("hash"), "plan-1", expiresAt)
	s.Require().NoError(err)
	return token
}

func (s *MagicTokenSuite) newPaidToken(expiresAt time.Time) entities.MagicToken {
	token := s.newPendingToken(expiresAt)
	paidToken, err := token.MarkPaid("sub-001", "+5511999990000", "user@test.com", "ext-1", s.now)
	s.Require().NoError(err)
	return paidToken
}

func (s *MagicTokenSuite) newConsumedToken(expiresAt time.Time) entities.MagicToken {
	token := s.newPaidToken(expiresAt)
	consumedToken, err := token.MarkConsumed("user-1", "+5511999990000", valueobjects.ActivationPathDirect, s.now)
	s.Require().NoError(err)
	return consumedToken
}

func (s *MagicTokenSuite) TestNewMagicToken() {
	type args struct {
		id        string
		hash      []byte
		planID    string
		expiresAt time.Time
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(entities.MagicToken, error)
	}{
		{
			name: "deve criar token pendente",
			args: args{id: "id-1", hash: []byte("hash"), planID: "plan-1", expiresAt: s.now.Add(7 * 24 * time.Hour)},
			expect: func(token entities.MagicToken, err error) {
				s.Require().NoError(err)
				s.Equal(valueobjects.TokenStatusPending, token.Status())
			},
		},
		{
			name: "deve falhar sem id",
			args: args{id: "", hash: []byte("hash"), planID: "plan-1", expiresAt: s.now.Add(7 * 24 * time.Hour)},
			expect: func(token entities.MagicToken, err error) {
				s.Error(err)
				s.Zero(token)
			},
		},
		{
			name: "deve falhar sem hash",
			args: args{id: "id-1", hash: nil, planID: "plan-1", expiresAt: s.now.Add(7 * 24 * time.Hour)},
			expect: func(token entities.MagicToken, err error) {
				s.Error(err)
				s.Zero(token)
			},
		},
		{
			name: "deve falhar sem plan id",
			args: args{id: "id-1", hash: []byte("hash"), planID: "", expiresAt: s.now.Add(7 * 24 * time.Hour)},
			expect: func(token entities.MagicToken, err error) {
				s.Error(err)
				s.Zero(token)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			token, err := entities.NewMagicToken(
				scenario.args.id,
				scenario.args.hash,
				scenario.args.planID,
				scenario.args.expiresAt,
			)
			scenario.expect(token, err)
		})
	}
}

func (s *MagicTokenSuite) TestMarkPaid() {
	type args struct {
		token entities.MagicToken
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(entities.MagicToken, error)
	}{
		{
			name: "deve transicionar de pendente para pago",
			args: args{token: s.newPendingToken(s.now.Add(7 * 24 * time.Hour))},
			expect: func(token entities.MagicToken, err error) {
				s.Require().NoError(err)
				s.Equal(valueobjects.TokenStatusPaid, token.Status())
				s.Equal("sub-001", token.SubscriptionID())
				s.Equal("+5511999990000", token.CustomerMobileE164())
				s.Equal("user@test.com", token.CustomerEmail())
				s.Equal("ext-1", token.ExternalSaleID())
			},
		},
		{
			name: "deve manter dados quando ja estiver pago",
			args: args{token: s.newPaidToken(s.now.Add(7 * 24 * time.Hour))},
			expect: func(token entities.MagicToken, err error) {
				s.Require().NoError(err)
				s.Equal("+5511999990000", token.CustomerMobileE164())
				s.Equal("sub-001", token.SubscriptionID())
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			paidToken, err := scenario.args.token.MarkPaid("sub-001", "+5511999990000", "user@test.com", "ext-1", s.now)
			if scenario.name == "deve manter dados quando ja estiver pago" {
				paidToken, err = scenario.args.token.MarkPaid("sub-002", "+5511999990001", "other@test.com", "ext-2", s.now)
			}
			scenario.expect(paidToken, err)
		})
	}
}

func (s *MagicTokenSuite) TestMarkConsumed() {
	type args struct {
		token entities.MagicToken
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(entities.MagicToken, error)
	}{
		{
			name: "deve consumir token pago",
			args: args{token: s.newPaidToken(s.now.Add(7 * 24 * time.Hour))},
			expect: func(token entities.MagicToken, err error) {
				s.Require().NoError(err)
				s.Equal(valueobjects.TokenStatusConsumed, token.Status())
				s.Equal("user-1", token.ConsumedByUserID())
				s.Equal(valueobjects.ActivationPathDirect, token.ActivationPath())
			},
		},
		{
			name: "deve falhar ao consumir token pendente",
			args: args{token: s.newPendingToken(s.now.Add(7 * 24 * time.Hour))},
			expect: func(token entities.MagicToken, err error) {
				s.ErrorIs(err, domain.ErrTransitionNotAllowed)
				s.Equal(valueobjects.TokenStatusPending, token.Status())
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			consumedToken, err := scenario.args.token.MarkConsumed("user-1", "+5511999990000", valueobjects.ActivationPathDirect, s.now)
			scenario.expect(consumedToken, err)
		})
	}
}

func (s *MagicTokenSuite) TestMarkExpired() {
	type args struct {
		token entities.MagicToken
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(entities.MagicToken, error)
	}{
		{
			name: "deve expirar token pendente",
			args: args{token: s.newPendingToken(s.now.Add(-time.Hour))},
			expect: func(token entities.MagicToken, err error) {
				s.Require().NoError(err)
				s.Equal(valueobjects.TokenStatusExpired, token.Status())
			},
		},
		{
			name: "deve manter token consumido",
			args: args{token: s.newConsumedToken(s.now.Add(7 * 24 * time.Hour))},
			expect: func(token entities.MagicToken, err error) {
				s.Require().NoError(err)
				s.Equal(valueobjects.TokenStatusConsumed, token.Status())
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			expiredToken, err := scenario.args.token.MarkExpired()
			scenario.expect(expiredToken, err)
		})
	}
}

func (s *MagicTokenSuite) TestHelpers() {
	scenarios := []struct {
		name   string
		expect func()
	}{
		{
			name: "deve identificar expiracao pelo expires at",
			expect: func() {
				token := s.newPendingToken(s.now.Add(-time.Minute))
				s.True(token.IsExpiredAt(s.now))
			},
		},
		{
			name: "deve indicar sem outreach quando nao enviado",
			expect: func() {
				token := s.newPendingToken(s.now.Add(7 * 24 * time.Hour))
				s.False(token.HasOutreach())
			},
		},
		{
			name: "deve registrar outreach para token pago",
			expect: func() {
				token := s.newPaidToken(s.now.Add(7 * 24 * time.Hour))
				outreachedToken, err := token.MarkOutreachSent(s.now)
				s.Require().NoError(err)
				s.True(outreachedToken.HasOutreach())
				s.Equal(s.now, outreachedToken.OutreachSentAt())
			},
		},
		{
			name: "deve falhar ao registrar outreach para token pendente",
			expect: func() {
				token := s.newPendingToken(s.now.Add(7 * 24 * time.Hour))
				outreachedToken, err := token.MarkOutreachSent(s.now)
				s.ErrorIs(err, domain.ErrTransitionNotAllowed)
				s.Equal(valueobjects.TokenStatusPending, outreachedToken.Status())
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, scenario.expect)
	}
}

func (s *MagicTokenSuite) TestIsActivationWindowOpen() {
	window := 24 * time.Hour

	scenarios := []struct {
		name   string
		token  func() entities.MagicToken
		now    time.Time
		expect bool
	}{
		{
			name: "dentro da janela",
			token: func() entities.MagicToken {
				return s.newPaidToken(s.now.Add(7 * 24 * time.Hour))
			},
			now:    s.now.Add(12 * time.Hour),
			expect: true,
		},
		{
			name: "exatamente na borda da janela",
			token: func() entities.MagicToken {
				return s.newPaidToken(s.now.Add(7 * 24 * time.Hour))
			},
			now:    s.now.Add(24 * time.Hour),
			expect: true,
		},
		{
			name: "apos a janela",
			token: func() entities.MagicToken {
				return s.newPaidToken(s.now.Add(7 * 24 * time.Hour))
			},
			now:    s.now.Add(25 * time.Hour),
			expect: false,
		},
		{
			name: "now antes de paidAt",
			token: func() entities.MagicToken {
				return s.newPaidToken(s.now.Add(7 * 24 * time.Hour))
			},
			now:    s.now.Add(-1 * time.Hour),
			expect: false,
		},
		{
			name: "token com status nao pago",
			token: func() entities.MagicToken {
				return s.newConsumedToken(s.now.Add(7 * 24 * time.Hour))
			},
			now:    s.now.Add(12 * time.Hour),
			expect: false,
		},
		{
			name: "token pendente",
			token: func() entities.MagicToken {
				return s.newPendingToken(s.now.Add(7 * 24 * time.Hour))
			},
			now:    s.now.Add(12 * time.Hour),
			expect: false,
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			result := scenario.token().IsActivationWindowOpen(scenario.now, window)
			s.Equal(scenario.expect, result)
		})
	}
}
