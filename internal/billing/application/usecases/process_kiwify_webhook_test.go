package usecases_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases"
	ucmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases/mocks"
)

const (
	metricReceived        = "billing_webhooks_received_total"
	metricTrackingCarrier = "billing_kiwify_tracking_carrier_total"
	logSignatureInvalid   = "billing.webhook.signature_invalid"
	logLegacyCarrierSeen  = "kiwify.tracking.legacy_carrier_seen"
)

type ProcessKiwifyWebhookTelemetrySuite struct {
	suite.Suite
	o11y         *fake.Provider
	saleApproved *ucmocks.ProcessSaleApproved
	subRenewed   *ucmocks.ProcessSubscriptionRenewed
	subLate      *ucmocks.ProcessSubscriptionLate
	subCanceled  *ucmocks.ProcessSubscriptionCanceled
	refund       *ucmocks.ProcessRefundOrChargeback
	eventRepo    *ucmocks.KiwifyEventRepository
	uc           *usecases.ProcessKiwifyWebhook
}

func TestProcessKiwifyWebhookTelemetrySuite(t *testing.T) {
	suite.Run(t, new(ProcessKiwifyWebhookTelemetrySuite))
}

func (s *ProcessKiwifyWebhookTelemetrySuite) SetupTest() {
	s.o11y = fake.NewProvider()
	s.saleApproved = ucmocks.NewProcessSaleApproved(s.T())
	s.subRenewed = ucmocks.NewProcessSubscriptionRenewed(s.T())
	s.subLate = ucmocks.NewProcessSubscriptionLate(s.T())
	s.subCanceled = ucmocks.NewProcessSubscriptionCanceled(s.T())
	s.refund = ucmocks.NewProcessRefundOrChargeback(s.T())
	s.eventRepo = ucmocks.NewKiwifyEventRepository(s.T())
	s.eventRepo.EXPECT().Persist(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	s.uc = usecases.NewProcessKiwifyWebhook(
		s.saleApproved,
		s.subRenewed,
		s.subLate,
		s.subCanceled,
		s.refund,
		s.eventRepo,
		s.o11y,
	)
}

func (s *ProcessKiwifyWebhookTelemetrySuite) bodyWithCarrier(eventType, sck, s1, src string) []byte {
	payload := fmt.Sprintf(
		`{"order_id":"order-%s","webhook_event_type":"%s","subscription_id":"sub-123","Product":{"product_id":"prod-1","product_name":"P"},"Customer":{"email":"a@b.com","mobile":"+5511900000000","CPF":"00000000000"},"Subscription":{"status":"active","start_date":"2026-06-08T14:53:19.679Z","next_payment":"2026-07-08T14:53:23.137Z"},"TrackingParameters":{"sck":"%s","s1":"%s","src":"%s"},"approved_date":"2026-06-08 11:53","updated_at":"2026-06-08 11:53","created_at":"2026-06-08 11:53"}`,
		eventType, eventType, sck, s1, src,
	)
	return []byte(payload)
}

func (s *ProcessKiwifyWebhookTelemetrySuite) counterValuesByLabel(metric, labelKey string) map[string]int64 {
	c := s.o11y.Metrics().(*fake.FakeMetrics).GetCounter(metric)
	if c == nil {
		return nil
	}
	totals := make(map[string]int64)
	for _, v := range c.GetValues() {
		label := ""
		for _, f := range v.Fields {
			if f.Key == labelKey {
				label = f.StringValue()
				break
			}
		}
		totals[label] += v.Value
	}
	return totals
}

func (s *ProcessKiwifyWebhookTelemetrySuite) logEntries() []fake.LogEntry {
	return s.o11y.Logger().(*fake.FakeLogger).GetEntries()
}

func (s *ProcessKiwifyWebhookTelemetrySuite) TestDeveContarSignatureStatusValidEmOrderApproved() {
	s.saleApproved.EXPECT().Execute(mock.Anything, mock.Anything).Return(nil).Once()
	body := s.bodyWithCarrier("order_approved", "tok-sck", "", "")

	err := s.uc.Execute(context.Background(), input.ProcessKiwifyWebhookInput{RawBody: body, SignatureStatus: "valid"})

	require.NoError(s.T(), err)
	totals := s.counterValuesByLabel(metricReceived, "signature_status")
	assert.Equal(s.T(), int64(1), totals["valid"])
}

func (s *ProcessKiwifyWebhookTelemetrySuite) TestDeveContarSignatureStatusInvalidESinalizarLog() {
	body := s.bodyWithCarrier("order_approved", "tok-sck", "", "")

	err := s.uc.Execute(context.Background(), input.ProcessKiwifyWebhookInput{RawBody: body, SignatureStatus: "invalid"})

	require.Error(s.T(), err)
	totals := s.counterValuesByLabel(metricReceived, "signature_status")
	assert.Equal(s.T(), int64(1), totals["invalid"])

	var warn *fake.LogEntry
	for i, entry := range s.logEntries() {
		if entry.Message == logSignatureInvalid {
			warn = &s.logEntries()[i]
			break
		}
	}
	require.NotNil(s.T(), warn, "esperado warn %q", logSignatureInvalid)
	assert.Equal(s.T(), observability.LogLevelWarn, warn.Level)
}

func (s *ProcessKiwifyWebhookTelemetrySuite) TestDeveContarSignatureStatusRotated() {
	s.saleApproved.EXPECT().Execute(mock.Anything, mock.Anything).Return(nil).Once()
	body := s.bodyWithCarrier("order_approved", "tok-sck", "", "")

	err := s.uc.Execute(context.Background(), input.ProcessKiwifyWebhookInput{RawBody: body, SignatureStatus: "rotated"})

	require.NoError(s.T(), err)
	totals := s.counterValuesByLabel(metricReceived, "signature_status")
	assert.Equal(s.T(), int64(1), totals["rotated"])
}

func (s *ProcessKiwifyWebhookTelemetrySuite) TestDeveSegmentarCarrierPorTrackingParameters() {
	scenarios := []struct {
		name            string
		sck             string
		s1              string
		src             string
		expectedLabel   string
		expectLegacyLog bool
	}{
		{name: "sck dominante", sck: "tok-sck", expectedLabel: "sck"},
		{name: "s1 legado", s1: "tok-s1", expectedLabel: "s1", expectLegacyLog: true},
		{name: "src legado", src: "tok-src", expectedLabel: "src", expectLegacyLog: true},
		{name: "nenhum carrier", expectedLabel: "none"},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			s.SetupTest()
			s.saleApproved.EXPECT().Execute(mock.Anything, mock.Anything).Return(nil).Maybe()
			body := s.bodyWithCarrier("order_approved", sc.sck, sc.s1, sc.src)

			err := s.uc.Execute(context.Background(), input.ProcessKiwifyWebhookInput{RawBody: body, SignatureStatus: "valid"})

			require.NoError(s.T(), err)
			totals := s.counterValuesByLabel(metricTrackingCarrier, "carrier")
			assert.Equal(s.T(), int64(1), totals[sc.expectedLabel], "carrier=%s totals=%v", sc.expectedLabel, totals)

			seenLegacy := false
			for _, entry := range s.logEntries() {
				if entry.Message == logLegacyCarrierSeen {
					seenLegacy = true
					break
				}
			}
			assert.Equal(s.T(), sc.expectLegacyLog, seenLegacy, "esperado log legacy_carrier_seen=%v para carrier=%s", sc.expectLegacyLog, sc.expectedLabel)
		})
	}
}

func (s *ProcessKiwifyWebhookTelemetrySuite) TestDeveContarCarrierEnoOpsSemDispatchDownstream() {
	body := s.bodyWithCarrier("billet_created", "", "", "")

	err := s.uc.Execute(context.Background(), input.ProcessKiwifyWebhookInput{RawBody: body, SignatureStatus: "valid"})

	require.NoError(s.T(), err)
	receivedTotals := s.counterValuesByLabel(metricReceived, "signature_status")
	assert.Equal(s.T(), int64(1), receivedTotals["valid"])
	carrierTotals := s.counterValuesByLabel(metricTrackingCarrier, "carrier")
	assert.Equal(s.T(), int64(1), carrierTotals["none"])
}
