package binding

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
)

type RecurrenceManagerAdapterSuite struct {
	suite.Suite
	ctx context.Context
}

func TestRecurrenceManagerAdapterSuite(t *testing.T) {
	suite.Run(t, new(RecurrenceManagerAdapterSuite))
}

func (s *RecurrenceManagerAdapterSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *RecurrenceManagerAdapterSuite) TestPrincipalCtx_InjectsPrincipalFromInboundIdentity() {
	adapter := &recurrenceManagerAdapter{}
	userID := uuid.New()
	inbound := agent.WithToolInvocationContext(s.ctx, userID.String(), "wamid-1", 0)

	got, err := adapter.principalCtx(inbound)

	s.Require().NoError(err)
	principal, ok := auth.FromContext(got)
	s.Require().True(ok)
	s.Equal(userID, principal.UserID)
	s.Equal(auth.SourceWhatsApp, principal.Source)
}

func (s *RecurrenceManagerAdapterSuite) TestPrincipalCtx_MissingIdentity_Errors() {
	adapter := &recurrenceManagerAdapter{}

	_, err := adapter.principalCtx(s.ctx)

	s.Error(err)
}

func (s *RecurrenceManagerAdapterSuite) TestPrincipalCtx_PreexistingPrincipal_Preserved() {
	adapter := &recurrenceManagerAdapter{}
	userID := uuid.New()
	withPrincipal := auth.WithPrincipal(s.ctx, auth.Principal{UserID: userID, Source: auth.SourceWhatsApp})

	got, err := adapter.principalCtx(withPrincipal)

	s.Require().NoError(err)
	principal, ok := auth.FromContext(got)
	s.Require().True(ok)
	s.Equal(userID, principal.UserID)
}
