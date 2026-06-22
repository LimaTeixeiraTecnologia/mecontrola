package status_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/status"
)

type ParserSuite struct {
	suite.Suite
}

func TestParserSuite(t *testing.T) {
	suite.Run(t, new(ParserSuite))
}

func (s *ParserSuite) TestExtractStatuses() {
	scenarios := []struct {
		name      string
		raw       string
		expectLen int
		assert    func([]status.MessageStatus)
		expectErr bool
	}{
		{
			name:      "status delivered valido",
			raw:       `{"entry":[{"id":"e1","changes":[{"field":"messages","value":{"messaging_product":"whatsapp","statuses":[{"id":"wamid-1","status":"delivered","timestamp":"1686000000","recipient_id":"5511999999999"}]}}]}]}`,
			expectLen: 1,
			assert: func(out []status.MessageStatus) {
				s.Equal("wamid-1", out[0].MessageID)
				s.Equal("delivered", out[0].Status)
				s.Equal("5511999999999", out[0].RecipientID)
				s.Equal("1686000000", out[0].Timestamp)
				s.Empty(out[0].ErrorCode)
			},
		},
		{
			name:      "status failed com erro",
			raw:       `{"entry":[{"id":"e1","changes":[{"field":"messages","value":{"statuses":[{"id":"wamid-2","status":"failed","timestamp":"1686000001","recipient_id":"5511888888888","errors":[{"code":131026,"title":"Message undeliverable"}]}]}}]}]}`,
			expectLen: 1,
			assert: func(out []status.MessageStatus) {
				s.Equal("failed", out[0].Status)
				s.Equal("131026", out[0].ErrorCode)
				s.Equal("Message undeliverable", out[0].ErrorTitle)
			},
		},
		{
			name:      "multiplos statuses",
			raw:       `{"entry":[{"id":"e1","changes":[{"field":"messages","value":{"statuses":[{"id":"w1","status":"sent","timestamp":"1"},{"id":"w2","status":"read","timestamp":"2"}]}}]}]}`,
			expectLen: 2,
		},
		{
			name:      "ignora status sem id ou sem status",
			raw:       `{"entry":[{"id":"e1","changes":[{"field":"messages","value":{"statuses":[{"id":"","status":"sent"},{"id":"w3","status":""}]}}]}]}`,
			expectLen: 0,
		},
		{
			name:      "payload sem statuses",
			raw:       `{"entry":[{"id":"e1","changes":[{"field":"messages","value":{"messages":[{"id":"m1"}]}}]}]}`,
			expectLen: 0,
		},
		{
			name:      "entry vazio",
			raw:       `{"entry":[]}`,
			expectLen: 0,
		},
		{
			name:      "json invalido",
			raw:       `not-json`,
			expectErr: true,
		},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			out, err := status.ExtractStatuses([]byte(sc.raw))
			if sc.expectErr {
				s.Require().Error(err)
				return
			}
			s.Require().NoError(err)
			s.Len(out, sc.expectLen)
			if sc.assert != nil {
				sc.assert(out)
			}
		})
	}
}
