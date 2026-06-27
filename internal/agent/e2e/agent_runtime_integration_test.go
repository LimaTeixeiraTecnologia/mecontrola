//go:build integration

package e2e_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/capability"
	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	appservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	agentrepos "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/repositories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

type runtimeRouteStub struct {
	result appservices.RouteResult
	calls  int
}

func (s *runtimeRouteStub) handle(_ context.Context, _ appservices.Principal, _, _, _, _ string) appservices.RouteResult {
	s.calls++
	return s.result
}

type runtimeThreadGateway struct {
	factory appinterfaces.AgentThreadRepositoryFactory
	unit    uow.UnitOfWork
}

func (g *runtimeThreadGateway) GetOrCreate(ctx context.Context, userID uuid.UUID, channel string) (entities.Thread, error) {
	var resolved entities.Thread
	op := func(ctx context.Context, db database.DBTX) error {
		repo := g.factory.AgentThreadRepository(db)
		existing, found, err := repo.GetByUserAndChannel(ctx, userID, channel)
		if err != nil {
			return err
		}
		if found {
			resolved = existing
			return nil
		}
		created, err := entities.NewThread(userID, channel)
		if err != nil {
			return err
		}
		persisted, err := repo.Upsert(ctx, created)
		if err != nil {
			return err
		}
		resolved = persisted
		return nil
	}
	if err := g.unit.Do(ctx, op); err != nil {
		return entities.Thread{}, fmt.Errorf("agent: thread get_or_create: %w", err)
	}
	return resolved, nil
}

type runtimeRunGateway struct {
	factory appinterfaces.AgentRunRepositoryFactory
	unit    uow.UnitOfWork
}

func (g *runtimeRunGateway) Insert(ctx context.Context, run entities.Run) error {
	op := func(ctx context.Context, db database.DBTX) error {
		return g.factory.AgentRunRepository(db).Insert(ctx, run)
	}
	if err := g.unit.Do(ctx, op); err != nil {
		return fmt.Errorf("agent: run insert: %w", err)
	}
	return nil
}

func (g *runtimeRunGateway) Finish(ctx context.Context, run entities.Run) error {
	op := func(ctx context.Context, db database.DBTX) error {
		return g.factory.AgentRunRepository(db).UpdateOnFinish(ctx, run)
	}
	if err := g.unit.Do(ctx, op); err != nil {
		return fmt.Errorf("agent: run finish: %w", err)
	}
	return nil
}

type AgentRuntimeIntegrationSuite struct {
	suite.Suite
	db      *sqlx.DB
	threads appservices.ThreadGateway
	runs    appservices.RunGateway
}

func TestAgentRuntimeIntegrationSuite(t *testing.T) {
	suite.Run(t, new(AgentRuntimeIntegrationSuite))
}

func (s *AgentRuntimeIntegrationSuite) SetupSuite() {
	db, _ := testcontainer.Postgres(s.T())
	s.db = db

	threadFactory := agentrepos.NewThreadRepositoryFactory(noop.NewProvider())
	runFactory := agentrepos.NewRunRepositoryFactory(noop.NewProvider())
	unit := uow.NewUnitOfWork(db)

	s.threads = &runtimeThreadGateway{factory: threadFactory, unit: unit}
	s.runs = &runtimeRunGateway{factory: runFactory, unit: unit}
}

func (s *AgentRuntimeIntegrationSuite) insertUser() uuid.UUID {
	userID := uuid.New()
	number := fmt.Sprintf("+5511%09d", time.Now().UnixNano()%1000000000)
	_, err := s.db.ExecContext(context.Background(),
		`INSERT INTO mecontrola.users (id, whatsapp_number, status, created_at, updated_at)
		 VALUES ($1, $2, 'ACTIVE', now(), now())`,
		userID, number,
	)
	s.Require().NoError(err)
	return userID
}

func (s *AgentRuntimeIntegrationSuite) newRuntime(result appservices.RouteResult) (*appservices.AgentRuntime, *runtimeRouteStub) {
	catalog, err := capability.BuildCatalog()
	s.Require().NoError(err)
	stub := &runtimeRouteStub{result: result}
	rt := appservices.NewAgentRuntime(noop.NewProvider(), catalog, appservices.RouterFunc(stub.handle), s.threads, s.runs)
	return rt, stub
}

func (s *AgentRuntimeIntegrationSuite) TestRoutedRunPersisted() {
	ctx := context.Background()
	userID := s.insertUser()
	principal := appservices.Principal{UserID: userID}

	expected := appservices.RouteResult{Reply: "lançado", Outcome: tools.OutcomeRouted, Kind: intent.KindCreateCard}
	rt, stub := s.newRuntime(expected)

	got := rt.Execute(ctx, principal, "whatsapp", "+5511999999999", "cadastrar cartão nubank", "wamid.routed-1")

	s.Equal(expected, got)
	s.Equal(1, stub.calls)

	var threadID uuid.UUID
	err := s.db.QueryRowContext(ctx,
		`SELECT id FROM mecontrola.agent_threads WHERE user_id = $1 AND channel = $2`,
		userID, "whatsapp",
	).Scan(&threadID)
	s.Require().NoError(err)
	s.NotEqual(uuid.Nil, threadID)

	var (
		runThreadID uuid.UUID
		status      string
		outcome     string
		workflow    string
		toolName    string
		intentKind  string
		errText     string
		duration    int64
		startedAt   time.Time
		endedAt     time.Time
	)
	err = s.db.QueryRowContext(ctx,
		`SELECT thread_id, status, outcome, workflow, tool_name, intent_kind, error,
		        duration_ms, started_at, ended_at
		   FROM mecontrola.agent_runs WHERE user_id = $1`,
		userID,
	).Scan(&runThreadID, &status, &outcome, &workflow, &toolName, &intentKind,
		&errText, &duration, &startedAt, &endedAt)
	s.Require().NoError(err)

	s.Equal(threadID, runThreadID)
	s.Equal("succeeded", status)
	s.Equal(tools.OutcomeRouted.String(), outcome)
	s.Equal("cards", workflow)
	s.Equal(intent.KindCreateCard.String(), toolName)
	s.Equal(intent.KindCreateCard.String(), intentKind)
	s.Empty(errText)
	s.GreaterOrEqual(duration, int64(0))
	s.False(startedAt.IsZero())
	s.False(endedAt.IsZero())
	s.False(endedAt.Before(startedAt))
}

func (s *AgentRuntimeIntegrationSuite) TestThreadReusedAcrossRuns() {
	ctx := context.Background()
	userID := s.insertUser()
	principal := appservices.Principal{UserID: userID}

	rt1, _ := s.newRuntime(appservices.RouteResult{Outcome: tools.OutcomeRouted, Kind: intent.KindCreateCard})
	s.Require().Equal(tools.OutcomeRouted,
		rt1.Execute(ctx, principal, "whatsapp", "+5511999999999", "cartão 1", "wamid.reuse-1").Outcome)

	rt2, _ := s.newRuntime(appservices.RouteResult{Outcome: tools.OutcomeRouted, Kind: intent.KindListCards})
	s.Require().Equal(tools.OutcomeRouted,
		rt2.Execute(ctx, principal, "whatsapp", "+5511999999999", "meus cartões", "wamid.reuse-2").Outcome)

	var threadCount int
	err := s.db.QueryRowContext(ctx,
		`SELECT count(*) FROM mecontrola.agent_threads WHERE user_id = $1 AND channel = $2`,
		userID, "whatsapp",
	).Scan(&threadCount)
	s.Require().NoError(err)
	s.Equal(1, threadCount)

	var (
		runCount    int
		distinctThr int
	)
	err = s.db.QueryRowContext(ctx,
		`SELECT count(*), count(DISTINCT thread_id) FROM mecontrola.agent_runs WHERE user_id = $1`,
		userID,
	).Scan(&runCount, &distinctThr)
	s.Require().NoError(err)
	s.Equal(2, runCount)
	s.Equal(1, distinctThr)
}

func (s *AgentRuntimeIntegrationSuite) TestFailureRunPersisted() {
	ctx := context.Background()
	userID := s.insertUser()
	principal := appservices.Principal{UserID: userID}

	expected := appservices.RouteResult{Reply: "falhou", Outcome: tools.OutcomeUsecaseError, Kind: intent.KindCreateCard}
	rt, _ := s.newRuntime(expected)

	got := rt.Execute(ctx, principal, "whatsapp", "+5511999999999", "cadastrar cartão", "wamid.failed-1")
	s.Equal(expected, got)

	var (
		status  string
		errText string
	)
	err := s.db.QueryRowContext(ctx,
		`SELECT status, error FROM mecontrola.agent_runs WHERE user_id = $1`,
		userID,
	).Scan(&status, &errText)
	s.Require().NoError(err)
	s.Equal("failed", status)
	s.Equal(tools.OutcomeUsecaseError.String(), errText)
}

func (s *AgentRuntimeIntegrationSuite) TestRouteResultIdenticalWithUnknownUser() {
	ctx := context.Background()
	principal := appservices.Principal{UserID: uuid.New()}

	expected := appservices.RouteResult{Reply: "ok mesmo sem persistir", Outcome: tools.OutcomeRouted, Kind: intent.KindCreateCard}
	rt, stub := s.newRuntime(expected)

	got := rt.Execute(ctx, principal, "whatsapp", "+5511999999999", "qualquer", "wamid.degraded-1")

	s.Equal(expected, got)
	s.Equal(1, stub.calls)

	var runCount int
	err := s.db.QueryRowContext(ctx,
		`SELECT count(*) FROM mecontrola.agent_runs WHERE user_id = $1`,
		principal.UserID,
	).Scan(&runCount)
	s.Require().NoError(err)
	s.Equal(0, runCount)
}
