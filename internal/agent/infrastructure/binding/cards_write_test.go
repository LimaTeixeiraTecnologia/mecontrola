package binding_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	appservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/binding"
	cardinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	cardoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
)

type fakeListCardsUC struct {
	out cardoutput.CardList
	err error
}

func (f *fakeListCardsUC) Execute(_ context.Context, _ cardinput.ListCards) (cardoutput.CardList, error) {
	return f.out, f.err
}

type fakeUpdateCardUC struct {
	out   cardoutput.Card
	err   error
	gotIn cardinput.UpdateCard
	calls int
}

func (f *fakeUpdateCardUC) Execute(_ context.Context, in cardinput.UpdateCard) (cardoutput.Card, error) {
	f.calls++
	f.gotIn = in
	return f.out, f.err
}

type fakeSoftDeleteCardUC struct {
	err   error
	gotIn cardinput.SoftDeleteCard
	calls int
}

func (f *fakeSoftDeleteCardUC) Execute(_ context.Context, in cardinput.SoftDeleteCard) error {
	f.calls++
	f.gotIn = in
	return f.err
}

type CardsWriteBindingSuite struct {
	suite.Suite
	ctx    context.Context
	userID uuid.UUID
	cardID string
}

func TestCardsWriteBindingSuite(t *testing.T) {
	suite.Run(t, new(CardsWriteBindingSuite))
}

func (s *CardsWriteBindingSuite) SetupTest() {
	s.ctx = context.Background()
	s.userID = uuid.New()
	s.cardID = uuid.NewString()
}

func (s *CardsWriteBindingSuite) cardList() cardoutput.CardList {
	return cardoutput.CardList{Items: []cardoutput.Card{
		{ID: s.cardID, Name: "Nubank Roxinho", Nickname: "nubank", ClosingDay: 10, DueDay: 17},
	}}
}

func (s *CardsWriteBindingSuite) TestCardUpdater_ResolvesByNameAndMapsPointers() {
	lister := &fakeListCardsUC{out: s.cardList()}
	updateUC := &fakeUpdateCardUC{out: cardoutput.Card{Nickname: "nubank", Name: "Nubank Roxinho", ClosingDay: 5, DueDay: 17, LimitCents: 500000}}
	adapter := binding.NewCardUpdaterAdapter(lister, updateUC)

	day := 5
	in, err := intent.NewUpdateCard(intent.UpdateCardFields{CardName: "nubank", ClosingDay: &day})
	s.Require().NoError(err)

	result, err := adapter.Execute(s.ctx, s.userID, in)
	s.Require().NoError(err)
	s.Equal(1, updateUC.calls)
	s.Equal(s.cardID, updateUC.gotIn.ID.String())
	s.Equal(s.userID, updateUC.gotIn.UserID)
	s.Require().NotNil(updateUC.gotIn.ClosingDay)
	s.Equal(5, *updateUC.gotIn.ClosingDay)
	s.Nil(updateUC.gotIn.Name)
	s.Nil(updateUC.gotIn.Nickname)
	s.Equal(int64(500000), result.LimitCents)
}

func (s *CardsWriteBindingSuite) TestCardUpdater_NotFoundReturnsAgentSentinel() {
	lister := &fakeListCardsUC{out: cardoutput.CardList{}}
	updateUC := &fakeUpdateCardUC{}
	adapter := binding.NewCardUpdaterAdapter(lister, updateUC)

	name := "premium"
	in, err := intent.NewUpdateCard(intent.UpdateCardFields{CardName: "inexistente", Nickname: &name})
	s.Require().NoError(err)

	_, err = adapter.Execute(s.ctx, s.userID, in)
	s.Require().Error(err)
	s.True(errors.Is(err, appservices.ErrAgentCardNotFound))
	s.Equal(0, updateUC.calls)
}

func (s *CardsWriteBindingSuite) TestCardUpdater_AmbiguousReturnsAgentSentinel() {
	lister := &fakeListCardsUC{out: cardoutput.CardList{Items: []cardoutput.Card{
		{ID: uuid.NewString(), Name: "Cartao Itau Black", Nickname: "itau black"},
		{ID: uuid.NewString(), Name: "Cartao Itau Gold", Nickname: "itau gold"},
	}}}
	updateUC := &fakeUpdateCardUC{}
	adapter := binding.NewCardUpdaterAdapter(lister, updateUC)

	name := "novo"
	in, err := intent.NewUpdateCard(intent.UpdateCardFields{CardName: "itau", Nickname: &name})
	s.Require().NoError(err)

	_, err = adapter.Execute(s.ctx, s.userID, in)
	s.Require().Error(err)
	s.True(errors.Is(err, appservices.ErrAgentCardAmbiguous))
	s.Equal(0, updateUC.calls)
}

func (s *CardsWriteBindingSuite) TestCardUpdater_ListerErrorIsWrapped() {
	lister := &fakeListCardsUC{err: errors.New("db down")}
	updateUC := &fakeUpdateCardUC{}
	adapter := binding.NewCardUpdaterAdapter(lister, updateUC)

	name := "novo"
	in, err := intent.NewUpdateCard(intent.UpdateCardFields{CardName: "nubank", Nickname: &name})
	s.Require().NoError(err)

	_, err = adapter.Execute(s.ctx, s.userID, in)
	s.Require().Error(err)
	s.Contains(err.Error(), "listar cartões")
	s.Equal(0, updateUC.calls)
}

func (s *CardsWriteBindingSuite) TestCardUpdater_InvalidCardIDIsWrapped() {
	lister := &fakeListCardsUC{out: cardoutput.CardList{Items: []cardoutput.Card{
		{ID: "not-a-uuid", Name: "Nubank Roxinho", Nickname: "nubank"},
	}}}
	updateUC := &fakeUpdateCardUC{}
	adapter := binding.NewCardUpdaterAdapter(lister, updateUC)

	name := "novo"
	in, err := intent.NewUpdateCard(intent.UpdateCardFields{CardName: "nubank", Nickname: &name})
	s.Require().NoError(err)

	_, err = adapter.Execute(s.ctx, s.userID, in)
	s.Require().Error(err)
	s.Contains(err.Error(), "card id")
	s.Equal(0, updateUC.calls)
}

func (s *CardsWriteBindingSuite) TestCardUpdater_RenamesAndChangesDays() {
	lister := &fakeListCardsUC{out: s.cardList()}
	updateUC := &fakeUpdateCardUC{out: cardoutput.Card{Nickname: "nu", Name: "Nubank Ultravioleta", ClosingDay: 8, DueDay: 15}}
	adapter := binding.NewCardUpdaterAdapter(lister, updateUC)

	newName := "Nubank Ultravioleta"
	newNick := "nu"
	closing := 8
	due := 15
	in, err := intent.NewUpdateCard(intent.UpdateCardFields{
		CardName:   "nubank",
		Name:       &newName,
		Nickname:   &newNick,
		ClosingDay: &closing,
		DueDay:     &due,
	})
	s.Require().NoError(err)

	result, err := adapter.Execute(s.ctx, s.userID, in)
	s.Require().NoError(err)
	s.Equal(1, updateUC.calls)
	s.Require().NotNil(updateUC.gotIn.Name)
	s.Equal("Nubank Ultravioleta", *updateUC.gotIn.Name)
	s.Require().NotNil(updateUC.gotIn.Nickname)
	s.Equal("nu", *updateUC.gotIn.Nickname)
	s.Require().NotNil(updateUC.gotIn.ClosingDay)
	s.Equal(8, *updateUC.gotIn.ClosingDay)
	s.Require().NotNil(updateUC.gotIn.DueDay)
	s.Equal(15, *updateUC.gotIn.DueDay)
	s.Equal("Nubank Ultravioleta", result.Name)
	s.Equal(15, result.DueDay)
}

func (s *CardsWriteBindingSuite) TestCardUpdater_ResolvesByPartialContainsSingleMatch() {
	lister := &fakeListCardsUC{out: cardoutput.CardList{Items: []cardoutput.Card{
		{ID: s.cardID, Name: "Nubank Roxinho", Nickname: "principal"},
	}}}
	updateUC := &fakeUpdateCardUC{out: cardoutput.Card{Name: "Nubank Roxinho"}}
	adapter := binding.NewCardUpdaterAdapter(lister, updateUC)

	due := 20
	in, err := intent.NewUpdateCard(intent.UpdateCardFields{CardName: "roxinho", DueDay: &due})
	s.Require().NoError(err)

	_, err = adapter.Execute(s.ctx, s.userID, in)
	s.Require().NoError(err)
	s.Equal(1, updateUC.calls)
	s.Equal(s.cardID, updateUC.gotIn.ID.String())
}

func (s *CardsWriteBindingSuite) TestCardUpdater_PropagatesUsecaseError() {
	lister := &fakeListCardsUC{out: s.cardList()}
	updateUC := &fakeUpdateCardUC{err: errors.New("boom")}
	adapter := binding.NewCardUpdaterAdapter(lister, updateUC)

	name := "novo"
	in, err := intent.NewUpdateCard(intent.UpdateCardFields{CardName: "nubank", Nickname: &name})
	s.Require().NoError(err)

	_, err = adapter.Execute(s.ctx, s.userID, in)
	s.Require().Error(err)
	s.False(errors.Is(err, appservices.ErrAgentCardNotFound))
}

func (s *CardsWriteBindingSuite) TestCardDeleter_ResolvesByNameAndDeletes() {
	lister := &fakeListCardsUC{out: s.cardList()}
	deleteUC := &fakeSoftDeleteCardUC{}
	adapter := binding.NewCardDeleterAdapter(lister, deleteUC)

	result, err := adapter.Execute(s.ctx, s.userID, "nubank")
	s.Require().NoError(err)
	s.Equal(1, deleteUC.calls)
	s.Equal(s.cardID, deleteUC.gotIn.ID.String())
	s.Equal(s.userID, deleteUC.gotIn.UserID)
	s.Equal("nubank", result.Name)
}

func (s *CardsWriteBindingSuite) TestCardDeleter_NotFoundReturnsAgentSentinel() {
	lister := &fakeListCardsUC{out: cardoutput.CardList{}}
	deleteUC := &fakeSoftDeleteCardUC{}
	adapter := binding.NewCardDeleterAdapter(lister, deleteUC)

	_, err := adapter.Execute(s.ctx, s.userID, "inexistente")
	s.Require().Error(err)
	s.True(errors.Is(err, appservices.ErrAgentCardNotFound))
	s.Equal(0, deleteUC.calls)
}

func (s *CardsWriteBindingSuite) TestCardDeleter_ListerErrorIsWrapped() {
	lister := &fakeListCardsUC{err: errors.New("db down")}
	deleteUC := &fakeSoftDeleteCardUC{}
	adapter := binding.NewCardDeleterAdapter(lister, deleteUC)

	_, err := adapter.Execute(s.ctx, s.userID, "nubank")
	s.Require().Error(err)
	s.Contains(err.Error(), "listar cartões")
	s.Equal(0, deleteUC.calls)
}

func (s *CardsWriteBindingSuite) TestCardDeleter_InvalidCardIDIsWrapped() {
	lister := &fakeListCardsUC{out: cardoutput.CardList{Items: []cardoutput.Card{
		{ID: "not-a-uuid", Name: "Nubank Roxinho", Nickname: "nubank"},
	}}}
	deleteUC := &fakeSoftDeleteCardUC{}
	adapter := binding.NewCardDeleterAdapter(lister, deleteUC)

	_, err := adapter.Execute(s.ctx, s.userID, "nubank")
	s.Require().Error(err)
	s.Contains(err.Error(), "card id")
	s.Equal(0, deleteUC.calls)
}

func (s *CardsWriteBindingSuite) TestCardDeleter_LabelFallsBackToNameWhenNicknameEmpty() {
	lister := &fakeListCardsUC{out: cardoutput.CardList{Items: []cardoutput.Card{
		{ID: s.cardID, Name: "Nubank Roxinho", Nickname: "   "},
	}}}
	deleteUC := &fakeSoftDeleteCardUC{}
	adapter := binding.NewCardDeleterAdapter(lister, deleteUC)

	result, err := adapter.Execute(s.ctx, s.userID, "Nubank Roxinho")
	s.Require().NoError(err)
	s.Equal(1, deleteUC.calls)
	s.Equal("Nubank Roxinho", result.Name)
}

func (s *CardsWriteBindingSuite) TestCardDeleter_EmptyNameReturnsAgentSentinel() {
	lister := &fakeListCardsUC{out: s.cardList()}
	deleteUC := &fakeSoftDeleteCardUC{}
	adapter := binding.NewCardDeleterAdapter(lister, deleteUC)

	_, err := adapter.Execute(s.ctx, s.userID, "   ")
	s.Require().Error(err)
	s.True(errors.Is(err, appservices.ErrAgentCardNotFound))
	s.Equal(0, deleteUC.calls)
}

func (s *CardsWriteBindingSuite) TestCardDeleter_ResolvesByPartialContainsSingleMatch() {
	lister := &fakeListCardsUC{out: cardoutput.CardList{Items: []cardoutput.Card{
		{ID: s.cardID, Name: "Nubank Roxinho", Nickname: "principal"},
	}}}
	deleteUC := &fakeSoftDeleteCardUC{}
	adapter := binding.NewCardDeleterAdapter(lister, deleteUC)

	result, err := adapter.Execute(s.ctx, s.userID, "roxinho")
	s.Require().NoError(err)
	s.Equal(1, deleteUC.calls)
	s.Equal("principal", result.Name)
}

func (s *CardsWriteBindingSuite) TestCardDeleter_PropagatesUsecaseError() {
	lister := &fakeListCardsUC{out: s.cardList()}
	deleteUC := &fakeSoftDeleteCardUC{err: errors.New("boom")}
	adapter := binding.NewCardDeleterAdapter(lister, deleteUC)

	_, err := adapter.Execute(s.ctx, s.userID, "nubank")
	s.Require().Error(err)
	s.False(errors.Is(err, appservices.ErrAgentCardNotFound))
}
