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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

type ResolveCategoryForWriteSuite struct {
	suite.Suite
	ctx           context.Context
	obs           observability.Observability
	categoryRepo  *mockInterfaces.CategoryRepository
	versionReader *mockInterfaces.VersionReader
	useCase       *ResolveCategoryForWrite
}

func TestResolveCategoryForWriteSuite(t *testing.T) {
	suite.Run(t, new(ResolveCategoryForWriteSuite))
}

func (s *ResolveCategoryForWriteSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.categoryRepo = mockInterfaces.NewCategoryRepository(s.T())
	s.versionReader = mockInterfaces.NewVersionReader(s.T())
	s.useCase = NewResolveCategoryForWrite(s.categoryRepo, s.versionReader, s.obs)
}

func (s *ResolveCategoryForWriteSuite) TestExecute() {
	rootID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	subID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	otherRootID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	deprecatedAt := time.Now()

	type args struct {
		in *input.ResolveCategoryForWriteInput
	}
	type dependencies struct {
		categoryRepo  *mockInterfaces.CategoryRepository
		versionReader *mockInterfaces.VersionReader
	}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(out any, err error)
	}{
		{
			name: "aceite: raiz e subcategoria validas, kind e versao corretos",
			args: args{in: &input.ResolveCategoryForWriteInput{
				RootCategoryID:  rootID,
				SubcategoryID:   subID,
				Kind:            valueobjects.KindExpense,
				ExpectedVersion: 1,
			}},
			dependencies: dependencies{
				versionReader: func() *mockInterfaces.VersionReader {
					s.versionReader.EXPECT().Current(mock.Anything).Return(int64(1), nil).Once()
					return s.versionReader
				}(),
				categoryRepo: func() *mockInterfaces.CategoryRepository {
					s.categoryRepo.EXPECT().GetByID(mock.Anything, rootID).Return(entities.Category{
						ID:   rootID,
						Slug: "custo-fixo",
						Name: "Custo Fixo",
						Kind: valueobjects.KindExpense,
					}, nil).Once()
					s.categoryRepo.EXPECT().GetByID(mock.Anything, subID).Return(entities.Category{
						ID:       subID,
						Slug:     "aluguel",
						Name:     "Aluguel",
						Kind:     valueobjects.KindExpense,
						ParentID: &rootID,
					}, nil).Once()
					return s.categoryRepo
				}(),
			},
			expect: func(out any, err error) {
				s.NoError(err)
				s.NotNil(out)
			},
		},
		{
			name: "root inexistente: retorna ErrRootCategoryNotFound",
			args: args{in: &input.ResolveCategoryForWriteInput{
				RootCategoryID:  rootID,
				SubcategoryID:   subID,
				Kind:            valueobjects.KindExpense,
				ExpectedVersion: 1,
			}},
			dependencies: dependencies{
				versionReader: func() *mockInterfaces.VersionReader {
					s.versionReader.EXPECT().Current(mock.Anything).Return(int64(1), nil).Once()
					return s.versionReader
				}(),
				categoryRepo: func() *mockInterfaces.CategoryRepository {
					s.categoryRepo.EXPECT().GetByID(mock.Anything, rootID).Return(entities.Category{}, interfaces.ErrNotFound).Once()
					return s.categoryRepo
				}(),
			},
			expect: func(out any, err error) {
				s.ErrorIs(err, ErrRootCategoryNotFound)
				s.Nil(out)
			},
		},
		{
			name: "leaf inexistente: retorna ErrSubcategoryNotFound",
			args: args{in: &input.ResolveCategoryForWriteInput{
				RootCategoryID:  rootID,
				SubcategoryID:   subID,
				Kind:            valueobjects.KindExpense,
				ExpectedVersion: 1,
			}},
			dependencies: dependencies{
				versionReader: func() *mockInterfaces.VersionReader {
					s.versionReader.EXPECT().Current(mock.Anything).Return(int64(1), nil).Once()
					return s.versionReader
				}(),
				categoryRepo: func() *mockInterfaces.CategoryRepository {
					s.categoryRepo.EXPECT().GetByID(mock.Anything, rootID).Return(entities.Category{
						ID:   rootID,
						Slug: "custo-fixo",
						Name: "Custo Fixo",
						Kind: valueobjects.KindExpense,
					}, nil).Once()
					s.categoryRepo.EXPECT().GetByID(mock.Anything, subID).Return(entities.Category{}, interfaces.ErrNotFound).Once()
					return s.categoryRepo
				}(),
			},
			expect: func(out any, err error) {
				s.ErrorIs(err, ErrSubcategoryNotFound)
				s.Nil(out)
			},
		},
		{
			name: "root deprecated: retorna ErrCategoryDeprecated",
			args: args{in: &input.ResolveCategoryForWriteInput{
				RootCategoryID:  rootID,
				SubcategoryID:   subID,
				Kind:            valueobjects.KindExpense,
				ExpectedVersion: 1,
			}},
			dependencies: dependencies{
				versionReader: func() *mockInterfaces.VersionReader {
					s.versionReader.EXPECT().Current(mock.Anything).Return(int64(1), nil).Once()
					return s.versionReader
				}(),
				categoryRepo: func() *mockInterfaces.CategoryRepository {
					s.categoryRepo.EXPECT().GetByID(mock.Anything, rootID).Return(entities.Category{
						ID:           rootID,
						Slug:         "custo-fixo",
						Name:         "Custo Fixo",
						Kind:         valueobjects.KindExpense,
						DeprecatedAt: &deprecatedAt,
					}, nil).Once()
					return s.categoryRepo
				}(),
			},
			expect: func(out any, err error) {
				s.ErrorIs(err, ErrCategoryDeprecated)
				s.Nil(out)
			},
		},
		{
			name: "leaf deprecated: retorna ErrCategoryDeprecated",
			args: args{in: &input.ResolveCategoryForWriteInput{
				RootCategoryID:  rootID,
				SubcategoryID:   subID,
				Kind:            valueobjects.KindExpense,
				ExpectedVersion: 1,
			}},
			dependencies: dependencies{
				versionReader: func() *mockInterfaces.VersionReader {
					s.versionReader.EXPECT().Current(mock.Anything).Return(int64(1), nil).Once()
					return s.versionReader
				}(),
				categoryRepo: func() *mockInterfaces.CategoryRepository {
					s.categoryRepo.EXPECT().GetByID(mock.Anything, rootID).Return(entities.Category{
						ID:   rootID,
						Slug: "custo-fixo",
						Name: "Custo Fixo",
						Kind: valueobjects.KindExpense,
					}, nil).Once()
					s.categoryRepo.EXPECT().GetByID(mock.Anything, subID).Return(entities.Category{
						ID:           subID,
						Slug:         "aluguel",
						Name:         "Aluguel",
						Kind:         valueobjects.KindExpense,
						ParentID:     &rootID,
						DeprecatedAt: &deprecatedAt,
					}, nil).Once()
					return s.categoryRepo
				}(),
			},
			expect: func(out any, err error) {
				s.ErrorIs(err, ErrCategoryDeprecated)
				s.Nil(out)
			},
		},
		{
			name: "leaf de outra raiz: retorna ErrLeafNotFromRoot",
			args: args{in: &input.ResolveCategoryForWriteInput{
				RootCategoryID:  rootID,
				SubcategoryID:   subID,
				Kind:            valueobjects.KindExpense,
				ExpectedVersion: 1,
			}},
			dependencies: dependencies{
				versionReader: func() *mockInterfaces.VersionReader {
					s.versionReader.EXPECT().Current(mock.Anything).Return(int64(1), nil).Once()
					return s.versionReader
				}(),
				categoryRepo: func() *mockInterfaces.CategoryRepository {
					s.categoryRepo.EXPECT().GetByID(mock.Anything, rootID).Return(entities.Category{
						ID:   rootID,
						Slug: "custo-fixo",
						Name: "Custo Fixo",
						Kind: valueobjects.KindExpense,
					}, nil).Once()
					s.categoryRepo.EXPECT().GetByID(mock.Anything, subID).Return(entities.Category{
						ID:       subID,
						Slug:     "salario",
						Name:     "Salario",
						Kind:     valueobjects.KindExpense,
						ParentID: &otherRootID,
					}, nil).Once()
					return s.categoryRepo
				}(),
			},
			expect: func(out any, err error) {
				s.ErrorIs(err, ErrLeafNotFromRoot)
				s.Nil(out)
			},
		},
		{
			name: "root sem leaf: subcategory eh raiz retorna ErrRootWithoutLeaf",
			args: args{in: &input.ResolveCategoryForWriteInput{
				RootCategoryID:  rootID,
				SubcategoryID:   otherRootID,
				Kind:            valueobjects.KindExpense,
				ExpectedVersion: 1,
			}},
			dependencies: dependencies{
				versionReader: func() *mockInterfaces.VersionReader {
					s.versionReader.EXPECT().Current(mock.Anything).Return(int64(1), nil).Once()
					return s.versionReader
				}(),
				categoryRepo: func() *mockInterfaces.CategoryRepository {
					s.categoryRepo.EXPECT().GetByID(mock.Anything, rootID).Return(entities.Category{
						ID:   rootID,
						Slug: "custo-fixo",
						Name: "Custo Fixo",
						Kind: valueobjects.KindExpense,
					}, nil).Once()
					s.categoryRepo.EXPECT().GetByID(mock.Anything, otherRootID).Return(entities.Category{
						ID:   otherRootID,
						Slug: "lazer",
						Name: "Lazer",
						Kind: valueobjects.KindExpense,
					}, nil).Once()
					return s.categoryRepo
				}(),
			},
			expect: func(out any, err error) {
				s.ErrorIs(err, ErrRootWithoutLeaf)
				s.Nil(out)
			},
		},
		{
			name: "kind divergente: root com kind diferente retorna ErrKindMismatch",
			args: args{in: &input.ResolveCategoryForWriteInput{
				RootCategoryID:  rootID,
				SubcategoryID:   subID,
				Kind:            valueobjects.KindIncome,
				ExpectedVersion: 1,
			}},
			dependencies: dependencies{
				versionReader: func() *mockInterfaces.VersionReader {
					s.versionReader.EXPECT().Current(mock.Anything).Return(int64(1), nil).Once()
					return s.versionReader
				}(),
				categoryRepo: func() *mockInterfaces.CategoryRepository {
					s.categoryRepo.EXPECT().GetByID(mock.Anything, rootID).Return(entities.Category{
						ID:   rootID,
						Slug: "custo-fixo",
						Name: "Custo Fixo",
						Kind: valueobjects.KindExpense,
					}, nil).Once()
					return s.categoryRepo
				}(),
			},
			expect: func(out any, err error) {
				s.ErrorIs(err, ErrKindMismatch)
				s.Nil(out)
			},
		},
		{
			name: "kind divergente: leaf com kind diferente retorna ErrKindMismatch",
			args: args{in: &input.ResolveCategoryForWriteInput{
				RootCategoryID:  rootID,
				SubcategoryID:   subID,
				Kind:            valueobjects.KindExpense,
				ExpectedVersion: 1,
			}},
			dependencies: dependencies{
				versionReader: func() *mockInterfaces.VersionReader {
					s.versionReader.EXPECT().Current(mock.Anything).Return(int64(1), nil).Once()
					return s.versionReader
				}(),
				categoryRepo: func() *mockInterfaces.CategoryRepository {
					s.categoryRepo.EXPECT().GetByID(mock.Anything, rootID).Return(entities.Category{
						ID:   rootID,
						Slug: "custo-fixo",
						Name: "Custo Fixo",
						Kind: valueobjects.KindExpense,
					}, nil).Once()
					s.categoryRepo.EXPECT().GetByID(mock.Anything, subID).Return(entities.Category{
						ID:       subID,
						Slug:     "salario",
						Name:     "Salario",
						Kind:     valueobjects.KindIncome,
						ParentID: &rootID,
					}, nil).Once()
					return s.categoryRepo
				}(),
			},
			expect: func(out any, err error) {
				s.ErrorIs(err, ErrKindMismatch)
				s.Nil(out)
			},
		},
		{
			name: "version drift: versao atual diferente da esperada retorna ErrVersionDrift",
			args: args{in: &input.ResolveCategoryForWriteInput{
				RootCategoryID:  rootID,
				SubcategoryID:   subID,
				Kind:            valueobjects.KindExpense,
				ExpectedVersion: 5,
			}},
			dependencies: dependencies{
				versionReader: func() *mockInterfaces.VersionReader {
					s.versionReader.EXPECT().Current(mock.Anything).Return(int64(6), nil).Once()
					return s.versionReader
				}(),
				categoryRepo: s.categoryRepo,
			},
			expect: func(out any, err error) {
				s.ErrorIs(err, ErrVersionDrift)
				s.Nil(out)
			},
		},
		{
			name: "root id nulo: retorna erro de validacao",
			args: args{in: &input.ResolveCategoryForWriteInput{
				RootCategoryID:  uuid.Nil,
				SubcategoryID:   subID,
				Kind:            valueobjects.KindExpense,
				ExpectedVersion: 1,
			}},
			dependencies: dependencies{
				versionReader: s.versionReader,
				categoryRepo:  s.categoryRepo,
			},
			expect: func(out any, err error) {
				s.ErrorIs(err, input.ErrRootCategoryIDRequired)
				s.Nil(out)
			},
		},
		{
			name: "subcategory id nulo: retorna erro de validacao",
			args: args{in: &input.ResolveCategoryForWriteInput{
				RootCategoryID:  rootID,
				SubcategoryID:   uuid.Nil,
				Kind:            valueobjects.KindExpense,
				ExpectedVersion: 1,
			}},
			dependencies: dependencies{
				versionReader: s.versionReader,
				categoryRepo:  s.categoryRepo,
			},
			expect: func(out any, err error) {
				s.ErrorIs(err, input.ErrSubcategoryIDRequired)
				s.Nil(out)
			},
		},
		{
			name: "kind invalido: retorna erro de validacao",
			args: args{in: &input.ResolveCategoryForWriteInput{
				RootCategoryID:  rootID,
				SubcategoryID:   subID,
				Kind:            valueobjects.Kind(0),
				ExpectedVersion: 1,
			}},
			dependencies: dependencies{
				versionReader: s.versionReader,
				categoryRepo:  s.categoryRepo,
			},
			expect: func(out any, err error) {
				s.ErrorIs(err, input.ErrInvalidKind)
				s.Nil(out)
			},
		},
		{
			name: "version zero: retorna erro de validacao",
			args: args{in: &input.ResolveCategoryForWriteInput{
				RootCategoryID:  rootID,
				SubcategoryID:   subID,
				Kind:            valueobjects.KindExpense,
				ExpectedVersion: 0,
			}},
			dependencies: dependencies{
				versionReader: s.versionReader,
				categoryRepo:  s.categoryRepo,
			},
			expect: func(out any, err error) {
				s.ErrorIs(err, input.ErrVersionRequired)
				s.Nil(out)
			},
		},
		{
			name: "erro ao ler versao: retorna erro de infraestrutura",
			args: args{in: &input.ResolveCategoryForWriteInput{
				RootCategoryID:  rootID,
				SubcategoryID:   subID,
				Kind:            valueobjects.KindExpense,
				ExpectedVersion: 1,
			}},
			dependencies: dependencies{
				versionReader: func() *mockInterfaces.VersionReader {
					s.versionReader.EXPECT().Current(mock.Anything).Return(int64(0), errors.New("db error")).Once()
					return s.versionReader
				}(),
				categoryRepo: s.categoryRepo,
			},
			expect: func(out any, err error) {
				s.Error(err)
				s.Contains(err.Error(), "ler versao")
				s.Nil(out)
			},
		},
		{
			name: "aceite: verifica campos do output",
			args: args{in: &input.ResolveCategoryForWriteInput{
				RootCategoryID:  rootID,
				SubcategoryID:   subID,
				Kind:            valueobjects.KindExpense,
				ExpectedVersion: 7,
			}},
			dependencies: dependencies{
				versionReader: func() *mockInterfaces.VersionReader {
					s.versionReader.EXPECT().Current(mock.Anything).Return(int64(7), nil).Once()
					return s.versionReader
				}(),
				categoryRepo: func() *mockInterfaces.CategoryRepository {
					s.categoryRepo.EXPECT().GetByID(mock.Anything, rootID).Return(entities.Category{
						ID:   rootID,
						Slug: "custo-fixo",
						Name: "Custo Fixo",
						Kind: valueobjects.KindExpense,
					}, nil).Once()
					s.categoryRepo.EXPECT().GetByID(mock.Anything, subID).Return(entities.Category{
						ID:       subID,
						Slug:     "aluguel",
						Name:     "Aluguel",
						Kind:     valueobjects.KindExpense,
						ParentID: &rootID,
					}, nil).Once()
					return s.categoryRepo
				}(),
			},
			expect: func(out any, err error) {
				s.NoError(err)
				s.NotNil(out)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			uc := NewResolveCategoryForWrite(scenario.dependencies.categoryRepo, scenario.dependencies.versionReader, s.obs)
			result, err := uc.Execute(s.ctx, scenario.args.in)
			scenario.expect(result, err)
		})
	}
}

func (s *ResolveCategoryForWriteSuite) TestExecute_OutputFields() {
	rootID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	subID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	s.versionReader.EXPECT().Current(mock.Anything).Return(int64(42), nil).Once()
	s.categoryRepo.EXPECT().GetByID(mock.Anything, rootID).Return(entities.Category{
		ID:   rootID,
		Slug: "custo-fixo",
		Name: "Custo Fixo",
		Kind: valueobjects.KindExpense,
	}, nil).Once()
	s.categoryRepo.EXPECT().GetByID(mock.Anything, subID).Return(entities.Category{
		ID:       subID,
		Slug:     "aluguel",
		Name:     "Aluguel",
		Kind:     valueobjects.KindExpense,
		ParentID: &rootID,
	}, nil).Once()

	result, err := s.useCase.Execute(s.ctx, &input.ResolveCategoryForWriteInput{
		RootCategoryID:  rootID,
		SubcategoryID:   subID,
		Kind:            valueobjects.KindExpense,
		ExpectedVersion: 42,
	})

	s.NoError(err)
	s.NotNil(result)
	s.Equal(rootID, result.RootCategoryID)
	s.Equal(subID, result.SubcategoryID)
	s.Equal("expense", result.Kind)
	s.Equal("Custo Fixo > Aluguel", result.Path)
	s.Equal("custo-fixo", result.RootSlug)
	s.Equal("aluguel", result.SubcategorySlug)
	s.Equal("Custo Fixo", result.CategoryName)
	s.Equal("Aluguel", result.SubcategoryName)
	s.Equal(int64(42), result.EditorialVersion)
	s.False(result.Deprecated)
}
