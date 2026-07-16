package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
	mockInterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces/mocks"
	uowMocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/option"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type SearchEditCandidatesSuite struct {
	suite.Suite
	ctx       context.Context
	userID    uuid.UUID
	brazilLoc *time.Location
	factory   *mockInterfaces.RepositoryFactory
	repo      *mockInterfaces.TransactionRepository
	uow       *uowMocks.UnitOfWorkOutputTransaction
}

func TestSearchEditCandidatesSuite(t *testing.T) {
	suite.Run(t, new(SearchEditCandidatesSuite))
}

func (s *SearchEditCandidatesSuite) SetupTest() {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	s.Require().NoError(err)

	s.userID = uuid.New()
	s.brazilLoc = loc
	s.ctx = auth.WithPrincipal(context.Background(), auth.Principal{UserID: s.userID, Source: auth.SourceHeader})
	s.factory = mockInterfaces.NewRepositoryFactory(s.T())
	s.repo = mockInterfaces.NewTransactionRepository(s.T())
	s.factory.EXPECT().TransactionRepository(mock.Anything).Return(s.repo).Maybe()
	s.uow = uowMocks.NewUnitOfWorkOutputTransaction(s.T())
}

func (s *SearchEditCandidatesSuite) makeTransaction(desc string, amountCents int64) *entities.Transaction {
	userID := valueobjects.UserIDFromUUID(s.userID)
	dir := valueobjects.DirectionOutcome
	pm := valueobjects.PaymentMethodPix
	amount, _ := valueobjects.NewMoney(amountCents)
	d, _ := valueobjects.NewDescription(desc)
	catID := valueobjects.CategoryIDFromUUID(uuid.New())
	rm, _ := valueobjects.NewRefMonth("2026-06")
	now := time.Now().UTC()
	tx := entities.Reconstitute(
		uuid.New(), userID, dir, pm, amount, d, catID,
		option.None[valueobjects.SubcategoryID](),
		"Cat", "", valueobjects.CategoryWriteEvidence{}, rm, now, option.None[valueobjects.CardID](), option.None[valueobjects.InstallmentCount](), option.None[valueobjects.CardBillingSnapshot](), 1, nil, now, now,
	)
	return &tx
}

func (s *SearchEditCandidatesSuite) TestExecute() {
	explicitRefMonth, _ := valueobjects.NewRefMonth("2026-06")
	currentRefMonth := valueobjects.RefMonthFromTime(time.Now().UTC(), s.brazilLoc)

	type args struct {
		ctx         context.Context
		amountCents int64
		term        string
		refMonth    string
		limit       int
	}
	type dependencies struct {
		repo *mockInterfaces.TransactionRepository
	}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(result []output.Transaction, err error)
	}{
		{
			name: "era 25: busca só por valor",
			args: args{ctx: s.ctx, amountCents: 2500, term: "", refMonth: "", limit: 0},
			dependencies: dependencies{
				repo: func() *mockInterfaces.TransactionRepository {
					s.repo.EXPECT().
						SearchEditCandidates(mock.Anything, s.userID, int64(2500), "", currentRefMonth, 5).
						Return([]*entities.Transaction{s.makeTransaction("Farmácia", 2500)}, nil).
						Once()
					return s.repo
				}(),
			},
			expect: func(result []output.Transaction, err error) {
				s.Require().NoError(err)
				s.Require().Len(result, 1)
				s.Equal(int64(2500), result[0].AmountCents)
			},
		},
		{
			name: "aquele mercado: busca só por termo",
			args: args{ctx: s.ctx, amountCents: 0, term: "mercado", refMonth: "", limit: 0},
			dependencies: dependencies{
				repo: func() *mockInterfaces.TransactionRepository {
					s.repo.EXPECT().
						SearchEditCandidates(mock.Anything, s.userID, int64(0), "mercado", currentRefMonth, 5).
						Return([]*entities.Transaction{s.makeTransaction("Mercado Extra", 4200)}, nil).
						Once()
					return s.repo
				}(),
			},
			expect: func(result []output.Transaction, err error) {
				s.Require().NoError(err)
				s.Require().Len(result, 1)
				s.Equal("Mercado Extra", result[0].Description)
			},
		},
		{
			name: "valor e termo informados juntos",
			args: args{ctx: s.ctx, amountCents: 4200, term: "mercado", refMonth: "", limit: 0},
			dependencies: dependencies{
				repo: func() *mockInterfaces.TransactionRepository {
					s.repo.EXPECT().
						SearchEditCandidates(mock.Anything, s.userID, int64(4200), "mercado", currentRefMonth, 5).
						Return([]*entities.Transaction{s.makeTransaction("Mercado Extra", 4200)}, nil).
						Once()
					return s.repo
				}(),
			},
			expect: func(result []output.Transaction, err error) {
				s.Require().NoError(err)
				s.Require().Len(result, 1)
			},
		},
		{
			name:         "valor e termo ambos vazios retorna erro de validação",
			args:         args{ctx: s.ctx, amountCents: 0, term: "", refMonth: "", limit: 0},
			dependencies: dependencies{repo: s.repo},
			expect: func(result []output.Transaction, err error) {
				s.Require().Error(err)
				s.Nil(result)
			},
		},
		{
			name: "ref_month explícito é respeitado",
			args: args{ctx: s.ctx, amountCents: 1000, term: "", refMonth: "2026-06", limit: 0},
			dependencies: dependencies{
				repo: func() *mockInterfaces.TransactionRepository {
					s.repo.EXPECT().
						SearchEditCandidates(mock.Anything, s.userID, int64(1000), "", explicitRefMonth, 5).
						Return(nil, nil).
						Once()
					return s.repo
				}(),
			},
			expect: func(result []output.Transaction, err error) {
				s.Require().NoError(err)
			},
		},
		{
			name:         "ref_month inválido retorna erro",
			args:         args{ctx: s.ctx, amountCents: 1000, term: "", refMonth: "bad", limit: 0},
			dependencies: dependencies{repo: s.repo},
			expect: func(result []output.Transaction, err error) {
				s.Require().Error(err)
			},
		},
		{
			name: "limit acima do teto é limitado a 5",
			args: args{ctx: s.ctx, amountCents: 1000, term: "", refMonth: "", limit: 999},
			dependencies: dependencies{
				repo: func() *mockInterfaces.TransactionRepository {
					s.repo.EXPECT().
						SearchEditCandidates(mock.Anything, s.userID, int64(1000), "", currentRefMonth, 5).
						Return(nil, nil).
						Once()
					return s.repo
				}(),
			},
			expect: func(result []output.Transaction, err error) {
				s.Require().NoError(err)
			},
		},
		{
			name:         "não autenticado retorna erro",
			args:         args{ctx: context.Background(), amountCents: 1000, term: "", refMonth: "", limit: 0},
			dependencies: dependencies{repo: s.repo},
			expect: func(result []output.Transaction, err error) {
				s.Require().Error(err)
			},
		},
		{
			name: "erro de infraestrutura é propagado",
			args: args{ctx: s.ctx, amountCents: 1000, term: "", refMonth: "", limit: 0},
			dependencies: dependencies{
				repo: func() *mockInterfaces.TransactionRepository {
					s.repo.EXPECT().
						SearchEditCandidates(mock.Anything, s.userID, int64(1000), "", currentRefMonth, 5).
						Return(nil, errors.New("db down")).
						Once()
					return s.repo
				}(),
			},
			expect: func(result []output.Transaction, err error) {
				s.Require().Error(err)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			uc := NewSearchEditCandidates(s.factory, s.uow, s.brazilLoc, fake.NewProvider())
			result, err := uc.Execute(scenario.args.ctx, scenario.args.amountCents, scenario.args.term, scenario.args.refMonth, scenario.args.limit)
			scenario.expect(result, err)
		})
	}
}
