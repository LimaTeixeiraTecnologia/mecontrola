package services

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
)

type stubParser struct{}

func (stubParser) Parse(context.Context, uuid.UUID, string) (ParsedIntent, error) {
	return ParsedIntent{}, nil
}

type stubFallback struct{}

func (stubFallback) Reply(context.Context, uuid.UUID, string, string) (string, error) {
	return "", nil
}

type stubWhatsApp struct{}

func (stubWhatsApp) SendTextMessage(context.Context, string, string) error { return nil }

type AuthzRetrySuite struct {
	suite.Suite
	ctx context.Context
}

func TestAuthzRetrySuite(t *testing.T) {
	suite.Run(t, new(AuthzRetrySuite))
}

func (s *AuthzRetrySuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *AuthzRetrySuite) newGuardRouter() *IntentRouter {
	router, err := NewIntentRouter(noop.NewProvider(), IntentRouterDeps{
		Parser:          stubParser{},
		Fallback:        stubFallback{},
		WhatsAppGateway: stubWhatsApp{},
		Location:        time.UTC,
	})
	s.Require().NoError(err, "NewIntentRouter")
	return router
}

func (s *AuthzRetrySuite) TestAuthorizeWrite_AllowsMatchingPrincipal() {
	router := s.newGuardRouter()
	owner := uuid.New()
	s.True(router.authorizeWrite(s.ctx, Principal{UserID: owner}, owner, intent.KindRecordExpense, ChannelWhatsApp), "esperava autorizacao para userID igual ao principal")
}

func (s *AuthzRetrySuite) TestAuthorizeWrite_DeniesDivergentUserID() {
	router := s.newGuardRouter()
	principal := Principal{UserID: uuid.New()}
	attacker := uuid.New()
	s.False(router.authorizeWrite(s.ctx, principal, attacker, intent.KindRecordExpense, ChannelWhatsApp), "esperava negacao quando userID efetivo diverge do principal")
}

func (s *AuthzRetrySuite) TestAuthorizeWrite_DeniesNilUserID() {
	router := s.newGuardRouter()
	s.False(router.authorizeWrite(s.ctx, Principal{UserID: uuid.Nil}, uuid.Nil, intent.KindCreateCard, ChannelWhatsApp), "esperava negacao para userID nulo")
}
