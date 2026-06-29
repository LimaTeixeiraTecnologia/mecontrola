package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory/mocks"
)

type RecallSuite struct {
	suite.Suite
	ctx        context.Context
	obs        *fake.Provider
	recallMock *mocks.SemanticRecall
}

func TestRecallSuite(t *testing.T) {
	suite.Run(t, new(RecallSuite))
}

func (s *RecallSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.recallMock = mocks.NewSemanticRecall(s.T())
}

func (s *RecallSuite) TestExecute() {
	type args struct {
		in input.RecallInput
	}
	type dependencies struct {
		recallMock *mocks.SemanticRecall
	}

	embedding := []float32{0.1, 0.2, 0.3}
	expectedHits := []memory.RecallHit{
		{ResourceID: "user-123", ThreadID: "whatsapp", Content: "resultado", Score: 0.95},
	}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(hits []memory.RecallHit, err error)
	}{
		{
			name: "deve retornar hits semanticos",
			args: args{in: input.RecallInput{
				ResourceID: "user-123",
				Query:      "despesa do mercado",
				Embedding:  embedding,
				K:          5,
			}},
			dependencies: dependencies{
				recallMock: func() *mocks.SemanticRecall {
					s.recallMock.EXPECT().
						Recall(mock.Anything, "user-123", "despesa do mercado", embedding, 5).
						Return(expectedHits, nil).
						Once()
					return s.recallMock
				}(),
			},
			expect: func(hits []memory.RecallHit, err error) {
				s.NoError(err)
				s.Len(hits, 1)
				s.Equal("resultado", hits[0].Content)
			},
		},
		{
			name: "deve retornar erro de validacao quando resource_id vazio",
			args: args{in: input.RecallInput{
				ResourceID: "",
				Query:      "query",
				Embedding:  embedding,
				K:          5,
			}},
			dependencies: dependencies{recallMock: s.recallMock},
			expect: func(hits []memory.RecallHit, err error) {
				s.Error(err)
				s.ErrorIs(err, memory.ErrEmptyResourceID)
				s.Nil(hits)
			},
		},
		{
			name: "deve retornar erro de validacao quando embedding vazio",
			args: args{in: input.RecallInput{
				ResourceID: "user-123",
				Query:      "query",
				Embedding:  nil,
				K:          5,
			}},
			dependencies: dependencies{recallMock: s.recallMock},
			expect: func(hits []memory.RecallHit, err error) {
				s.Error(err)
				s.Nil(hits)
			},
		},
		{
			name: "deve retornar erro de validacao quando k zero",
			args: args{in: input.RecallInput{
				ResourceID: "user-123",
				Query:      "query",
				Embedding:  embedding,
				K:          0,
			}},
			dependencies: dependencies{recallMock: s.recallMock},
			expect: func(hits []memory.RecallHit, err error) {
				s.Error(err)
				s.Nil(hits)
			},
		},
		{
			name: "deve propagar erro do recall",
			args: args{in: input.RecallInput{
				ResourceID: "user-123",
				Query:      "despesa",
				Embedding:  embedding,
				K:          3,
			}},
			dependencies: dependencies{
				recallMock: func() *mocks.SemanticRecall {
					s.recallMock.EXPECT().
						Recall(mock.Anything, "user-123", "despesa", embedding, 3).
						Return(nil, errors.New("db error")).
						Once()
					return s.recallMock
				}(),
			},
			expect: func(hits []memory.RecallHit, err error) {
				s.Error(err)
				s.Nil(hits)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			uc := NewRecall(scenario.dependencies.recallMock, s.obs)
			hits, err := uc.Execute(s.ctx, scenario.args.in)
			scenario.expect(hits, err)
		})
	}
}
