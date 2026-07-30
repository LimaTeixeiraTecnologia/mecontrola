package binding

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	agentsifaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	mockInterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces/mocks"
	txusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases"
	uowMocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/option"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

var errRecurringTemplateNotFound = errors.New("template não encontrado")

type RecurrenceManagerAdapterSuite struct {
	suite.Suite
	ctx        context.Context
	userID     uuid.UUID
	templateID uuid.UUID
	catID      uuid.UUID
	subID      uuid.UUID
	factory    *mockInterfaces.RepositoryFactory
	repo       *mockInterfaces.RecurringTemplateRepository
	catVal     *mockInterfaces.CategoryValidator
	catGate    *mockInterfaces.CategoryWriteGate
	adapter    agentsifaces.RecurrenceManager
}

func TestRecurrenceManagerAdapterSuite(t *testing.T) {
	suite.Run(t, new(RecurrenceManagerAdapterSuite))
}

func (s *RecurrenceManagerAdapterSuite) SetupTest() {
	s.userID = uuid.New()
	s.templateID = uuid.New()
	s.catID = uuid.New()
	s.subID = uuid.New()
	s.ctx = auth.WithPrincipal(context.Background(), auth.Principal{UserID: s.userID, Source: auth.SourceWhatsApp})
	s.factory = mockInterfaces.NewRepositoryFactory(s.T())
	s.repo = mockInterfaces.NewRecurringTemplateRepository(s.T())
	s.factory.EXPECT().RecurringTemplateRepository(mock.Anything).Return(s.repo).Maybe()
	s.catVal = mockInterfaces.NewCategoryValidator(s.T())
	s.catGate = mockInterfaces.NewCategoryWriteGate(s.T())
	uow := uowMocks.NewUnitOfWorkRecurringTemplate(s.T())
	getRT := txusecases.NewGetRecurringTemplate(s.factory, uow, fake.NewProvider())
	updateRT := txusecases.NewUpdateRecurringTemplate(s.factory, uow, s.catVal, s.catGate, fake.NewProvider())
	s.adapter = NewRecurrenceManagerAdapter(nil, updateRT, nil, nil, getRT, fake.NewProvider())
}

func (s *RecurrenceManagerAdapterSuite) existingTemplate() *entities.RecurringTemplate {
	amount, _ := valueobjects.NewMoney(150000)
	desc, _ := valueobjects.NewDescription("Aluguel")
	dom, _ := valueobjects.NewDayOfMonth(5)
	inst, _ := valueobjects.NewInstallmentCount(1)
	now := time.Now().UTC()
	t := entities.NewRecurringTemplate(
		s.templateID,
		valueobjects.UserIDFromUUID(s.userID),
		valueobjects.DirectionOutcome,
		valueobjects.PaymentMethodPix,
		option.None[valueobjects.CardID](),
		amount, desc,
		valueobjects.CategoryIDFromUUID(s.catID),
		option.Some(valueobjects.SubcategoryIDFromUUID(s.subID)),
		"Custo Fixo", "Aluguel",
		valueobjects.CategoryWriteEvidence{},
		valueobjects.FrequencyMonthly,
		dom, inst,
		now, option.None[time.Time](), now,
	)
	return &t
}

func (s *RecurrenceManagerAdapterSuite) TestUpdateRecurrence_MergesPartialPayloadWithCurrentTemplate() {
	existing := s.existingTemplate()
	catSnap := interfaces.CategorySnapshot{ID: s.catID, Name: "Custo Fixo"}
	newAmount := int64(160000)

	s.repo.EXPECT().GetByID(mock.Anything, s.templateID, s.userID).Return(existing, nil).Twice()
	s.catVal.EXPECT().Validate(mock.Anything, s.catID, mock.Anything).Return(catSnap, nil).Once()
	s.catGate.EXPECT().Approve(mock.Anything, mock.Anything).Return(valueobjects.CategoryWriteEvidence{}, nil).Once()

	var updated *entities.RecurringTemplate
	s.repo.EXPECT().UpdateWithVersion(mock.Anything, mock.Anything, mock.AnythingOfType("int64")).
		Run(func(_ context.Context, t *entities.RecurringTemplate, _ int64) {
			updated = t
		}).Return(nil).Once()

	ref, err := s.adapter.UpdateRecurrence(s.ctx, s.templateID.String(), agentsifaces.RawUpdateRecurrence{
		AmountCents: &newAmount,
		Version:     1,
	})

	s.Require().NoError(err)
	s.Equal(s.templateID, ref.ID)
	s.Require().NotNil(updated)
	s.Equal(int64(160000), updated.Amount().Cents())
	s.Equal("Aluguel", updated.Description().String())
	s.Equal(5, updated.DayOfMonth().Value())
	s.Equal(valueobjects.DirectionOutcome, updated.Direction())
	s.Equal(valueobjects.PaymentMethodPix, updated.PaymentMethod())
	s.Equal(valueobjects.FrequencyMonthly, updated.Frequency())
}

func (s *RecurrenceManagerAdapterSuite) TestUpdateRecurrence_PropagatesGetError() {
	s.repo.EXPECT().GetByID(mock.Anything, s.templateID, s.userID).Return(nil, errRecurringTemplateNotFound).Once()

	_, err := s.adapter.UpdateRecurrence(s.ctx, s.templateID.String(), agentsifaces.RawUpdateRecurrence{Version: 1})

	s.Require().Error(err)
	s.ErrorIs(err, errRecurringTemplateNotFound)
}
