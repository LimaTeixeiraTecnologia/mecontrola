package valueobjects_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

type WebhookEventIDSuite struct {
	suite.Suite
}

func TestWebhookEventID(t *testing.T) {
	suite.Run(t, new(WebhookEventIDSuite))
}

func (s *WebhookEventIDSuite) TestValid() {
	cases := []struct {
		name  string
		input string
	}{
		{name: "uuid v4 minusculo", input: "550e8400-e29b-41d4-a716-446655440000"},
		{name: "uuid v4 maiusculo", input: "550E8400-E29B-41D4-A716-446655440000"},
		{name: "uuid v4 gerado", input: "f47ac10b-58cc-4372-a567-0e02b2c3d479"},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			got, err := valueobjects.NewWebhookEventID(tc.input)
			s.NoError(err)
			s.NotEmpty(got.String())
			s.False(got.IsZero())
		})
	}
}

func (s *WebhookEventIDSuite) TestInvalid() {
	cases := []struct {
		name  string
		input string
	}{
		{name: "vazio", input: ""},
		{name: "nao uuid", input: "not-a-uuid"},
		{name: "uuid v1", input: "6ba7b810-9dad-11d1-80b4-00c04fd430c8"},
		{name: "uuid v3", input: "a3bb189e-8bf9-3888-9912-ace4e6543002"},
		{name: "uuid v5", input: "74738ff5-5367-5958-9aee-98fffdcd1876"},
		{name: "ulid", input: "01ARZ3NDEKTSV4RRFFQ69G5FAV"},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			_, err := valueobjects.NewWebhookEventID(tc.input)
			s.True(errors.Is(err, valueobjects.ErrInvalidWebhookEventID))
		})
	}
}

func (s *WebhookEventIDSuite) TestIsZero() {
	var w valueobjects.WebhookEventID
	s.True(w.IsZero())
}
