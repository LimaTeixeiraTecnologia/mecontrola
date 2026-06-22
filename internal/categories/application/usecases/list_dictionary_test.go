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
	mockInterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

type ListDictionarySuite struct {
	suite.Suite

	ctx           context.Context
	obs           observability.Observability
	repo          *mockInterfaces.DictionaryRepository
	versionReader *mockInterfaces.VersionReader
	useCase       *ListDictionary
}

func TestListDictionarySuite(t *testing.T) {
	suite.Run(t, new(ListDictionarySuite))
}

func (s *ListDictionarySuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.repo = mockInterfaces.NewDictionaryRepository(s.T())
	s.versionReader = mockInterfaces.NewVersionReader(s.T())
	s.useCase = NewListDictionary(s.repo, s.versionReader, s.obs)
}

func (s *ListDictionarySuite) TestExecute_HappyPath() {
	s.versionReader.EXPECT().Current(mock.Anything).Return(int64(42), nil).Once()

	categoryID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	entries := []entities.DictionaryEntry{
		{
			ID:           uuid.MustParse("22222222-2222-2222-2222-222222222222"),
			CategoryID:   categoryID,
			Kind:         valueobjects.KindExpense,
			Term:         "Aluguel",
			SignalType:   valueobjects.SignalTypeCanonicalName,
			Confidence:   valueobjects.ConfidenceHigh,
			IsAmbiguous:  false,
			DeprecatedAt: nil,
		},
	}

	s.repo.EXPECT().List(mock.Anything, mock.Anything).Return(entries, "next-cursor", nil).Once()

	result, err := s.useCase.Execute(s.ctx, &input.ListDictionaryInput{})

	s.NoError(err)
	s.NotNil(result)
	s.Equal(int64(42), result.Version)
	s.Len(result.Entries, 1)
	s.Equal("Aluguel", result.Entries[0].Term)
	s.Equal("next-cursor", result.NextCursor)
}

func (s *ListDictionarySuite) TestExecute_WithPagination() {
	s.versionReader.EXPECT().Current(mock.Anything).Return(int64(1), nil).Once()

	kind := valueobjects.KindIncome
	signalType := valueobjects.SignalTypeAlias
	categoryID := "cat-123"
	cursor := "some-cursor"
	pageSize := 25

	s.repo.EXPECT().List(mock.Anything, mock.Anything).Return([]entities.DictionaryEntry{}, "", nil).Once()

	result, err := s.useCase.Execute(s.ctx, &input.ListDictionaryInput{
		Kind:       &kind,
		SignalType: &signalType,
		CategoryID: &categoryID,
		Cursor:     cursor,
		PageSize:   pageSize,
	})

	s.NoError(err)
	s.NotNil(result)
}

func (s *ListDictionarySuite) TestExecute_DefaultPageSize() {
	s.versionReader.EXPECT().Current(mock.Anything).Return(int64(1), nil).Once()

	s.repo.EXPECT().List(mock.Anything, mock.Anything).Return([]entities.DictionaryEntry{}, "", nil).Once()

	result, err := s.useCase.Execute(s.ctx, &input.ListDictionaryInput{
		PageSize: 0,
	})

	s.NoError(err)
	s.NotNil(result)
}

func (s *ListDictionarySuite) TestExecute_MaxPageSize() {
	s.versionReader.EXPECT().Current(mock.Anything).Return(int64(1), nil).Once()

	s.repo.EXPECT().List(mock.Anything, mock.Anything).Return([]entities.DictionaryEntry{}, "", nil).Once()

	result, err := s.useCase.Execute(s.ctx, &input.ListDictionaryInput{
		PageSize: 500,
	})

	s.NoError(err)
	s.NotNil(result)
}

func (s *ListDictionarySuite) TestExecute_VersionError() {
	s.versionReader.EXPECT().Current(mock.Anything).Return(int64(0), errors.New("db error")).Once()

	result, err := s.useCase.Execute(s.ctx, &input.ListDictionaryInput{})

	s.Error(err)
	s.Nil(result)
	s.Contains(err.Error(), "ler versao")
}

func (s *ListDictionarySuite) TestExecute_RepoError() {
	s.versionReader.EXPECT().Current(mock.Anything).Return(int64(1), nil).Once()
	s.repo.EXPECT().List(mock.Anything, mock.Anything).Return(nil, "", errors.New("db error")).Once()

	result, err := s.useCase.Execute(s.ctx, &input.ListDictionaryInput{})

	s.Error(err)
	s.Nil(result)
	s.Contains(err.Error(), "listar dicionario")
}

func (s *ListDictionarySuite) TestExecute_WithDeprecatedEntries() {
	s.versionReader.EXPECT().Current(mock.Anything).Return(int64(42), nil).Once()

	categoryID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	deprecatedAt := time.Now()
	entries := []entities.DictionaryEntry{
		{
			ID:           uuid.MustParse("22222222-2222-2222-2222-222222222222"),
			CategoryID:   categoryID,
			Kind:         valueobjects.KindExpense,
			Term:         "Old Term",
			SignalType:   valueobjects.SignalTypeAlias,
			Confidence:   valueobjects.ConfidenceHigh,
			IsAmbiguous:  false,
			DeprecatedAt: &deprecatedAt,
		},
	}

	s.repo.EXPECT().List(mock.Anything, mock.Anything).Return(entries, "", nil).Once()

	result, err := s.useCase.Execute(s.ctx, &input.ListDictionaryInput{})

	s.NoError(err)
	s.NotNil(result)
	s.Len(result.Entries, 1)
	s.NotNil(result.Entries[0].DeprecatedAt)
}
