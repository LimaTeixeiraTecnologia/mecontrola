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
	resolveBySlugUC := catusecases.NewResolveBySlug(s.catRepoMock, o11y)
	return NewCategoriesReaderAdapter(searchDictUC, resolveBySlugUC, nil, o11y)
}

func (s *CategoriesReaderAdapterSuite) TestResolveRootsBySlug_Success() {
	type args struct {
		slugs []string
	}
	type dependencies struct {
		catRepo *catifacemocks.CategoryRepository
	}

	rootID := uuid.New()

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(result map[string]uuid.UUID, err error)
	}{
		{
			name: "deve resolver slug com sucesso",
			args: args{slugs: []string{"custo-fixo"}},
			dependencies: dependencies{
				catRepo: func() *catifacemocks.CategoryRepository {
					s.catRepoMock.EXPECT().
						List(mock.Anything, mock.Anything).
						Return([]entities.Category{{ID: rootID, Slug: "custo-fixo", Kind: valueobjects.KindExpense}}, nil).
						Once()
					return s.catRepoMock
				}(),
			},
			expect: func(result map[string]uuid.UUID, err error) {
				s.NoError(err)
				s.Equal(rootID, result["custo-fixo"])
			},
		},
		{
			name: "deve retornar erro quando slug não encontrado",
			args: args{slugs: []string{"inexistente"}},
			dependencies: dependencies{
				catRepo: func() *catifacemocks.CategoryRepository {
					s.catRepoMock.EXPECT().
						List(mock.Anything, mock.Anything).
						Return([]entities.Category{}, nil).
						Once()
					return s.catRepoMock
				}(),
			},
			expect: func(result map[string]uuid.UUID, err error) {
				s.Error(err)
				s.Nil(result)
			},
		},
		{
			name: "deve retornar erro quando repositório falha",
			args: args{slugs: []string{"custo-fixo"}},
			dependencies: dependencies{
				catRepo: func() *catifacemocks.CategoryRepository {
					s.catRepoMock.EXPECT().
						List(mock.Anything, mock.Anything).
						Return(nil, errors.New("db error")).
						Once()
					return s.catRepoMock
				}(),
			},
			expect: func(result map[string]uuid.UUID, err error) {
				s.Error(err)
				s.Nil(result)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			adapter := s.buildAdapter()
			result, err := adapter.ResolveRootsBySlug(s.ctx, scenario.args.slugs)
			scenario.expect(result, err)
		})
	}
}

func (s *CategoriesReaderAdapterSuite) TestSearchDictionary_InvalidKind() {
	adapter := s.buildAdapter()
	_, err := adapter.SearchDictionary(s.ctx, "mercado", "invalid_kind")
	s.Error(err)
	s.ErrorIs(err, agentsifaces.ErrCategoriesReaderUnavailable)
}
