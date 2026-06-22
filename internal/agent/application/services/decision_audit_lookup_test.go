package services

import (
	"context"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	agentinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	interfacesmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	uowmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow/mocks"
)

type DecisionAuditLookupSuite struct {
	suite.Suite

	ctx     context.Context
	obs     observability.Observability
	repo    *interfacesmocks.AgentDecisionRepository
	factory *interfacesmocks.AgentDecisionRepositoryFactory
	uow     *uowmocks.UnitOfWork
}

func TestDecisionAuditLookupSuite(t *testing.T) {
	suite.Run(t, new(DecisionAuditLookupSuite))
}

func (s *DecisionAuditLookupSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.repo = interfacesmocks.NewAgentDecisionRepository(s.T())
	s.factory = interfacesmocks.NewAgentDecisionRepositoryFactory(s.T())
	s.uow = uowmocks.NewUnitOfWork(s.T())
}

func (s *DecisionAuditLookupSuite) TestLookup() {
	type args struct {
		messageID string
	}

	type dependencies struct {
		repo    *interfacesmocks.AgentDecisionRepository
		factory *interfacesmocks.AgentDecisionRepositoryFactory
		uow     *uowmocks.UnitOfWork
	}

	runUoW := func() *uowmocks.UnitOfWork {
		s.uow.EXPECT().Do(mock.Anything, mock.Anything).RunAndReturn(func(ctx context.Context, fn func(context.Context, database.DBTX) error) error {
			return fn(ctx, nil)
		})
		return s.uow
	}
	bindFactory := func() *interfacesmocks.AgentDecisionRepositoryFactory {
		s.factory.EXPECT().AgentDecisionRepository(mock.Anything).Return(s.repo)
		return s.factory
	}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(reply string, found bool)
	}{
		{
			name: "decisao inexistente retorna found=false",
			args: args{messageID: "wamid.miss"},
			dependencies: dependencies{
				repo: func() *interfacesmocks.AgentDecisionRepository {
					s.repo.EXPECT().FindByMessage(mock.Anything, mock.AnythingOfType("uuid.UUID"), "whatsapp", "wamid.miss").
						Return(agentinterfaces.AgentDecisionSnapshot{}, false, nil).Once()
					return s.repo
				}(),
				factory: bindFactory(),
				uow:     runUoW(),
			},
			expect: func(reply string, found bool) {
				s.False(found)
				s.Empty(reply)
			},
		},
		{
			name: "decisao existente decodifica reply redigida",
			args: args{messageID: "wamid.hit"},
			dependencies: dependencies{
				repo: func() *interfacesmocks.AgentDecisionRepository {
					s.repo.EXPECT().FindByMessage(mock.Anything, mock.AnythingOfType("uuid.UUID"), "whatsapp", "wamid.hit").
						Return(agentinterfaces.AgentDecisionSnapshot{Status: "executed", RedactedResponse: []byte(`{"redacted":"Lancei R$ 58,00 no iFood"}`)}, true, nil).Once()
					return s.repo
				}(),
				factory: s.factory,
				uow:     s.uow,
			},
			expect: func(reply string, found bool) {
				s.True(found)
				s.Equal("Lancei R$ 58,00 no iFood", reply)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			auditor := newDecisionAuditor(s.obs, DecisionAuditDeps{Factory: scenario.dependencies.factory, UoW: scenario.dependencies.uow}, nil)
			reply, found := auditor.lookup(s.ctx, uuid.New(), "whatsapp", scenario.args.messageID)
			scenario.expect(reply, found)
		})
	}
}

func (s *DecisionAuditLookupSuite) TestLookupDisabledReturnsFalse() {
	auditor := newDecisionAuditor(s.obs, DecisionAuditDeps{}, nil)
	reply, found := auditor.lookup(s.ctx, uuid.New(), "whatsapp", "wamid.x")
	s.False(found)
	s.Empty(reply)
}

func (s *DecisionAuditLookupSuite) TestDecodeRedactedReply() {
	type args struct {
		raw string
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(reply string)
	}{
		{name: "json com redacted", args: args{raw: `{"redacted":"oi"}`}, expect: func(reply string) { s.Equal("oi", reply) }},
		{name: "json vazio", args: args{raw: `{}`}, expect: func(reply string) { s.Empty(reply) }},
		{name: "vazio", args: args{raw: ``}, expect: func(reply string) { s.Empty(reply) }},
		{name: "nao json", args: args{raw: `not json`}, expect: func(reply string) { s.Empty(reply) }},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			scenario.expect(decodeRedactedReply([]byte(scenario.args.raw)))
		})
	}
}
