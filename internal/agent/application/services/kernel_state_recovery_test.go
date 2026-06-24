package services_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow/steps"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/pendingexpense"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type StateRecoverySuite struct {
	suite.Suite
	ctx context.Context
}

func TestStateRecoverySuite(t *testing.T) {
	suite.Run(t, new(StateRecoverySuite))
}

func (s *StateRecoverySuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *StateRecoverySuite) TestResumeStatePreservesAmbiguousData() { //nolint:revive // test: scenario requires >40 statements to verify state merge correctness across suspend/resume boundary
	obs := fake.NewProvider()
	store := newE2EStore()
	engine := platform.NewEngine[steps.ExpenseState](store, obs)

	userID := uuid.New()
	channel := "whatsapp"

	candidates := []string{"Prazeres > Academia", "Custo Fixo > Academia"}

	resolver := func(_ context.Context, st steps.ExpenseState) (steps.ExpenseState, error) {
		if st.ForceCategory == nil && st.AwaitingKind == "" {
			return st, &tools.CategoryAmbiguousError{
				Hint:       "academia",
				Candidates: candidates,
			}
		}
		cat := candidates[0]
		if st.ForceCategory != nil {
			cat = *st.ForceCategory
		}
		st.CategoryID = cat
		st.CategoryPath = cat
		return st, nil
	}

	persist := func(_ context.Context, st steps.ExpenseState) (steps.PersistResult, error) {
		s.Equal(int64(5800), st.AmountCents,
			"persist should receive original AmountCents from suspended state")
		return steps.PersistResult{AmountCents: 5800, CategoryPath: candidates[0]}, nil
	}

	def := buildE2ETransactionsWriteDef(resolver, persist)

	initial := steps.ExpenseState{
		UserID:          userID,
		Channel:         channel,
		AmountCents:     5800,
		Merchant:        "academia",
		PaymentMethod:   "debit",
		Direction:       "outcome",
		TransactionKind: pendingexpense.TransactionKindExpense,
	}

	result1, err := engine.Start(s.ctx, def, userID.String()+":"+channel, initial)
	s.NoError(err)
	s.Equal(platform.RunStatusSuspended, result1.Status)
	s.Equal(pendingexpense.AwaitingCategoryChoice, result1.State.AwaitingKind)
	s.Equal(candidates, result1.State.Candidates)
	s.Equal(int64(5800), result1.State.AmountCents)

	snap, found, _ := store.Load(s.ctx, "transactions_write", userID.String()+":"+channel)
	s.True(found)
	storedState, _ := platform.NewCodec[steps.ExpenseState]().Decode(snap.State)
	s.Equal(pendingexpense.AwaitingCategoryChoice, storedState.AwaitingKind)
	s.Equal(candidates, storedState.Candidates)
	s.Equal(int64(5800), storedState.AmountCents)

	minimalResumeState := steps.ExpenseState{
		UserID:     userID,
		Channel:    channel,
		ResumeText: "1",
	}
	resumeBytes, _ := json.Marshal(minimalResumeState)

	result2, resumeErr := engine.Resume(s.ctx, def, userID.String()+":"+channel, resumeBytes)

	s.NoError(resumeErr)
	s.Equal(platform.RunStatusSucceeded, result2.Status, "resume should complete workflow")

	assert.Equal(s.T(), int64(5800), result2.State.AmountCents,
		"BUG: AmountCents was lost on resume! Expected 5800 but got %d",
		result2.State.AmountCents)
	assert.Equal(s.T(), candidates, result2.State.Candidates,
		"BUG: Candidates were lost on resume!")
	assert.Equal(s.T(), pendingexpense.AwaitingKind(""), result2.State.AwaitingKind,
		"AwaitingKind should be cleared after resume, but got %s", result2.State.AwaitingKind)
	assert.Equal(s.T(), tools.OutcomeRouted, result2.State.Outcome)
}
