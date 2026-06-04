package valueobjects_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

type ExternalEventIDSuite struct {
	suite.Suite
}

func TestExternalEventID(t *testing.T) {
	suite.Run(t, new(ExternalEventIDSuite))
}

func (s *ExternalEventIDSuite) TestCascade() {
	cases := []struct {
		name        string
		input       []byte
		expected    string
		expectedErr error
		checkPrefix string
	}{
		{
			name:     "id presente no root",
			input:    []byte(`{"id":"abc"}`),
			expected: "abc",
		},
		{
			name:     "order.id presente",
			input:    []byte(`{"order":{"id":"xyz"}}`),
			expected: "xyz",
		},
		{
			name:        "fallback para sha256",
			input:       []byte(`{}`),
			checkPrefix: "sha256:",
		},
		{
			name:        "json inválido retorna erro",
			input:       []byte(`not-json`),
			expectedErr: valueobjects.ErrMalformedPayload,
		},
		{
			name:        "empty payload retorna erro",
			input:       []byte{},
			expectedErr: valueobjects.ErrEmptyPayload,
		},
		{
			name:        "nil payload retorna erro",
			input:       nil,
			expectedErr: valueobjects.ErrEmptyPayload,
		},
		{
			name:     "id com espaco em branco e extraido com trim",
			input:    []byte(`{"id":"  abc  "}`),
			expected: "abc",
		},
		{
			name:     "id vazio cai para order.id",
			input:    []byte(`{"id":"","order":{"id":"fallback"}}`),
			expected: "fallback",
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			got, err := valueobjects.NewExternalEventIDCascade(tc.input)
			if tc.expectedErr != nil {
				s.True(errors.Is(err, tc.expectedErr), "esperado %v, got %v", tc.expectedErr, err)
				return
			}
			s.NoError(err)
			if tc.checkPrefix != "" {
				s.True(strings.HasPrefix(got.String(), tc.checkPrefix),
					"esperado prefixo %q, got %q", tc.checkPrefix, got.String())
				hexPart := strings.TrimPrefix(got.String(), tc.checkPrefix)
				s.Len(hexPart, 64, "sha256 hex deve ter 64 caracteres")
			} else {
				s.Equal(tc.expected, got.String())
			}
		})
	}
}

func (s *ExternalEventIDSuite) TestIsZero() {
	var e valueobjects.ExternalEventID
	s.True(e.IsZero())

	got, err := valueobjects.NewExternalEventIDCascade([]byte(`{"id":"x"}`))
	s.NoError(err)
	s.False(got.IsZero())
}
