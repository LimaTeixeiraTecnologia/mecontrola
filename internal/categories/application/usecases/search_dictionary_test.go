package usecases_test

import (
	"context"
	"errors"
	"testing"

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

type SearchDictionarySuite struct {
	suite.Suite

	ctx           context.Context
	repo          *mockInterfaces.DictionaryRepository
	categoryRepo  *mockInterfaces.CategoryRepository
	versionReader *mockInterfaces.VersionReader
	resolver      *services.CandidateResolver
	useCase       *usecases.SearchDictionary
}

func TestSearchDictionarySuite(t *testing.T) {
	suite.Run(t, new(SearchDictionarySuite))
}

func (s *SearchDictionarySuite) SetupTest() {
	s.ctx = context.Background()
	s.repo = mockInterfaces.NewDictionaryRepository(s.T())
	s.categoryRepo = mockInterfaces.NewCategoryRepository(s.T())
	s.versionReader = mockInterfaces.NewVersionReader(s.T())
	s.resolver = services.NewCandidateResolver()
	s.useCase = usecases.NewSearchDictionary(s.repo, s.categoryRepo, s.versionReader, s.resolver, noop.NewProvider())
}

func (s *SearchDictionarySuite) TestExecute_HappyPath_SingleMatch() {
	s.versionReader.EXPECT().Current(s.ctx).Return(int64(42), nil).Once()

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

	s.repo.EXPECT().Search(s.ctx, mock.Anything).Return(entries, nil).Once()

	s.categoryRepo.EXPECT().GetByID(s.ctx, categoryID).Return(entities.Category{
		ID:   categoryID,
		Name: "Aluguel",
		Kind: valueobjects.KindExpense,
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
	s.versionReader.EXPECT().Current(s.ctx).Return(int64(42), nil).Once()

	s.repo.EXPECT().Search(s.ctx, mock.Anything).Return([]entities.DictionaryEntry{}, nil).Once()

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
	s.versionReader.EXPECT().Current(s.ctx).Return(int64(0), errors.New("db error")).Once()

	result, err := s.useCase.Execute(s.ctx, &input.SearchDictionaryInput{
		Query: "aluguel",
		Kind:  valueobjects.KindExpense,
	})

	s.Error(err)
	s.Nil(result)
	s.Contains(err.Error(), "ler versao")
}

func (s *SearchDictionarySuite) TestExecute_RepoError() {
	s.versionReader.EXPECT().Current(s.ctx).Return(int64(42), nil).Once()
	s.repo.EXPECT().Search(s.ctx, mock.Anything).Return(nil, errors.New("db error")).Once()

	result, err := s.useCase.Execute(s.ctx, &input.SearchDictionaryInput{
		Query: "aluguel",
		Kind:  valueobjects.KindExpense,
	})

	s.Error(err)
	s.Nil(result)
	s.Contains(err.Error(), "buscar dicionario")
}

func (s *SearchDictionarySuite) TestExecute_QueryNormalization() {
	s.versionReader.EXPECT().Current(s.ctx).Return(int64(42), nil).Once()

	s.repo.EXPECT().Search(s.ctx, mock.Anything).Return([]entities.DictionaryEntry{}, nil).Once()

	result, err := s.useCase.Execute(s.ctx, &input.SearchDictionaryInput{
		Query: "  ALUGUEL!!!  ",
		Kind:  valueobjects.KindExpense,
	})

	s.NoError(err)
	s.NotNil(result)
}

func (s *SearchDictionarySuite) TestExecute_AmbiguousMatch() {
	s.versionReader.EXPECT().Current(s.ctx).Return(int64(42), nil).Once()

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

	s.repo.EXPECT().Search(s.ctx, mock.Anything).Return(entries, nil).Once()

	s.categoryRepo.EXPECT().GetByID(s.ctx, categoryID1).Return(entities.Category{
		ID:   categoryID1,
		Name: "Transporte Recorrente",
		Kind: valueobjects.KindExpense,
	}, nil).Once()

	s.categoryRepo.EXPECT().GetByID(s.ctx, categoryID2).Return(entities.Category{
		ID:   categoryID2,
		Name: "Transporte Lazer",
		Kind: valueobjects.KindExpense,
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
