//go:build integration

package binding_test

import (
	"context"
	"sync"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	agentsifaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	agentusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/infrastructure/binding"
	agentpersistence "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/infrastructure/persistence"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
	txifaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	txusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
	txrepos "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/repositories"
)

type ca09TxPublisher struct{}

func (p *ca09TxPublisher) PublishCreated(_ context.Context, _ database.DBTX, _ entities.TransactionCreated) error {
	return nil
}
func (p *ca09TxPublisher) PublishUpdated(_ context.Context, _ database.DBTX, _ entities.TransactionUpdated) error {
	return nil
}
func (p *ca09TxPublisher) PublishDeleted(_ context.Context, _ database.DBTX, _ entities.TransactionDeleted) error {
	return nil
}

type ca09CategoryValidator struct{ catID uuid.UUID }

func (v *ca09CategoryValidator) Validate(_ context.Context, _ uuid.UUID, _ *uuid.UUID) (txifaces.CategorySnapshot, error) {
	return txifaces.CategorySnapshot{ID: v.catID, Name: "Alimentação"}, nil
}

type CA09ReconciledIntegrationSuite struct {
	suite.Suite
	ctx        context.Context
	db         database.DBTX
	adapter    agentsifaces.TransactionsLedger
	ledgerRepo agentusecases.WriteLedgerRepository
	idemUC     *agentusecases.IdempotentWrite
}

func TestCA09ReconciledIntegrationSuite(t *testing.T) {
	suite.Run(t, new(CA09ReconciledIntegrationSuite))
}

func (s *CA09ReconciledIntegrationSuite) SetupSuite() {
	s.ctx = context.Background()
	db, _ := testcontainer.Postgres(s.T())
	s.db = db
	o11y := fake.NewProvider()
	factory := txrepos.NewRepositoryFactory(o11y)
	catID := uuid.New()

	userID := uuid.New()
	_, err := db.ExecContext(s.ctx, `
		INSERT INTO mecontrola.users (id, whatsapp_number, status, created_at, updated_at)
		VALUES ($1, '+5511900000002', 'ACTIVE', now(), now())`, userID)
	s.Require().NoError(err)

	createTx := txusecases.NewCreateTransaction(
		factory,
		uow.NewUnitOfWork(db),
		&ca09CategoryValidator{catID: catID},
		services.TransactionWorkflow{},
		&ca09TxPublisher{},
		o11y,
	)

	snapshot, err := valueobjects.NewCardBillingSnapshot(20, 25)
	s.Require().NoError(err)
	cpWorkflow := services.NewCardPurchaseWorkflow()
	createCP := txusecases.NewCreateCardPurchase(
		factory,
		&stubCardLookup{snapshot: snapshot},
		&stubCategoryValidator{catID: catID},
		&cpWorkflow,
		&noopCPPublisher{},
		uow.NewUnitOfWork(db),
		id.NewUUIDGenerator(),
		o11y,
	)

	getMS := txusecases.NewGetMonthlySummary(factory, uow.NewUnitOfWork(db), o11y)
	listME := txusecases.NewListMonthlyEntries(factory, uow.NewUnitOfWork(db), o11y)

	s.adapter = binding.NewTransactionsLedgerAdapter(
		createTx, createCP, nil, nil, nil, nil, listME, getMS, o11y,
	)

	s.ledgerRepo = agentpersistence.NewWriteLedgerRepository(db, o11y)
	s.idemUC = agentusecases.NewIdempotentWrite(s.ledgerRepo, o11y)
}

func (s *CA09ReconciledIntegrationSuite) authedCtx(userID uuid.UUID) context.Context {
	return auth.WithPrincipal(s.ctx, auth.Principal{UserID: userID, Source: auth.SourceWhatsApp})
}

func (s *CA09ReconciledIntegrationSuite) TestCA09_ConcurrentSameOriginReturnsReconciledNeverUsecaseError() {
	userID := uuid.New()
	_, err := s.db.ExecContext(s.ctx, `
		INSERT INTO mecontrola.users (id, whatsapp_number, status, created_at, updated_at)
		VALUES ($1, $2, 'ACTIVE', now(), now())`, userID, "+5511900000003-"+uuid.NewString())
	s.Require().NoError(err)

	ctx := s.authedCtx(userID)
	catID := uuid.New()
	wamid := "wamid-ca09-" + uuid.NewString()

	const goroutines = 5
	var wg sync.WaitGroup
	outcomes := make([]agent.ToolOutcome, goroutines)
	errs := make([]error, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			res, writeErr := s.idemUC.Execute(ctx, userID, wamid, 0, "create_transaction", "transaction",
				func(innerCtx context.Context) (uuid.UUID, bool, error) {
					ref, createErr := s.adapter.CreateTransaction(innerCtx, agentsifaces.RawTransaction{
						Direction:       "income",
						PaymentMethod:   "pix",
						AmountCents:     1000,
						Description:     "renda ca09",
						CategoryID:      catID,
						OccurredAt:      "2026-07-01",
						OriginWamid:     wamid,
						OriginItemSeq:   0,
						OriginOperation: "create_transaction",
					})
					if createErr != nil {
						return uuid.Nil, false, createErr
					}
					return ref.ID, ref.Reconciled, nil
				},
			)
			outcomes[idx] = res.Outcome
			errs[idx] = writeErr
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		s.NoError(err, "CA-09: goroutine %d must not return error", i)
		s.NotEqual(agent.ToolOutcomeUsecaseError, outcomes[i],
			"CA-09: goroutine %d must never return usecaseError", i)
	}

	var count int
	err = s.db.QueryRowContext(ctx,
		`SELECT count(*) FROM mecontrola.transactions WHERE user_id = $1 AND deleted_at IS NULL`,
		userID,
	).Scan(&count)
	s.Require().NoError(err)
	s.Equal(1, count, "CA-09: exactly 1 domain mutation despite concurrent writes")

	var ledgerCount int
	err = s.db.QueryRowContext(ctx,
		`SELECT count(*) FROM mecontrola.agents_write_ledger WHERE wamid = $1`,
		wamid,
	).Scan(&ledgerCount)
	s.Require().NoError(err)
	s.Equal(1, ledgerCount, "CA-09: exactly 1 ledger entry for the wamid")
}

func (s *CA09ReconciledIntegrationSuite) TestCA09_ReconciledOutcomeMapsCorrectly() {
	userID := uuid.New()
	_, err := s.db.ExecContext(s.ctx, `
		INSERT INTO mecontrola.users (id, whatsapp_number, status, created_at, updated_at)
		VALUES ($1, $2, 'ACTIVE', now(), now())`, userID, "+5511900000004-"+uuid.NewString())
	s.Require().NoError(err)

	ctx := s.authedCtx(userID)
	catID := uuid.New()
	wamid := "wamid-ca09-rec-" + uuid.NewString()

	write := func() (agentusecases.IdempotentWriteResult, error) {
		return s.idemUC.Execute(ctx, userID, wamid, 0, "create_transaction", "transaction",
			func(innerCtx context.Context) (uuid.UUID, bool, error) {
				ref, createErr := s.adapter.CreateTransaction(innerCtx, agentsifaces.RawTransaction{
					Direction:       "income",
					PaymentMethod:   "pix",
					AmountCents:     2000,
					Description:     "renda rec",
					CategoryID:      catID,
					OccurredAt:      "2026-07-01",
					OriginWamid:     wamid,
					OriginItemSeq:   0,
					OriginOperation: "create_transaction",
				})
				if createErr != nil {
					return uuid.Nil, false, createErr
				}
				return ref.ID, ref.Reconciled, nil
			},
		)
	}

	r1, err := write()
	s.Require().NoError(err)
	s.Equal(agent.ToolOutcomeRouted, r1.Outcome, "first write must be routed")

	r2, err := write()
	s.Require().NoError(err)
	s.Equal(agent.ToolOutcomeReplay, r2.Outcome, "CA-09: second write must be replay via ledger")
	s.Equal(r1.ResourceID, r2.ResourceID, "CA-09: replay must return same resourceID")
	s.NotEqual(agent.ToolOutcomeUsecaseError, r2.Outcome, "CA-09: must never be usecaseError")
}
