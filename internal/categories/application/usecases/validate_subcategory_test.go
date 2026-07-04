package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/interfaces"
	mockInterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

type ValidateSubcategorySuite struct {
	suite.Suite
	ctx     context.Context
	obs     observability.Observability
	repo    *mockInterfaces.CategoryRepository
	useCase *ValidateSubcategory
}

func TestValidateSubcategorySuite(t *testing.T) {
	suite.Run(t, new(ValidateSubcategorySuite))
}

func (s *ValidateSubcategorySuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.repo = mockInterfaces.NewCategoryRepository(s.T())
	s.useCase = NewValidateSubcategory(s.repo, s.obs)
}

func (s *ValidateSubcategorySuite) TestExecute_SubcategoriaAtiva() {
	subID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	rootID := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	s.repo.EXPECT().GetByID(mock.Anything, subID).Return(entities.Category{
		ID:             subID,
		ParentID:       &rootID,
		Slug:           "aluguel",
		Name:           "Aluguel",
		Kind:           valueobjects.KindExpense,
		AllocationType: valueobjects.AllocationTypeConsumption,
	}, nil).Once()

	s.repo.EXPECT().GetByID(mock.Anything, rootID).Return(entities.Category{
		ID:             rootID,
		Slug:           "custo-fixo",
		Name:           "Custo Fixo",
		Kind:           valueobjects.KindExpense,
		AllocationType: valueobjects.AllocationTypeConsumption,
	}, nil).Once()

	result, err := s.useCase.Execute(s.ctx, subID, rootID)

	s.NoError(err)
	s.Equal("expense.custo_fixo", result.ParentSlug)
	s.Equal("Aluguel", result.CategoryName)
	s.Equal("Custo Fixo", result.ParentName)
	s.Equal("expense", result.Kind)
	s.False(result.Deprecated)
}

func (s *ValidateSubcategorySuite) TestExecute_FilhaDireta_ExpectedParentNil() {
	subID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	rootID := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	s.repo.EXPECT().GetByID(mock.Anything, subID).Return(entities.Category{
		ID:             subID,
		ParentID:       &rootID,
		Slug:           "aluguel",
		Name:           "Aluguel",
		Kind:           valueobjects.KindExpense,
		AllocationType: valueobjects.AllocationTypeConsumption,
	}, nil).Once()

	s.repo.EXPECT().GetByID(mock.Anything, rootID).Return(entities.Category{
		ID:             rootID,
		Slug:           "custo-fixo",
		Name:           "Custo Fixo",
		Kind:           valueobjects.KindExpense,
		AllocationType: valueobjects.AllocationTypeConsumption,
	}, nil).Once()

	result, err := s.useCase.Execute(s.ctx, subID, uuid.Nil)

	s.NoError(err)
	s.Equal("expense.custo_fixo", result.ParentSlug)
}

func (s *ValidateSubcategorySuite) TestExecute_SubcategoriaDeOutraRaiz() {
	subID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	rootID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	otherRootID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	s.repo.EXPECT().GetByID(mock.Anything, subID).Return(entities.Category{
		ID:             subID,
		ParentID:       &rootID,
		Slug:           "aluguel",
		Name:           "Aluguel",
		Kind:           valueobjects.KindExpense,
		AllocationType: valueobjects.AllocationTypeConsumption,
	}, nil).Once()

	result, err := s.useCase.Execute(s.ctx, subID, otherRootID)

	s.ErrorIs(err, ErrSubcategoryNotDirectChild)
	s.Equal(ValidateSubcategoryResult{}, result)
}

func (s *ValidateSubcategorySuite) TestExecute_SubcategoriaDeprecada() {
	subID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	rootID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	deprecatedAt := time.Now()

	s.repo.EXPECT().GetByID(mock.Anything, subID).Return(entities.Category{
		ID:             subID,
		ParentID:       &rootID,
		Slug:           "aluguel",
		Name:           "Aluguel",
		Kind:           valueobjects.KindExpense,
		AllocationType: valueobjects.AllocationTypeConsumption,
		DeprecatedAt:   &deprecatedAt,
	}, nil).Once()

	s.repo.EXPECT().GetByID(mock.Anything, rootID).Return(entities.Category{
		ID:             rootID,
		Slug:           "custo-fixo",
		Name:           "Custo Fixo",
		Kind:           valueobjects.KindExpense,
		AllocationType: valueobjects.AllocationTypeConsumption,
	}, nil).Once()

	result, err := s.useCase.Execute(s.ctx, subID, rootID)

	s.NoError(err)
	s.True(result.Deprecated)
}

func (s *ValidateSubcategorySuite) TestExecute_CategoriaEhRaiz() {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	s.repo.EXPECT().GetByID(mock.Anything, id).Return(entities.Category{
		ID:             id,
		Slug:           "custo-fixo",
		Kind:           valueobjects.KindExpense,
		AllocationType: valueobjects.AllocationTypeConsumption,
	}, nil).Once()

	result, err := s.useCase.Execute(s.ctx, id, uuid.Nil)

	s.ErrorIs(err, ErrSubcategoryNotRoot)
	s.Equal(ValidateSubcategoryResult{}, result)
}

func (s *ValidateSubcategorySuite) TestExecute_CategoriaNaoEncontrada() {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	s.repo.EXPECT().GetByID(mock.Anything, id).Return(entities.Category{}, interfaces.ErrNotFound).Once()

	_, err := s.useCase.Execute(s.ctx, id, uuid.Nil)

	s.ErrorIs(err, ErrCategoryNotFound)
}

func (s *ValidateSubcategorySuite) TestExecute_PaiNaoEncontrado() {
	subID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	rootID := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	s.repo.EXPECT().GetByID(mock.Anything, subID).Return(entities.Category{
		ID:             subID,
		ParentID:       &rootID,
		Slug:           "aluguel",
		Kind:           valueobjects.KindExpense,
		AllocationType: valueobjects.AllocationTypeConsumption,
	}, nil).Once()

	s.repo.EXPECT().GetByID(mock.Anything, rootID).Return(entities.Category{}, errors.New("db error")).Once()

	_, err := s.useCase.Execute(s.ctx, subID, rootID)

	s.Error(err)
	s.Contains(err.Error(), "buscar categoria pai")
}

func (s *ValidateSubcategorySuite) TestExecute_ErroBuscarCategoria() {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	s.repo.EXPECT().GetByID(mock.Anything, id).Return(entities.Category{}, errors.New("db error")).Once()

	_, err := s.useCase.Execute(s.ctx, id, uuid.Nil)

	s.Error(err)
	s.Contains(err.Error(), "buscar categoria")
}
