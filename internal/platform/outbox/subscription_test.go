package outbox_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type SubscriptionNameSuite struct {
	suite.Suite
}

func TestSubscriptionName(t *testing.T) {
	suite.Run(t, new(SubscriptionNameSuite))
}

func (s *SubscriptionNameSuite) TestNewSubscriptionName() {
	scenarios := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "nome valido minimo (3 chars)",
			input:   "abc",
			wantErr: false,
		},
		{
			name:    "nome valido com hifen",
			input:   "notif-email",
			wantErr: false,
		},
		{
			name:    "nome valido com underscore",
			input:   "finance_settled",
			wantErr: false,
		},
		{
			name:    "nome valido com numeros",
			input:   "handler123",
			wantErr: false,
		},
		{
			name:    "nome valido 64 chars",
			input:   "abcdefghij01234567890123456789012345678901234567890123456789012a",
			wantErr: false,
		},
		{
			name:    "nome comecando com numero e invalido",
			input:   "1notif",
			wantErr: true,
		},
		{
			name:    "nome muito curto (2 chars) e invalido",
			input:   "ab",
			wantErr: true,
		},
		{
			name:    "nome muito longo (65+ chars) e invalido",
			input:   "abcdefghij012345678901234567890123456789012345678901234567890123456",
			wantErr: true,
		},
		{
			name:    "nome com letra maiuscula e invalido",
			input:   "NotifEmail",
			wantErr: true,
		},
		{
			name:    "nome vazio e invalido",
			input:   "",
			wantErr: true,
		},
	}
	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			sn, err := outbox.NewSubscriptionName(sc.input)
			if sc.wantErr {
				s.Error(err)
				s.Equal("", sn.String())
			} else {
				s.NoError(err)
				s.Equal(sc.input, sn.String())
			}
		})
	}
}
