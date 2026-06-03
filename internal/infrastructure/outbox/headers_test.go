package outbox_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/outbox"
)

type HeadersSuite struct {
	suite.Suite
}

func TestHeaders(t *testing.T) {
	suite.Run(t, new(HeadersSuite))
}

func (s *HeadersSuite) TestWithTrace_RetornaNovoMapa() {
	h := outbox.Headers{}
	h2 := h.WithTrace("00-trace-id-01")
	v, ok := h2.Get("traceparent")
	s.True(ok)
	s.Equal("00-trace-id-01", v)
	_, original := h.Get("traceparent")
	s.False(original, "WithTrace nao deve mutar o original")
}

func (s *HeadersSuite) TestGet_ChaveExistente() {
	h := outbox.Headers{"correlation_id": "corr-1"}
	v, ok := h.Get("correlation_id")
	s.True(ok)
	s.Equal("corr-1", v)
}

func (s *HeadersSuite) TestGet_ChaveAusente() {
	h := outbox.Headers{}
	_, ok := h.Get("correlation_id")
	s.False(ok)
}

func (s *HeadersSuite) TestValidate_ChavesCanonimas() {
	scenarios := []struct {
		name    string
		headers outbox.Headers
		wantErr bool
	}{
		{
			name:    "headers vazio e valido",
			headers: outbox.Headers{},
			wantErr: false,
		},
		{
			name:    "traceparent e valido",
			headers: outbox.Headers{"traceparent": "val"},
			wantErr: false,
		},
		{
			name:    "correlation_id e valido",
			headers: outbox.Headers{"correlation_id": "val"},
			wantErr: false,
		},
		{
			name:    "causation_id e valido",
			headers: outbox.Headers{"causation_id": "val"},
			wantErr: false,
		},
		{
			name:    "todas as canonicas sao validas",
			headers: outbox.Headers{"traceparent": "a", "correlation_id": "b", "causation_id": "c"},
			wantErr: false,
		},
		{
			name:    "chave desconhecida e invalida",
			headers: outbox.Headers{"custom_key": "val"},
			wantErr: true,
		},
		{
			name:    "mistura canonica e desconhecida e invalida",
			headers: outbox.Headers{"traceparent": "a", "unknown": "b"},
			wantErr: true,
		},
	}
	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			err := sc.headers.Validate()
			if sc.wantErr {
				s.Error(err)
			} else {
				s.NoError(err)
			}
		})
	}
}
