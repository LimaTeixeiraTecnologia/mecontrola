package usecases

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
)

type ComposeConversationalReplySuite struct {
	suite.Suite
	ctx context.Context
}

func TestComposeConversationalReplySuite(t *testing.T) {
	suite.Run(t, new(ComposeConversationalReplySuite))
}

func (s *ComposeConversationalReplySuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *ComposeConversationalReplySuite) newSUT(fi *fakeInterpreter, maxTokens int) *ComposeConversationalReply {
	uc, err := NewComposeConversationalReply(fi, maxTokens, fake.NewProvider())
	s.Require().NoError(err)
	return uc
}

func (s *ComposeConversationalReplySuite) TestNilDeps() {
	_, err := NewComposeConversationalReply(nil, 200, fake.NewProvider())
	s.Require().Error(err)

	_, err = NewComposeConversationalReply(&fakeInterpreter{}, 200, nil)
	s.Require().Error(err)
}

func (s *ComposeConversationalReplySuite) TestEmptyText() {
	uc := s.newSUT(&fakeInterpreter{}, 200)
	_, err := uc.Execute(s.ctx, ComposeConversationalInput{UserID: uuid.New(), Channel: "whatsapp", Text: "   "})
	s.Require().ErrorIs(err, ErrComposeEmptyText)
}

func (s *ComposeConversationalReplySuite) TestReturnsTrimmedProse() {
	fi := &fakeInterpreter{resp: interfaces.LLMResponse{RawJSON: []byte("  Vamos organizar suas finanças! 💪  ")}}
	uc := s.newSUT(fi, 150)

	out, err := uc.Execute(s.ctx, ComposeConversationalInput{UserID: uuid.New(), Channel: "whatsapp", Text: "oi, tudo bem?"})
	s.Require().NoError(err)
	s.Equal("Vamos organizar suas finanças! 💪", out.Reply)
}

func (s *ComposeConversationalReplySuite) TestSendsFreeTextContract() {
	fi := &fakeInterpreter{resp: interfaces.LLMResponse{RawJSON: []byte("ok")}}
	uc := s.newSUT(fi, 150)

	_, err := uc.Execute(s.ctx, ComposeConversationalInput{UserID: uuid.New(), Channel: "whatsapp", Text: "me ajuda"})
	s.Require().NoError(err)

	s.True(fi.lastRequest.FreeText)
	s.Equal(150, fi.lastRequest.MaxTokens)
	s.Nil(fi.lastRequest.JSONSchema)
	s.Contains(fi.lastRequest.SystemPrompt, "Conexão")
	s.Contains(fi.lastRequest.SystemPrompt, "Atualização Automática")
	s.Contains(fi.lastRequest.SystemPrompt, "Lançamentos (transações)")
	s.Contains(fi.lastRequest.SystemPrompt, "Kiwify")
	s.Contains(fi.lastRequest.SystemPrompt, "rascunho")
	s.Contains(fi.lastRequest.SystemPrompt, "80%")
}

func (s *ComposeConversationalReplySuite) TestProviderErrorReturnsDeterministicRedirect() {
	fi := &fakeInterpreter{err: fmt.Errorf("wrap: %w", services.ErrFallbackChainExhausted)}
	uc := s.newSUT(fi, 200)

	out, err := uc.Execute(s.ctx, ComposeConversationalInput{UserID: uuid.New(), Channel: "whatsapp", Text: "qualquer coisa"})
	s.Require().NoError(err)
	s.NotEmpty(out.Reply)
	s.Contains(out.Reply, "finanças")
}

func (s *ComposeConversationalReplySuite) TestEmptyProviderReplyReturnsRedirect() {
	fi := &fakeInterpreter{resp: interfaces.LLMResponse{RawJSON: []byte("   ")}}
	uc := s.newSUT(fi, 200)

	out, err := uc.Execute(s.ctx, ComposeConversationalInput{UserID: uuid.New(), Channel: "whatsapp", Text: "oi"})
	s.Require().NoError(err)
	s.NotEmpty(out.Reply)
}

func (s *ComposeConversationalReplySuite) TestGenericProviderErrorStillReplies() {
	fi := &fakeInterpreter{err: errors.New("transport down")}
	uc := s.newSUT(fi, 200)

	out, err := uc.Execute(s.ctx, ComposeConversationalInput{UserID: uuid.New(), Channel: "whatsapp", Text: "oi"})
	s.Require().NoError(err)
	s.NotEmpty(out.Reply)
}
