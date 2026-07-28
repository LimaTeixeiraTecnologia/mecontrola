package tools

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	imocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces/mocks"
)

func TestBuildQueryDayToolTodayDefault(t *testing.T) {
	ledger := imocks.NewTransactionsLedger(t)
	today := time.Now().UTC().Format("2006-01-02")

	ledger.EXPECT().GetDaySummary(mock.Anything, testUserID, today).
		Return(interfaces.DaySummary{
			Day:          today,
			IncomeCents:  50000,
			OutcomeCents: 7500,
			TotalCents:   42500,
			Entries: []interfaces.MonthlyEntry{
				{Kind: interfaces.EntryKindTransaction, ID: testResourceID.String(), RefMonth: "2026-07", AmountCents: 4500, Direction: "outcome", Description: "Farmácia", CreatedAt: time.Now()},
			},
		}, nil).Once()

	handle := BuildQueryDayTool(ledger)
	assert.Equal(t, "query_day", handle.ID())

	argsJSON, _ := json.Marshal(QueryDayInput{})
	out, _, err := handle.Invoke(identityCtx("msg-qd", 0), argsJSON)
	require.NoError(t, err)

	var result QueryDayOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, "ok", result.Outcome)
	assert.Equal(t, today, result.Day)
	assert.Equal(t, int64(50000), result.IncomeCents)
	assert.Equal(t, "R$ 500,00", result.IncomeBRL)
	assert.Equal(t, int64(7500), result.OutcomeCents)
	assert.Equal(t, "R$ 75,00", result.OutcomeBRL)
	assert.Equal(t, int64(42500), result.TotalCents)
	assert.Equal(t, "R$ 425,00", result.TotalBRL)
	assert.Len(t, result.Entries, 1)
	assert.Equal(t, "R$ 45,00", result.Entries[0].AmountBRL)
}

func TestBuildQueryDayToolYesterday(t *testing.T) {
	ledger := imocks.NewTransactionsLedger(t)
	yesterday := time.Now().UTC().Add(-24 * time.Hour).Format("2006-01-02")

	ledger.EXPECT().GetDaySummary(mock.Anything, testUserID, yesterday).
		Return(interfaces.DaySummary{Day: yesterday, Entries: []interfaces.MonthlyEntry{}}, nil).Once()

	handle := BuildQueryDayTool(ledger)
	argsJSON, _ := json.Marshal(QueryDayInput{DayRefKind: "yesterday"})
	out, _, err := handle.Invoke(identityCtx("msg-qd2", 0), argsJSON)
	require.NoError(t, err)

	var result QueryDayOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, yesterday, result.Day)
	assert.Equal(t, int64(0), result.OutcomeCents)
}

func TestBuildQueryDayToolLedgerError(t *testing.T) {
	ledger := imocks.NewTransactionsLedger(t)
	today := time.Now().UTC().Format("2006-01-02")

	ledger.EXPECT().GetDaySummary(mock.Anything, testUserID, today).
		Return(interfaces.DaySummary{}, errors.New("db error")).Once()

	handle := BuildQueryDayTool(ledger)
	argsJSON, _ := json.Marshal(QueryDayInput{})
	_, _, err := handle.Invoke(identityCtx("msg-qd3", 0), argsJSON)
	require.Error(t, err)
}
