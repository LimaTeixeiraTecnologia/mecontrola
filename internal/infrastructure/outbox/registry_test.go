package outbox_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/outbox"
)

// RegistrySuite cobre a interface Registry e sua implementação staticRegistry.
type RegistrySuite struct {
	suite.Suite
}

func TestRegistry(t *testing.T) {
	suite.Run(t, new(RegistrySuite))
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func (s *RegistrySuite) mustSubscriptionName(raw string) outbox.SubscriptionName {
	n, err := outbox.NewSubscriptionName(raw)
	s.Require().NoError(err)
	return n
}

func (s *RegistrySuite) mustEventName(raw string) events.EventName {
	n, err := events.NewEventName(raw)
	s.Require().NoError(err)
	return n
}

func (s *RegistrySuite) dummyHandler() outbox.Handler {
	return func(_ context.Context, _ outbox.Event) error { return nil }
}

// ─── Register: Cenário 1 — duplicidade detectada ───────────────────────────

func (s *RegistrySuite) TestRegister_DuplicateSubscription_ReturnsError() {
	scenarios := []struct {
		name          string
		registerTwice outbox.Subscription
	}{
		{
			name: "mesmo name e mesmo event_type registrados duas vezes",
			registerTwice: outbox.Subscription{
				Name:      s.mustSubscriptionName("notif-email"),
				EventType: s.mustEventName("identity.user-created"),
				Handler:   s.dummyHandler(),
			},
		},
		{
			name: "outro par duplicado com diferentes valores",
			registerTwice: outbox.Subscription{
				Name:      s.mustSubscriptionName("finance-settled"),
				EventType: s.mustEventName("finance.payment-received"),
				Handler:   s.dummyHandler(),
			},
		},
	}

	for _, tc := range scenarios {
		s.Run(tc.name, func() {
			reg := outbox.NewRegistry()

			err := reg.Register(tc.registerTwice)
			s.Require().NoError(err, "primeiro Register deve ter sucesso")

			err = reg.Register(tc.registerTwice)
			s.Require().Error(err)
			s.True(errors.Is(err, outbox.ErrDuplicateSubscription),
				"deve retornar ErrDuplicateSubscription via errors.Is")
		})
	}
}

// ─── Register: Cenário 1b — duplicidade em ordem diferente ────────────────

func (s *RegistrySuite) TestRegister_DuplicateSubscription_DifferentOrderStillDetected() {
	reg := outbox.NewRegistry()

	subA := outbox.Subscription{
		Name:      s.mustSubscriptionName("handler-alpha"),
		EventType: s.mustEventName("identity.user-updated"),
		Handler:   s.dummyHandler(),
	}
	subB := outbox.Subscription{
		Name:      s.mustSubscriptionName("handler-beta"),
		EventType: s.mustEventName("identity.user-updated"),
		Handler:   s.dummyHandler(),
	}

	s.Require().NoError(reg.Register(subA))
	s.Require().NoError(reg.Register(subB))

	// Repetir subA depois de subB deve falhar.
	err := reg.Register(subA)
	s.Require().Error(err)
	s.True(errors.Is(err, outbox.ErrDuplicateSubscription))
}

// ─── Register: Cenário 2 — mesmo Name, EventType diferente é permitido ────

func (s *RegistrySuite) TestRegister_SameNameDifferentEventType_IsAllowed() {
	scenarios := []struct {
		name    string
		nameRaw string
		typeA   string
		typeB   string
	}{
		{
			name:    "handler reutilizado em dois event_types distintos",
			nameRaw: "notif-email",
			typeA:   "identity.user-created",
			typeB:   "identity.user-updated",
		},
		{
			name:    "outro handler reutilizado em dois modulos distintos",
			nameRaw: "finance-alert",
			typeA:   "finance.payment-received",
			typeB:   "identity.user-deleted",
		},
	}

	for _, tc := range scenarios {
		s.Run(tc.name, func() {
			reg := outbox.NewRegistry()

			subA := outbox.Subscription{
				Name:      s.mustSubscriptionName(tc.nameRaw),
				EventType: s.mustEventName(tc.typeA),
				Handler:   s.dummyHandler(),
			}
			subB := outbox.Subscription{
				Name:      s.mustSubscriptionName(tc.nameRaw),
				EventType: s.mustEventName(tc.typeB),
				Handler:   s.dummyHandler(),
			}

			s.Require().NoError(reg.Register(subA), "subA deve ser aceito")
			s.Require().NoError(reg.Register(subB), "subB com mesmo Name mas EventType diferente deve ser aceito (D-09)")
		})
	}
}

// ─── Register: cardinalidade 1×N (RF-08) ────────────────────────────────────

func (s *RegistrySuite) TestRegister_MultipleHandlersForSameEventType_AllRegistered() {
	reg := outbox.NewRegistry()

	eventType := s.mustEventName("finance.payment-received")

	subs := []outbox.Subscription{
		{Name: s.mustSubscriptionName("handler-one"), EventType: eventType, Handler: s.dummyHandler()},
		{Name: s.mustSubscriptionName("handler-two"), EventType: eventType, Handler: s.dummyHandler()},
		{Name: s.mustSubscriptionName("handler-tri"), EventType: eventType, Handler: s.dummyHandler()},
	}

	for _, sub := range subs {
		s.Require().NoError(reg.Register(sub))
	}

	result := reg.SubscriptionsFor(eventType)
	s.Len(result, 3, "todos os 3 handlers devem estar registrados para o mesmo event_type")
}

// ─── SubscriptionsFor: Cenário 3 — event_type desconhecido retorna nil ─────

func (s *RegistrySuite) TestSubscriptionsFor_UnknownEventType_ReturnsNil() {
	scenarios := []struct {
		name      string
		eventType string
	}{
		{
			name:      "event_type nunca registrado",
			eventType: "identity.user-deleted",
		},
		{
			name:      "event_type de outro modulo",
			eventType: "finance.payment-received",
		},
	}

	for _, tc := range scenarios {
		s.Run(tc.name, func() {
			reg := outbox.NewRegistry()
			// Registrar apenas um event_type diferente para garantir registry não-vazio.
			s.Require().NoError(reg.Register(outbox.Subscription{
				Name:      s.mustSubscriptionName("notif-email"),
				EventType: s.mustEventName("identity.user-created"),
				Handler:   s.dummyHandler(),
			}))

			result := reg.SubscriptionsFor(s.mustEventName(tc.eventType))
			s.Nil(result, "SubscriptionsFor de event_type desconhecido deve retornar nil")
		})
	}
}

// ─── SubscriptionsFor: Cenário 4 — cópia defensiva ──────────────────────────

func (s *RegistrySuite) TestSubscriptionsFor_ReturnedSlice_IsDefensiveCopy() {
	reg := outbox.NewRegistry()
	eventType := s.mustEventName("identity.user-created")

	originalSub := outbox.Subscription{
		Name:      s.mustSubscriptionName("notif-email"),
		EventType: eventType,
		Handler:   s.dummyHandler(),
	}
	s.Require().NoError(reg.Register(originalSub))

	// Primeira chamada — obtém cópia e corrompe o slice.
	first := reg.SubscriptionsFor(eventType)
	s.Require().Len(first, 1)
	// Truncar o slice retornado não deve afetar o estado interno.
	first = first[:0]
	_ = first

	// Segunda chamada deve continuar retornando 1 subscription.
	second := reg.SubscriptionsFor(eventType)
	s.Len(second, 1,
		"mutacao do slice retornado nao deve corromper o estado interno do Registry")
}

// ─── Validate: Cenário 5 — Handler nil causa falha ──────────────────────────

func (s *RegistrySuite) TestValidate_NilHandler_ReturnsError() {
	scenarios := []struct {
		name string
		sub  outbox.Subscription
	}{
		{
			name: "handler nil explicitamente",
			sub: outbox.Subscription{
				Name:      s.mustSubscriptionName("notif-email"),
				EventType: s.mustEventName("identity.user-created"),
				Handler:   nil,
			},
		},
		{
			name: "handler nil em segundo elemento",
			sub: outbox.Subscription{
				Name:      s.mustSubscriptionName("finance-alert"),
				EventType: s.mustEventName("finance.payment-received"),
				Handler:   nil,
			},
		},
	}

	for _, tc := range scenarios {
		s.Run(tc.name, func() {
			reg := outbox.NewRegistry()
			s.Require().NoError(reg.Register(tc.sub))

			err := reg.Validate()
			s.Require().Error(err)
			s.Contains(err.Error(), "Handler nil",
				"mensagem de erro deve identificar que o Handler e nil")
		})
	}
}

// ─── Validate: registry válido com múltiplos handlers ───────────────────────

func (s *RegistrySuite) TestValidate_ValidRegistry_ReturnsNil() {
	reg := outbox.NewRegistry()

	subs := []outbox.Subscription{
		{
			Name:      s.mustSubscriptionName("handler-one"),
			EventType: s.mustEventName("identity.user-created"),
			Handler:   s.dummyHandler(),
		},
		{
			Name:      s.mustSubscriptionName("handler-two"),
			EventType: s.mustEventName("identity.user-created"),
			Handler:   s.dummyHandler(),
		},
		{
			Name:      s.mustSubscriptionName("finance-alert"),
			EventType: s.mustEventName("finance.payment-received"),
			Handler:   s.dummyHandler(),
		},
	}

	for _, sub := range subs {
		s.Require().NoError(reg.Register(sub))
	}

	s.Require().NoError(reg.Validate(), "registry totalmente valido deve passar em Validate")
}

// ─── Validate: registry vazio é válido ──────────────────────────────────────

func (s *RegistrySuite) TestValidate_EmptyRegistry_ReturnsNil() {
	reg := outbox.NewRegistry()
	s.Require().NoError(reg.Validate(), "registry vazio deve passar em Validate")
}
