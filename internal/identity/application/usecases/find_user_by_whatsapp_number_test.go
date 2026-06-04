package usecases_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/observability/fakes"
)

type FindUserByWhatsAppNumberSuite struct {
	suite.Suite
}

func TestFindUserByWhatsAppNumberSuite(t *testing.T) {
	suite.Run(t, new(FindUserByWhatsAppNumberSuite))
}

func (s *FindUserByWhatsAppNumberSuite) TestExecute() {
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	number, _ := valueobjects.NewWhatsAppNumber("+5511987654321")
	id, _ := entities.NewUserID("550e8400-e29b-41d4-a716-446655440000")
	user, _ := entities.NewUser(entities.NewUserParams{
		ID:        id,
		Number:    number,
		CreatedAt: now,
		UpdatedAt: now,
	})

	scenarios := []struct {
		name      string
		rawNumber string
		setup     func(repo *mocks.UserRepository)
		wantUser  *entities.User
		wantErr   bool
	}{
		{
			name:      "happy path retorna user",
			rawNumber: "+5511987654321",
			setup: func(repo *mocks.UserRepository) {
				repo.EXPECT().FindByWhatsAppNumber(mock.Anything, number).Return(user, nil)
			},
			wantUser: user,
		},
		{
			name:      "erro de validação de VO — número inválido",
			rawNumber: "invalid",
			setup:     func(_ *mocks.UserRepository) {},
			wantErr:   true,
		},
		{
			name:      "erro de repositório",
			rawNumber: "+5511987654321",
			setup: func(repo *mocks.UserRepository) {
				repo.EXPECT().FindByWhatsAppNumber(mock.Anything, number).Return(nil, errors.New("db error"))
			},
			wantErr: true,
		},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			repo := mocks.NewUserRepository(s.T())
			uc := usecases.NewFindUserByWhatsAppNumberUseCase(repo, fakes.NoopObservability(), fakes.NoopUsecaseMetrics())
			sc.setup(repo)

			got, err := uc.Execute(context.Background(), sc.rawNumber)
			if sc.wantErr {
				s.Error(err)
				s.Nil(got)
			} else {
				s.NoError(err)
				s.Equal(sc.wantUser, got)
			}
		})
	}
}
