//go:build integration

package workflows_test

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	agentsifaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	agentusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/infrastructure/binding"
	agentpersistence "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/infrastructure/persistence"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	agentpostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent/infrastructure/postgres"
	mempostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory/infrastructure/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
	workflowpg "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow/infrastructure/postgres"
)

var errCardCreateConfirmIntegrationInfra = errors.New("workflows: falha de infraestrutura simulada na integração")

type cardCreateInfraFailingManager struct {
	agentsifaces.CardManager
}

func (m *cardCreateInfraFailingManager) CreateCard(_ context.Context, _ agentsifaces.NewCard) (agentsifaces.CardRef, error) {
	return agentsifaces.CardRef{}, errCardCreateConfirmIntegrationInfra
}

type cardCreateIntegrationIdemAdapter struct {
	uc *agentusecases.IdempotentWrite
}

func (a cardCreateIntegrationIdemAdapter) Execute(
	ctx context.Context,
	userID uuid.UUID,
	wamid string,
	itemSeq int,
	operation string,
	resourceKind string,
	write workflows.IdempotentWriteFn,
	isDomainErr workflows.DomainErrorClassifier,
) (uuid.UUID, agent.ToolOutcome, error) {
	res, err := a.uc.Execute(ctx, userID, wamid, itemSeq, operation, resourceKind, agentusecases.WriteFn(write), isDomainErr)
	return res.ResourceID, res.Outcome, err
}

type CardCreateConfirmWorkflowIntegrationSuite struct {
	suite.Suite
	ctx   context.Context
	db    *sqlx.DB
	cfg   *configs.Config
	cards agentsifaces.CardManager
}

func TestCardCreateConfirmWorkflowIntegrationSuite(t *testing.T) {
	suite.Run(t, new(CardCreateConfirmWorkflowIntegrationSuite))
}

func (s *CardCreateConfirmWorkflowIntegrationSuite) SetupSuite() {
	s.ctx = context.Background()

	db, _ := testcontainer.Postgres(s.T())
	s.db = db

	cfg, err := configs.LoadConfig("../../../..")
	s.Require().NoError(err, "carregar config")
	cfg.TransactionsConfig.Enabled = true
	s.cfg = cfg

	o11y := fake.NewProvider()
	passthrough := func(next http.Handler) http.Handler { return next }

	cardModule, err := card.NewCardModule(s.ctx, s.cfg, o11y, s.db, passthrough, nil, nil)
	s.Require().NoError(err, "card module")

	s.cards = binding.NewCardManagerAdapter(
		cardModule.CreateCardUC,
		cardModule.ListCardsUC,
		cardModule.GetCardUC,
		cardModule.ResolveCardByNicknameUC,
		cardModule.CountCardsUC,
		cardModule.BestPurchaseDayUC,
		cardModule.UpdateCardUC,
		cardModule.SoftDeleteCardUC,
		nil,
		cardModule.IsBankRecognizedUC,
		o11y,
	)
}

func (s *CardCreateConfirmWorkflowIntegrationSuite) newUser() uuid.UUID {
	userID := uuid.New()
	_, err := s.db.ExecContext(s.ctx, `
		INSERT INTO mecontrola.users (id, whatsapp_number, status, created_at, updated_at)
		VALUES ($1, $2, 'ACTIVE', now(), now())`,
		userID, "+55119"+uuid.NewString()[:8],
	)
	s.Require().NoError(err)
	return userID
}

func (s *CardCreateConfirmWorkflowIntegrationSuite) countActiveCards(userID uuid.UUID) int {
	var n int
	err := s.db.QueryRowContext(s.ctx,
		`SELECT count(*) FROM mecontrola.cards WHERE user_id = $1 AND deleted_at IS NULL`,
		userID,
	).Scan(&n)
	s.Require().NoError(err)
	return n
}

func (s *CardCreateConfirmWorkflowIntegrationSuite) countCardsByNickname(userID uuid.UUID, nickname string) int {
	var n int
	err := s.db.QueryRowContext(s.ctx,
		`SELECT count(*) FROM mecontrola.cards WHERE user_id = $1 AND nickname = $2 AND deleted_at IS NULL`,
		userID, nickname,
	).Scan(&n)
	s.Require().NoError(err)
	return n
}

func (s *CardCreateConfirmWorkflowIntegrationSuite) loadCardClosingDay(userID uuid.UUID, nickname string) int {
	var closingDay int
	err := s.db.QueryRowContext(s.ctx,
		`SELECT closing_day FROM mecontrola.cards WHERE user_id = $1 AND nickname = $2 AND deleted_at IS NULL LIMIT 1`,
		userID, nickname,
	).Scan(&closingDay)
	s.Require().NoError(err)
	return closingDay
}

func (s *CardCreateConfirmWorkflowIntegrationSuite) buildEngineAndContinuer(cards agentsifaces.CardManager) (
	workflow.Engine[workflows.CardCreateState],
	workflow.Definition[workflows.CardCreateState],
	*agentusecases.CardCreateConfirmContinuer,
) {
	o11y := fake.NewProvider()

	store := workflowpg.NewPostgresStore(o11y, s.db)
	engine := workflow.NewEngine[workflows.CardCreateState](store, o11y)

	writeLedgerRepo := agentpersistence.NewWriteLedgerRepository(s.db, o11y)
	idem := agentusecases.NewIdempotentWrite(writeLedgerRepo, o11y)
	idemAdapter := cardCreateIntegrationIdemAdapter{uc: idem}

	threads := mempostgres.NewThreadRepository(s.db, o11y)
	runs := agentpostgres.NewRunStore(s.db)

	def := workflows.BuildCardCreateConfirmWorkflow(idemAdapter, cards)
	continuer := agentusecases.NewCardCreateConfirmContinuer(engine, def, threads, runs, o11y)

	return engine, def, continuer
}

func (s *CardCreateConfirmWorkflowIntegrationSuite) baseState(userID uuid.UUID, nickname, bank string, dueDay int) workflows.CardCreateState {
	return workflows.CardCreateState{
		Status:    workflows.CardCreateStatusActive,
		UserID:    userID,
		Nickname:  nickname,
		Bank:      bank,
		DueDay:    dueDay,
		MessageID: "wamid-start-" + uuid.NewString(),
	}
}

func (s *CardCreateConfirmWorkflowIntegrationSuite) TestInteg_RecognizedBank_AcceptCreatesCard_ClosingDayDerived() {
	userID := s.newUser()
	engine, def, continuer := s.buildEngineAndContinuer(s.cards)
	threadID := "peer-integ-" + uuid.NewString()
	nickname := "Nubank-" + uuid.NewString()[:8]

	key := workflows.CardCreateKey(userID.String())
	_, err := engine.Start(s.ctx, def, key, s.baseState(userID, nickname, "Nubank", 10))
	s.Require().NoError(err)

	wamid := "wamid-integ-" + uuid.NewString()
	handled, reply, contErr := continuer.Continue(s.ctx, userID.String(), threadID, "sim", wamid)
	s.Require().NoError(contErr)
	s.True(handled)
	s.Contains(reply, "✅")

	s.Equal(1, s.countCardsByNickname(userID, nickname))
	s.Equal(3, s.loadCardClosingDay(userID, nickname), "RF-07: fechamento derivado para Nubank (dueDay=10, daysBeforeDue=7) deve ser 3, ignorando qualquer valor informado")
}

func (s *CardCreateConfirmWorkflowIntegrationSuite) TestInteg_UnrecognizedBank_AcceptCreatesCard_ClosingDayExplicit() {
	userID := s.newUser()
	engine, def, continuer := s.buildEngineAndContinuer(s.cards)
	threadID := "peer-integ-" + uuid.NewString()
	nickname := "XP-" + uuid.NewString()[:8]

	key := workflows.CardCreateKey(userID.String())
	state := s.baseState(userID, nickname, "XP Desconhecida "+uuid.NewString()[:8], 1)
	state.ClosingDay = 20
	state.ClosingDayProvided = true
	_, err := engine.Start(s.ctx, def, key, state)
	s.Require().NoError(err)

	wamid := "wamid-integ-" + uuid.NewString()
	handled, reply, contErr := continuer.Continue(s.ctx, userID.String(), threadID, "sim", wamid)
	s.Require().NoError(contErr)
	s.True(handled)
	s.Contains(reply, "✅")

	s.Equal(1, s.countCardsByNickname(userID, nickname))
	s.Equal(20, s.loadCardClosingDay(userID, nickname), "RF-08: fechamento explícito informado pelo usuário deve ser gravado tal qual")
}

func (s *CardCreateConfirmWorkflowIntegrationSuite) TestInteg_ReplaySameWamid_DoesNotDuplicateCard() {
	userID := s.newUser()
	engine, def, continuer := s.buildEngineAndContinuer(s.cards)
	threadID := "peer-integ-" + uuid.NewString()
	nickname := "Nubank-" + uuid.NewString()[:8]

	key := workflows.CardCreateKey(userID.String())
	_, err := engine.Start(s.ctx, def, key, s.baseState(userID, nickname, "Nubank", 15))
	s.Require().NoError(err)

	wamid := "wamid-integ-replay-" + uuid.NewString()

	handled1, reply1, err1 := continuer.Continue(s.ctx, userID.String(), threadID, "sim", wamid)
	s.Require().NoError(err1)
	s.True(handled1)
	s.Contains(reply1, "✅")

	handled2, reply2, err2 := continuer.Continue(s.ctx, userID.String(), threadID, "sim", wamid)
	s.Require().NoError(err2)
	s.False(handled2, "RF-14: sem run suspenso após conclusão, o segundo Continue com o mesmo wamid deve ser no-op")
	s.Empty(reply2)

	s.Equal(1, s.countCardsByNickname(userID, nickname), "RF-14: replay do mesmo wamid não deve criar um segundo cartão")
}

func (s *CardCreateConfirmWorkflowIntegrationSuite) TestInteg_DuplicateNickname_ErrNicknameConflict_NoDuplicate() {
	userID := s.newUser()
	nickname := "Duplicado-" + uuid.NewString()[:8]

	_, createErr := s.cards.CreateCard(s.ctx, agentsifaces.NewCard{
		UserID:             userID,
		Nickname:           nickname,
		Bank:               "Nubank",
		DueDay:             10,
		ClosingDayProvided: false,
	})
	s.Require().NoError(createErr, "RF-12 setup: cartão ativo pré-existente com o apelido a ser duplicado")

	engine, def, continuer := s.buildEngineAndContinuer(s.cards)
	threadID := "peer-integ-" + uuid.NewString()

	key := workflows.CardCreateKey(userID.String())
	_, err := engine.Start(s.ctx, def, key, s.baseState(userID, nickname, "Nubank", 10))
	s.Require().NoError(err)

	wamid := "wamid-integ-" + uuid.NewString()
	handled, reply, contErr := continuer.Continue(s.ctx, userID.String(), threadID, "sim", wamid)
	s.Require().NoError(contErr, "RF-12: conflito de apelido é outcome de domínio, run conclui sem erro de infra")
	s.True(handled)
	s.Contains(reply, "apelido")

	s.Equal(1, s.countCardsByNickname(userID, nickname), "RF-12: nenhuma duplicata deve ser criada após o conflito")
}

func (s *CardCreateConfirmWorkflowIntegrationSuite) TestInteg_InfraFailure_RunErrorPopulated() {
	userID := s.newUser()
	failing := &cardCreateInfraFailingManager{CardManager: s.cards}
	engine, def, continuer := s.buildEngineAndContinuer(failing)
	threadID := "peer-integ-" + uuid.NewString()
	nickname := "Falha-" + uuid.NewString()[:8]

	key := workflows.CardCreateKey(userID.String())
	_, err := engine.Start(s.ctx, def, key, s.baseState(userID, nickname, "Nubank", 10))
	s.Require().NoError(err)

	wamid := "wamid-integ-fail-" + uuid.NewString()
	_, _, contErr := continuer.Continue(s.ctx, userID.String(), threadID, "sim", wamid)
	s.Require().Error(contErr, "RF-15: falha de infraestrutura deve propagar erro real, nunca ser engolida")

	var runError, runStatus string
	scanErr := s.db.QueryRowContext(s.ctx,
		`SELECT error, status FROM mecontrola.platform_runs WHERE resource_id = $1 AND thread_id = $2 ORDER BY started_at DESC LIMIT 1`,
		userID.String(), threadID,
	).Scan(&runError, &runStatus)
	s.Require().NoError(scanErr)
	s.NotEmpty(runError, "RF-15: platform_runs.error deve estar preenchido, nunca vazio")
	s.Equal("failed", runStatus)

	var lastError, wfStatus string
	scanErr = s.db.QueryRowContext(s.ctx,
		`SELECT last_error, status FROM mecontrola.workflow_runs WHERE workflow = $1 AND correlation_key = $2`,
		workflows.CardCreateConfirmWorkflowID, key,
	).Scan(&lastError, &wfStatus)
	s.Require().NoError(scanErr)
	s.NotEmpty(lastError, "RF-15: workflow_runs.last_error (mecanismo do kernel) deve estar preenchido")
	s.Equal("failed", wfStatus)

	s.Equal(0, s.countCardsByNickname(userID, nickname))
}

func (s *CardCreateConfirmWorkflowIntegrationSuite) TestInteg_MutualExclusion_SecondStartReturnsErrRunAlreadyExists() {
	userID := s.newUser()
	engine, def, _ := s.buildEngineAndContinuer(s.cards)
	nickname := "Mutex-" + uuid.NewString()[:8]

	key := workflows.CardCreateKey(userID.String())
	_, err := engine.Start(s.ctx, def, key, s.baseState(userID, nickname, "Nubank", 10))
	s.Require().NoError(err, "primeiro Start deve suspender aguardando confirmação")

	_, err2 := engine.Start(s.ctx, def, key, s.baseState(userID, (nickname+"-o"), "Nubank", 12))
	s.Require().Error(err2, "RF-18: segundo Start com a mesma key deve falhar por exclusão mútua")
	s.Require().True(errors.Is(err2, workflow.ErrRunAlreadyExists), "RF-18: erro deve ser ErrRunAlreadyExists")

	s.Equal(0, s.countCardsByNickname(userID, nickname))
	s.Equal(0, s.countCardsByNickname(userID, (nickname+"-o")))
}

func (s *CardCreateConfirmWorkflowIntegrationSuite) TestInteg_TTLExpired_CancelsWithoutCreatingCard() {
	userID := s.newUser()
	engine, def, continuer := s.buildEngineAndContinuer(s.cards)
	threadID := "peer-integ-" + uuid.NewString()
	nickname := "TTL-" + uuid.NewString()[:8]

	state := s.baseState(userID, nickname, "Nubank", 10)
	state.SuspendedAt = time.Now().UTC().Add(-16 * time.Minute)

	key := workflows.CardCreateKey(userID.String())
	_, err := engine.Start(s.ctx, def, key, state)
	s.Require().NoError(err)

	wamid := "wamid-integ-ttl-" + uuid.NewString()
	handled, reply, contErr := continuer.Continue(s.ctx, userID.String(), threadID, "sim", wamid)
	s.Require().NoError(contErr)
	s.False(handled, "RF-04: TTL expirado deve devolver handled=false para o texto seguir ao ParseInbound")
	s.Empty(reply)

	s.Equal(0, s.countCardsByNickname(userID, nickname), "RF-04: confirmação expirada não deve criar cartão")
}

func (s *CardCreateConfirmWorkflowIntegrationSuite) TestInteg_Reaper_RunOrfaoEncerraFailedNuncaSuspended() {
	userID := s.newUser()
	engine, def, _ := s.buildEngineAndContinuer(s.cards)
	nickname := "Orfao-" + uuid.NewString()[:8]

	key := workflows.CardCreateKey(userID.String())
	_, err := engine.Start(s.ctx, def, key, s.baseState(userID, nickname, "Nubank", 10))
	s.Require().NoError(err)

	_, execErr := s.db.ExecContext(s.ctx,
		`UPDATE mecontrola.workflow_runs SET updated_at = now() - interval '20 minutes' WHERE workflow = $1 AND correlation_key = $2`,
		workflows.CardCreateConfirmWorkflowID, key,
	)
	s.Require().NoError(execErr)

	o11y := fake.NewProvider()
	store := workflowpg.NewPostgresStore(o11y, s.db)
	reaper := workflow.NewStaleSuspendedReaper(store, workflows.CardCreateConfirmWorkflowID, 15*time.Minute, 100, o11y)

	reaped, reapErr := reaper.Reap(s.ctx)
	s.Require().NoError(reapErr)
	s.GreaterOrEqual(reaped, int64(1))

	var status string
	scanErr := s.db.QueryRowContext(s.ctx,
		`SELECT status FROM mecontrola.workflow_runs WHERE workflow = $1 AND correlation_key = $2`,
		workflows.CardCreateConfirmWorkflowID, key,
	).Scan(&status)
	s.Require().NoError(scanErr)
	s.Equal("failed", status, "RF-46: reaper deve marcar run órfão como failed, nunca permanecer suspended")

	s.Equal(0, s.countCardsByNickname(userID, nickname), "run órfão reapeado não deve criar cartão")
}

func (s *CardCreateConfirmWorkflowIntegrationSuite) TestInteg_ConcorrenciaConfirmacaoSimultanea_UmUnicoCartao() {
	userID := s.newUser()
	engine, def, _ := s.buildEngineAndContinuer(s.cards)
	threadID := "peer-concurrent-" + uuid.NewString()
	nickname := "Concorrente-" + uuid.NewString()[:8]

	key := workflows.CardCreateKey(userID.String())
	_, err := engine.Start(s.ctx, def, key, s.baseState(userID, nickname, "Nubank", 10))
	s.Require().NoError(err)

	const goroutines = 5
	type result struct {
		handled bool
		err     error
	}
	results := make(chan result, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_, _, localContinuer := s.buildEngineAndContinuer(s.cards)
			wamid := "wamid-concurrent-" + uuid.NewString()
			handled, _, contErr := localContinuer.Continue(s.ctx, userID.String(), threadID, "sim", wamid)
			results <- result{handled: handled, err: contErr}
		}()
	}
	wg.Wait()
	close(results)

	handledCount := 0
	for r := range results {
		if r.err != nil {
			s.Contains(r.err.Error(), "version conflict", "RF-45: única classe de erro aceitável em concorrência é o CAS otimista do kernel — perdedor nunca reexecuta")
			continue
		}
		if r.handled {
			handledCount++
		}
	}

	s.GreaterOrEqual(handledCount, 1, "ao menos uma goroutine deve concluir a confirmação")
	s.Equal(1, s.countCardsByNickname(userID, nickname), "concorrência sobre a mesma confirmação deve produzir exatamente 1 cartão")
}
