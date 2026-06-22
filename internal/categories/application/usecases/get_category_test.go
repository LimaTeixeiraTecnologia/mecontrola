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

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/interfaces"
	mockInterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

type GetCategorySuite struct {
	suite.Suite

	ctx           context.Context
	obs           observability.Observability
	repo          *mockInterfaces.CategoryRepository
	versionReader *mockInterfaces.VersionReader
	useCase       *GetCategory
}

func TestGetCategorySuite(t *testing.T) {
	suite.Run(t, new(GetCategorySuite))
}

func (s *GetCategorySuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.repo = mockInterfaces.NewCategoryRepository(s.T())
	s.versionReader = mockInterfaces.NewVersionReader(s.T())
	s.useCase = NewGetCategory(s.repo, s.versionReader, services.NewPTBRCollator(), s.obs)
}

func (s *GetCategorySuite) TestExecute_RootWithSubcategories() {
	s.versionReader.EXPECT().Current(mock.Anything).Return(int64(42), nil).Once()

	rootID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	subID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	rootCategory := entities.Category{
		ID:             rootID,
		Slug:           "custo-fixo",
		Name:           "Custo Fixo",
		Kind:           valueobjects.KindExpense,
		AllocationType: valueobjects.AllocationTypeConsumption,
	}

	s.repo.EXPECT().GetByID(mock.Anything, rootID).Return(rootCategory, nil).Once()
	s.repo.EXPECT().List(mock.Anything, mock.Anything).Return([]entities.Category{
		{
			ID:             subID,
			Slug:           "aluguel",
			Name:           "Aluguel",
			Kind:           valueobjects.KindExpense,
			ParentID:       &rootID,
			AllocationType: valueobjects.AllocationTypeConsumption,
		},
	}, nil).Once()

	result, err := s.useCase.Execute(s.ctx, &input.GetCategoryInput{
		ID: rootID,
	})

	s.NoError(err)
	s.NotNil(result)
	s.Equal("Custo Fixo", result.Name)
	s.Equal("Custo Fixo", result.Path)
	s.Len(result.Subcategories, 1)
	s.Equal("Aluguel", result.Subcategories[0].Name)
}

func (s *GetCategorySuite) TestExecute_SubcategoryWithPath() {
	s.versionReader.EXPECT().Current(mock.Anything).Return(int64(42), nil).Once()

	rootID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	subID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	subCategory := entities.Category{
		ID:             subID,
		Slug:           "aluguel",
		Name:           "Aluguel",
		Kind:           valueobjects.KindExpense,
		ParentID:       &rootID,
		AllocationType: valueobjects.AllocationTypeConsumption,
	}

	rootCategory := entities.Category{
		ID:             rootID,
		Slug:           "custo-fixo",
		Name:           "Custo Fixo",
		Kind:           valueobjects.KindExpense,
		AllocationType: valueobjects.AllocationTypeConsumption,
	}

	s.repo.EXPECT().GetByID(mock.Anything, subID).Return(subCategory, nil).Once()
	s.repo.EXPECT().GetByID(mock.Anything, rootID).Return(rootCategory, nil).Once()

	result, err := s.useCase.Execute(s.ctx, &input.GetCategoryInput{
		ID: subID,
	})

	s.NoError(err)
	s.NotNil(result)
	s.Equal("Aluguel", result.Name)
	s.Equal("Custo Fixo > Aluguel", result.Path)
	s.Nil(result.Subcategories)
}

func (s *GetCategorySuite) TestExecute_NotFound() {
	s.versionReader.EXPECT().Current(mock.Anything).Return(int64(42), nil).Once()

	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	s.repo.EXPECT().GetByID(mock.Anything, id).Return(entities.Category{}, interfaces.ErrNotFound).Once()

	result, err := s.useCase.Execute(s.ctx, &input.GetCategoryInput{
		ID: id,
	})

	s.ErrorIs(err, ErrCategoryNotFound)
	s.Nil(result)
}

func (s *GetCategorySuite) TestExecute_DeprecatedWithoutFlag() {
	s.versionReader.EXPECT().Current(mock.Anything).Return(int64(42), nil).Once()

	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	deprecatedAt := time.Now()

	category := entities.Category{
		ID:             id,
		Slug:           "old",
		Name:           "Old",
		Kind:           valueobjects.KindExpense,
		AllocationType: valueobjects.AllocationTypeConsumption,
		DeprecatedAt:   &deprecatedAt,
	}

	s.repo.EXPECT().GetByID(mock.Anything, id).Return(category, nil).Once()

	result, err := s.useCase.Execute(s.ctx, &input.GetCategoryInput{
		ID:                id,
		IncludeDeprecated: false,
	})

	s.ErrorIs(err, ErrCategoryNotFound)
	s.Nil(result)
}

func (s *GetCategorySuite) TestExecute_DeprecatedWithFlag() {
	s.versionReader.EXPECT().Current(mock.Anything).Return(int64(42), nil).Once()

	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	deprecatedAt := time.Now()

	category := entities.Category{
		ID:             id,
		Slug:           "old",
		Name:           "Old",
		Kind:           valueobjects.KindExpense,
		AllocationType: valueobjects.AllocationTypeConsumption,
		DeprecatedAt:   &deprecatedAt,
		ParentID:       &id,
	}

	s.repo.EXPECT().GetByID(mock.Anything, id).Return(category, nil).Once()
	s.repo.EXPECT().GetByID(mock.Anything, id).Return(category, nil).Once()

	result, err := s.useCase.Execute(s.ctx, &input.GetCategoryInput{
		ID:                id,
		IncludeDeprecated: true,
	})

	s.NoError(err)
	s.NotNil(result)
	s.NotNil(result.DeprecatedAt)
}

func (s *GetCategorySuite) TestExecute_RootWithoutChildren() {
	s.versionReader.EXPECT().Current(mock.Anything).Return(int64(1), nil).Once()

	rootID := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	rootCategory := entities.Category{
		ID:             rootID,
		Slug:           "metas",
		Name:           "Metas",
		Kind:           valueobjects.KindExpense,
		AllocationType: valueobjects.AllocationTypeConsumption,
	}

	s.repo.EXPECT().GetByID(mock.Anything, rootID).Return(rootCategory, nil).Once()
	s.repo.EXPECT().List(mock.Anything, mock.Anything).Return([]entities.Category{}, nil).Once()

	result, err := s.useCase.Execute(s.ctx, &input.GetCategoryInput{
		ID: rootID,
	})

	s.NoError(err)
	s.NotNil(result)
	s.Equal("Metas", result.Path)
	s.Len(result.Subcategories, 0)
}

func (s *GetCategorySuite) TestExecute_VersionError() {
	s.versionReader.EXPECT().Current(mock.Anything).Return(int64(0), errors.New("db error")).Once()

	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	result, err := s.useCase.Execute(s.ctx, &input.GetCategoryInput{
		ID: id,
	})

	s.Error(err)
	s.Nil(result)
	s.Contains(err.Error(), "ler versao")
}
