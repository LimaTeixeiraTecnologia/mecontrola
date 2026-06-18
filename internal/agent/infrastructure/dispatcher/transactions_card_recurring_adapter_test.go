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

type stubCreateCardPurchase struct {
	called       bool
	gotInput     transactionsinput.RawCreateCardPurchase
	gotPrincipal auth.Principal
	resp         transactionsoutput.CardPurchase
	err          error
}

func (s *stubCreateCardPurchase) Execute(ctx context.Context, raw transactionsinput.RawCreateCardPurchase) (transactionsoutput.CardPurchase, error) {
	s.called = true
	s.gotInput = raw
	s.gotPrincipal, _ = auth.FromContext(ctx)
	return s.resp, s.err
}

type stubCreateRecurringTemplate struct {
	called       bool
	gotInput     transactionsinput.RawCreateRecurringTemplate
	gotPrincipal auth.Principal
	resp         transactionsoutput.RecurringTemplate
	err          error
}

func (s *stubCreateRecurringTemplate) Execute(ctx context.Context, raw transactionsinput.RawCreateRecurringTemplate) (transactionsoutput.RecurringTemplate, error) {
	s.called = true
	s.gotInput = raw
	s.gotPrincipal, _ = auth.FromContext(ctx)
	return s.resp, s.err
}

type stubListRecurringTemplates struct {
	called bool
	resp   transactionsusecases.RecurringTemplatePage
	err    error
}

func (s *stubListRecurringTemplates) Execute(_ context.Context, _ bool, _ string, _ int) (transactionsusecases.RecurringTemplatePage, error) {
	s.called = true
	return s.resp, s.err
}

func newAdapterWithAll(
	cp dispatcher.CreateCardPurchaseUseCase,
	rc dispatcher.CreateRecurringTemplateUseCase,
	lr dispatcher.ListRecurringTemplatesUseCase,
) *dispatcher.TransactionsAdapter {
	return dispatcher.NewTransactionsAdapterFull(
		&stubListTransactionsForCreate{},
		&stubCreateTransaction{},
		&stubDeleteTransaction{},
		&stubGetTransaction{},
		cp, rc, lr,
	)
}

func TestTransactionsAdapter_CreateCardPurchase_HappyPath(t *testing.T) {
	cardID := uuid.New()
	categoryID := uuid.New()
	cp := &stubCreateCardPurchase{resp: transactionsoutput.CardPurchase{
		TotalAmountCents:  120000,
		InstallmentsTotal: 6,
	}}
	sut := newAdapterWithAll(cp, nil, nil)

	payload := json.RawMessage(`{
		"amount_cents": 120000,
		"card_id": "` + cardID.String() + `",
		"installments": 6,
		"description": "eletronico",
		"category_id": "` + categoryID.String() + `",
		"occurred_at": "2026-06-17T12:00:00Z"
	}`)

	userID := uuid.New()
	reply, err := sut.CreateCardPurchase(context.Background(), userID, payload)
	require.NoError(t, err)
	assert.True(t, cp.called)
	assert.Equal(t, userID, cp.gotPrincipal.UserID)
	assert.Equal(t, cardID, cp.gotInput.CardID)
	assert.Equal(t, int64(120000), cp.gotInput.TotalAmountCents)
	assert.Equal(t, 6, cp.gotInput.InstallmentsTotal)
	assert.Equal(t, categoryID, cp.gotInput.CategoryID)
	assert.Contains(t, reply, "1200,00")
	assert.Contains(t, reply, "6x")
}

func TestTransactionsAdapter_CreateCardPurchase_InvalidAmountRejects(t *testing.T) {
	cardID := uuid.New()
	categoryID := uuid.New()
	sut := newAdapterWithAll(&stubCreateCardPurchase{}, nil, nil)

	cases := []struct {
		name    string
		payload string
	}{
		{"zero", `{"amount_cents":0,"card_id":"` + cardID.String() + `","installments":2,"category_id":"` + categoryID.String() + `"}`},
		{"negative", `{"amount_cents":-100,"card_id":"` + cardID.String() + `","installments":2,"category_id":"` + categoryID.String() + `"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := sut.CreateCardPurchase(context.Background(), uuid.New(), json.RawMessage(tc.payload))
			require.Error(t, err)
			assert.True(t, errors.Is(err, dispatcher.ErrTransactionsCreateInvalidPayload))
		})
	}
}

func TestTransactionsAdapter_CreateCardPurchase_InvalidCardIDRejects(t *testing.T) {
	categoryID := uuid.New()
	sut := newAdapterWithAll(&stubCreateCardPurchase{}, nil, nil)

	payload := json.RawMessage(`{"amount_cents":1000,"card_id":"not-uuid","installments":2,"category_id":"` + categoryID.String() + `"}`)
	_, err := sut.CreateCardPurchase(context.Background(), uuid.New(), payload)
	require.Error(t, err)
	assert.True(t, errors.Is(err, dispatcher.ErrTransactionsCreateInvalidPayload))
}

func TestTransactionsAdapter_CreateCardPurchase_InstallmentsBelowTwoRejects(t *testing.T) {
	cardID := uuid.New()
	categoryID := uuid.New()
	sut := newAdapterWithAll(&stubCreateCardPurchase{}, nil, nil)

	for _, installments := range []int{0, 1} {
		payload := json.RawMessage(`{"amount_cents":1000,"card_id":"` + cardID.String() + `","installments":` +
			func() string {
				if installments == 0 {
					return "0"
				}
				return "1"
			}() + `,"category_id":"` + categoryID.String() + `"}`)
		_, err := sut.CreateCardPurchase(context.Background(), uuid.New(), payload)
		require.Error(t, err)
		assert.True(t, errors.Is(err, dispatcher.ErrTransactionsCreateInvalidPayload))
	}
}

func TestTransactionsAdapter_CreateCardPurchase_UCErrorPropagates(t *testing.T) {
	cardID := uuid.New()
	categoryID := uuid.New()
	cp := &stubCreateCardPurchase{err: errors.New("db error")}
	sut := newAdapterWithAll(cp, nil, nil)

	payload := json.RawMessage(`{"amount_cents":1000,"card_id":"` + cardID.String() + `","installments":2,"category_id":"` + categoryID.String() + `"}`)
	_, err := sut.CreateCardPurchase(context.Background(), uuid.New(), payload)
	require.Error(t, err)
	assert.True(t, cp.called)
}

func TestTransactionsAdapter_CreateRecurring_HappyPath_Outcome(t *testing.T) {
	categoryID := uuid.New()
	rc := &stubCreateRecurringTemplate{resp: transactionsoutput.RecurringTemplate{
		AmountCents: 120000,
		Direction:   "outcome",
		Frequency:   "monthly",
		Description: "aluguel",
	}}
	sut := newAdapterWithAll(nil, rc, nil)

	payload := json.RawMessage(`{
		"amount_cents": 120000,
		"direction": "outcome",
		"frequency": "monthly",
		"day_of_month": 5,
		"description": "aluguel",
		"category_id": "` + categoryID.String() + `"
	}`)

	userID := uuid.New()
	reply, err := sut.CreateRecurring(context.Background(), userID, payload)
	require.NoError(t, err)
	assert.True(t, rc.called)
	assert.Equal(t, userID, rc.gotPrincipal.UserID)
	assert.Equal(t, int64(120000), rc.gotInput.AmountCents)
	assert.Equal(t, "outcome", rc.gotInput.Direction)
	assert.Equal(t, "monthly", rc.gotInput.Frequency)
	assert.Equal(t, 5, rc.gotInput.DayOfMonth)
	assert.Equal(t, categoryID, rc.gotInput.CategoryID)
	assert.Contains(t, reply, "saida")
	assert.Contains(t, reply, "mensal")
}

func TestTransactionsAdapter_CreateRecurring_HappyPath_Income(t *testing.T) {
	categoryID := uuid.New()
	rc := &stubCreateRecurringTemplate{resp: transactionsoutput.RecurringTemplate{
		AmountCents: 500000,
		Direction:   "income",
		Frequency:   "monthly",
		Description: "salario",
	}}
	sut := newAdapterWithAll(nil, rc, nil)

	payload := json.RawMessage(`{
		"amount_cents": 500000,
		"direction": "income",
		"description": "salario",
		"category_id": "` + categoryID.String() + `"
	}`)

	reply, err := sut.CreateRecurring(context.Background(), uuid.New(), payload)
	require.NoError(t, err)
	assert.Contains(t, reply, "entrada")
	assert.Contains(t, reply, "mensal")
	assert.Equal(t, "monthly", rc.gotInput.Frequency)
	assert.Equal(t, 1, rc.gotInput.DayOfMonth)
}

func TestTransactionsAdapter_CreateRecurring_DefaultsFrequencyToMonthly(t *testing.T) {
	categoryID := uuid.New()
	rc := &stubCreateRecurringTemplate{resp: transactionsoutput.RecurringTemplate{
		AmountCents: 1000, Direction: "income", Frequency: "monthly", Description: "x",
	}}
	sut := newAdapterWithAll(nil, rc, nil)

	payload := json.RawMessage(`{"amount_cents":1000,"direction":"income","category_id":"` + categoryID.String() + `"}`)
	_, err := sut.CreateRecurring(context.Background(), uuid.New(), payload)
	require.NoError(t, err)
	assert.Equal(t, "monthly", rc.gotInput.Frequency)
}

func TestTransactionsAdapter_CreateRecurring_InvalidDirectionRejects(t *testing.T) {
	categoryID := uuid.New()
	sut := newAdapterWithAll(nil, &stubCreateRecurringTemplate{}, nil)

	payload := json.RawMessage(`{"amount_cents":1000,"direction":"expense","category_id":"` + categoryID.String() + `"}`)
	_, err := sut.CreateRecurring(context.Background(), uuid.New(), payload)
	require.Error(t, err)
	assert.True(t, errors.Is(err, dispatcher.ErrTransactionsCreateInvalidPayload))
}

func TestTransactionsAdapter_CreateRecurring_InvalidFrequencyRejects(t *testing.T) {
	categoryID := uuid.New()
	sut := newAdapterWithAll(nil, &stubCreateRecurringTemplate{}, nil)

	payload := json.RawMessage(`{"amount_cents":1000,"direction":"income","frequency":"weekly","category_id":"` + categoryID.String() + `"}`)
	_, err := sut.CreateRecurring(context.Background(), uuid.New(), payload)
	require.Error(t, err)
	assert.True(t, errors.Is(err, dispatcher.ErrTransactionsCreateInvalidPayload))
}

func TestTransactionsAdapter_ListRecurring_EmptyList(t *testing.T) {
	lr := &stubListRecurringTemplates{resp: transactionsusecases.RecurringTemplatePage{}}
	sut := newAdapterWithAll(nil, nil, lr)

	reply, err := sut.ListRecurring(context.Background(), uuid.New(), nil)
	require.NoError(t, err)
	assert.True(t, lr.called)
	assert.Contains(t, reply, "Nenhuma")
}

func TestTransactionsAdapter_ListRecurring_WithTemplates(t *testing.T) {
	lr := &stubListRecurringTemplates{resp: transactionsusecases.RecurringTemplatePage{
		Templates: []transactionsoutput.RecurringTemplate{
			{AmountCents: 120000, Direction: "outcome", Frequency: "monthly", Description: "aluguel"},
			{AmountCents: 500000, Direction: "income", Frequency: "monthly", Description: "salario"},
		},
	}}
	sut := newAdapterWithAll(nil, nil, lr)

	reply, err := sut.ListRecurring(context.Background(), uuid.New(), nil)
	require.NoError(t, err)
	assert.True(t, lr.called)
	assert.Contains(t, reply, "2 recorrencia")
	assert.Contains(t, reply, "saida")
	assert.Contains(t, reply, "entrada")
	assert.Contains(t, reply, "mensal")
	assert.Contains(t, reply, "aluguel")
}

func TestTransactionsAdapter_ListRecurring_UCErrorPropagates(t *testing.T) {
	lr := &stubListRecurringTemplates{err: errors.New("db error")}
	sut := newAdapterWithAll(nil, nil, lr)

	_, err := sut.ListRecurring(context.Background(), uuid.New(), nil)
	require.Error(t, err)
	assert.True(t, lr.called)
}
