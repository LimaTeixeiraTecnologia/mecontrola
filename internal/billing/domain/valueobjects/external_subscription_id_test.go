package valueobjects_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

type ExternalSubscriptionIDSuite struct {
	suite.Suite
}

func TestExternalSubscriptionID(t *testing.T) {
	suite.Run(t, new(ExternalSubscriptionIDSuite))
}

func (s *ExternalSubscriptionIDSuite) TestValid() {
	cases := []struct {
		name  string
		input string
	}{
		{name: "id simples", input: "sub-123"},
		{name: "uuid", input: "550e8400-e29b-41d4-a716-446655440000"},
		{name: "alfanumerico", input: "KIWIFY_SUB_ABC123"},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			got, err := valueobjects.NewExternalSubscriptionID(tc.input)
			s.NoError(err)
			s.Equal(tc.input, got.String())
			s.False(got.IsZero())
		})
	}
}

func (s *ExternalSubscriptionIDSuite) TestInvalid() {
	cases := []struct {
		name        string
		input       string
		expectedErr error
	}{
		{name: "vazio", input: "", expectedErr: valueobjects.ErrEmptyExternalSubscriptionID},
		{name: "apenas espacos", input: "   ", expectedErr: valueobjects.ErrEmptyExternalSubscriptionID},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			_, err := valueobjects.NewExternalSubscriptionID(tc.input)
			s.True(errors.Is(err, tc.expectedErr))
		})
	}
}

func (s *ExternalSubscriptionIDSuite) TestIsZero() {
	var e valueobjects.ExternalSubscriptionID
	s.True(e.IsZero())
}
