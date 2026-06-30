package usecases

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/suite"

	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

const validClearToken = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

type stubActivationTemplate struct {
	renderFn func(in ActivationTemplateInput) (string, string, error)
	lastIn   ActivationTemplateInput
}

func (s *stubActivationTemplate) Render(in ActivationTemplateInput) (string, string, error) {
	s.lastIn = in
	if s.renderFn != nil {
		return s.renderFn(in)
	}
	return "<html></html>", "text", nil
}

type stubEmailSender struct {
	sendFn func(ctx context.Context, msg appinterfaces.EmailMessage) error
}

func (s *stubEmailSender) Send(ctx context.Context, msg appinterfaces.EmailMessage) error {
	if s.sendFn != nil {
		return s.sendFn(ctx, msg)
	}
	return nil
}

type stubMagicTokenRepository struct {
	findByHashFn func(ctx context.Context, hash []byte) (entities.MagicToken, error)
}

func (s *stubMagicTokenRepository) FindByHash(ctx context.Context, hash []byte) (entities.MagicToken, error) {
	if s.findByHashFn != nil {
		return s.findByHashFn(ctx, hash)
	}
	return entities.MagicToken{}, domain.ErrTokenNotFound
}

func (s *stubMagicTokenRepository) Insert(_ context.Context, _ entities.MagicToken) error {
	return nil
}

func (s *stubMagicTokenRepository) FindPaidByMobileForFallback(_ context.Context, _ string) (entities.MagicToken, error) {
	return entities.MagicToken{}, domain.ErrTokenNotFound
}

func (s *stubMagicTokenRepository) FindPaidForOutreach(_ context.Context, _ time.Time, _ int) ([]entities.MagicToken, error) {
	return nil, nil
}

func (s *stubMagicTokenRepository) UpdateMarkPaid(_ context.Context, _ entities.MagicToken) error {
	return nil
}

func (s *stubMagicTokenRepository) UpdateMarkConsumed(_ context.Context, _ entities.MagicToken) error {
	return nil
}

func (s *stubMagicTokenRepository) UpdateMarkOutreachSent(_ context.Context, _ string, _ time.Time) error {
	return nil
}

func (s *stubMagicTokenRepository) UpdateMarkOutreachReset(_ context.Context, _ string) error {
	return nil
}

func (s *stubMagicTokenRepository) BulkExpire(_ context.Context, _ time.Time, _ int) ([]entities.MagicToken, error) {
	return nil, nil
}

func (s *stubMagicTokenRepository) CountPaidUnconsumed(_ context.Context) (int64, error) {
	return 0, nil
}

func newPaidToken() entities.MagicToken {
	t, _ := entities.NewMagicToken("id-1", []byte("hash"), "plan-1", time.Now().Add(24*time.Hour))
	t, _ = t.MarkPaid("sub-1", "+5511999999999", "u@e.com", "sale-1", time.Now())
	return t
}

func newConsumedToken() entities.MagicToken {
	t, _ := entities.NewMagicToken("id-2", []byte("hash"), "plan-1", time.Now().Add(24*time.Hour))
	t, _ = t.MarkPaid("sub-1", "+5511999999999", "u@e.com", "sale-1", time.Now())
	t, _ = t.MarkConsumed("user-1", "+5511999999999", valueobjects.ActivationPathDirect, time.Now())
	return t
}

func newExpiredToken() entities.MagicToken {
	t, _ := entities.NewMagicToken("id-3", []byte("hash"), "plan-1", time.Now().Add(24*time.Hour))
	t, _ = t.MarkExpired()
	return t
}

type SendActivationEmailSuite struct {
	suite.Suite
	ctx      context.Context
	obs      observability.Observability
	botNum   string
	tokenTTL time.Duration
}

func TestSendActivationEmailSuite(t *testing.T) {
	suite.Run(t, new(SendActivationEmailSuite))
}

func (s *SendActivationEmailSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.botNum = "+5511999999999"
	s.tokenTTL = 24 * time.Hour
}

func (s *SendActivationEmailSuite) TestExecute() {
	type args struct {
		input SendActivationEmailInput
	}
	type dependencies struct {
		template *stubActivationTemplate
		sender   *stubEmailSender
		repo     *stubMagicTokenRepository
	}

	paidRepo := &stubMagicTokenRepository{findByHashFn: func(_ context.Context, _ []byte) (entities.MagicToken, error) {
		return newPaidToken(), nil
	}}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(tmpl *stubActivationTemplate, err error)
	}{
		{
			name: "token e email validos: WaMeURL contem wa.me com ATIVAR pré-preenchido",
			args: args{input: SendActivationEmailInput{
				ClearToken:    validClearToken,
				CustomerEmail: "user@example.com",
			}},
			dependencies: dependencies{
				template: &stubActivationTemplate{},
				sender:   &stubEmailSender{},
				repo:     paidRepo,
			},
			expect: func(tmpl *stubActivationTemplate, err error) {
				s.NoError(err)
				s.Contains(tmpl.lastIn.WaMeURL, "wa.me/")
				s.Contains(tmpl.lastIn.WaMeURL, "?text=ATIVAR%20")
				s.Contains(tmpl.lastIn.WaMeURL, validClearToken)
			},
		},
		{
			name: "token e email validos: SupportURL sem query param",
			args: args{input: SendActivationEmailInput{
				ClearToken:    validClearToken,
				CustomerEmail: "user@example.com",
			}},
			dependencies: dependencies{
				template: &stubActivationTemplate{},
				sender:   &stubEmailSender{},
				repo:     paidRepo,
			},
			expect: func(tmpl *stubActivationTemplate, err error) {
				s.NoError(err)
				s.NotContains(tmpl.lastIn.SupportURL, "?text=")
				s.True(strings.HasPrefix(tmpl.lastIn.SupportURL, "https://wa.me/"))
			},
		},
		{
			name: "email vazio: retorna nil sem chamar template",
			args: args{input: SendActivationEmailInput{
				ClearToken:    validClearToken,
				CustomerEmail: "",
			}},
			dependencies: dependencies{
				template: &stubActivationTemplate{
					renderFn: func(in ActivationTemplateInput) (string, string, error) {
						panic("template nao deve ser chamado")
					},
				},
				sender: &stubEmailSender{},
				repo:   &stubMagicTokenRepository{},
			},
			expect: func(tmpl *stubActivationTemplate, err error) {
				s.NoError(err)
			},
		},
		{
			name: "token vazio: retorna erro sem chamar template",
			args: args{input: SendActivationEmailInput{
				ClearToken:    "",
				CustomerEmail: "user@example.com",
			}},
			dependencies: dependencies{
				template: &stubActivationTemplate{
					renderFn: func(in ActivationTemplateInput) (string, string, error) {
						panic("template nao deve ser chamado")
					},
				},
				sender: &stubEmailSender{},
				repo:   &stubMagicTokenRepository{},
			},
			expect: func(tmpl *stubActivationTemplate, err error) {
				s.Error(err)
			},
		},
		{
			name: "render falha: erro propagado com wrapping render template",
			args: args{input: SendActivationEmailInput{
				ClearToken:    validClearToken,
				CustomerEmail: "user@example.com",
			}},
			dependencies: dependencies{
				template: &stubActivationTemplate{
					renderFn: func(in ActivationTemplateInput) (string, string, error) {
						return "", "", errors.New("template broken")
					},
				},
				sender: &stubEmailSender{},
				repo:   paidRepo,
			},
			expect: func(tmpl *stubActivationTemplate, err error) {
				s.Error(err)
				s.Contains(err.Error(), "render template")
			},
		},
		{
			name: "send falha: retorna erro com wrapping send e incrementa contador send_failed",
			args: args{input: SendActivationEmailInput{
				ClearToken:    validClearToken,
				CustomerEmail: "user@example.com",
			}},
			dependencies: dependencies{
				template: &stubActivationTemplate{},
				sender: &stubEmailSender{
					sendFn: func(ctx context.Context, msg appinterfaces.EmailMessage) error {
						return errors.New("smtp error")
					},
				},
				repo: paidRepo,
			},
			expect: func(tmpl *stubActivationTemplate, err error) {
				s.Error(err)
				s.Contains(err.Error(), "send")
				fakeMetrics, ok := s.obs.Metrics().(*fake.FakeMetrics)
				s.Require().True(ok)
				counter := fakeMetrics.GetCounter("onboarding_activation_email_dispatched_total")
				s.NotNil(counter)
				found := false
				for _, v := range counter.GetValues() {
					for _, f := range v.Fields {
						if f.Key == "result" && f.StringValue() == "send_failed" {
							found = true
						}
					}
				}
				s.True(found, "esperado incremento do contador com result=send_failed")
			},
		},
		{
			name: "token consumed: skip silencioso sem enviar email",
			args: args{input: SendActivationEmailInput{
				ClearToken:    validClearToken,
				CustomerEmail: "user@example.com",
			}},
			dependencies: dependencies{
				template: &stubActivationTemplate{
					renderFn: func(in ActivationTemplateInput) (string, string, error) {
						panic("template nao deve ser chamado para token consumed")
					},
				},
				sender: &stubEmailSender{},
				repo: &stubMagicTokenRepository{findByHashFn: func(_ context.Context, _ []byte) (entities.MagicToken, error) {
					return newConsumedToken(), nil
				}},
			},
			expect: func(tmpl *stubActivationTemplate, err error) {
				s.NoError(err)
			},
		},
		{
			name: "token expired: skip silencioso sem enviar email",
			args: args{input: SendActivationEmailInput{
				ClearToken:    validClearToken,
				CustomerEmail: "user@example.com",
			}},
			dependencies: dependencies{
				template: &stubActivationTemplate{
					renderFn: func(in ActivationTemplateInput) (string, string, error) {
						panic("template nao deve ser chamado para token expired")
					},
				},
				sender: &stubEmailSender{},
				repo: &stubMagicTokenRepository{findByHashFn: func(_ context.Context, _ []byte) (entities.MagicToken, error) {
					return newExpiredToken(), nil
				}},
			},
			expect: func(tmpl *stubActivationTemplate, err error) {
				s.NoError(err)
			},
		},
		{
			name: "token not found: envia email normalmente",
			args: args{input: SendActivationEmailInput{
				ClearToken:    validClearToken,
				CustomerEmail: "user@example.com",
			}},
			dependencies: dependencies{
				template: &stubActivationTemplate{},
				sender:   &stubEmailSender{},
				repo:     &stubMagicTokenRepository{},
			},
			expect: func(tmpl *stubActivationTemplate, err error) {
				s.NoError(err)
			},
		},
		{
			name: "repo error inesperado: retorna erro wrapped",
			args: args{input: SendActivationEmailInput{
				ClearToken:    validClearToken,
				CustomerEmail: "user@example.com",
			}},
			dependencies: dependencies{
				template: &stubActivationTemplate{},
				sender:   &stubEmailSender{},
				repo: &stubMagicTokenRepository{findByHashFn: func(_ context.Context, _ []byte) (entities.MagicToken, error) {
					return entities.MagicToken{}, errors.New("db down")
				}},
			},
			expect: func(tmpl *stubActivationTemplate, err error) {
				s.Error(err)
				s.Contains(err.Error(), "find token")
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			uc := NewSendActivationEmail(
				scenario.dependencies.repo,
				scenario.dependencies.sender,
				scenario.dependencies.template,
				s.botNum,
				"noreply@mecontrola.app.br",
				"MeControla",
				"",
				s.tokenTTL,
				s.obs,
			)
			err := uc.Execute(s.ctx, scenario.args.input)
			scenario.expect(scenario.dependencies.template, err)
		})
	}
}
