package usecases_test

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/interfaces"
	mockInterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

type ResolveBySlugSuite struct {
	suite.Suite
	ctx     context.Context
	repo    *mockInterfaces.CategoryRepository
	useCase *usecases.ResolveBySlug
}

func TestResolveBySlugSuite(t *testing.T) {
	suite.Run(t, new(ResolveBySlugSuite))
}

func (s *ResolveBySlugSuite) SetupTest() {
	s.ctx = context.Background()
	s.repo = mockInterfaces.NewCategoryRepository(s.T())
	s.useCase = usecases.NewResolveBySlug(s.repo, noop.NewProvider())
}

func (s *ResolveBySlugSuite) TestExecute_SlugUnico() {
	rootID := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	s.repo.EXPECT().List(s.ctx, interfaces.CategoryQuery{IncludeDeprecated: false}).Return([]entities.Category{
		{
			ID:             rootID,
			Slug:           "custo-fixo",
			Kind:           valueobjects.KindExpense,
			AllocationType: valueobjects.AllocationTypeConsumption,
		},
	}, nil).Once()

	result, err := s.useCase.Execute(s.ctx, []string{"custo-fixo"})

	s.NoError(err)
	s.Len(result, 1)
	s.Equal(rootID, result["custo-fixo"])
}

func (s *ResolveBySlugSuite) TestExecute_MultiplosSlugs() {
	id1 := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	id2 := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	s.repo.EXPECT().List(s.ctx, interfaces.CategoryQuery{IncludeDeprecated: false}).Return([]entities.Category{
		{
			ID:             id1,
			Slug:           "custo-fixo",
			Kind:           valueobjects.KindExpense,
			AllocationType: valueobjects.AllocationTypeConsumption,
		},
		{
			ID:             id2,
			Slug:           "renda",
			Kind:           valueobjects.KindIncome,
			AllocationType: valueobjects.AllocationTypeConsumption,
		},
	}, nil).Once()

	result, err := s.useCase.Execute(s.ctx, []string{"custo-fixo", "renda"})

	s.NoError(err)
	s.Len(result, 2)
	s.Equal(id1, result["custo-fixo"])
	s.Equal(id2, result["renda"])
}

func (s *ResolveBySlugSuite) TestExecute_SliceVazia() {
	s.repo.EXPECT().List(s.ctx, interfaces.CategoryQuery{IncludeDeprecated: false}).Return([]entities.Category{}, nil).Once()

	result, err := s.useCase.Execute(s.ctx, []string{})

	s.NoError(err)
	s.Empty(result)
}

func (s *ResolveBySlugSuite) TestExecute_SlugNaoEncontrado() {
	rootID := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	s.repo.EXPECT().List(s.ctx, interfaces.CategoryQuery{IncludeDeprecated: false}).Return([]entities.Category{
		{
			ID:             rootID,
			Slug:           "custo-fixo",
			Kind:           valueobjects.KindExpense,
			AllocationType: valueobjects.AllocationTypeConsumption,
		},
	}, nil).Once()

	result, err := s.useCase.Execute(s.ctx, []string{"inexistente"})

	s.Nil(result)
	s.True(errors.Is(err, usecases.ErrCategoryNotFound))
}

func (s *ResolveBySlugSuite) TestExecute_SubcategoriasIgnoradas() {
	rootID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	subID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	s.repo.EXPECT().List(s.ctx, interfaces.CategoryQuery{IncludeDeprecated: false}).Return([]entities.Category{
		{
			ID:             rootID,
			Slug:           "custo-fixo",
			Kind:           valueobjects.KindExpense,
			AllocationType: valueobjects.AllocationTypeConsumption,
		},
		{
			ID:             subID,
			Slug:           "aluguel",
			Kind:           valueobjects.KindExpense,
			ParentID:       &rootID,
			AllocationType: valueobjects.AllocationTypeConsumption,
		},
	}, nil).Once()

	result, err := s.useCase.Execute(s.ctx, []string{"aluguel"})

	s.Nil(result)
	s.True(errors.Is(err, usecases.ErrCategoryNotFound))
}

func (s *ResolveBySlugSuite) TestExecute_ErroNoRepositorio() {
	s.repo.EXPECT().List(s.ctx, interfaces.CategoryQuery{IncludeDeprecated: false}).Return(nil, errors.New("db error")).Once()

	result, err := s.useCase.Execute(s.ctx, []string{"qualquer"})

	s.Nil(result)
	s.Error(err)
	s.Contains(err.Error(), "listar categorias")
}

func (s *ResolveBySlugSuite) TestExecute_DeprecatedIgnorada() {
	rootID := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	s.repo.EXPECT().List(s.ctx, interfaces.CategoryQuery{IncludeDeprecated: false}).Return([]entities.Category{
		{
			ID:             rootID,
			Slug:           "custo-fixo",
			Kind:           valueobjects.KindExpense,
			AllocationType: valueobjects.AllocationTypeConsumption,
		},
	}, nil).Once()

	result, err := s.useCase.Execute(s.ctx, []string{"antigo"})

	s.Nil(result)
	s.True(errors.Is(err, usecases.ErrCategoryNotFound))
}
