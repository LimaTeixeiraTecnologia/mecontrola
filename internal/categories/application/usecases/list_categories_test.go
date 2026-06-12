package usecases_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/input"
	mockInterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

type ListCategoriesSuite struct {
	suite.Suite

	ctx           context.Context
	repo          *mockInterfaces.CategoryRepository
	versionReader *mockInterfaces.VersionReader
	useCase       *usecases.ListCategories
}

func TestListCategoriesSuite(t *testing.T) {
	suite.Run(t, new(ListCategoriesSuite))
}

func (s *ListCategoriesSuite) SetupTest() {
	s.ctx = context.Background()
	s.repo = mockInterfaces.NewCategoryRepository(s.T())
	s.versionReader = mockInterfaces.NewVersionReader(s.T())
	s.useCase = usecases.NewListCategories(s.repo, s.versionReader, services.NewPTBRCollator(), noop.NewProvider())
}

func (s *ListCategoriesSuite) TestExecute_HappyPath_Tree() {
	s.versionReader.EXPECT().Current(s.ctx).Return(int64(42), nil).Once()

	rootID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	subID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	categories := []entities.Category{
		{
			ID:             rootID,
			Slug:           "custo-fixo",
			Name:           "Custo Fixo",
			Kind:           valueobjects.KindExpense,
			AllocationType: valueobjects.AllocationTypeConsumption,
		},
		{
			ID:             subID,
			Slug:           "aluguel",
			Name:           "Aluguel",
			Kind:           valueobjects.KindExpense,
			ParentID:       &rootID,
			AllocationType: valueobjects.AllocationTypeConsumption,
		},
	}

	s.repo.EXPECT().List(s.ctx, mock.Anything).Return(categories, nil).Once()

	result, err := s.useCase.Execute(s.ctx, &input.ListCategoriesInput{})

	s.NoError(err)
	s.NotNil(result)
	s.Equal(int64(42), result.Version)
	s.Len(result.Categories, 1)
	s.Equal("Custo Fixo", result.Categories[0].Name)
	s.Len(result.Categories[0].Subcategories, 1)
	s.Equal("Aluguel", result.Categories[0].Subcategories[0].Name)
}

func (s *ListCategoriesSuite) TestExecute_FilterByKind() {
	s.versionReader.EXPECT().Current(s.ctx).Return(int64(1), nil).Once()

	kind := valueobjects.KindIncome
	s.repo.EXPECT().List(s.ctx, mock.Anything).Return([]entities.Category{}, nil).Once()

	result, err := s.useCase.Execute(s.ctx, &input.ListCategoriesInput{
		Kind: &kind,
	})

	s.NoError(err)
	s.NotNil(result)
}

func (s *ListCategoriesSuite) TestExecute_FilterByParentID() {
	s.versionReader.EXPECT().Current(s.ctx).Return(int64(1), nil).Once()

	parentID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	s.repo.EXPECT().List(s.ctx, mock.Anything).Return([]entities.Category{}, nil).Once()

	result, err := s.useCase.Execute(s.ctx, &input.ListCategoriesInput{
		ParentID: &parentID,
	})

	s.NoError(err)
	s.NotNil(result)
}

func (s *ListCategoriesSuite) TestExecute_IncludeDeprecated() {
	s.versionReader.EXPECT().Current(s.ctx).Return(int64(1), nil).Once()

	s.repo.EXPECT().List(s.ctx, mock.Anything).Return([]entities.Category{}, nil).Once()

	result, err := s.useCase.Execute(s.ctx, &input.ListCategoriesInput{
		IncludeDeprecated: true,
	})

	s.NoError(err)
	s.NotNil(result)
}

func (s *ListCategoriesSuite) TestExecute_VersionError() {
	s.versionReader.EXPECT().Current(s.ctx).Return(int64(0), errors.New("db error")).Once()

	result, err := s.useCase.Execute(s.ctx, &input.ListCategoriesInput{})

	s.Error(err)
	s.Nil(result)
	s.Contains(err.Error(), "ler versao")
}

func (s *ListCategoriesSuite) TestExecute_RepoError() {
	s.versionReader.EXPECT().Current(s.ctx).Return(int64(1), nil).Once()
	s.repo.EXPECT().List(s.ctx, mock.Anything).Return(nil, errors.New("db error")).Once()

	result, err := s.useCase.Execute(s.ctx, &input.ListCategoriesInput{})

	s.Error(err)
	s.Nil(result)
	s.Contains(err.Error(), "listar categorias")
}

func (s *ListCategoriesSuite) TestExecute_WithDeprecatedCategories() {
	s.versionReader.EXPECT().Current(s.ctx).Return(int64(42), nil).Once()

	rootID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	deprecatedAt := time.Now()

	categories := []entities.Category{
		{
			ID:             rootID,
			Slug:           "old-category",
			Name:           "Old Category",
			Kind:           valueobjects.KindExpense,
			AllocationType: valueobjects.AllocationTypeConsumption,
			DeprecatedAt:   &deprecatedAt,
		},
	}

	s.repo.EXPECT().List(s.ctx, mock.Anything).Return(categories, nil).Once()

	result, err := s.useCase.Execute(s.ctx, &input.ListCategoriesInput{
		IncludeDeprecated: true,
	})

	s.NoError(err)
	s.NotNil(result)
	s.Len(result.Categories, 1)
	s.NotNil(result.Categories[0].DeprecatedAt)
}
