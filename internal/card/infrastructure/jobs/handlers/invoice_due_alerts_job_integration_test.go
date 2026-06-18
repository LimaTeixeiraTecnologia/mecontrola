//go:build integration

package handlers_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card"
	cardinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/notification"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

type stubChannelGateway struct{}

func (s *stubChannelGateway) SendText(_ context.Context, _, _, _ string) error { return nil }
func (s *stubChannelGateway) SendActivationTemplate(_ context.Context, _, _, _, _ string) (string, error) {
	return "", fmt.Errorf("unsupported")
}

type stubUserChannelResolver struct {
	userID string
	phone  string
}

func (r *stubUserChannelResolver) ResolvePreferred(_ context.Context, userID uuid.UUID) (cardinterfaces.UserChannelPreference, bool, error) {
	if userID.String() == r.userID {
		return cardinterfaces.UserChannelPreference{
			Channel:    notification.ChannelWhatsApp,
			ExternalID: r.phone,
		}, true, nil
	}
	return cardinterfaces.UserChannelPreference{}, false, nil
}

func dueDayWindowSetJob(now time.Time, windowDays int) map[int]struct{} {
	set := make(map[int]struct{}, windowDays+2)
	for i := 0; i <= windowDays+1; i++ {
		set[now.AddDate(0, 0, i).Day()] = struct{}{}
	}
	return set
}

func pickJobDueDays(now time.Time, windowDays int) (inWindow int, outWindow int) {
	set := dueDayWindowSetJob(now, windowDays)
	inWindow = now.AddDate(0, 0, 1).Day()
	for d := 1; d <= 28; d++ {
		if _, ok := set[d]; !ok {
			outWindow = d
			break
		}
	}
	return inWindow, outWindow
}

func seedUser(t *testing.T, db *sqlx.DB, userID string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	number := fmt.Sprintf("+5511%09d", time.Now().UnixNano()%1000000000)
	_, err := db.ExecContext(ctx,
		`INSERT INTO mecontrola.users (id, whatsapp_number, status, created_at, updated_at)
		 VALUES ($1, $2, 'ACTIVE', now(), now())
		 ON CONFLICT (id) DO NOTHING`,
		userID, number,
	)
	require.NoError(t, err)
}

func seedCardWithDueDay(t *testing.T, db *sqlx.DB, cardID, userID string, dueDay int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	nick := fmt.Sprintf("card-%d-%d", dueDay, time.Now().UnixNano()%1000000000)
	_, err := db.ExecContext(ctx,
		`INSERT INTO mecontrola.cards (id, user_id, name, nickname, closing_day, due_day, limit_cents, version, created_at, updated_at)
		 VALUES ($1, $2, 'Test Card', $3, 5, $4, 500000, 1, now(), now())`,
		cardID, userID, nick, dueDay,
	)
	require.NoError(t, err)
}

func countAlertsSent(t *testing.T, db *sqlx.DB, cardID, userID string) int {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var n int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mecontrola.card_invoice_alerts_sent WHERE card_id = $1 AND user_id = $2`,
		cardID, userID,
	).Scan(&n)
	require.NoError(t, err)
	return n
}

func countOutboxByCard(t *testing.T, db *sqlx.DB, cardID, eventType string) int {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var n int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mecontrola.outbox_events WHERE aggregate_id = $1 AND event_type = $2`,
		cardID, eventType,
	).Scan(&n)
	require.NoError(t, err)
	return n
}

func passthrough(next http.Handler) http.Handler { return next }

func buildTestModule(t *testing.T, db *sqlx.DB, userID, phone string) card.CardModule {
	t.Helper()
	o11y := noop.NewProvider()
	cfg := &configs.Config{
		CardConfig: configs.CardConfig{
			InvoiceDueAlertsEnabled: true,
			InvoiceDueWindowDays:    3,
			InvoiceDueScanLimit:     100,
		},
		AuthRateLimit: configs.AuthRateLimitConfig{
			PerUserPerMin: 60000,
			PerUserBurst:  60000,
		},
		OutboxConfig: configs.OutboxConfig{
			RetryMaxAttempts: 3,
		},
	}
	mod, err := card.NewCardModule(
		context.Background(),
		cfg,
		o11y,
		db,
		passthrough,
		&stubChannelGateway{},
		&stubUserChannelResolver{userID: userID, phone: phone},
	)
	require.NoError(t, err)
	return mod
}

func TestInvoiceDueAlertsJob_Run_DispatchesEventForCardsDueWithinWindow(t *testing.T) {
	db, _ := testcontainer.Postgres(t)

	now := time.Now().UTC()
	inDay, _ := pickJobDueDays(now, 3)

	userID := uuid.New().String()
	cardID := uuid.New().String()

	seedUser(t, db, userID)
	seedCardWithDueDay(t, db, cardID, userID, inDay)

	mod := buildTestModule(t, db, userID, "+5511900000001")
	require.NotNil(t, mod.InvoiceDueAlertsJob)

	err := mod.InvoiceDueAlertsJob.Run(context.Background())
	require.NoError(t, err)

	require.Equal(t, 1, countAlertsSent(t, db, cardID, userID))
	require.Equal(t, 1, countOutboxByCard(t, db, cardID, "card.invoice_due.v1"))
}

func TestInvoiceDueAlertsJob_Run_IsIdempotent(t *testing.T) {
	db, _ := testcontainer.Postgres(t)

	now := time.Now().UTC()
	inDay, _ := pickJobDueDays(now, 3)

	userID := uuid.New().String()
	cardID := uuid.New().String()

	seedUser(t, db, userID)
	seedCardWithDueDay(t, db, cardID, userID, inDay)

	mod := buildTestModule(t, db, userID, "+5511900000002")
	require.NotNil(t, mod.InvoiceDueAlertsJob)

	require.NoError(t, mod.InvoiceDueAlertsJob.Run(context.Background()))
	require.NoError(t, mod.InvoiceDueAlertsJob.Run(context.Background()))

	require.Equal(t, 1, countAlertsSent(t, db, cardID, userID))
}

func TestInvoiceDueAlertsJob_Run_NoCandidates_NoError(t *testing.T) {
	db, _ := testcontainer.Postgres(t)

	now := time.Now().UTC()
	_, outDay := pickJobDueDays(now, 3)
	if outDay == 0 {
		t.Skip("nao ha due_day fora da janela disponivel hoje")
	}

	userID := uuid.New().String()
	cardID := uuid.New().String()

	seedUser(t, db, userID)
	seedCardWithDueDay(t, db, cardID, userID, outDay)

	mod := buildTestModule(t, db, userID, "+5511900000003")
	require.NotNil(t, mod.InvoiceDueAlertsJob)

	err := mod.InvoiceDueAlertsJob.Run(context.Background())
	require.NoError(t, err)

	require.Equal(t, 0, countAlertsSent(t, db, cardID, userID))
}
