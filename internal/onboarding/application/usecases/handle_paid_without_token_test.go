package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
)

type HandlePaidWithoutTokenSuite struct {
	suite.Suite
	ctx        context.Context
	obs        observability.Observability
	signalRepo *mocks.SupportSignalRepository
	idGen      id.Generator
}

func TestHandlePaidWithoutToken(t *testing.T) {
	suite.Run(t, new(HandlePaidWithoutTokenSuite))
}

func (s *HandlePaidWithoutTokenSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.signalRepo = mocks.NewSupportSignalRepository(s.T())
	s.idGen = id.NewUUIDGenerator()
}

func (s *HandlePaidWithoutTokenSuite) TestExecute() {
	type args struct {
		in input.HandlePaidWithoutTokenInput
	}
	type dependencies struct {
		signalRepo *mocks.SupportSignalRepository
	}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(err error)
	}{
		{
			name: "deve retornar erro de validacao quando ExternalSaleID vazio",
			args: args{
				in: input.HandlePaidWithoutTokenInput{
					ExternalSaleID: "",
					PaidAt:         time.Now().UTC(),
				},
			},
			dependencies: dependencies{signalRepo: s.signalRepo},
			expect: func(err error) {
				s.Error(err)
				s.ErrorIs(err, input.ErrExternalSaleIDRequired)
			},
		},
		{
			name: "deve retornar erro de validacao quando PaidAt zero",
			args: args{
				in: input.HandlePaidWithoutTokenInput{
					ExternalSaleID: "sale-001",
					PaidAt:         time.Time{},
				},
			},
			dependencies: dependencies{signalRepo: s.signalRepo},
			expect: func(err error) {
				s.Error(err)
				s.ErrorIs(err, input.ErrPaidAtRequired)
			},
		},
		{
			name: "deve retornar erro quando Insert falha",
			args: args{
				in: input.HandlePaidWithoutTokenInput{
					ExternalSaleID:     "sale-002",
					CustomerMobileE164: "+5511999990001",
					CustomerEmail:      "user@example.com",
					PaidAt:             time.Now().UTC(),
				},
			},
			dependencies: dependencies{
				signalRepo: func() *mocks.SupportSignalRepository {
					s.signalRepo.EXPECT().
						Insert(mock.Anything, mock.Anything).
						Return(errors.New("db error")).
						Once()
					return s.signalRepo
				}(),
			},
			expect: func(err error) {
				s.Error(err)
				s.ErrorContains(err, "insert signal")
			},
		},
		{
			name: "deve inserir sinal com sucesso",
			args: args{
				in: input.HandlePaidWithoutTokenInput{
					ExternalSaleID:     "sale-003",
					CustomerMobileE164: "+5511999990002",
					CustomerEmail:      "ok@example.com",
					PaidAt:             time.Now().UTC(),
				},
			},
			dependencies: dependencies{
				signalRepo: func() *mocks.SupportSignalRepository {
					s.signalRepo.EXPECT().
						Insert(mock.Anything, mock.Anything).
						Return(nil).
						Once()
					return s.signalRepo
				}(),
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			uc := NewHandlePaidWithoutToken(scenario.dependencies.signalRepo, s.idGen, s.obs)
			err := uc.Execute(s.ctx, scenario.args.in)
			scenario.expect(err)
		})
	}
}
