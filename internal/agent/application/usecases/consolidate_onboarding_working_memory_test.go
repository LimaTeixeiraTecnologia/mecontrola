package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	agentinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	agententities "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/entities"
	onbusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

type stubContextReaderForWM struct {
	result onbusecases.GetOnboardingContextResult
	err    error
}

func (r *stubContextReaderForWM) Execute(_ context.Context, _ onbusecases.GetOnboardingContextInput) (onbusecases.GetOnboardingContextResult, error) {
	return r.result, r.err
}

type stubWMRepoForUC struct {
	getResult agententities.WorkingMemory
	getFound  bool
	getErr    error
	upsertErr error
	upserted  *agententities.WorkingMemory
}

func (r *stubWMRepoForUC) Get(_ context.Context, _ uuid.UUID) (agententities.WorkingMemory, bool, error) {
	return r.getResult, r.getFound, r.getErr
}

func (r *stubWMRepoForUC) Upsert(_ context.Context, wm agententities.WorkingMemory) error {
	r.upserted = &wm
	return r.upsertErr
}

type stubWMRepoFactoryForUC struct {
	repo *stubWMRepoForUC
}

func (f *stubWMRepoFactoryForUC) WorkingMemoryRepository(_ database.DBTX) agentinterfaces.WorkingMemoryRepository {
	return f.repo
}

type stubProcessedEventRepoForUC struct {
	isProcessed    bool
	isProcessedErr error
	markErr        error
	marked         []uuid.UUID
}

func (r *stubProcessedEventRepoForUC) IsProcessed(_ context.Context, _ uuid.UUID) (bool, error) {
	return r.isProcessed, r.isProcessedErr
}

func (r *stubProcessedEventRepoForUC) MarkProcessed(_ context.Context, eventID uuid.UUID, _ string, _ uuid.UUID, _ time.Time) error {
	r.marked = append(r.marked, eventID)
	return r.markErr
}

type stubProcessedEventFactoryForUC struct {
	repo *stubProcessedEventRepoForUC
}

func (f *stubProcessedEventFactoryForUC) ProcessedEventRepository(_ database.DBTX) agentinterfaces.ProcessedEventRepository {
	return f.repo
}

type uowStubForWM struct{}

func (u *uowStubForWM) DBTX() database.DBTX { return nil }

func (u *uowStubForWM) Do(_ context.Context, fn func(context.Context, database.DBTX) error) error {
	return fn(context.Background(), nil)
}

func TestConsolidateOnboardingWorkingMemorySuite(t *testing.T) {
	suite.Run(t, new(ConsolidateOnboardingWorkingMemorySuite))
}

type ConsolidateOnboardingWorkingMemorySuite struct {
	suite.Suite
	ctx context.Context
}

func (s *ConsolidateOnboardingWorkingMemorySuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *ConsolidateOnboardingWorkingMemorySuite) newSUT(
	reader *stubContextReaderForWM,
	wmRepo *stubWMRepoForUC,
	processedRepo *stubProcessedEventRepoForUC,
	o11y observability.Observability,
) *ConsolidateOnboardingWorkingMemory {
	return NewConsolidateOnboardingWorkingMemory(
		&uowStubForWM{},
		reader,
		&stubWMRepoFactoryForUC{repo: wmRepo},
		&stubProcessedEventFactoryForUC{repo: processedRepo},
		o11y,
	)
}

func (s *ConsolidateOnboardingWorkingMemorySuite) TestExecute_CreatesWorkingMemoryAndMarksProcessed() {
	reader := &stubContextReaderForWM{
		result: onbusecases.GetOnboardingContextResult{
			Found:       true,
			Objective:   "quitar dívidas",
			IncomeCents: 500000,
			CustomSplit: []onbusecases.OnboardingAllocationView{
				{Kind: "fixed_cost", BasisPoints: 4000},
				{Kind: "knowledge", BasisPoints: 1000},
				{Kind: "pleasures", BasisPoints: 1500},
				{Kind: "goals", BasisPoints: 2000},
				{Kind: "financial_freedom", BasisPoints: 1500},
			},
		},
	}
	wmRepo := &stubWMRepoForUC{getFound: false}
	processedRepo := &stubProcessedEventRepoForUC{}
	userID := uuid.New()
	eventID := uuid.New()
	occurredAt := time.Now().UTC().Truncate(time.Second)

	sut := s.newSUT(reader, wmRepo, processedRepo, fake.NewProvider())
	err := sut.Execute(s.ctx, ConsolidateOnboardingWorkingMemoryInput{
		UserID:     userID,
		EventID:    eventID,
		EventType:  "onboarding.completed",
		OccurredAt: occurredAt,
	})

	s.NoError(err)
	s.NotNil(wmRepo.upserted)
	s.Contains(wmRepo.upserted.Content, "quitar dívidas")
	s.Contains(wmRepo.upserted.Content, "R$ 5.000,00")
	s.Contains(wmRepo.upserted.Content, "💰 Custo Fixo")
	s.Contains(wmRepo.upserted.Content, "40,00%")
	s.Equal(occurredAt, wmRepo.upserted.UpdatedAt)
	s.Len(processedRepo.marked, 1)
	s.Equal(eventID, processedRepo.marked[0])
}

func (s *ConsolidateOnboardingWorkingMemorySuite) TestExecute_SessionNotFound_ReturnsNil() {
	reader := &stubContextReaderForWM{result: onbusecases.GetOnboardingContextResult{Found: false}}
	wmRepo := &stubWMRepoForUC{}
	processedRepo := &stubProcessedEventRepoForUC{}

	sut := s.newSUT(reader, wmRepo, processedRepo, fake.NewProvider())
	err := sut.Execute(s.ctx, ConsolidateOnboardingWorkingMemoryInput{
		UserID:  uuid.New(),
		EventID: uuid.New(),
	})

	s.NoError(err)
	s.Nil(wmRepo.upserted)
	s.Empty(processedRepo.marked)
}

func (s *ConsolidateOnboardingWorkingMemorySuite) TestExecute_AlreadyProcessed_SkipsUpsert() {
	reader := &stubContextReaderForWM{
		result: onbusecases.GetOnboardingContextResult{
			Found:       true,
			Objective:   "investir",
			IncomeCents: 800000,
		},
	}
	wmRepo := &stubWMRepoForUC{}
	processedRepo := &stubProcessedEventRepoForUC{isProcessed: true}

	sut := s.newSUT(reader, wmRepo, processedRepo, fake.NewProvider())
	err := sut.Execute(s.ctx, ConsolidateOnboardingWorkingMemoryInput{
		UserID:  uuid.New(),
		EventID: uuid.New(),
	})

	s.NoError(err)
	s.Nil(wmRepo.upserted)
	s.Empty(processedRepo.marked)
}

func (s *ConsolidateOnboardingWorkingMemorySuite) TestExecute_ExistingWorkingMemory_DoesNotOverwrite() {
	reader := &stubContextReaderForWM{
		result: onbusecases.GetOnboardingContextResult{
			Found:       true,
			Objective:   "novo objetivo",
			IncomeCents: 100000,
		},
	}
	existing := agententities.NewWorkingMemory(uuid.New())
	existing.Content = "conteúdo existente"
	wmRepo := &stubWMRepoForUC{getFound: true, getResult: existing}
	processedRepo := &stubProcessedEventRepoForUC{}

	sut := s.newSUT(reader, wmRepo, processedRepo, fake.NewProvider())
	err := sut.Execute(s.ctx, ConsolidateOnboardingWorkingMemoryInput{
		UserID:  uuid.New(),
		EventID: uuid.New(),
	})

	s.NoError(err)
	s.Nil(wmRepo.upserted)
	s.Len(processedRepo.marked, 1)
}

func (s *ConsolidateOnboardingWorkingMemorySuite) TestExecute_MissingUserID_ReturnsError() {
	sut := s.newSUT(&stubContextReaderForWM{}, &stubWMRepoForUC{}, &stubProcessedEventRepoForUC{}, fake.NewProvider())
	err := sut.Execute(s.ctx, ConsolidateOnboardingWorkingMemoryInput{
		EventID: uuid.New(),
	})
	s.Error(err)
}

func (s *ConsolidateOnboardingWorkingMemorySuite) TestExecute_MissingEventID_ReturnsError() {
	sut := s.newSUT(&stubContextReaderForWM{}, &stubWMRepoForUC{}, &stubProcessedEventRepoForUC{}, fake.NewProvider())
	err := sut.Execute(s.ctx, ConsolidateOnboardingWorkingMemoryInput{
		UserID: uuid.New(),
	})
	s.Error(err)
}

func (s *ConsolidateOnboardingWorkingMemorySuite) TestExecute_ContextReaderError_ReturnsError() {
	reader := &stubContextReaderForWM{err: errors.New("db error")}
	sut := s.newSUT(reader, &stubWMRepoForUC{}, &stubProcessedEventRepoForUC{}, fake.NewProvider())
	err := sut.Execute(s.ctx, ConsolidateOnboardingWorkingMemoryInput{
		UserID:  uuid.New(),
		EventID: uuid.New(),
	})
	s.Error(err)
}

func (s *ConsolidateOnboardingWorkingMemorySuite) TestExecute_WorkingMemoryUpsertError_ReturnsError() {
	reader := &stubContextReaderForWM{
		result: onbusecases.GetOnboardingContextResult{
			Found:       true,
			Objective:   "objetivo",
			IncomeCents: 100000,
		},
	}
	wmRepo := &stubWMRepoForUC{getFound: false, upsertErr: errors.New("upsert failed")}
	processedRepo := &stubProcessedEventRepoForUC{}

	sut := s.newSUT(reader, wmRepo, processedRepo, fake.NewProvider())
	err := sut.Execute(s.ctx, ConsolidateOnboardingWorkingMemoryInput{
		UserID:  uuid.New(),
		EventID: uuid.New(),
	})
	s.Error(err)
	s.Empty(processedRepo.marked)
}

func (s *ConsolidateOnboardingWorkingMemorySuite) TestExecute_MarkProcessedConflict_TreatedAsProcessed() {
	reader := &stubContextReaderForWM{
		result: onbusecases.GetOnboardingContextResult{
			Found:       true,
			Objective:   "objetivo",
			IncomeCents: 100000,
		},
	}
	wmRepo := &stubWMRepoForUC{getFound: false}
	processedRepo := &stubProcessedEventRepoForUC{markErr: agentinterfaces.ErrProcessedEventAlreadyExists}

	sut := s.newSUT(reader, wmRepo, processedRepo, fake.NewProvider())
	err := sut.Execute(s.ctx, ConsolidateOnboardingWorkingMemoryInput{
		UserID:  uuid.New(),
		EventID: uuid.New(),
	})
	s.NoError(err)
	s.NotNil(wmRepo.upserted)
}

func TestBuildOnboardingWorkingMemory(t *testing.T) {
	snapshot := onbusecases.GetOnboardingContextResult{
		Objective:   "viajar",
		IncomeCents: 300000,
		Cards: []onbusecases.OnboardingCardView{
			{Name: "nubank"},
		},
		CustomSplit: []onbusecases.OnboardingAllocationView{
			{Kind: "fixed_cost", BasisPoints: 5000},
		},
	}

	content := buildOnboardingWorkingMemory(snapshot)

	require.Contains(t, content, "viajar")
	require.Contains(t, content, "R$ 3.000,00")
	require.Contains(t, content, "nubank")
	require.Contains(t, content, "💰 Custo Fixo")
	require.Contains(t, content, "50,00%")
}
