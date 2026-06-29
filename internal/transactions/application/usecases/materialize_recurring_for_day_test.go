package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	mockInterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces/mocks"
	uowMocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/option"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type MaterializeRecurringForDaySuite struct {
	suite.Suite
	ctx             context.Context
	factory         *mockInterfaces.RepositoryFactory
	templateRepo    *mockInterfaces.RecurringTemplateRepository
	materializeRepo *mockInterfaces.RecurringMaterializationRepository
	uow             *uowMocks.UnitOfWorkVoid
	txCreator       *uowMocks.TransactionCreator
	cardCreator     *uowMocks.CardPurchaseCreator
	brazilLoc       *time.Location
}

func TestMaterializeRecurringForDaySuite(t *testing.T) {
	suite.Run(t, new(MaterializeRecurringForDaySuite))
}

func (s *MaterializeRecurringForDaySuite) SetupTest() {
	s.ctx = context.Background()
	s.factory = mockInterfaces.NewRepositoryFactory(s.T())
	s.templateRepo = mockInterfaces.NewRecurringTemplateRepository(s.T())
	s.materializeRepo = mockInterfaces.NewRecurringMaterializationRepository(s.T())
	s.factory.EXPECT().RecurringTemplateRepository(mock.Anything).Return(s.templateRepo).Maybe()
	s.factory.EXPECT().RecurringMaterializationRepository(mock.Anything).Return(s.materializeRepo).Maybe()
	loc, _ := time.LoadLocation("America/Sao_Paulo")
	s.brazilLoc = loc
	s.uow = uowMocks.NewUnitOfWorkVoid(s.T())
	s.txCreator = uowMocks.NewTransactionCreator(s.T())
	s.cardCreator = uowMocks.NewCardPurchaseCreator(s.T())
}

func (s *MaterializeRecurringForDaySuite) referenceToday() time.Time {
	return time.Date(2026, time.January, 15, 12, 0, 0, 0, s.brazilLoc)
}

func (s *MaterializeRecurringForDaySuite) buildTemplate(userID uuid.UUID, day int, today time.Time) *entities.RecurringTemplate {
	dir := valueobjects.DirectionIncome
	pm := valueobjects.PaymentMethodPix
	amount, _ := valueobjects.NewMoney(5000)
	desc, _ := valueobjects.NewDescription("Salário")
	catID := valueobjects.CategoryIDFromUUID(uuid.New())
	freq := valueobjects.FrequencyMonthly
	dom, _ := valueobjects.NewDayOfMonth(day)
	inst, _ := valueobjects.NewInstallmentCount(1)
	now := time.Now().UTC()
	t := entities.NewRecurringTemplate(
		uuid.New(),
		valueobjects.UserIDFromUUID(userID),
		dir, pm,
		option.None[valueobjects.CardID](),
		amount, desc, catID,
		option.None[valueobjects.SubcategoryID](),
		"Receita", "",
		freq, dom, inst,
		today.Add(-24*time.Hour), option.None[time.Time](), now,
	)
	return &t
}

func (s *MaterializeRecurringForDaySuite) TestExecute_NoTemplatesForDay_ReturnsNil() {
	today := s.referenceToday()
	day := today.Day()

	s.templateRepo.EXPECT().
		FindActiveByDayOfMonth(mock.Anything, day, mock.Anything, interfaces.Cursor{}, 200).
		Return(nil, interfaces.Cursor{}, nil).Once()

	uc := NewMaterializeRecurringForDay(
		nil, s.factory, s.uow, services.RecurringWorkflow{}, nil, nil, s.brazilLoc, fake.NewProvider(),
	)

	err := uc.Execute(s.ctx, today.UTC())
	s.Require().NoError(err)
}

func (s *MaterializeRecurringForDaySuite) TestExecute_LockNotAcquired_SkipsTemplate() {
	today := s.referenceToday()
	day := today.Day()

	template := s.buildTemplate(uuid.New(), day, today)
	refMonth := valueobjects.RefMonthFromTime(today, s.brazilLoc)

	s.templateRepo.EXPECT().
		FindActiveByDayOfMonth(mock.Anything, day, mock.Anything, interfaces.Cursor{}, 200).
		Return([]*entities.RecurringTemplate{template}, interfaces.Cursor{}, nil).Once()

	s.materializeRepo.EXPECT().
		TryAdvisoryLock(mock.Anything, template.ID(), refMonth).
		Return(false, func() {}, nil).Once()

	uc := NewMaterializeRecurringForDay(
		nil, s.factory, s.uow, services.RecurringWorkflow{}, nil, nil, s.brazilLoc, fake.NewProvider(),
	)

	err := uc.Execute(s.ctx, today.UTC())
	s.Require().NoError(err)
}

func (s *MaterializeRecurringForDaySuite) TestExecute_AlreadyMaterialized_SkipsCreate() {
	today := s.referenceToday()
	day := today.Day()

	template := s.buildTemplate(uuid.New(), day, today)
	refMonth := valueobjects.RefMonthFromTime(today, s.brazilLoc)

	s.templateRepo.EXPECT().
		FindActiveByDayOfMonth(mock.Anything, day, mock.Anything, interfaces.Cursor{}, 200).
		Return([]*entities.RecurringTemplate{template}, interfaces.Cursor{}, nil).Once()

	s.materializeRepo.EXPECT().
		TryAdvisoryLock(mock.Anything, template.ID(), refMonth).
		Return(true, func() {}, nil).Once()

	s.materializeRepo.EXPECT().
		InsertIfAbsent(mock.Anything, template.ID(), refMonth, (*uuid.UUID)(nil), (*uuid.UUID)(nil), mock.Anything).
		Return(false, nil).Once()

	s.materializeRepo.EXPECT().
		IsCompleted(mock.Anything, template.ID(), refMonth).
		Return(true, nil).Once()

	uc := NewMaterializeRecurringForDay(
		nil, s.factory, s.uow, services.RecurringWorkflow{}, nil, nil, s.brazilLoc, fake.NewProvider(),
	)

	err := uc.Execute(s.ctx, today.UTC())
	s.Require().NoError(err)
}

func (s *MaterializeRecurringForDaySuite) TestExecute_MaterializesAsTransaction() {
	today := s.referenceToday()
	day := today.Day()

	template := s.buildTemplate(uuid.New(), day, today)
	refMonth := valueobjects.RefMonthFromTime(today, s.brazilLoc)

	s.templateRepo.EXPECT().
		FindActiveByDayOfMonth(mock.Anything, day, mock.Anything, interfaces.Cursor{}, 200).
		Return([]*entities.RecurringTemplate{template}, interfaces.Cursor{}, nil).Once()

	s.materializeRepo.EXPECT().
		TryAdvisoryLock(mock.Anything, template.ID(), refMonth).
		Return(true, func() {}, nil).Once()

	s.materializeRepo.EXPECT().
		InsertIfAbsent(mock.Anything, template.ID(), refMonth, (*uuid.UUID)(nil), (*uuid.UUID)(nil), mock.Anything).
		Return(true, nil).Once()

	txID := uuid.New()
	s.txCreator.On("Execute", mock.Anything, mock.Anything).
		Return(output.Transaction{ID: txID}, nil).Once()

	s.materializeRepo.EXPECT().
		MarkCompleted(mock.Anything, template.ID(), refMonth, mock.Anything, (*uuid.UUID)(nil)).
		Return(nil).Once()

	uc := NewMaterializeRecurringForDay(
		nil, s.factory, s.uow, services.RecurringWorkflow{}, s.txCreator, nil, s.brazilLoc, fake.NewProvider(),
	)

	err := uc.Execute(s.ctx, today.UTC())
	s.Require().NoError(err)
}

func (s *MaterializeRecurringForDaySuite) TestExecute_TransactionCreatorError_ReturnsError() {
	today := s.referenceToday()
	day := today.Day()

	template := s.buildTemplate(uuid.New(), day, today)
	refMonth := valueobjects.RefMonthFromTime(today, s.brazilLoc)

	s.templateRepo.EXPECT().
		FindActiveByDayOfMonth(mock.Anything, day, mock.Anything, interfaces.Cursor{}, 200).
		Return([]*entities.RecurringTemplate{template}, interfaces.Cursor{}, nil).Once()

	s.materializeRepo.EXPECT().
		TryAdvisoryLock(mock.Anything, template.ID(), refMonth).
		Return(true, func() {}, nil).Once()

	s.materializeRepo.EXPECT().
		InsertIfAbsent(mock.Anything, template.ID(), refMonth, (*uuid.UUID)(nil), (*uuid.UUID)(nil), mock.Anything).
		Return(true, nil).Once()

	s.txCreator.On("Execute", mock.Anything, mock.Anything).
		Return(output.Transaction{}, errors.New("create failed")).Once()

	uc := NewMaterializeRecurringForDay(
		nil, s.factory, s.uow, services.RecurringWorkflow{}, s.txCreator, nil, s.brazilLoc, fake.NewProvider(),
	)

	err := uc.Execute(s.ctx, today.UTC())
	s.Require().Error(err)
}

func (s *MaterializeRecurringForDaySuite) TestExecute_MaterializesAsCardPurchase() {
	today := s.referenceToday()
	day := today.Day()

	userID := uuid.New()
	cardID := valueobjects.CardIDFromUUID(uuid.New())
	dir := valueobjects.DirectionOutcome
	pm := valueobjects.PaymentMethodCreditCard
	amount, _ := valueobjects.NewMoney(5000)
	desc, _ := valueobjects.NewDescription("Assinatura")
	catID := valueobjects.CategoryIDFromUUID(uuid.New())
	freq := valueobjects.FrequencyMonthly
	dom, _ := valueobjects.NewDayOfMonth(day)
	inst, _ := valueobjects.NewInstallmentCount(1)
	now := time.Now().UTC()
	t := entities.NewRecurringTemplate(
		uuid.New(),
		valueobjects.UserIDFromUUID(userID),
		dir, pm,
		option.Some(cardID),
		amount, desc, catID,
		option.None[valueobjects.SubcategoryID](),
		"Despesa", "",
		freq, dom, inst,
		today.Add(-24*time.Hour), option.None[time.Time](), now,
	)
	template := &t
	refMonth := valueobjects.RefMonthFromTime(today, s.brazilLoc)

	s.templateRepo.EXPECT().
		FindActiveByDayOfMonth(mock.Anything, day, mock.Anything, interfaces.Cursor{}, 200).
		Return([]*entities.RecurringTemplate{template}, interfaces.Cursor{}, nil).Once()

	s.materializeRepo.EXPECT().
		TryAdvisoryLock(mock.Anything, template.ID(), refMonth).
		Return(true, func() {}, nil).Once()

	s.materializeRepo.EXPECT().
		InsertIfAbsent(mock.Anything, template.ID(), refMonth, (*uuid.UUID)(nil), (*uuid.UUID)(nil), mock.Anything).
		Return(true, nil).Once()

	purchaseID := uuid.New()
	s.cardCreator.On("Execute", mock.Anything, mock.Anything).
		Return(output.CardPurchase{ID: purchaseID}, nil).Once()

	s.materializeRepo.EXPECT().
		MarkCompleted(mock.Anything, template.ID(), refMonth, (*uuid.UUID)(nil), mock.Anything).
		Return(nil).Once()

	uc := NewMaterializeRecurringForDay(
		nil, s.factory, s.uow, services.RecurringWorkflow{}, nil, s.cardCreator, s.brazilLoc, fake.NewProvider(),
	)

	err := uc.Execute(s.ctx, today.UTC())
	s.Require().NoError(err)
}

func (s *MaterializeRecurringForDaySuite) TestExecute_CardPurchaseCreatorError() {
	today := s.referenceToday()
	day := today.Day()

	userID := uuid.New()
	cardID := valueobjects.CardIDFromUUID(uuid.New())
	dir := valueobjects.DirectionOutcome
	pm := valueobjects.PaymentMethodCreditCard
	amount, _ := valueobjects.NewMoney(5000)
	desc, _ := valueobjects.NewDescription("Assinatura")
	catID := valueobjects.CategoryIDFromUUID(uuid.New())
	freq := valueobjects.FrequencyMonthly
	dom, _ := valueobjects.NewDayOfMonth(day)
	inst, _ := valueobjects.NewInstallmentCount(1)
	now := time.Now().UTC()
	t := entities.NewRecurringTemplate(
		uuid.New(),
		valueobjects.UserIDFromUUID(userID),
		dir, pm,
		option.Some(cardID),
		amount, desc, catID,
		option.None[valueobjects.SubcategoryID](),
		"Despesa", "",
		freq, dom, inst,
		today.Add(-24*time.Hour), option.None[time.Time](), now,
	)
	template := &t
	refMonth := valueobjects.RefMonthFromTime(today, s.brazilLoc)

	s.templateRepo.EXPECT().
		FindActiveByDayOfMonth(mock.Anything, day, mock.Anything, interfaces.Cursor{}, 200).
		Return([]*entities.RecurringTemplate{template}, interfaces.Cursor{}, nil).Once()

	s.materializeRepo.EXPECT().
		TryAdvisoryLock(mock.Anything, template.ID(), refMonth).
		Return(true, func() {}, nil).Once()

	s.materializeRepo.EXPECT().
		InsertIfAbsent(mock.Anything, template.ID(), refMonth, (*uuid.UUID)(nil), (*uuid.UUID)(nil), mock.Anything).
		Return(true, nil).Once()

	s.cardCreator.On("Execute", mock.Anything, mock.Anything).
		Return(output.CardPurchase{}, errors.New("card create failed")).Once()

	uc := NewMaterializeRecurringForDay(
		nil, s.factory, s.uow, services.RecurringWorkflow{}, nil, s.cardCreator, s.brazilLoc, fake.NewProvider(),
	)

	err := uc.Execute(s.ctx, today.UTC())
	s.Require().Error(err)
}
