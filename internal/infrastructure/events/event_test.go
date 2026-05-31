package events_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/events"
)

// EventSuite testa os value objects de Event (EventID, EventName, ModuleName).
type EventSuite struct {
	suite.Suite
}

func TestEvent(t *testing.T) {
	suite.Run(t, new(EventSuite))
}

func (s *EventSuite) TestEventID() {
	scenarios := []struct {
		name    string
		input   string
		wantErr bool
		expect  func(id events.EventID, err error)
	}{
		{
			name:    "deve criar EventID com valor válido",
			input:   "01ARZ3NDEKTSV4RRFFQ69G5FAV",
			wantErr: false,
			expect: func(id events.EventID, err error) {
				s.NoError(err)
				s.Equal("01ARZ3NDEKTSV4RRFFQ69G5FAV", id.String())
			},
		},
		{
			name:    "deve retornar erro com valor vazio",
			input:   "",
			wantErr: true,
			expect: func(_ events.EventID, err error) {
				s.Error(err)
			},
		},
		{
			name:    "deve retornar erro com valor apenas espaços em branco",
			input:   "   ",
			wantErr: true,
			expect: func(_ events.EventID, err error) {
				s.Error(err)
			},
		},
	}
	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			id, err := events.NewEventID(sc.input)
			sc.expect(id, err)
		})
	}
}

func (s *EventSuite) TestEventName() {
	scenarios := []struct {
		name    string
		input   string
		wantErr bool
		want    string
		errMsg  string
	}{
		{
			name:  "deve criar EventName com identity.user-created",
			input: "identity.user-created",
			want:  "identity.user-created",
		},
		{
			name:  "deve criar EventName com conversation.message-received",
			input: "conversation.message-received",
			want:  "conversation.message-received",
		},
		{
			name:  "deve criar EventName com finance.payment-charged",
			input: "finance.payment-charged",
			want:  "finance.payment-charged",
		},
		{
			name:    "deve retornar erro com valor vazio",
			input:   "",
			wantErr: true,
		},
		{
			name:    "deve retornar erro sem separador ponto",
			input:   "usercreated",
			wantErr: true,
			errMsg:  "formato",
		},
	}
	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			n, err := events.NewEventName(sc.input)
			if sc.wantErr {
				s.Error(err)
				if sc.errMsg != "" {
					s.Contains(err.Error(), sc.errMsg)
				}
				return
			}
			s.Require().NoError(err)
			s.Equal(sc.want, n.String())
		})
	}
}

func (s *EventSuite) TestModuleName() {
	scenarios := []struct {
		name    string
		input   string
		wantErr bool
		want    string
	}{
		{name: "deve criar ModuleName identity", input: "identity", want: "identity"},
		{name: "deve criar ModuleName conversation", input: "conversation", want: "conversation"},
		{name: "deve criar ModuleName agent", input: "agent", want: "agent"},
		{name: "deve criar ModuleName finance", input: "finance", want: "finance"},
		{name: "deve criar ModuleName notifications", input: "notifications", want: "notifications"},
		{name: "deve criar ModuleName telemetry", input: "telemetry", want: "telemetry"},
		{name: "deve retornar erro com valor vazio", input: "", wantErr: true},
		{name: "deve retornar erro com módulo desconhecido", input: "billing", wantErr: true},
		{name: "deve retornar erro com módulo em maiúscula", input: "Identity", wantErr: true},
		{name: "deve retornar erro com nome parcial", input: "ident", wantErr: true},
	}
	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			m, err := events.NewModuleName(sc.input)
			if sc.wantErr {
				s.Error(err)
				return
			}
			s.Require().NoError(err)
			s.Equal(sc.want, m.String())
		})
	}
}
