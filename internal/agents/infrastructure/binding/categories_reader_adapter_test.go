package binding

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"

	agentsifaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	catifacemocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/interfaces/mocks"
	catusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

type CategoriesReaderAdapterSuite struct {
	suite.Suite
	ctx          context.Context
	catRepoMock  *catifacemocks.CategoryRepository
	dictRepoMock *catifacemocks.DictionaryRepository
	versionMock  *catifacemocks.VersionReader
}

func TestCategoriesReaderAdapterSuite(t *testing.T) {
	suite.Run(t, new(CategoriesReaderAdapterSuite))
}

func (s *CategoriesReaderAdapterSuite) SetupTest() {
	s.ctx = context.Background()
	s.catRepoMock = catifacemocks.NewCategoryRepository(s.T())
	s.dictRepoMock = catifacemocks.NewDictionaryRepository(s.T())
	s.versionMock = catifacemocks.NewVersionReader(s.T())
}

func (s *CategoriesReaderAdapterSuite) buildAdapter() agentsifaces.CategoriesReader {
	o11y := fake.NewProvider()
	resolver := services.NewCandidateResolver()
	searchDictUC := catusecases.NewSearchDictionary(s.dictRepoMock, s.catRepoMock, s.versionMock, resolver, o11y)
	resolveCategoryForWriteUC := catusecases.NewResolveCategoryForWrite(s.catRepoMock, s.versionMock, o11y)
	return NewCategoriesReaderAdapter(searchDictUC, resolveCategoryForWriteUC, nil, o11y)
}

func (s *CategoriesReaderAdapterSuite) TestSearchDictionary_InvalidKind() {
	adapter := s.buildAdapter()
	_, err := adapter.SearchDictionary(s.ctx, "mercado", "invalid_kind")
	s.Error(err)
	s.ErrorIs(err, agentsifaces.ErrCategoriesReaderUnavailable)
}

func (s *CategoriesReaderAdapterSuite) TestSearchDictionary_PreservesAllFields() {
	rootID := uuid.New()
	leafID := uuid.New()

	s.versionMock.EXPECT().Current(mock.Anything).Return(int64(3), nil).Once()
	s.dictRepoMock.EXPECT().
		Search(mock.Anything, mock.Anything).
		Return([]entities.DictionaryEntry{
			{
				CategoryID:  leafID,
				Term:        "restaurante",
				SignalType:  valueobjects.SignalTypeAlias,
				Confidence:  valueobjects.ConfidenceHigh,
				IsAmbiguous: false,
			},
		}, nil).Once()
	s.catRepoMock.EXPECT().
		ListByIDs(mock.Anything, mock.Anything).
		Return([]entities.Category{
			{ID: leafID, Slug: "restaurante", Kind: valueobjects.KindExpense, ParentID: &rootID},
			{ID: rootID, Slug: "alimentacao", Kind: valueobjects.KindExpense},
		}, nil).Once()

	adapter := s.buildAdapter()
	result, err := adapter.SearchDictionary(s.ctx, "restaurante", "expense")

	s.NoError(err)
	s.Equal(agentsifaces.ClassifyOutcomeMatched, result.Outcome)
	s.Equal(int64(3), result.Version)
	s.Require().Len(result.Candidates, 1)
	c := result.Candidates[0]
	s.Equal(leafID, c.CategoryID)
	s.Equal(rootID, c.RootCategoryID)
	s.NotEmpty(c.SignalType)
	s.NotEmpty(c.Confidence)
	s.NotEmpty(c.MatchQuality)
}

func (s *CategoriesReaderAdapterSuite) TestResolveForWrite_Success() {
	rootID := uuid.New()
	leafID := uuid.New()

	s.versionMock.EXPECT().Current(mock.Anything).Return(int64(2), nil).Once()
	s.catRepoMock.EXPECT().
		GetByID(mock.Anything, rootID).
		Return(entities.Category{ID: rootID, Slug: "custo-fixo", Kind: valueobjects.KindExpense}, nil).Once()
	s.catRepoMock.EXPECT().
		GetByID(mock.Anything, leafID).
		Return(entities.Category{ID: leafID, Slug: "aluguel", Kind: valueobjects.KindExpense, ParentID: &rootID}, nil).Once()

	adapter := s.buildAdapter()
	decision, err := adapter.ResolveForWrite(s.ctx, agentsifaces.CategoryWriteRequest{
		RootCategoryID:  rootID,
		SubcategoryID:   leafID,
		Kind:            agentsifaces.CategoryKindExpense,
		ExpectedVersion: 2,
	})

	s.NoError(err)
	s.Equal(rootID, decision.RootCategoryID)
	s.Equal(leafID, decision.SubcategoryID)
	s.Equal(agentsifaces.CategoryKindExpense, decision.Kind)
	s.NotEmpty(decision.Path)
	s.Equal(int64(2), decision.EditorialVersion)
	s.False(decision.Deprecated)
	s.NotEmpty(decision.RootSlug)
	s.NotEmpty(decision.SubcategorySlug)
}

func (s *CategoriesReaderAdapterSuite) TestResolveForWrite_InvalidKind() {
	adapter := s.buildAdapter()
	_, err := adapter.ResolveForWrite(s.ctx, agentsifaces.CategoryWriteRequest{
		RootCategoryID:  uuid.New(),
		SubcategoryID:   uuid.New(),
		ExpectedVersion: 1,
	})
	s.Error(err)
	s.ErrorIs(err, agentsifaces.ErrCategoriesReaderUnavailable)
}

func (s *CategoriesReaderAdapterSuite) TestResolveForWrite_RepoError() {
	rootID := uuid.New()
	leafID := uuid.New()

	s.versionMock.EXPECT().Current(mock.Anything).Return(int64(1), nil).Once()
	s.catRepoMock.EXPECT().
		GetByID(mock.Anything, rootID).
		Return(entities.Category{}, errors.New("db error")).Once()

	adapter := s.buildAdapter()
	_, err := adapter.ResolveForWrite(s.ctx, agentsifaces.CategoryWriteRequest{
		RootCategoryID:  rootID,
		SubcategoryID:   leafID,
		Kind:            agentsifaces.CategoryKindExpense,
		ExpectedVersion: 1,
	})
	s.Error(err)
}
