package usecases

import (
	"context"
	"errors"
	"testing"

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

type SearchDictionarySuite struct {
	suite.Suite

	ctx           context.Context
	obs           observability.Observability
	repo          *mockInterfaces.DictionaryRepository
	categoryRepo  *mockInterfaces.CategoryRepository
	versionReader *mockInterfaces.VersionReader
	resolver      *services.CandidateResolver
	useCase       *SearchDictionary
}

func TestSearchDictionarySuite(t *testing.T) {
	suite.Run(t, new(SearchDictionarySuite))
}

func (s *SearchDictionarySuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.repo = mockInterfaces.NewDictionaryRepository(s.T())
	s.categoryRepo = mockInterfaces.NewCategoryRepository(s.T())
	s.versionReader = mockInterfaces.NewVersionReader(s.T())
	s.resolver = services.NewCandidateResolver()
	s.useCase = NewSearchDictionary(s.repo, s.categoryRepo, s.versionReader, s.resolver, s.obs)
}

func (s *SearchDictionarySuite) TestExecute_HappyPath_SingleMatch() {
	s.versionReader.EXPECT().Current(mock.Anything).Return(int64(42), nil).Once()

	categoryID := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	entries := []entities.DictionaryEntry{
		{
			ID:          uuid.MustParse("33333333-3333-3333-3333-333333333333"),
			CategoryID:  categoryID,
			Kind:        valueobjects.KindExpense,
			Term:        "aluguel",
			SignalType:  valueobjects.SignalTypeCanonicalName,
			Confidence:  valueobjects.ConfidenceHigh,
			IsAmbiguous: false,
		},
	}

	s.repo.EXPECT().Search(mock.Anything, mock.Anything).Return(entries, nil).Once()

	s.categoryRepo.EXPECT().ListByIDs(mock.Anything, []uuid.UUID{categoryID}).Return([]entities.Category{
		{ID: categoryID, Name: "Aluguel", Kind: valueobjects.KindExpense},
	}, nil).Once()

	result, err := s.useCase.Execute(s.ctx, &input.SearchDictionaryInput{
		Query: "aluguel",
		Kind:  valueobjects.KindExpense,
	})

	s.NoError(err)
	s.NotNil(result)
	s.Equal("candidates", result.Result)
	s.Equal(int64(42), result.Version)
}

func (s *SearchDictionarySuite) TestExecute_NoMatch() {
	s.versionReader.EXPECT().Current(mock.Anything).Return(int64(42), nil).Once()

	s.repo.EXPECT().Search(mock.Anything, mock.Anything).Return([]entities.DictionaryEntry{}, nil).Once()
	s.repo.EXPECT().SearchTokens(mock.Anything, mock.Anything).Return([]entities.DictionaryEntry{}, nil).Once()
	s.repo.EXPECT().SearchFuzzy(mock.Anything, mock.Anything).Return([]entities.DictionaryEntry{}, nil).Once()

	result, err := s.useCase.Execute(s.ctx, &input.SearchDictionaryInput{
		Query: "xyz123",
		Kind:  valueobjects.KindExpense,
	})

	s.NoError(err)
	s.NotNil(result)
	s.Equal("no_match", result.Result)
	s.Len(result.Candidates, 0)
}

func (s *SearchDictionarySuite) TestExecute_InvalidKind() {
	result, err := s.useCase.Execute(s.ctx, &input.SearchDictionaryInput{
		Query: "aluguel",
		Kind:  valueobjects.Kind(0),
	})

	s.ErrorIs(err, input.ErrInvalidKind)
	s.Nil(result)
}

func (s *SearchDictionarySuite) TestExecute_InvalidQuery_TooShort() {
	result, err := s.useCase.Execute(s.ctx, &input.SearchDictionaryInput{
		Query: "ab",
		Kind:  valueobjects.KindExpense,
	})

	s.ErrorIs(err, input.ErrInvalidQuery)
	s.Nil(result)
}

func (s *SearchDictionarySuite) TestExecute_InvalidQuery_Empty() {
	result, err := s.useCase.Execute(s.ctx, &input.SearchDictionaryInput{
		Query: "",
		Kind:  valueobjects.KindExpense,
	})

	s.ErrorIs(err, input.ErrInvalidQuery)
	s.Nil(result)
}

func (s *SearchDictionarySuite) TestExecute_InvalidQuery_OnlySpaces() {
	result, err := s.useCase.Execute(s.ctx, &input.SearchDictionaryInput{
		Query: "   ",
		Kind:  valueobjects.KindExpense,
	})

	s.ErrorIs(err, input.ErrInvalidQuery)
	s.Nil(result)
}

func (s *SearchDictionarySuite) TestExecute_InvalidQuery_OnlyPunctuation() {
	result, err := s.useCase.Execute(s.ctx, &input.SearchDictionaryInput{
		Query: "...!!!",
		Kind:  valueobjects.KindExpense,
	})

	s.ErrorIs(err, input.ErrInvalidQuery)
	s.Nil(result)
}

func (s *SearchDictionarySuite) TestExecute_VersionError() {
	s.versionReader.EXPECT().Current(mock.Anything).Return(int64(0), errors.New("db error")).Once()

	result, err := s.useCase.Execute(s.ctx, &input.SearchDictionaryInput{
		Query: "aluguel",
		Kind:  valueobjects.KindExpense,
	})

	s.Error(err)
	s.Nil(result)
	s.Contains(err.Error(), "ler versao")
}

func (s *SearchDictionarySuite) TestExecute_RepoError() {
	s.versionReader.EXPECT().Current(mock.Anything).Return(int64(42), nil).Once()
	s.repo.EXPECT().Search(mock.Anything, mock.Anything).Return(nil, errors.New("db error")).Once()

	result, err := s.useCase.Execute(s.ctx, &input.SearchDictionaryInput{
		Query: "aluguel",
		Kind:  valueobjects.KindExpense,
	})

	s.Error(err)
	s.Nil(result)
	s.Contains(err.Error(), "buscar dicionario")
}

func (s *SearchDictionarySuite) TestExecute_QueryNormalization() {
	s.versionReader.EXPECT().Current(mock.Anything).Return(int64(42), nil).Once()

	s.repo.EXPECT().Search(mock.Anything, mock.Anything).Return([]entities.DictionaryEntry{}, nil).Once()
	s.repo.EXPECT().SearchTokens(mock.Anything, mock.Anything).Return([]entities.DictionaryEntry{}, nil).Once()
	s.repo.EXPECT().SearchFuzzy(mock.Anything, mock.Anything).Return([]entities.DictionaryEntry{}, nil).Once()

	result, err := s.useCase.Execute(s.ctx, &input.SearchDictionaryInput{
		Query: "  ALUGUEL!!!  ",
		Kind:  valueobjects.KindExpense,
	})

	s.NoError(err)
	s.NotNil(result)
}

func (s *SearchDictionarySuite) TestExecute_TrimsQueryBeforeRepoSearch() {
	s.versionReader.EXPECT().Current(mock.Anything).Return(int64(42), nil).Once()

	categoryID := uuid.MustParse("55555555-5555-5555-5555-555555555555")

	entries := []entities.DictionaryEntry{
		{
			ID:          uuid.MustParse("66666666-6666-6666-6666-666666666666"),
			CategoryID:  categoryID,
			Kind:        valueobjects.KindExpense,
			Term:        "energia",
			SignalType:  valueobjects.SignalTypeCanonicalName,
			Confidence:  valueobjects.ConfidenceHigh,
			IsAmbiguous: false,
		},
	}

	s.repo.EXPECT().
		Search(mock.Anything, interfaces.DictionarySearchQuery{
			Kind:              valueobjects.KindExpense,
			Term:              "energia",
			Limit:             100,
			IncludeDeprecated: false,
		}).
		Return(entries, nil).
		Once()

	s.categoryRepo.EXPECT().ListByIDs(mock.Anything, []uuid.UUID{categoryID}).Return([]entities.Category{
		{ID: categoryID, Name: "Energia", Kind: valueobjects.KindExpense},
	}, nil).Once()

	result, err := s.useCase.Execute(s.ctx, &input.SearchDictionaryInput{
		Query: "  energia  ",
		Kind:  valueobjects.KindExpense,
	})

	s.NoError(err)
	s.NotNil(result)
	s.Equal("candidates", result.Result)
	s.Len(result.Candidates, 1)
}

func (s *SearchDictionarySuite) TestExecute_TokenFallbackWhenExactEmpty() {
	s.versionReader.EXPECT().Current(mock.Anything).Return(int64(42), nil).Once()

	categoryID := uuid.MustParse("77777777-7777-7777-7777-777777777777")

	s.repo.EXPECT().Search(mock.Anything, mock.Anything).Return([]entities.DictionaryEntry{}, nil).Once()
	s.repo.EXPECT().
		SearchTokens(mock.Anything, interfaces.DictionaryTokenSearchQuery{
			Kind:   valueobjects.KindExpense,
			Tokens: []string{"netflix"},
			Limit:  100,
		}).
		Return([]entities.DictionaryEntry{
			{
				ID:         uuid.MustParse("88888888-8888-8888-8888-888888888888"),
				CategoryID: categoryID,
				Kind:       valueobjects.KindExpense,
				Term:       "netflix",
				SignalType: valueobjects.SignalTypeMerchant,
				Confidence: valueobjects.ConfidenceHigh,
			},
		}, nil).
		Once()

	s.categoryRepo.EXPECT().ListByIDs(mock.Anything, []uuid.UUID{categoryID}).Return([]entities.Category{
		{ID: categoryID, Name: "Streaming", Kind: valueobjects.KindExpense},
	}, nil).Once()

	result, err := s.useCase.Execute(s.ctx, &input.SearchDictionaryInput{
		Query: "paguei netflix hoje",
		Kind:  valueobjects.KindExpense,
	})

	s.NoError(err)
	s.NotNil(result)
	s.Equal("candidates", result.Result)
	s.Len(result.Candidates, 1)
	s.Equal("token", result.Candidates[0].MatchQuality)
}

func (s *SearchDictionarySuite) TestExecute_FuzzyFallbackWhenExactAndTokenEmpty() {
	s.versionReader.EXPECT().Current(mock.Anything).Return(int64(42), nil).Once()

	categoryID := uuid.MustParse("99999999-9999-9999-9999-999999999999")

	s.repo.EXPECT().Search(mock.Anything, mock.Anything).Return([]entities.DictionaryEntry{}, nil).Once()
	s.repo.EXPECT().SearchTokens(mock.Anything, mock.Anything).Return([]entities.DictionaryEntry{}, nil).Once()
	s.repo.EXPECT().
		SearchFuzzy(mock.Anything, interfaces.DictionaryFuzzySearchQuery{
			Kind:          valueobjects.KindExpense,
			Tokens:        []string{"netflyx"},
			MinSimilarity: 0.4,
			Limit:         100,
		}).
		Return([]entities.DictionaryEntry{
			{
				ID:         uuid.MustParse("aaaa1111-aaaa-1111-aaaa-111111111111"),
				CategoryID: categoryID,
				Kind:       valueobjects.KindExpense,
				Term:       "netflix",
				SignalType: valueobjects.SignalTypeCanonicalName,
				Confidence: valueobjects.ConfidenceHigh,
			},
		}, nil).
		Once()

	s.categoryRepo.EXPECT().ListByIDs(mock.Anything, []uuid.UUID{categoryID}).Return([]entities.Category{
		{ID: categoryID, Name: "Streaming", Kind: valueobjects.KindExpense},
	}, nil).Once()

	result, err := s.useCase.Execute(s.ctx, &input.SearchDictionaryInput{
		Query: "netflyx",
		Kind:  valueobjects.KindExpense,
	})

	s.NoError(err)
	s.NotNil(result)
	s.Equal("candidates", result.Result)
	s.Len(result.Candidates, 1)
	s.Equal("fuzzy", result.Candidates[0].MatchQuality)
	s.Less(result.Candidates[0].Score, valueobjects.ScoreAutoThreshold)
}

func (s *SearchDictionarySuite) TestExecute_AmbiguousMatch() {
	s.versionReader.EXPECT().Current(mock.Anything).Return(int64(42), nil).Once()

	categoryID1 := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	categoryID2 := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	entries := []entities.DictionaryEntry{
		{
			ID:          uuid.MustParse("33333333-3333-3333-3333-333333333333"),
			CategoryID:  categoryID1,
			Kind:        valueobjects.KindExpense,
			Term:        "uber",
			SignalType:  valueobjects.SignalTypeMerchant,
			Confidence:  valueobjects.ConfidenceLow,
			IsAmbiguous: true,
		},
		{
			ID:          uuid.MustParse("44444444-4444-4444-4444-444444444444"),
			CategoryID:  categoryID2,
			Kind:        valueobjects.KindExpense,
			Term:        "uber",
			SignalType:  valueobjects.SignalTypeMerchant,
			Confidence:  valueobjects.ConfidenceLow,
			IsAmbiguous: true,
		},
	}

	s.repo.EXPECT().Search(mock.Anything, mock.Anything).Return(entries, nil).Once()

	s.categoryRepo.EXPECT().ListByIDs(mock.Anything, []uuid.UUID{categoryID1, categoryID2}).Return([]entities.Category{
		{ID: categoryID1, Name: "Transporte Recorrente", Kind: valueobjects.KindExpense},
		{ID: categoryID2, Name: "Transporte Lazer", Kind: valueobjects.KindExpense},
	}, nil).Once()

	result, err := s.useCase.Execute(s.ctx, &input.SearchDictionaryInput{
		Query: "uber",
		Kind:  valueobjects.KindExpense,
	})

	s.NoError(err)
	s.NotNil(result)
	s.Equal("candidates", result.Result)
	s.True(len(result.Candidates) > 0)
}

func (s *SearchDictionarySuite) TestExecute_FetchesMissingParents() {
	s.versionReader.EXPECT().Current(mock.Anything).Return(int64(42), nil).Once()

	parentID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	childID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")

	entries := []entities.DictionaryEntry{
		{
			ID:         uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc"),
			CategoryID: childID,
			Kind:       valueobjects.KindExpense,
			Term:       "aluguel",
			SignalType: valueobjects.SignalTypeCanonicalName,
			Confidence: valueobjects.ConfidenceHigh,
		},
	}

	s.repo.EXPECT().Search(mock.Anything, mock.Anything).Return(entries, nil).Once()

	s.categoryRepo.EXPECT().ListByIDs(mock.Anything, []uuid.UUID{childID}).Return([]entities.Category{
		{ID: childID, Name: "Aluguel", Kind: valueobjects.KindExpense, ParentID: &parentID},
	}, nil).Once()

	s.categoryRepo.EXPECT().ListByIDs(mock.Anything, []uuid.UUID{parentID}).Return([]entities.Category{
		{ID: parentID, Name: "Custo Fixo", Kind: valueobjects.KindExpense},
	}, nil).Once()

	result, err := s.useCase.Execute(s.ctx, &input.SearchDictionaryInput{
		Query: "aluguel",
		Kind:  valueobjects.KindExpense,
	})

	s.NoError(err)
	s.NotNil(result)
	s.Equal("candidates", result.Result)
	s.Len(result.Candidates, 1)
	s.Equal("Custo Fixo > Aluguel", result.Candidates[0].Path)
	s.Equal(parentID, result.Candidates[0].RootCategoryID)
}

func (s *SearchDictionarySuite) TestExecute_SkipsParentFetchWhenAllRoots() {
	s.versionReader.EXPECT().Current(mock.Anything).Return(int64(42), nil).Once()

	rootID := uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")

	entries := []entities.DictionaryEntry{
		{
			ID:         uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"),
			CategoryID: rootID,
			Kind:       valueobjects.KindExpense,
			Term:       "metas",
			SignalType: valueobjects.SignalTypeCanonicalName,
			Confidence: valueobjects.ConfidenceHigh,
		},
	}

	s.repo.EXPECT().Search(mock.Anything, mock.Anything).Return(entries, nil).Once()
	s.categoryRepo.EXPECT().ListByIDs(mock.Anything, []uuid.UUID{rootID}).Return([]entities.Category{
		{ID: rootID, Name: "Metas", Kind: valueobjects.KindExpense},
	}, nil).Once()

	result, err := s.useCase.Execute(s.ctx, &input.SearchDictionaryInput{
		Query: "metas",
		Kind:  valueobjects.KindExpense,
	})

	s.NoError(err)
	s.NotNil(result)
	s.Equal("candidates", result.Result)
}

func (s *SearchDictionarySuite) TestExecute_PropagatesBatchFetchError() {
	s.versionReader.EXPECT().Current(mock.Anything).Return(int64(42), nil).Once()

	categoryID := uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff")
	entries := []entities.DictionaryEntry{
		{
			ID:         uuid.MustParse("11111111-2222-3333-4444-555555555555"),
			CategoryID: categoryID,
			Kind:       valueobjects.KindExpense,
			Term:       "energia",
			SignalType: valueobjects.SignalTypeCanonicalName,
			Confidence: valueobjects.ConfidenceHigh,
		},
	}

	s.repo.EXPECT().Search(mock.Anything, mock.Anything).Return(entries, nil).Once()
	s.categoryRepo.EXPECT().ListByIDs(mock.Anything, []uuid.UUID{categoryID}).Return(nil, errors.New("db error")).Once()

	result, err := s.useCase.Execute(s.ctx, &input.SearchDictionaryInput{
		Query: "energia",
		Kind:  valueobjects.KindExpense,
	})

	s.Error(err)
	s.Nil(result)
	s.Contains(err.Error(), "buscar categorias")
}
