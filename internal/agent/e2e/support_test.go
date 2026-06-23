//go:build integration || e2e

package e2e_test

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/onboardingv2draft"
	agentvo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	identityvo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
	identityrepos "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/repositories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

type Reply struct {
	To   string
	Text string
}

type CapturingGateway struct {
	mu      sync.Mutex
	replies []Reply
}

func (g *CapturingGateway) SendTextMessage(_ context.Context, toE164, text string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.replies = append(g.replies, Reply{To: toE164, Text: text})
	return nil
}

func (g *CapturingGateway) LastReply() (Reply, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.replies) == 0 {
		return Reply{}, false
	}
	return g.replies[len(g.replies)-1], true
}

func (g *CapturingGateway) All() []Reply {
	g.mu.Lock()
	defer g.mu.Unlock()
	cp := make([]Reply, len(g.replies))
	copy(cp, g.replies)
	return cp
}

func (g *CapturingGateway) Reset() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.replies = nil
}

type TelegramReply struct {
	ChatID int64
	Text   string
}

type CapturingTelegramGateway struct {
	mu      sync.Mutex
	replies []TelegramReply
}

func (g *CapturingTelegramGateway) SendTextMessage(_ context.Context, chatID int64, text string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.replies = append(g.replies, TelegramReply{ChatID: chatID, Text: text})
	return nil
}

func (g *CapturingTelegramGateway) LastReply() (TelegramReply, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.replies) == 0 {
		return TelegramReply{}, false
	}
	return g.replies[len(g.replies)-1], true
}

func (g *CapturingTelegramGateway) Reset() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.replies = nil
}

type parserAdapter struct{ uc *usecases.ParseInbound }

func (a *parserAdapter) Parse(ctx context.Context, userID uuid.UUID, text string) (services.ParsedIntent, error) {
	out, err := a.uc.Execute(ctx, usecases.ParseInboundInput{UserID: userID, Text: text})
	if err != nil {
		return services.ParsedIntent{}, err
	}
	return services.ParsedIntent{Intent: out.Intent, Confidence: out.Confidence, Raw: out.Raw}, nil
}

func fullConfidence() agentvo.Confidence {
	confidence, _ := agentvo.NewConfidence(1)
	return confidence
}

func strPtr(v string) *string { return &v }

func intPtr(v int) *int { return &v }

type StubParser struct {
	table     map[string]intent.Intent
	defaultFn func() intent.Intent
}

func NewStubParser(table map[string]intent.Intent, defaultFn func() intent.Intent) *StubParser {
	return &StubParser{table: table, defaultFn: defaultFn}
}

func (s *StubParser) Parse(_ context.Context, _ uuid.UUID, text string) (services.ParsedIntent, error) {
	if in, ok := s.table[text]; ok {
		return services.ParsedIntent{Intent: in, Confidence: fullConfidence()}, nil
	}
	if s.defaultFn != nil {
		return services.ParsedIntent{Intent: s.defaultFn(), Confidence: fullConfidence()}, nil
	}
	unknown, _ := intent.NewUnknown(text)
	return services.ParsedIntent{Intent: unknown, Confidence: fullConfidence()}, nil
}

type StubFallback struct {
	DefaultReply string
}

func (s *StubFallback) Reply(_ context.Context, _ uuid.UUID, _, _ string) (string, error) {
	if s.DefaultReply != "" {
		return s.DefaultReply, nil
	}
	return "Não entendi. Pode reformular?", nil
}

func SeedActiveUserWA(t *testing.T, db database.DBTX, waNumber string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	o11y := noop.NewProvider()
	factory := identityrepos.NewRepositoryFactory(o11y)

	wa, err := identityvo.NewWhatsAppNumber(waNumber)
	if err != nil {
		t.Fatalf("e2e.seed: invalid wa number %q: %v", waNumber, err)
	}
	candidate := entities.New(wa)
	user, err := factory.UserRepository(db).UpsertByWhatsAppNumber(ctx, candidate, time.Now().UTC())
	if err != nil {
		t.Fatalf("e2e.seed: upsert user: %v", err)
	}
	userID, err := uuid.Parse(user.ID())
	if err != nil {
		t.Fatalf("e2e.seed: parse user id: %v", err)
	}

	entitlement := interfaces.EntitlementRecord{
		UserID:         userID.String(),
		SubscriptionID: uuid.New().String(),
		Status:         "ACTIVE",
		PeriodEnd:      time.Now().UTC().Add(365 * 24 * time.Hour),
	}
	if upsertErr := factory.EntitlementRepository(db).Upsert(ctx, entitlement); upsertErr != nil {
		t.Fatalf("e2e.seed: upsert entitlement: %v", upsertErr)
	}
	return userID
}

func SeedTelegramIdentity(t *testing.T, db database.DBTX, userID uuid.UUID, telegramUserID int64) {
	t.Helper()
	ctx := context.Background()
	o11y := noop.NewProvider()
	factory := identityrepos.NewRepositoryFactory(o11y)

	channel := identityvo.ChannelTelegram()
	externalID, err := identityvo.NewExternalID(channel, strconv.FormatInt(telegramUserID, 10))
	if err != nil {
		t.Fatalf("e2e.seed: telegram external_id %d: %v", telegramUserID, err)
	}
	identityID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("e2e.seed: telegram identity id: %v", err)
	}
	identity, err := entities.NewUserIdentity(identityID, userID, channel, externalID, time.Now().UTC())
	if err != nil {
		t.Fatalf("e2e.seed: build telegram user_identity: %v", err)
	}
	if insertErr := factory.UserIdentityRepository(db).Insert(ctx, identity); insertErr != nil {
		t.Fatalf("e2e.seed: insert telegram user_identity: %v", insertErr)
	}
}

type noopV2Session struct{}

func (noopV2Session) Load(_ context.Context, _ uuid.UUID, _ string) (onboardingv2draft.Draft, bool, error) {
	return onboardingv2draft.Draft{}, false, nil
}
func (noopV2Session) Save(_ context.Context, _ uuid.UUID, _ string, _ onboardingv2draft.Draft) error {
	return nil
}
func (noopV2Session) Clear(_ context.Context, _ uuid.UUID, _ string) error { return nil }
