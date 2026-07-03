//go:build integration

package agent_test

import (
	"context"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	agentpostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent/infrastructure/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory"
	mempostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory/infrastructure/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

type substrateStubAgent struct {
	id           string
	instructions string
	result       agent.Result
}

func (a *substrateStubAgent) ID() string           { return a.id }
func (a *substrateStubAgent) Instructions() string { return a.instructions }

func (a *substrateStubAgent) Execute(_ context.Context, _ agent.Request) (agent.Result, error) {
	return a.result, nil
}

func (a *substrateStubAgent) Stream(_ context.Context, _ agent.Request) (agent.ResultStream, error) {
	return nil, nil
}

type SubstrateIntegrationSuite struct {
	suite.Suite
	ctx context.Context
	db  *sqlx.DB
}

func TestSubstrateIntegrationSuite(t *testing.T) {
	suite.Run(t, new(SubstrateIntegrationSuite))
}

func (s *SubstrateIntegrationSuite) SetupSuite() {
	s.ctx = context.Background()
	s.db, _ = testcontainer.Postgres(s.T())
}

func (s *SubstrateIntegrationSuite) buildRuntime(agentID string, result agent.Result, writeTools ...string) (agent.AgentRuntime, string) {
	obs := fake.NewProvider()
	ag := &substrateStubAgent{id: agentID, instructions: "test", result: result}
	reg := agent.NewAgentRegistry()
	reg.Register(ag)

	rt := agent.NewAgentRuntime(
		reg,
		mempostgres.NewThreadRepository(s.db, obs),
		mempostgres.NewMessageRepository(s.db, obs),
		mempostgres.NewWorkingMemoryRepository(s.db, obs),
		agentpostgres.NewRunStore(s.db),
		obs,
		agent.WithWriteToolSet(writeTools...),
	)
	return rt, uuid.NewString()
}

func (s *SubstrateIntegrationSuite) TestRF39_RoleToolPersistedInPlatformMessages() {
	rt, resourceID := s.buildRuntime("agent-rf39", agent.Result{
		Content: "Despesa registrada!",
		ToolCalls: []agent.ToolCallRecord{
			{Tool: "register_expense", Outcome: agent.ToolCallOutcomeSuccess, Content: `{"resourceId":"tx-rf39"}`},
		},
	}, "register_expense")

	outcome, err := rt.Execute(s.ctx, agent.InboundRequest{
		AgentID:    "agent-rf39",
		ResourceID: resourceID,
		ThreadID:   "thr-" + uuid.NewString(),
		Message:    "registrar despesa",
		MessageID:  "msg-" + uuid.NewString(),
	})
	s.Require().NoError(err)
	s.Equal(agent.RunStatusSucceeded, outcome.Status)

	var count int
	err = s.db.QueryRowContext(s.ctx,
		`SELECT COUNT(*) FROM mecontrola.platform_messages WHERE resource_id = $1 AND role = $2`,
		resourceID, string(memory.RoleTool),
	).Scan(&count)
	s.Require().NoError(err)
	s.Greater(count, 0, "EP-05 corrigido: platform_messages deve conter role=tool após escrita real")
}

func (s *SubstrateIntegrationSuite) TestRF38_WriteToolFailed_RunStatusFailedInDB() {
	rt, resourceID := s.buildRuntime("agent-rf38", agent.Result{
		Content: "Despesa registrada com sucesso!",
		ToolCalls: []agent.ToolCallRecord{
			{Tool: "register_expense", Outcome: agent.ToolCallOutcomeError, Content: "usecase error"},
		},
	}, "register_expense")

	outcome, err := rt.Execute(s.ctx, agent.InboundRequest{
		AgentID:    "agent-rf38",
		ResourceID: resourceID,
		ThreadID:   "thr-" + uuid.NewString(),
		Message:    "registrar despesa",
		MessageID:  "msg-" + uuid.NewString(),
	})
	s.Require().NoError(err)
	s.Equal(agent.RunStatusFailed, outcome.Status, "EP-01 corrigido: run deve ser Failed quando write tool falhou")
}

func (s *SubstrateIntegrationSuite) TestRF38_CreateRecurrenceFailed_RunStatusFailedInDB() {
	rt, resourceID := s.buildRuntime("agent-rf38-rec", agent.Result{
		Content: "Recorrência criada com sucesso!",
		ToolCalls: []agent.ToolCallRecord{
			{Tool: "create_recurrence", Outcome: agent.ToolCallOutcomeError, Content: "usecase error"},
		},
	}, "register_expense", "register_income", "register_card_purchase", "create_recurrence")

	outcome, err := rt.Execute(s.ctx, agent.InboundRequest{
		AgentID:    "agent-rf38-rec",
		ResourceID: resourceID,
		ThreadID:   "thr-" + uuid.NewString(),
		Message:    "criar recorrência",
		MessageID:  "msg-" + uuid.NewString(),
	})
	s.Require().NoError(err)
	s.Equal(agent.RunStatusFailed, outcome.Status, "A-04: create_recurrence no WriteToolSet deve reprovar run quando falha")
}
