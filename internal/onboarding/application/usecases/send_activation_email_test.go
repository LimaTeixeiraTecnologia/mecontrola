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
)

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
	}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(tmpl *stubActivationTemplate, err error)
	}{
		{
			name: "token e email validos: WaMeURL contem wa.me com ATIVAR pré-preenchido",
			args: args{input: SendActivationEmailInput{
				ClearToken:    "ABC123",
				CustomerEmail: "user@example.com",
			}},
			dependencies: func() dependencies {
				return dependencies{
					template: &stubActivationTemplate{},
					sender:   &stubEmailSender{},
				}
			}(),
			expect: func(tmpl *stubActivationTemplate, err error) {
				s.NoError(err)
				s.Contains(tmpl.lastIn.WaMeURL, "wa.me/")
				s.Contains(tmpl.lastIn.WaMeURL, "?text=ATIVAR%20")
				s.Contains(tmpl.lastIn.WaMeURL, "ABC123")
			},
		},
		{
			name: "token e email validos: SupportURL sem query param",
			args: args{input: SendActivationEmailInput{
				ClearToken:    "ABC123",
				CustomerEmail: "user@example.com",
			}},
			dependencies: func() dependencies {
				return dependencies{
					template: &stubActivationTemplate{},
					sender:   &stubEmailSender{},
				}
			}(),
			expect: func(tmpl *stubActivationTemplate, err error) {
				s.NoError(err)
				s.NotContains(tmpl.lastIn.SupportURL, "?text=")
				s.True(strings.HasPrefix(tmpl.lastIn.SupportURL, "https://wa.me/"))
			},
		},
		{
			name: "email vazio: retorna nil sem chamar template",
			args: args{input: SendActivationEmailInput{
				ClearToken:    "ABC123",
				CustomerEmail: "",
			}},
			dependencies: func() dependencies {
				tmpl := &stubActivationTemplate{
					renderFn: func(in ActivationTemplateInput) (string, string, error) {
						panic("template nao deve ser chamado")
					},
				}
				return dependencies{
					template: tmpl,
					sender:   &stubEmailSender{},
				}
			}(),
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
			dependencies: func() dependencies {
				tmpl := &stubActivationTemplate{
					renderFn: func(in ActivationTemplateInput) (string, string, error) {
						panic("template nao deve ser chamado")
					},
				}
				return dependencies{
					template: tmpl,
					sender:   &stubEmailSender{},
				}
			}(),
			expect: func(tmpl *stubActivationTemplate, err error) {
				s.Error(err)
			},
		},
		{
			name: "render falha: erro propagado com wrapping render template",
			args: args{input: SendActivationEmailInput{
				ClearToken:    "ABC123",
				CustomerEmail: "user@example.com",
			}},
			dependencies: func() dependencies {
				tmpl := &stubActivationTemplate{
					renderFn: func(in ActivationTemplateInput) (string, string, error) {
						return "", "", errors.New("template broken")
					},
				}
				return dependencies{
					template: tmpl,
					sender:   &stubEmailSender{},
				}
			}(),
			expect: func(tmpl *stubActivationTemplate, err error) {
				s.Error(err)
				s.Contains(err.Error(), "render template")
			},
		},
		{
			name: "send falha: retorna erro com wrapping send",
			args: args{input: SendActivationEmailInput{
				ClearToken:    "ABC123",
				CustomerEmail: "user@example.com",
			}},
			dependencies: func() dependencies {
				return dependencies{
					template: &stubActivationTemplate{},
					sender: &stubEmailSender{
						sendFn: func(ctx context.Context, msg appinterfaces.EmailMessage) error {
							return errors.New("smtp error")
						},
					},
				}
			}(),
			expect: func(tmpl *stubActivationTemplate, err error) {
				s.Error(err)
				s.Contains(err.Error(), "send")
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			uc := NewSendActivationEmail(
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
