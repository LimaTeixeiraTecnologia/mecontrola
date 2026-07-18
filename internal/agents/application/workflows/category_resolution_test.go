package workflows

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces/mocks"
)

func TestSearchAndEnrichCandidates_SingleCandidate_Accepted(t *testing.T) {
	ctx := context.Background()
	rootID := uuid.New()
	subID := uuid.New()

	m := mocks.NewCategoriesReader(t)
	m.EXPECT().
		SearchDictionary(ctx, "supermercado", "expense").
		Return(interfaces.CategorySearchResult{
			Outcome: interfaces.ClassifyOutcomeMatched,
			Version: 3,
			Candidates: []interfaces.CategoryCandidate{
				{
					CategoryID:     subID,
					RootCategoryID: rootID,
					Path:           "custo-fixo > supermercado",
					MatchedTerm:    "supermercado",
					Score:          0.99,
					Confidence:     "high",
					MatchQuality:   "exact",
					MatchReason:    "direct match",
				},
			},
		}, nil).
		Once()
	m.EXPECT().
		ResolveForWrite(ctx, interfaces.CategoryWriteRequest{
			RootCategoryID:  rootID,
			SubcategoryID:   subID,
			Kind:            interfaces.CategoryKindExpense,
			ExpectedVersion: 3,
		}).
		Return(interfaces.CategoryWriteDecision{
			RootCategoryID:   rootID,
			SubcategoryID:    subID,
			Kind:             interfaces.CategoryKindExpense,
			Path:             "custo-fixo > supermercado",
			RootSlug:         "custo-fixo",
			SubcategorySlug:  "supermercado",
			EditorialVersion: 3,
		}, nil).
		Once()

	candidates, _, err := SearchAndEnrichCandidates(ctx, m, "supermercado", interfaces.CategoryKindExpense, 3)

	require.NoError(t, err)
	require.Len(t, candidates, 1)
	require.Equal(t, rootID, candidates[0].RootCategoryID)
	require.Equal(t, subID, candidates[0].SubcategoryID)
	require.Equal(t, "custo-fixo", candidates[0].RootSlug)
	require.Equal(t, "supermercado", candidates[0].SubcategorySlug)
}

func TestSearchAndEnrichCandidates_MultipleValidCandidates_G7_10(t *testing.T) {
	ctx := context.Background()
	rootID := uuid.New()
	sub1 := uuid.New()
	sub2 := uuid.New()
	sub3 := uuid.New()

	m := mocks.NewCategoriesReader(t)
	m.EXPECT().
		SearchDictionary(ctx, "saúde", "expense").
		Return(interfaces.CategorySearchResult{
			Outcome: interfaces.ClassifyOutcomeAmbiguous,
			Version: 2,
			Candidates: []interfaces.CategoryCandidate{
				{CategoryID: sub1, RootCategoryID: rootID, Path: "custo-fixo > plano-de-saude", MatchedTerm: "saude"},
				{CategoryID: sub2, RootCategoryID: rootID, Path: "custo-fixo > consultas-e-exames", MatchedTerm: "saude"},
				{CategoryID: sub3, RootCategoryID: rootID, Path: "custo-fixo > terapia-e-saude-mental", MatchedTerm: "saude"},
			},
		}, nil).
		Once()
	m.EXPECT().
		ResolveForWrite(ctx, interfaces.CategoryWriteRequest{RootCategoryID: rootID, SubcategoryID: sub1, Kind: interfaces.CategoryKindExpense, ExpectedVersion: 2}).
		Return(interfaces.CategoryWriteDecision{RootCategoryID: rootID, SubcategoryID: sub1, RootSlug: "custo-fixo", SubcategorySlug: "plano-de-saude", Path: "custo-fixo > plano-de-saude", EditorialVersion: 2}, nil).
		Once()
	m.EXPECT().
		ResolveForWrite(ctx, interfaces.CategoryWriteRequest{RootCategoryID: rootID, SubcategoryID: sub2, Kind: interfaces.CategoryKindExpense, ExpectedVersion: 2}).
		Return(interfaces.CategoryWriteDecision{RootCategoryID: rootID, SubcategoryID: sub2, RootSlug: "custo-fixo", SubcategorySlug: "consultas-e-exames", Path: "custo-fixo > consultas-e-exames", EditorialVersion: 2}, nil).
		Once()
	m.EXPECT().
		ResolveForWrite(ctx, interfaces.CategoryWriteRequest{RootCategoryID: rootID, SubcategoryID: sub3, Kind: interfaces.CategoryKindExpense, ExpectedVersion: 2}).
		Return(interfaces.CategoryWriteDecision{RootCategoryID: rootID, SubcategoryID: sub3, RootSlug: "custo-fixo", SubcategorySlug: "terapia-e-saude-mental", Path: "custo-fixo > terapia-e-saude-mental", EditorialVersion: 2}, nil).
		Once()

	candidates, _, err := SearchAndEnrichCandidates(ctx, m, "saúde", interfaces.CategoryKindExpense, 2)

	require.NoError(t, err)
	require.Len(t, candidates, 3)
	require.Equal(t, "plano-de-saude", candidates[0].SubcategorySlug)
	require.Equal(t, "consultas-e-exames", candidates[1].SubcategorySlug)
	require.Equal(t, "terapia-e-saude-mental", candidates[2].SubcategorySlug)
}

func TestSearchAndEnrichCandidates_RootOnlySkipped_G7_03(t *testing.T) {
	ctx := context.Background()
	rootID := uuid.New()

	m := mocks.NewCategoriesReader(t)
	m.EXPECT().
		SearchDictionary(ctx, "custo fixo", "expense").
		Return(interfaces.CategorySearchResult{
			Outcome: interfaces.ClassifyOutcomeMatched,
			Version: 1,
			Candidates: []interfaces.CategoryCandidate{
				{
					CategoryID:     rootID,
					RootCategoryID: rootID,
					Path:           "custo-fixo",
					MatchedTerm:    "custo fixo",
				},
			},
		}, nil).
		Once()

	candidates, _, err := SearchAndEnrichCandidates(ctx, m, "custo fixo", interfaces.CategoryKindExpense, 1)

	require.NoError(t, err)
	require.Empty(t, candidates)
}

func TestSearchAndEnrichCandidates_ResolveForWriteFails_CandidateRejected_G7_11(t *testing.T) {
	ctx := context.Background()
	rootID := uuid.New()
	sub1 := uuid.New()
	sub2 := uuid.New()

	m := mocks.NewCategoriesReader(t)
	m.EXPECT().
		SearchDictionary(ctx, "medicamento", "expense").
		Return(interfaces.CategorySearchResult{
			Outcome: interfaces.ClassifyOutcomeAmbiguous,
			Version: 1,
			Candidates: []interfaces.CategoryCandidate{
				{CategoryID: sub1, RootCategoryID: rootID, Path: "custo-fixo > medicamentos", MatchedTerm: "medicamento"},
				{CategoryID: sub2, RootCategoryID: rootID, Path: "custo-fixo > medicamentos-continuos", MatchedTerm: "medicamento"},
			},
		}, nil).
		Once()
	m.EXPECT().
		ResolveForWrite(ctx, interfaces.CategoryWriteRequest{RootCategoryID: rootID, SubcategoryID: sub1, Kind: interfaces.CategoryKindExpense, ExpectedVersion: 1}).
		Return(interfaces.CategoryWriteDecision{}, errors.New("deprecated")).
		Once()
	m.EXPECT().
		ResolveForWrite(ctx, interfaces.CategoryWriteRequest{RootCategoryID: rootID, SubcategoryID: sub2, Kind: interfaces.CategoryKindExpense, ExpectedVersion: 1}).
		Return(interfaces.CategoryWriteDecision{RootCategoryID: rootID, SubcategoryID: sub2, RootSlug: "custo-fixo", SubcategorySlug: "medicamentos-continuos", Path: "custo-fixo > medicamentos-continuos", EditorialVersion: 1}, nil).
		Once()

	candidates, _, err := SearchAndEnrichCandidates(ctx, m, "medicamento", interfaces.CategoryKindExpense, 1)

	require.NoError(t, err)
	require.Len(t, candidates, 1)
	require.Equal(t, "medicamentos-continuos", candidates[0].SubcategorySlug)
}

func TestSearchAndEnrichCandidates_AllCandidatesRejectByResolve_ZeroResult_G7_11(t *testing.T) {
	ctx := context.Background()
	rootID := uuid.New()
	subID := uuid.New()

	m := mocks.NewCategoriesReader(t)
	m.EXPECT().
		SearchDictionary(ctx, "farmácia", "expense").
		Return(interfaces.CategorySearchResult{
			Outcome: interfaces.ClassifyOutcomeMatched,
			Version: 2,
			Candidates: []interfaces.CategoryCandidate{
				{CategoryID: subID, RootCategoryID: rootID, Path: "custo-fixo > farmacia", MatchedTerm: "farmácia"},
			},
		}, nil).
		Once()
	m.EXPECT().
		ResolveForWrite(mock.Anything, mock.Anything).
		Return(interfaces.CategoryWriteDecision{}, errors.New("version mismatch")).
		Once()

	candidates, _, err := SearchAndEnrichCandidates(ctx, m, "farmácia", interfaces.CategoryKindExpense, 2)

	require.NoError(t, err)
	require.Empty(t, candidates)
}

func TestSearchAndEnrichCandidates_SearchError_ReturnsError(t *testing.T) {
	ctx := context.Background()

	m := mocks.NewCategoriesReader(t)
	m.EXPECT().
		SearchDictionary(ctx, "xyz", "expense").
		Return(interfaces.CategorySearchResult{}, errors.New("db error")).
		Once()

	_, _, err := SearchAndEnrichCandidates(ctx, m, "xyz", interfaces.CategoryKindExpense, 1)
	require.Error(t, err)
}

func TestSearchAndEnrichCandidates_G7_12_NewDescriptionResolves(t *testing.T) {
	ctx := context.Background()
	rootID := uuid.New()
	subID := uuid.New()

	m := mocks.NewCategoriesReader(t)
	m.EXPECT().
		SearchDictionary(ctx, "farmácia", "expense").
		Return(interfaces.CategorySearchResult{
			Outcome: interfaces.ClassifyOutcomeMatched,
			Version: 1,
			Candidates: []interfaces.CategoryCandidate{
				{CategoryID: subID, RootCategoryID: rootID, Path: "custo-fixo > medicamentos-e-farmacia", MatchedTerm: "farmácia"},
			},
		}, nil).
		Once()
	m.EXPECT().
		ResolveForWrite(ctx, interfaces.CategoryWriteRequest{RootCategoryID: rootID, SubcategoryID: subID, Kind: interfaces.CategoryKindExpense, ExpectedVersion: 1}).
		Return(interfaces.CategoryWriteDecision{RootCategoryID: rootID, SubcategoryID: subID, RootSlug: "custo-fixo", SubcategorySlug: "medicamentos-e-farmacia", Path: "custo-fixo > medicamentos-e-farmacia", EditorialVersion: 1}, nil).
		Once()

	candidates, _, err := SearchAndEnrichCandidates(ctx, m, "farmácia", interfaces.CategoryKindExpense, 1)

	require.NoError(t, err)
	require.Len(t, candidates, 1)
	require.Equal(t, subID, candidates[0].SubcategoryID)
	require.Equal(t, "medicamentos-e-farmacia", candidates[0].SubcategorySlug)
}

func TestBuildCandidateListText_MultipleEntries(t *testing.T) {
	candidates := []PendingCategoryCandidate{
		{RootSlug: "custo-fixo", SubcategorySlug: "plano-de-saude", Path: "custo-fixo > plano-de-saude"},
		{RootSlug: "custo-fixo", SubcategorySlug: "consultas-e-exames", Path: "custo-fixo > consultas-e-exames"},
		{RootSlug: "custo-fixo", SubcategorySlug: "terapia-e-saude-mental", Path: "custo-fixo > terapia-e-saude-mental"},
	}
	text := BuildCandidateListText(candidates)
	require.Contains(t, text, "1.")
	require.Contains(t, text, "2.")
	require.Contains(t, text, "3.")
	require.Contains(t, text, "custo-fixo > plano-de-saude")
	require.Contains(t, text, "custo-fixo > consultas-e-exames")
	require.Contains(t, text, "custo-fixo > terapia-e-saude-mental")
}

func TestBuildCandidateListText_Empty(t *testing.T) {
	text := BuildCandidateListText(nil)
	require.Empty(t, text)
}

func TestBuildCandidateListText_SingleEntry(t *testing.T) {
	candidates := []PendingCategoryCandidate{
		{Path: "custo-fixo > supermercado"},
	}
	text := BuildCandidateListText(candidates)
	require.Equal(t, "1. custo-fixo > supermercado", text)
}
