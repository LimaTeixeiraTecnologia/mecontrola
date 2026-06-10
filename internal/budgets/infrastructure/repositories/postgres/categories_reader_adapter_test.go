package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	budgetsinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	budgetspostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/repositories/postgres"
	catifacemocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/interfaces/mocks"
	catusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

type CategoriesReaderAdapterSuite struct {
	suite.Suite
}

func TestCategoriesReaderAdapterSuite(t *testing.T) {
	suite.Run(t, new(CategoriesReaderAdapterSuite))
}

func (s *CategoriesReaderAdapterSuite) buildAdapter(catRepo *catifacemocks.CategoryRepository, versionRepo *catifacemocks.VersionReader) budgetsinterfaces.CategoriesReader {
	o11y := noop.NewProvider()
	resolveUC := catusecases.NewResolveBySlug(catRepo, o11y)
	validateUC := catusecases.NewValidateSubcategory(catRepo, o11y)
	return budgetspostgres.NewCategoriesReaderAdapter(resolveUC, validateUC, versionRepo, o11y)
}

func (s *CategoriesReaderAdapterSuite) TestResolveRootsBySlug_Success() {
	catRepo := catifacemocks.NewCategoryRepository(s.T())
	versionRepo := catifacemocks.NewVersionReader(s.T())

	rootID := uuid.New()
	root := entities.Category{
		ID:   rootID,
		Slug: "custo-fixo",
		Kind: valueobjects.KindExpense,
	}

	catRepo.EXPECT().List(context.Background(), mock.Anything).Return([]entities.Category{root}, nil)

	adapter := s.buildAdapter(catRepo, versionRepo)

	result, err := adapter.ResolveRootsBySlug(context.Background(), []string{"expense.custo_fixo"})

	s.Require().NoError(err)
	s.Require().Len(result, 1)
	s.Equal(rootID, result["expense.custo_fixo"])
}

func (s *CategoriesReaderAdapterSuite) TestResolveRootsBySlug_SlugNotFound() {
	catRepo := catifacemocks.NewCategoryRepository(s.T())
	versionRepo := catifacemocks.NewVersionReader(s.T())

	catRepo.EXPECT().List(context.Background(), mock.Anything).Return([]entities.Category{}, nil)

	adapter := s.buildAdapter(catRepo, versionRepo)

	_, err := adapter.ResolveRootsBySlug(context.Background(), []string{"expense.custo_fixo"})

	s.Require().Error(err)
	s.True(errors.Is(err, budgetsinterfaces.ErrCategoriesReaderUnavailable))
}

func (s *CategoriesReaderAdapterSuite) TestResolveRootsBySlug_RepoError() {
	catRepo := catifacemocks.NewCategoryRepository(s.T())
	versionRepo := catifacemocks.NewVersionReader(s.T())

	catRepo.EXPECT().List(context.Background(), mock.Anything).Return(nil, errors.New("db down"))

	adapter := s.buildAdapter(catRepo, versionRepo)

	_, err := adapter.ResolveRootsBySlug(context.Background(), []string{"expense.custo_fixo"})

	s.Require().Error(err)
	s.True(errors.Is(err, budgetsinterfaces.ErrCategoriesReaderUnavailable))
}

func (s *CategoriesReaderAdapterSuite) TestValidateExpenseSubcategory_Active() {
	catRepo := catifacemocks.NewCategoryRepository(s.T())
	versionRepo := catifacemocks.NewVersionReader(s.T())

	rootID := uuid.New()
	subID := uuid.New()

	subcategory := entities.Category{
		ID:       subID,
		Slug:     "cartao-de-credito",
		Kind:     valueobjects.KindExpense,
		ParentID: &rootID,
	}
	parent := entities.Category{
		ID:   rootID,
		Slug: "custo-fixo",
		Kind: valueobjects.KindExpense,
	}

	catRepo.EXPECT().GetByID(context.Background(), subID).Return(subcategory, nil)
	catRepo.EXPECT().GetByID(context.Background(), rootID).Return(parent, nil)

	adapter := s.buildAdapter(catRepo, versionRepo)

	rootSlug, deprecated, err := adapter.ValidateExpenseSubcategory(context.Background(), subID)

	s.Require().NoError(err)
	s.Equal("expense.custo_fixo", rootSlug)
	s.False(deprecated)
}

func (s *CategoriesReaderAdapterSuite) TestValidateExpenseSubcategory_Deprecated() {
	catRepo := catifacemocks.NewCategoryRepository(s.T())
	versionRepo := catifacemocks.NewVersionReader(s.T())

	rootID := uuid.New()
	subID := uuid.New()
	depTime := time.Now()

	subcategory := entities.Category{
		ID:           subID,
		Slug:         "old-subcategory",
		Kind:         valueobjects.KindExpense,
		ParentID:     &rootID,
		DeprecatedAt: &depTime,
	}
	parent := entities.Category{
		ID:   rootID,
		Slug: "custo-fixo",
		Kind: valueobjects.KindExpense,
	}

	catRepo.EXPECT().GetByID(context.Background(), subID).Return(subcategory, nil)
	catRepo.EXPECT().GetByID(context.Background(), rootID).Return(parent, nil)

	adapter := s.buildAdapter(catRepo, versionRepo)

	rootSlug, deprecated, err := adapter.ValidateExpenseSubcategory(context.Background(), subID)

	s.Require().NoError(err)
	s.Equal("expense.custo_fixo", rootSlug)
	s.True(deprecated)
}

func (s *CategoriesReaderAdapterSuite) TestEditorialVersion_Success() {
	catRepo := catifacemocks.NewCategoryRepository(s.T())
	versionRepo := catifacemocks.NewVersionReader(s.T())

	versionRepo.EXPECT().Current(context.Background()).Return(int64(42), nil)

	adapter := s.buildAdapter(catRepo, versionRepo)

	v, err := adapter.EditorialVersion(context.Background())

	s.Require().NoError(err)
	s.Equal(int64(42), v)
}

func (s *CategoriesReaderAdapterSuite) TestEditorialVersion_Error() {
	catRepo := catifacemocks.NewCategoryRepository(s.T())
	versionRepo := catifacemocks.NewVersionReader(s.T())

	versionRepo.EXPECT().Current(context.Background()).Return(int64(0), errors.New("db down"))

	adapter := s.buildAdapter(catRepo, versionRepo)

	_, err := adapter.EditorialVersion(context.Background())

	s.Require().Error(err)
	s.True(errors.Is(err, budgetsinterfaces.ErrCategoriesReaderUnavailable))
}
