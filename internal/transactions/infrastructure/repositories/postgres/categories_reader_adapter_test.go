package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	catifacemocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/interfaces/mocks"
	catusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/entities"
	catvos "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/config"
	txpostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/repositories/postgres"
)

type CategoriesReaderAdapterSuite struct {
	suite.Suite
}

func TestCategoriesReaderAdapterSuite(t *testing.T) {
	suite.Run(t, new(CategoriesReaderAdapterSuite))
}

func (s *CategoriesReaderAdapterSuite) buildAdapter(catRepo *catifacemocks.CategoryRepository, versionRepo *catifacemocks.VersionReader) config.CategoriesReader {
	o11y := noop.NewProvider()
	resolveUC := catusecases.NewResolveBySlug(catRepo, o11y)
	validateUC := catusecases.NewValidateSubcategory(catRepo, o11y)
	return txpostgres.NewCategoriesReaderAdapter(resolveUC, validateUC, versionRepo, o11y)
}

func (s *CategoriesReaderAdapterSuite) TestResolveRootsBySlug_Success() {
	catRepo := catifacemocks.NewCategoryRepository(s.T())
	versionRepo := catifacemocks.NewVersionReader(s.T())

	slug := "custo-fixo"
	catID := uuid.New()

	catRepo.EXPECT().List(mock.Anything, mock.Anything).Return([]entities.Category{
		{ID: catID, Slug: slug, Name: "Custo Fixo", Kind: catvos.KindExpense},
	}, nil)

	adapter := s.buildAdapter(catRepo, versionRepo)

	result, err := adapter.ResolveRootsBySlug(context.Background(), []string{"expense." + slug})
	s.Require().NoError(err)
	s.Require().Len(result, 1)
	id, ok := result["expense."+slug]
	s.True(ok)
	s.Equal(catID, id)
}

func (s *CategoriesReaderAdapterSuite) TestResolveRootsBySlug_Error() {
	catRepo := catifacemocks.NewCategoryRepository(s.T())
	versionRepo := catifacemocks.NewVersionReader(s.T())

	catRepo.EXPECT().List(mock.Anything, mock.Anything).Return([]entities.Category(nil), errors.New("db error"))

	adapter := s.buildAdapter(catRepo, versionRepo)

	_, err := adapter.ResolveRootsBySlug(context.Background(), config.OfficialRootSlugs)
	s.Require().Error(err)
	s.True(errors.Is(err, interfaces.ErrCategoryNotFound))
}

func (s *CategoriesReaderAdapterSuite) TestValidateSubcategory_MapsRealNames() {
	catRepo := catifacemocks.NewCategoryRepository(s.T())
	versionRepo := catifacemocks.NewVersionReader(s.T())

	rootID := uuid.New()
	subID := uuid.New()

	catRepo.EXPECT().GetByID(mock.Anything, subID).Return(entities.Category{
		ID:       subID,
		ParentID: &rootID,
		Slug:     "aluguel",
		Name:     "Aluguel",
		Kind:     catvos.KindExpense,
	}, nil).Once()

	catRepo.EXPECT().GetByID(mock.Anything, rootID).Return(entities.Category{
		ID:   rootID,
		Slug: "custo-fixo",
		Name: "Custo Fixo",
		Kind: catvos.KindExpense,
	}, nil).Once()

	adapter := s.buildAdapter(catRepo, versionRepo)

	snapshot, err := adapter.ValidateSubcategory(context.Background(), subID, rootID)
	s.Require().NoError(err)
	s.Equal(subID, snapshot.ID)
	s.Equal("Aluguel", snapshot.Name)
	s.Equal("Custo Fixo", snapshot.ParentName)
	s.Equal("expense", snapshot.Kind)
	s.Require().NotNil(snapshot.ParentID)
	s.Equal(rootID, *snapshot.ParentID)
}

func (s *CategoriesReaderAdapterSuite) TestValidateSubcategory_NotDirectChild() {
	catRepo := catifacemocks.NewCategoryRepository(s.T())
	versionRepo := catifacemocks.NewVersionReader(s.T())

	rootID := uuid.New()
	otherRootID := uuid.New()
	subID := uuid.New()

	catRepo.EXPECT().GetByID(mock.Anything, subID).Return(entities.Category{
		ID:       subID,
		ParentID: &otherRootID,
		Slug:     "aluguel",
		Name:     "Aluguel",
		Kind:     catvos.KindExpense,
	}, nil).Once()

	adapter := s.buildAdapter(catRepo, versionRepo)

	_, err := adapter.ValidateSubcategory(context.Background(), subID, rootID)
	s.Require().Error(err)
	s.True(errors.Is(err, interfaces.ErrCategoryNotFound))
}

func (s *CategoriesReaderAdapterSuite) TestEditorialVersion_Success() {
	catRepo := catifacemocks.NewCategoryRepository(s.T())
	versionRepo := catifacemocks.NewVersionReader(s.T())

	versionRepo.EXPECT().Current(mock.Anything).Return(int64(42), nil)

	adapter := s.buildAdapter(catRepo, versionRepo)

	v, err := adapter.EditorialVersion(context.Background())
	s.Require().NoError(err)
	s.Equal(int64(42), v)
}

func (s *CategoriesReaderAdapterSuite) TestEditorialVersion_Error() {
	catRepo := catifacemocks.NewCategoryRepository(s.T())
	versionRepo := catifacemocks.NewVersionReader(s.T())

	versionRepo.EXPECT().Current(mock.Anything).Return(int64(0), errors.New("db error"))

	adapter := s.buildAdapter(catRepo, versionRepo)

	_, err := adapter.EditorialVersion(context.Background())
	s.Require().Error(err)
	s.True(errors.Is(err, interfaces.ErrCategoryNotFound))
}
