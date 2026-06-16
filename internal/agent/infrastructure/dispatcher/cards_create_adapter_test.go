package dispatcher_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/dispatcher"
	cardinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	cardoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
)

type stubCreateCard struct {
	called   bool
	gotInput cardinput.CreateCard
	resp     cardoutput.Card
	err      error
}

func (s *stubCreateCard) Execute(_ context.Context, in cardinput.CreateCard) (cardoutput.Card, error) {
	s.called = true
	s.gotInput = in
	return s.resp, s.err
}

type stubListCardsCreate struct{}

func (s *stubListCardsCreate) Execute(_ context.Context, _ cardinput.ListCards) (cardoutput.CardList, error) {
	return cardoutput.CardList{}, nil
}

func TestCardsAdapter_Create_HappyPath(t *testing.T) {
	create := &stubCreateCard{resp: cardoutput.Card{Nickname: "Nubank", ClosingDay: 5, DueDay: 12}}
	sut := dispatcher.NewCardsAdapterFull(&stubListCardsCreate{}, nil, create, nil, nil, nil)

	userID := uuid.New()
	payload := json.RawMessage(`{"nickname":"Nubank","name":"Nubank Roxinho","closing_day":5,"due_day":12}`)
	reply, err := sut.Create(context.Background(), userID, payload)

	require.NoError(t, err)
	assert.True(t, create.called)
	assert.Equal(t, userID, create.gotInput.UserID)
	assert.Equal(t, "Nubank", create.gotInput.Nickname)
	assert.Equal(t, "Nubank Roxinho", create.gotInput.Name)
	assert.Equal(t, 5, create.gotInput.ClosingDay)
	assert.Equal(t, 12, create.gotInput.DueDay)
	assert.Contains(t, reply, "Nubank")
	assert.Contains(t, reply, "5")
	assert.Contains(t, reply, "12")
}

func TestCardsAdapter_Create_FallsBackNameFromNickname(t *testing.T) {
	create := &stubCreateCard{resp: cardoutput.Card{Nickname: "Inter", ClosingDay: 1, DueDay: 10}}
	sut := dispatcher.NewCardsAdapterFull(&stubListCardsCreate{}, nil, create, nil, nil, nil)

	payload := json.RawMessage(`{"nickname":"Inter","closing_day":1,"due_day":10}`)
	_, err := sut.Create(context.Background(), uuid.New(), payload)
	require.NoError(t, err)
	assert.Equal(t, "Inter", create.gotInput.Name)
}

func TestCardsAdapter_Create_MissingNicknameRejects(t *testing.T) {
	sut := dispatcher.NewCardsAdapterFull(&stubListCardsCreate{}, nil, &stubCreateCard{}, nil, nil, nil)

	payload := json.RawMessage(`{"closing_day":5,"due_day":12}`)
	_, err := sut.Create(context.Background(), uuid.New(), payload)
	require.Error(t, err)
	assert.True(t, errors.Is(err, dispatcher.ErrCardsCreateInvalidPayload))
}

func TestCardsAdapter_Create_InvalidDaysRejects(t *testing.T) {
	sut := dispatcher.NewCardsAdapterFull(&stubListCardsCreate{}, nil, &stubCreateCard{}, nil, nil, nil)

	cases := []struct {
		name    string
		payload string
	}{
		{name: "closing zero", payload: `{"nickname":"X","closing_day":0,"due_day":10}`},
		{name: "closing > 31", payload: `{"nickname":"X","closing_day":32,"due_day":10}`},
		{name: "due zero", payload: `{"nickname":"X","closing_day":5,"due_day":0}`},
		{name: "due > 31", payload: `{"nickname":"X","closing_day":5,"due_day":99}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := sut.Create(context.Background(), uuid.New(), json.RawMessage(tc.payload))
			require.Error(t, err)
			assert.True(t, errors.Is(err, dispatcher.ErrCardsCreateInvalidPayload))
		})
	}
}

func TestCardsAdapter_Create_NilUseCase_ReturnsUnsupported(t *testing.T) {
	sut := dispatcher.NewCardsAdapter(&stubListCardsCreate{})
	_, err := sut.Create(context.Background(), uuid.New(), json.RawMessage(`{"nickname":"X","closing_day":5,"due_day":10}`))
	require.Error(t, err)
}
