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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	transactionsinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/input"
	transactionsoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
	transactionsusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases"
)

type stubCreateTransaction struct {
	called       bool
	gotInput     transactionsinput.RawCreateTransaction
	gotPrincipal auth.Principal
	resp         transactionsoutput.Transaction
	err          error
}

func (s *stubCreateTransaction) Execute(ctx context.Context, raw transactionsinput.RawCreateTransaction) (transactionsoutput.Transaction, error) {
	s.called = true
	s.gotInput = raw
	s.gotPrincipal, _ = auth.FromContext(ctx)
	return s.resp, s.err
}

type stubGetTransaction struct {
	called bool
	resp   transactionsoutput.Transaction
	err    error
}

func (s *stubGetTransaction) Execute(_ context.Context, _ string) (transactionsoutput.Transaction, error) {
	s.called = true
	return s.resp, s.err
}

type stubDeleteTransaction struct {
	called     bool
	gotID      string
	gotVersion int64
	err        error
}

func (s *stubDeleteTransaction) Execute(_ context.Context, txID string, version int64) error {
	s.called = true
	s.gotID = txID
	s.gotVersion = version
	return s.err
}

type stubListTransactionsForCreate struct{}

func (s *stubListTransactionsForCreate) Execute(_ context.Context, _, _ string, _ int) (transactionsusecases.TransactionPage, error) {
	return transactionsusecases.TransactionPage{}, nil
}

func TestTransactionsAdapter_Create_HappyPath(t *testing.T) {
	create := &stubCreateTransaction{resp: transactionsoutput.Transaction{
		AmountCents: 5050,
		Direction:   "expense",
		Description: "almoco",
	}}
	sut := dispatcher.NewTransactionsAdapterFull(&stubListTransactionsForCreate{}, create, &stubDeleteTransaction{}, &stubGetTransaction{})

	userID := uuid.New()
	categoryID := uuid.New()
	payload := json.RawMessage(`{
		"amount": 50.5,
		"type": "expense",
		"description": "almoço",
		"category_id": "` + categoryID.String() + `",
		"payment_method": "pix",
		"occurred_at": "2026-06-14"
	}`)

	reply, err := sut.Create(context.Background(), userID, payload)
	require.NoError(t, err)
	assert.True(t, create.called)
	assert.Equal(t, userID, create.gotPrincipal.UserID)
	assert.Equal(t, int64(5050), create.gotInput.AmountCents)
	assert.Equal(t, "outcome", create.gotInput.Direction)
	assert.Equal(t, "pix", create.gotInput.PaymentMethod)
	assert.Equal(t, categoryID, create.gotInput.CategoryID)
	assert.Contains(t, reply, "R$ 50,50")
}

func TestTransactionsAdapter_Create_InvalidAmountRejects(t *testing.T) {
	create := &stubCreateTransaction{}
	sut := dispatcher.NewTransactionsAdapterFull(&stubListTransactionsForCreate{}, create, &stubDeleteTransaction{}, &stubGetTransaction{})

	cases := []struct {
		name    string
		payload string
	}{
		{name: "zero amount", payload: `{"amount":0,"type":"expense","category_id":"` + uuid.New().String() + `"}`},
		{name: "negative amount", payload: `{"amount":-10,"type":"expense","category_id":"` + uuid.New().String() + `"}`},
		{name: "missing amount", payload: `{"type":"expense","category_id":"` + uuid.New().String() + `"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := sut.Create(context.Background(), uuid.New(), json.RawMessage(tc.payload))
			require.Error(t, err)
			assert.True(t, errors.Is(err, dispatcher.ErrTransactionsCreateInvalidPayload))
			assert.False(t, create.called)
		})
	}
}

func TestTransactionsAdapter_Create_InvalidDirectionRejects(t *testing.T) {
	create := &stubCreateTransaction{}
	sut := dispatcher.NewTransactionsAdapterFull(&stubListTransactionsForCreate{}, create, &stubDeleteTransaction{}, &stubGetTransaction{})

	payload := json.RawMessage(`{"amount":10,"type":"blocked","category_id":"` + uuid.New().String() + `"}`)
	_, err := sut.Create(context.Background(), uuid.New(), payload)
	require.Error(t, err)
	assert.True(t, errors.Is(err, dispatcher.ErrTransactionsCreateInvalidPayload))
}

func TestTransactionsAdapter_Create_InvalidCategoryIDRejects(t *testing.T) {
	sut := dispatcher.NewTransactionsAdapterFull(&stubListTransactionsForCreate{}, &stubCreateTransaction{}, &stubDeleteTransaction{}, &stubGetTransaction{})

	payload := json.RawMessage(`{"amount":10,"type":"expense","category_id":"not-a-uuid"}`)
	_, err := sut.Create(context.Background(), uuid.New(), payload)
	require.Error(t, err)
	assert.True(t, errors.Is(err, dispatcher.ErrTransactionsCreateInvalidPayload))
}

func TestTransactionsAdapter_Create_DefaultsPaymentMethod(t *testing.T) {
	create := &stubCreateTransaction{resp: transactionsoutput.Transaction{
		AmountCents: 100, Direction: "expense", Description: "x",
	}}
	sut := dispatcher.NewTransactionsAdapterFull(&stubListTransactionsForCreate{}, create, &stubDeleteTransaction{}, &stubGetTransaction{})

	payload := json.RawMessage(`{"amount":1,"type":"expense","description":"x","category_id":"` + uuid.New().String() + `"}`)
	_, err := sut.Create(context.Background(), uuid.New(), payload)
	require.NoError(t, err)
	assert.Equal(t, "other", create.gotInput.PaymentMethod)
}

func TestTransactionsAdapter_Delete_HappyPath(t *testing.T) {
	txID := uuid.New().String()
	get := &stubGetTransaction{resp: transactionsoutput.Transaction{Version: 7, AmountCents: 1234}}
	del := &stubDeleteTransaction{}
	sut := dispatcher.NewTransactionsAdapterFull(&stubListTransactionsForCreate{}, &stubCreateTransaction{}, del, get)

	payload := json.RawMessage(`{"id":"` + txID + `"}`)
	reply, err := sut.Delete(context.Background(), uuid.New(), payload)
	require.NoError(t, err)
	assert.True(t, get.called)
	assert.True(t, del.called)
	assert.Equal(t, txID, del.gotID)
	assert.Equal(t, int64(7), del.gotVersion)
	assert.Contains(t, reply, "R$ 12,34")
}

func TestTransactionsAdapter_Delete_MissingIDRejects(t *testing.T) {
	sut := dispatcher.NewTransactionsAdapterFull(&stubListTransactionsForCreate{}, &stubCreateTransaction{}, &stubDeleteTransaction{}, &stubGetTransaction{})

	_, err := sut.Delete(context.Background(), uuid.New(), json.RawMessage(`{}`))
	require.Error(t, err)
	assert.True(t, errors.Is(err, dispatcher.ErrTransactionsDeleteMissingID))
}

func TestTransactionsAdapter_Delete_InvalidUUIDRejects(t *testing.T) {
	sut := dispatcher.NewTransactionsAdapterFull(&stubListTransactionsForCreate{}, &stubCreateTransaction{}, &stubDeleteTransaction{}, &stubGetTransaction{})

	_, err := sut.Delete(context.Background(), uuid.New(), json.RawMessage(`{"id":"not-a-uuid"}`))
	require.Error(t, err)
}

func TestTransactionsAdapter_Delete_GetErrorPropagates(t *testing.T) {
	txID := uuid.New().String()
	get := &stubGetTransaction{err: errors.New("not found")}
	sut := dispatcher.NewTransactionsAdapterFull(&stubListTransactionsForCreate{}, &stubCreateTransaction{}, &stubDeleteTransaction{}, get)

	_, err := sut.Delete(context.Background(), uuid.New(), json.RawMessage(`{"id":"`+txID+`"}`))
	require.Error(t, err)
}
