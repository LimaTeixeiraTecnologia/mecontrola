package kiwify_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/http/client/kiwify"
)

// testDivergenceCounter é um counter de divergência de periodo para testes.
type testDivergenceCounter struct {
	calls []struct{ plan, sign string }
}

func (c *testDivergenceCounter) Add(plan, sign string) {
	c.calls = append(c.calls, struct{ plan, sign string }{plan, sign})
}

type PayloadMapperSuite struct {
	suite.Suite
	registry *kiwify.BillingPlansRegistry
	now      time.Time
}

func TestPayloadMapper(t *testing.T) {
	suite.Run(t, new(PayloadMapperSuite))
}

func (s *PayloadMapperSuite) SetupSuite() {
	s.now = time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	s.registry = kiwify.NewBillingPlansRegistryFromMap(map[string]valueobjects.PlanCode{
		"prod-monthly":   valueobjects.PlanCodeMonthly,
		"prod-quarterly": valueobjects.PlanCodeQuarterly,
		"prod-annual":    valueobjects.PlanCodeAnnual,
	})
}

func (s *PayloadMapperSuite) buildPayload(overrides map[string]any) []byte {
	base := map[string]any{
		"id":                 "event-001",
		"webhook_event_type": "compra_aprovada",
		"updated_at":         s.now.Format(time.RFC3339),
		"customer": map[string]any{
			"mobile": "11999990000",
			"email":  "test@example.com",
		},
		"product": map[string]any{"id": "prod-monthly"},
		"subscription": map[string]any{
			"id":                   "sub-001",
			"current_period_start": s.now.Format(time.RFC3339),
			"current_period_end":   s.now.Add(30 * 24 * time.Hour).Format(time.RFC3339),
		},
		"refund":   map[string]any{"amount_cents": 0},
		"tracking": map[string]any{},
	}
	for k, v := range overrides {
		base[k] = v
	}
	raw, _ := json.Marshal(base)
	return raw
}

func (s *PayloadMapperSuite) TestAllEventTypes() {
	mapper := kiwify.NewPayloadMapper(s.registry, nil)

	type testCase struct {
		name      string
		eventType string
		wantType  valueobjects.CanonicalEventType
	}

	cases := []testCase{
		{"compra_aprovada", "compra_aprovada", valueobjects.CanonicalEventPurchaseApproved},
		{"subscription_renewed", "subscription_renewed", valueobjects.CanonicalEventRenewed},
		{"subscription_late", "subscription_late", valueobjects.CanonicalEventLate},
		{"subscription_canceled", "subscription_canceled", valueobjects.CanonicalEventCanceled},
		{"compra_reembolsada", "compra_reembolsada", valueobjects.CanonicalEventRefunded},
		{"chargeback", "chargeback", valueobjects.CanonicalEventChargeback},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			raw := s.buildPayload(map[string]any{"webhook_event_type": tc.eventType})
			event, err := mapper.Parse(raw)
			s.Require().NoError(err)
			s.Equal(tc.wantType, event.Type)
		})
	}
}

func (s *PayloadMapperSuite) TestUnknownEventType() {
	mapper := kiwify.NewPayloadMapper(s.registry, nil)
	raw := s.buildPayload(map[string]any{"webhook_event_type": "unknown_type"})
	_, err := mapper.Parse(raw)
	s.Require().Error(err)
	s.ErrorIs(err, kiwify.ErrUnknownKiwifyEventType)
}

func (s *PayloadMapperSuite) TestTrackingCascade() {
	mapper := kiwify.NewPayloadMapper(s.registry, nil)

	type testCase struct {
		name     string
		tracking map[string]any
		wantSrc  string
	}

	cases := []testCase{
		{
			name:     "tracking.src vence sobre utm_content quando ambos presentes",
			tracking: map[string]any{"src": "source-value", "utm_content": "utm-value"},
			wantSrc:  "source-value",
		},
		{
			name:     "tracking.utm_content quando src ausente",
			tracking: map[string]any{"src": "", "utm_content": "utm-value"},
			wantSrc:  "utm-value",
		},
		{
			name:     "tracking.s1 quando src e utm_content ausentes",
			tracking: map[string]any{"s1": "s1-value"},
			wantSrc:  "s1-value",
		},
		{
			name:     "tracking.s2 quando src, utm_content e s1 ausentes",
			tracking: map[string]any{"s2": "s2-value"},
			wantSrc:  "s2-value",
		},
		{
			name:     "tracking.s3 como último fallback",
			tracking: map[string]any{"s3": "s3-value"},
			wantSrc:  "s3-value",
		},
		{
			name:     "signup token vazio quando nenhum campo presente",
			tracking: map[string]any{},
			wantSrc:  "",
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			raw := s.buildPayload(map[string]any{"tracking": tc.tracking})
			event, err := mapper.Parse(raw)
			s.Require().NoError(err)
			s.Equal(tc.wantSrc, event.SignupToken)
		})
	}
}

func (s *PayloadMapperSuite) TestPlanCodeMapping() {
	mapper := kiwify.NewPayloadMapper(s.registry, nil)

	type testCase struct {
		name      string
		productID string
		wantPlan  valueobjects.PlanCode
	}

	cases := []testCase{
		{"mensal", "prod-monthly", valueobjects.PlanCodeMonthly},
		{"trimestral", "prod-quarterly", valueobjects.PlanCodeQuarterly},
		{"anual", "prod-annual", valueobjects.PlanCodeAnnual},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			raw := s.buildPayload(map[string]any{
				"product": map[string]any{"id": tc.productID},
			})
			event, err := mapper.Parse(raw)
			s.Require().NoError(err)
			s.Equal(tc.wantPlan, event.PlanCode)
		})
	}
}

func (s *PayloadMapperSuite) TestPlanCodeNotFound() {
	mapper := kiwify.NewPayloadMapper(s.registry, nil)
	raw := s.buildPayload(map[string]any{
		"product": map[string]any{"id": "unknown-product-id"},
	})
	_, err := mapper.Parse(raw)
	s.Require().Error(err)
}

func (s *PayloadMapperSuite) TestPeriodDivergence_WithinTolerance() {
	counter := &testDivergenceCounter{}
	mapper := kiwify.NewPayloadMapper(s.registry, counter)

	// period_end +29 dias (30d plano mensal ± 14d tolerância — dentro)
	raw := s.buildPayload(map[string]any{
		"subscription": map[string]any{
			"id":                   "sub-001",
			"current_period_start": s.now.Format(time.RFC3339),
			"current_period_end":   s.now.Add(29 * 24 * time.Hour).Format(time.RFC3339),
		},
	})
	_, err := mapper.Parse(raw)
	s.Require().NoError(err)
	s.Empty(counter.calls, "dentro da tolerância não deve emitir métrica")
}

func (s *PayloadMapperSuite) TestPeriodDivergence_OutsideTolerance_MetricIncremented() {
	counter := &testDivergenceCounter{}
	mapper := kiwify.NewPayloadMapper(s.registry, counter)

	// period_end +20 dias além do esperado (30d + 20d = 50d > 30d + 14d tolerância)
	raw := s.buildPayload(map[string]any{
		"subscription": map[string]any{
			"id":                   "sub-001",
			"current_period_start": s.now.Format(time.RFC3339),
			"current_period_end":   s.now.Add(50 * 24 * time.Hour).Format(time.RFC3339),
		},
	})
	_, err := mapper.Parse(raw)
	s.Require().NoError(err, "divergência não deve bloquear o mapeamento (ADR-011)")
	s.Len(counter.calls, 1, "deve emitir métrica de divergência")
	s.Equal("MONTHLY", counter.calls[0].plan)
	s.Equal("ahead", counter.calls[0].sign)
}

func (s *PayloadMapperSuite) TestPeriodDivergence_Behind_MetricIncremented() {
	counter := &testDivergenceCounter{}
	mapper := kiwify.NewPayloadMapper(s.registry, counter)
	// period_end -20 dias aquém do esperado (30d - 20d = 10d < 30d - 14d tolerância)
	raw := s.buildPayload(map[string]any{
		"subscription": map[string]any{
			"id":                   "sub-001",
			"current_period_start": s.now.Format(time.RFC3339),
			"current_period_end":   s.now.Add(10 * 24 * time.Hour).Format(time.RFC3339),
		},
	})
	_, err := mapper.Parse(raw)
	s.Require().NoError(err, "divergência não deve bloquear o mapeamento (ADR-011)")
	s.Len(counter.calls, 1)
	s.Equal("behind", counter.calls[0].sign)
}

func (s *PayloadMapperSuite) TestNilCounter_NoopFallback() {
	// mapper com nil counter deve usar noop internamente (sem pânico)
	mapper := kiwify.NewPayloadMapper(s.registry, nil)
	// divergência que acionaria o counter (50 dias em vez de 30)
	raw := s.buildPayload(map[string]any{
		"subscription": map[string]any{
			"id":                   "sub-001",
			"current_period_start": s.now.Format(time.RFC3339),
			"current_period_end":   s.now.Add(50 * 24 * time.Hour).Format(time.RFC3339),
		},
	})
	_, err := mapper.Parse(raw)
	s.Require().NoError(err, "noop counter não deve causar pânico ou erro")
}

func (s *PayloadMapperSuite) TestInvalidJSON() {
	mapper := kiwify.NewPayloadMapper(s.registry, nil)
	_, err := mapper.Parse([]byte("{invalid json"))
	s.Require().Error(err)
	s.ErrorIs(err, kiwify.ErrPayloadDecode)
}

func (s *PayloadMapperSuite) TestWhatsAppNormalization() {
	mapper := kiwify.NewPayloadMapper(s.registry, nil)
	raw := s.buildPayload(map[string]any{
		"customer": map[string]any{
			"mobile": "11999990000",
			"email":  "test@example.com",
		},
	})
	event, err := mapper.Parse(raw)
	s.Require().NoError(err)
	// E.164 BR: 5511999990000
	s.Equal("+5511999990000", event.Customer.WhatsApp.String())
}

func (s *PayloadMapperSuite) TestPeriodEndFromPayload_NotCalculatedLocally() {
	mapper := kiwify.NewPayloadMapper(s.registry, nil)
	expectedEnd := s.now.Add(45 * 24 * time.Hour) // 45 dias != 30 dias esperado para MONTHLY
	raw := s.buildPayload(map[string]any{
		"subscription": map[string]any{
			"id":                   "sub-001",
			"current_period_start": s.now.Format(time.RFC3339),
			"current_period_end":   expectedEnd.Format(time.RFC3339),
		},
	})
	event, err := mapper.Parse(raw)
	s.Require().NoError(err)
	// period_end deve vir do payload, não calculado localmente (ADR-011, R15)
	s.WithinDuration(expectedEnd, event.PeriodEnd, time.Second)
}
