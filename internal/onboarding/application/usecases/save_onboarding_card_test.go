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

	onbinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/dtos/input"
	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	outboxmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox/mocks"
)

type SaveOnboardingCardSuite struct {
	suite.Suite
	obs         observability.Observability
	ctx         context.Context
	sessionRepo *mocks.OnboardingSessionRepository
	factory     *mocks.RepositoryFactory
	publisher   *outboxmocks.Publisher
	userID      uuid.UUID
}

func TestSaveOnboardingCardSuite(t *testing.T) {
	suite.Run(t, new(SaveOnboardingCardSuite))
}

func (s *SaveOnboardingCardSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.sessionRepo = mocks.NewOnboardingSessionRepository(s.T())
	s.factory = mocks.NewRepositoryFactory(s.T())
	s.publisher = outboxmocks.NewPublisher(s.T())
	s.factory.EXPECT().OnboardingSessionRepository(mock.Anything).Return(s.sessionRepo).Maybe()
	s.userID = uuid.MustParse("88888888-8888-8888-8888-888888888888")
}

func (s *SaveOnboardingCardSuite) newUC(cardCreator SynchronousCardCreator) *SaveOnboardingCard {
	return NewSaveOnboardingCard(
		&onboardingUoWStub{},
		s.factory,
		s.publisher,
		&onboardingFixedIDGen{id: "99999999-9999-9999-9999-999999999999"},
		s.obs,
		cardCreator,
	)
}

func (s *SaveOnboardingCardSuite) baseSession() entities.OnboardingSession {
	return entities.HydrateOnboardingSession(
		s.userID,
		entities.OnboardingChannelWhatsApp,
		entities.OnboardingSessionPayload{},
		time.Now().UTC(),
	)
}

func (s *SaveOnboardingCardSuite) TestHappyPath_ClosingDay() {
	type args struct {
		in onbinput.SaveOnboardingCardInput
	}
	type dependencies struct {
		cardCreator SynchronousCardCreator
	}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(out SaveOnboardingCardResult, err error)
	}{
		{
			name: "closing_day valido persiste cartao e publica evento",
			args: args{in: onbinput.SaveOnboardingCardInput{
				UserID:     s.userID,
				Nickname:   "nubank",
				ClosingDay: 10,
			}},
			dependencies: dependencies{cardCreator: nil},
			expect: func(out SaveOnboardingCardResult, err error) {
				s.NoError(err)
				s.Equal("nubank", out.Name)
				s.Equal(10, out.ClosingDay)
				s.Equal(1, out.CardCount)
			},
		},
	}

	for _, tc := range scenarios {
		s.Run(tc.name, func() {
			s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).Return(s.baseSession(), nil).Once()
			s.sessionRepo.EXPECT().Upsert(mock.Anything, mock.MatchedBy(func(sess entities.OnboardingSession) bool {
				return len(sess.Payload().Cards) == 1 && sess.Payload().Cards[0].Name == "nubank"
			})).Return(nil).Once()
			s.publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Once()

			uc := s.newUC(tc.dependencies.cardCreator)
			out, err := uc.Execute(s.ctx, tc.args.in)
			tc.expect(out, err)
		})
	}
}

func (s *SaveOnboardingCardSuite) TestValidationErrors() {
	type args struct {
		in onbinput.SaveOnboardingCardInput
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(err error)
	}{
		{
			name: "nickname vazio rejeitado antes de IO",
			args: args{in: onbinput.SaveOnboardingCardInput{
				UserID:     s.userID,
				Nickname:   "",
				ClosingDay: 10,
			}},
			expect: func(err error) {
				s.Error(err)
				s.ErrorIs(err, onbinput.ErrCardNicknameRequired)
			},
		},
		{
			name: "closing_day zero rejeitado antes de IO",
			args: args{in: onbinput.SaveOnboardingCardInput{
				UserID:     s.userID,
				Nickname:   "nubank",
				ClosingDay: 0,
			}},
			expect: func(err error) {
				s.Error(err)
				s.ErrorIs(err, onbinput.ErrCardClosingDayRange)
			},
		},
		{
			name: "closing_day 32 rejeitado antes de IO",
			args: args{in: onbinput.SaveOnboardingCardInput{
				UserID:     s.userID,
				Nickname:   "nubank",
				ClosingDay: 32,
			}},
			expect: func(err error) {
				s.Error(err)
				s.ErrorIs(err, onbinput.ErrCardClosingDayRange)
			},
		},
		{
			name: "user_id nulo rejeitado antes de IO",
			args: args{in: onbinput.SaveOnboardingCardInput{
				UserID:     uuid.Nil,
				Nickname:   "nubank",
				ClosingDay: 10,
			}},
			expect: func(err error) {
				s.Error(err)
				s.ErrorIs(err, onbinput.ErrCardUserIDRequired)
			},
		},
	}

	for _, tc := range scenarios {
		s.Run(tc.name, func() {
			uc := s.newUC(nil)
			_, err := uc.Execute(s.ctx, tc.args.in)
			tc.expect(err)
			s.sessionRepo.AssertNotCalled(s.T(), "Find", mock.Anything, mock.Anything)
		})
	}
}

func (s *SaveOnboardingCardSuite) TestNaoUso_NicknameVazio() {
	uc := s.newUC(nil)
	_, err := uc.Execute(s.ctx, onbinput.SaveOnboardingCardInput{
		UserID:     s.userID,
		Nickname:   "",
		ClosingDay: 10,
	})
	s.Error(err)
	s.sessionRepo.AssertNotCalled(s.T(), "Find", mock.Anything, mock.Anything)
}

func (s *SaveOnboardingCardSuite) TestSessionNotFound() {
	s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).
		Return(entities.OnboardingSession{}, appinterfaces.ErrOnboardingSessionNotFound).Once()

	uc := s.newUC(nil)
	_, err := uc.Execute(s.ctx, onbinput.SaveOnboardingCardInput{
		UserID:     s.userID,
		Nickname:   "nubank",
		ClosingDay: 10,
	})
	s.ErrorIs(err, appinterfaces.ErrOnboardingSessionNotFound)
}

func (s *SaveOnboardingCardSuite) TestCardCreatorDelegado() {
	type state struct {
		called     bool
		gotUserID  string
		gotNick    string
		gotClosing int
	}

	type dependencies struct {
		st        *state
		returnErr error
	}

	scenarios := []struct {
		name         string
		dependencies dependencies
		expect       func(out SaveOnboardingCardResult, err error, st *state)
	}{
		{
			name:         "card creator chamado com closing_day correto",
			dependencies: dependencies{st: &state{}, returnErr: nil},
			expect: func(out SaveOnboardingCardResult, err error, st *state) {
				s.NoError(err)
				s.True(st.called)
				s.Equal(s.userID.String(), st.gotUserID)
				s.Equal("nubank", st.gotNick)
				s.Equal(10, st.gotClosing)
			},
		},
		{
			name:         "card creator retorna erro propagado",
			dependencies: dependencies{st: &state{}, returnErr: errors.New("card: falha")},
			expect: func(out SaveOnboardingCardResult, err error, st *state) {
				s.Error(err)
				s.ErrorContains(err, "create card")
			},
		},
	}

	for _, tc := range scenarios {
		s.Run(tc.name, func() {
			dep := tc.dependencies

			s.sessionRepo.EXPECT().Find(mock.Anything, s.userID).Return(s.baseSession(), nil).Once()
			s.sessionRepo.EXPECT().Upsert(mock.Anything, mock.Anything).Return(nil).Once()

			if dep.returnErr == nil {
				s.publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Once()
			}

			creator := &synchronousCardCreatorFn{fn: func(ctx context.Context, userID, nickname string, closingDay int) error {
				dep.st.called = true
				dep.st.gotUserID = userID
				dep.st.gotNick = nickname
				dep.st.gotClosing = closingDay
				return dep.returnErr
			}}

			uc := s.newUC(creator)
			out, err := uc.Execute(s.ctx, onbinput.SaveOnboardingCardInput{
				UserID:     s.userID,
				Nickname:   "nubank",
				ClosingDay: 10,
			})
			tc.expect(out, err, dep.st)
		})
	}
}

type synchronousCardCreatorFn struct {
	fn func(ctx context.Context, userID, nickname string, closingDay int) error
}

func (f *synchronousCardCreatorFn) Execute(ctx context.Context, userID, nickname string, closingDay int) error {
	return f.fn(ctx, userID, nickname, closingDay)
}
