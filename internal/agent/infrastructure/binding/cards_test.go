package binding_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/binding"
	cardinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	cardoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
)

type fakeCreateCardUC struct {
	out   cardoutput.Card
	err   error
	gotIn cardinput.CreateCard
	calls int
}

func (f *fakeCreateCardUC) Execute(_ context.Context, in cardinput.CreateCard) (cardoutput.Card, error) {
	f.calls++
	f.gotIn = in
	return f.out, f.err
}

type fakeCountCardsUC struct {
	out   cardoutput.CardCount
	err   error
	gotIn cardinput.CountCards
	calls int
}

func (f *fakeCountCardsUC) Execute(_ context.Context, in cardinput.CountCards) (cardoutput.CardCount, error) {
	f.calls++
	f.gotIn = in
	return f.out, f.err
}

type CardsBindingSuite struct {
	suite.Suite
}

func TestCardsBindingSuite(t *testing.T) {
	suite.Run(t, new(CardsBindingSuite))
}

func (s *CardsBindingSuite) TestCardCreator_MapsIntentToInput() {
	uc := &fakeCreateCardUC{out: cardoutput.Card{Nickname: "nubank", Name: "Nubank Roxinho", ClosingDay: 10, DueDay: 17, LimitCents: 500000}}
	adapter := binding.NewCardCreatorAdapter(uc)
	userID := uuid.New()
	in, err := intent.NewCreateCard(intent.CreateCardFields{Nickname: "nubank", Name: "Nubank Roxinho", ClosingDay: 10, DueDay: 17, LimitCents: 500000})
	s.Require().NoError(err)

	result, err := adapter.Execute(context.Background(), userID, in)
	s.Require().NoError(err)
	s.Equal(1, uc.calls)
	s.Equal(userID, uc.gotIn.UserID)
	s.Equal("nubank", uc.gotIn.Nickname)
	s.Equal("Nubank Roxinho", uc.gotIn.Name)
	s.Equal(10, uc.gotIn.ClosingDay)
	s.Equal(int64(500000), result.LimitCents)
}

func (s *CardsBindingSuite) TestCardCreator_NameDefaultsToNickname() {
	uc := &fakeCreateCardUC{out: cardoutput.Card{Nickname: "nubank"}}
	adapter := binding.NewCardCreatorAdapter(uc)
	in, err := intent.NewCreateCard(intent.CreateCardFields{Nickname: "nubank"})
	s.Require().NoError(err)

	_, err = adapter.Execute(context.Background(), uuid.New(), in)
	s.Require().NoError(err)
	s.Equal("nubank", uc.gotIn.Name)
}

func (s *CardsBindingSuite) TestCardCreator_PropagatesError() {
	uc := &fakeCreateCardUC{err: errors.New("boom")}
	adapter := binding.NewCardCreatorAdapter(uc)
	in, err := intent.NewCreateCard(intent.CreateCardFields{Nickname: "nubank"})
	s.Require().NoError(err)

	_, err = adapter.Execute(context.Background(), uuid.New(), in)
	s.Require().Error(err)
}

func (s *CardsBindingSuite) TestCardCounter_ReturnsTotal() {
	uc := &fakeCountCardsUC{out: cardoutput.CardCount{Total: 4}}
	adapter := binding.NewCardCounterAdapter(uc)
	userID := uuid.New()

	total, err := adapter.Execute(context.Background(), userID)
	s.Require().NoError(err)
	s.Equal(int64(4), total)
	s.Equal(userID, uc.gotIn.UserID)
}

func (s *CardsBindingSuite) TestCardCounter_PropagatesError() {
	uc := &fakeCountCardsUC{err: errors.New("boom")}
	adapter := binding.NewCardCounterAdapter(uc)

	_, err := adapter.Execute(context.Background(), uuid.New())
	s.Require().Error(err)
}
