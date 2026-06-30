package config_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/config"
)

type OnboardingRuntimeConfigSuite struct {
	suite.Suite
}

func TestOnboardingRuntimeConfigSuite(t *testing.T) {
	suite.Run(t, new(OnboardingRuntimeConfigSuite))
}

func (s *OnboardingRuntimeConfigSuite) TestNewOnboardingRuntimeConfig() {
	type tc struct {
		name      string
		cfg       configs.OnboardingConfig
		waCfg     configs.WhatsAppConfig
		expectErr string
		assertOK  func(rc config.OnboardingRuntimeConfig)
	}

	cases := []tc{
		{
			name: "parsing válido",
			cfg: configs.OnboardingConfig{
				KiwifyCheckoutURLs:  "monthly=https://a\nquarterly=https://b",
				KiwifyAllowedHosts:  "host1.com, host2.com",
				TrustedProxies:      "10.0.0.1,10.0.0.2",
				CheckoutCORSOrigins: "https://x.com,https://y.com",
				TokenTTLDays:        7,
				OutreachGapHours:    24,
				MetaRetentionDays:   30,
			},
			waCfg: configs.WhatsAppConfig{
				WelcomeActivated:  "wa",
				AlreadyActive:     "aa",
				CodeAlreadyUsed:   "cau",
				PaymentProcessing: "pp",
				CodeExpired:       "ce",
				CodeInvalid:       "ci",
				SystemUnavailable: "su",
				InvalidCountry:    "ic",
				OnboardingIntro:   "intro",
			},
			assertOK: func(rc config.OnboardingRuntimeConfig) {
				s.Require().Len(rc.CheckoutURLs, 2)
				s.Equal("https://a", rc.CheckoutURLs["monthly"])
				s.Equal("https://b", rc.CheckoutURLs["quarterly"])
				s.Equal([]string{"host1.com", "host2.com"}, rc.KiwifyAllowedHosts)
				s.Equal([]string{"10.0.0.1", "10.0.0.2"}, rc.TrustedProxies)
				s.Equal([]string{"https://x.com", "https://y.com"}, rc.CheckoutCORSOrigins)
				s.Equal(7*24*time.Hour, rc.TokenTTL)
				s.Equal(24*time.Hour, rc.OutreachGap)
				s.Equal(30*24*time.Hour, rc.MetaRetention)
				s.Equal("wa", rc.Messages["welcome_activated"])
				s.Equal("ic", rc.Messages["invalid_country"])
				s.Equal("intro", rc.Messages["onboarding_intro"])
				s.Len(rc.Messages, 9)
			},
		},
		{
			name: "linha vazia e linha sem igual ignoradas",
			cfg: configs.OnboardingConfig{
				KiwifyCheckoutURLs: "\nmonthly=https://a\nlixo-sem-igual\n  \n",
			},
			assertOK: func(rc config.OnboardingRuntimeConfig) {
				s.Require().Len(rc.CheckoutURLs, 1)
				s.Equal("https://a", rc.CheckoutURLs["monthly"])
			},
		},
		{
			name: "chave duplicada",
			cfg: configs.OnboardingConfig{
				KiwifyCheckoutURLs: "monthly=https://a\nmonthly=https://b",
			},
			expectErr: "chave duplicada",
		},
		{
			name: "separador ; aceito (env-friendly)",
			cfg: configs.OnboardingConfig{
				KiwifyCheckoutURLs: "monthly=https://a;quarterly=https://b;annual=https://c",
			},
			assertOK: func(rc config.OnboardingRuntimeConfig) {
				s.Require().Len(rc.CheckoutURLs, 3)
				s.Equal("https://a", rc.CheckoutURLs["monthly"])
				s.Equal("https://b", rc.CheckoutURLs["quarterly"])
				s.Equal("https://c", rc.CheckoutURLs["annual"])
			},
		},
		{
			name: "separadores ; e \\n misturados",
			cfg: configs.OnboardingConfig{
				KiwifyCheckoutURLs: "monthly=https://a;quarterly=https://b\nannual=https://c",
			},
			assertOK: func(rc config.OnboardingRuntimeConfig) {
				s.Require().Len(rc.CheckoutURLs, 3)
				s.Equal("https://a", rc.CheckoutURLs["monthly"])
				s.Equal("https://b", rc.CheckoutURLs["quarterly"])
				s.Equal("https://c", rc.CheckoutURLs["annual"])
			},
		},
		{
			name: "string vazia é tolerada (modulo pode rodar sem checkout)",
			cfg: configs.OnboardingConfig{
				KiwifyCheckoutURLs: "",
			},
			assertOK: func(rc config.OnboardingRuntimeConfig) {
				s.Require().NotNil(rc.CheckoutURLs)
				s.Empty(rc.CheckoutURLs)
			},
		},
		{
			name: "conteúdo presente sem entrada válida produz erro",
			cfg: configs.OnboardingConfig{
				KiwifyCheckoutURLs: "lixo-sem-igual\n\n   \n",
			},
			expectErr: "sem entradas válidas",
		},
	}

	for _, c := range cases {
		s.Run(c.name, func() {
			rc, err := config.NewOnboardingRuntimeConfig(c.cfg, c.waCfg)
			if c.expectErr != "" {
				s.Require().Error(err)
				s.Contains(err.Error(), c.expectErr)
				return
			}
			s.Require().NoError(err)
			c.assertOK(rc)
		})
	}
}

func (s *OnboardingRuntimeConfigSuite) TestCSVEmptyProducesNil() {
	type tc struct {
		name  string
		field string
	}

	cases := []tc{
		{name: "KiwifyAllowedHosts vazio", field: "hosts"},
		{name: "TrustedProxies vazio", field: "proxies"},
		{name: "CheckoutCORSOrigins vazio", field: "origins"},
	}

	for _, c := range cases {
		s.Run(c.name, func() {
			cfg := configs.OnboardingConfig{
				KiwifyCheckoutURLs:  "monthly=https://a",
				KiwifyAllowedHosts:  "",
				TrustedProxies:      "",
				CheckoutCORSOrigins: "",
			}
			rc, err := config.NewOnboardingRuntimeConfig(cfg, configs.WhatsAppConfig{})
			s.Require().NoError(err)
			s.Nil(rc.KiwifyAllowedHosts)
			s.Nil(rc.TrustedProxies)
			s.Nil(rc.CheckoutCORSOrigins)
		})
	}
}
