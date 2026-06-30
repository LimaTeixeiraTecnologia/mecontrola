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
	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

type RecordJourneyTimestampSuite struct {
	suite.Suite
	ctx       context.Context
	obs       observability.Observability
	tokenRepo *mocks.MagicTokenRepository
}

func TestRecordJourneyTimestampSuite(t *testing.T) {
	suite.Run(t, new(RecordJourneyTimestampSuite))
}

func (s *RecordJourneyTimestampSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.tokenRepo = mocks.NewMagicTokenRepository(s.T())
}

func (s *RecordJourneyTimestampSuite) TestExecute() {
	type args struct {
		in input.RecordJourneyTimestampInput
	}
	type dependencies struct {
		tokenRepo *mocks.MagicTokenRepository
	}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(err error)
	}{
		{
			name: "deve gravar page_opened_at quando evento page_opened e token valido",
			args: args{in: func() input.RecordJourneyTimestampInput {
				tok, _ := valueobjects.NewToken()
				return input.RecordJourneyTimestampInput{ClearToken: tok.ClearText(), Event: input.JourneyEventPageOpened}
			}()},
			dependencies: func() dependencies {
				tok, _ := valueobjects.NewToken()
				mt, _ := entities.NewMagicToken("id-page", tok.Hash(), "plan-1", time.Now().UTC().Add(7*24*time.Hour))
				tok2, _ := valueobjects.TokenFromClear(input.RecordJourneyTimestampInput{}.ClearToken)
				_ = tok2
				s.tokenRepo.EXPECT().
					FindByHash(mock.Anything, mock.AnythingOfType("[]uint8")).
					Return(mt, nil).
					Once()
				s.tokenRepo.EXPECT().
					MarkPageOpened(mock.Anything, "id-page", mock.AnythingOfType("time.Time")).
					Return(nil).
					Once()
				return dependencies{tokenRepo: s.tokenRepo}
			}(),
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve gravar whatsapp_opened_at quando evento whatsapp_opened e token valido",
			args: args{in: func() input.RecordJourneyTimestampInput {
				tok, _ := valueobjects.NewToken()
				return input.RecordJourneyTimestampInput{ClearToken: tok.ClearText(), Event: input.JourneyEventWhatsAppOpened}
			}()},
			dependencies: func() dependencies {
				tok, _ := valueobjects.NewToken()
				mt, _ := entities.NewMagicToken("id-wa", tok.Hash(), "plan-1", time.Now().UTC().Add(7*24*time.Hour))
				s.tokenRepo.EXPECT().
					FindByHash(mock.Anything, mock.AnythingOfType("[]uint8")).
					Return(mt, nil).
					Once()
				s.tokenRepo.EXPECT().
					MarkWhatsAppOpened(mock.Anything, "id-wa", mock.AnythingOfType("time.Time")).
					Return(nil).
					Once()
				return dependencies{tokenRepo: s.tokenRepo}
			}(),
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve retornar nil quando token nao encontrado (nao vaza estado)",
			args: args{in: func() input.RecordJourneyTimestampInput {
				tok, _ := valueobjects.NewToken()
				return input.RecordJourneyTimestampInput{ClearToken: tok.ClearText(), Event: input.JourneyEventPageOpened}
			}()},
			dependencies: func() dependencies {
				s.tokenRepo.EXPECT().
					FindByHash(mock.Anything, mock.AnythingOfType("[]uint8")).
					Return(entities.MagicToken{}, domain.ErrTokenNotFound).
					Once()
				return dependencies{tokenRepo: s.tokenRepo}
			}(),
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve retornar nil quando evento invalido (sem io)",
			args: args{in: input.RecordJourneyTimestampInput{ClearToken: "algum-token", Event: "evento_invalido"}},
			dependencies: func() dependencies {
				return dependencies{tokenRepo: s.tokenRepo}
			}(),
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve retornar nil quando token vazio (sem io)",
			args: args{in: input.RecordJourneyTimestampInput{ClearToken: "", Event: input.JourneyEventPageOpened}},
			dependencies: func() dependencies {
				return dependencies{tokenRepo: s.tokenRepo}
			}(),
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve propagar erro do repositorio em MarkPageOpened",
			args: args{in: func() input.RecordJourneyTimestampInput {
				tok, _ := valueobjects.NewToken()
				return input.RecordJourneyTimestampInput{ClearToken: tok.ClearText(), Event: input.JourneyEventPageOpened}
			}()},
			dependencies: func() dependencies {
				tok, _ := valueobjects.NewToken()
				mt, _ := entities.NewMagicToken("id-err", tok.Hash(), "plan-1", time.Now().UTC().Add(7*24*time.Hour))
				s.tokenRepo.EXPECT().
					FindByHash(mock.Anything, mock.AnythingOfType("[]uint8")).
					Return(mt, nil).
					Once()
				s.tokenRepo.EXPECT().
					MarkPageOpened(mock.Anything, "id-err", mock.AnythingOfType("time.Time")).
					Return(errors.New("db error")).
					Once()
				return dependencies{tokenRepo: s.tokenRepo}
			}(),
			expect: func(err error) {
				s.Error(err)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			uc := NewRecordJourneyTimestamp(scenario.dependencies.tokenRepo, s.obs)
			err := uc.Execute(s.ctx, scenario.args.in)
			scenario.expect(err)
		})
	}
}
