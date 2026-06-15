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
	budgetsinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/input"
	budgetsoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
)

type stubListBudgets struct {
	called bool
	gotIn  budgetsinput.ListAlertsInput
	resp   budgetsoutput.ListAlertsOutput
	err    error
}

func (s *stubListBudgets) Execute(_ context.Context, in budgetsinput.ListAlertsInput) (budgetsoutput.ListAlertsOutput, error) {
	s.called = true
	s.gotIn = in
	return s.resp, s.err
}

func TestBudgetsAdapter_List_NoAlerts(t *testing.T) {
	uc := &stubListBudgets{resp: budgetsoutput.ListAlertsOutput{}}
	sut := dispatcher.NewBudgetsAdapter(uc)

	reply, err := sut.List(context.Background(), uuid.New(), json.RawMessage(`{"month":"2026-06"}`))
	require.NoError(t, err)
	assert.Contains(t, reply, "2026-06")
	assert.Contains(t, reply, "Nenhum")
	assert.Equal(t, "2026-06", uc.gotIn.Competence.String())
}

func TestBudgetsAdapter_List_WithAlerts(t *testing.T) {
	uc := &stubListBudgets{resp: budgetsoutput.ListAlertsOutput{
		Alerts: []budgetsoutput.AlertOutput{
			{RootSlug: "alimentacao", Threshold: 80, State: "TRIGGERED"},
			{RootSlug: "transporte", Threshold: 100, State: "EXCEEDED"},
		},
	}}
	sut := dispatcher.NewBudgetsAdapter(uc)

	reply, err := sut.List(context.Background(), uuid.New(), json.RawMessage(`{"month":"2026-06"}`))
	require.NoError(t, err)
	assert.Contains(t, reply, "alimentacao")
	assert.Contains(t, reply, "transporte")
	assert.Contains(t, reply, "80%")
	assert.Contains(t, reply, "100%")
}

func TestBudgetsAdapter_List_InvalidMonth_Rejects(t *testing.T) {
	uc := &stubListBudgets{}
	sut := dispatcher.NewBudgetsAdapter(uc)

	_, err := sut.List(context.Background(), uuid.New(), json.RawMessage(`{"month":"invalid"}`))
	require.Error(t, err)
}

func TestBudgetsAdapter_List_DefaultsToCurrentMonth(t *testing.T) {
	uc := &stubListBudgets{resp: budgetsoutput.ListAlertsOutput{}}
	sut := dispatcher.NewBudgetsAdapter(uc)

	reply, err := sut.List(context.Background(), uuid.New(), nil)
	require.NoError(t, err)
	assert.NotEmpty(t, reply)
	assert.NotNil(t, uc.gotIn.Competence)
}

func TestBudgetsAdapter_List_NilUseCaseUnsupported(t *testing.T) {
	sut := dispatcher.NewBudgetsAdapter(nil)
	_, err := sut.List(context.Background(), uuid.New(), nil)
	require.Error(t, err)
}

func TestBudgetsAdapter_List_UseCaseError_Propagates(t *testing.T) {
	uc := &stubListBudgets{err: errors.New("db down")}
	sut := dispatcher.NewBudgetsAdapter(uc)

	_, err := sut.List(context.Background(), uuid.New(), json.RawMessage(`{"month":"2026-06"}`))
	require.Error(t, err)
}
