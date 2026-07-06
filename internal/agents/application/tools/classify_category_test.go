package tools

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	imocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces/mocks"
)

type ClassifyCategoryToolSuite struct {
	suite.Suite
	ctx    context.Context
	rootID uuid.UUID
	leafID uuid.UUID
}

func TestClassifyCategoryToolSuite(t *testing.T) {
	suite.Run(t, new(ClassifyCategoryToolSuite))
}

func (s *ClassifyCategoryToolSuite) SetupTest() {
	s.ctx = context.Background()
	s.rootID = uuid.New()
	s.leafID = uuid.New()
}

func (s *ClassifyCategoryToolSuite) TestClassifyCategory() {
	scenarios := []struct {
		name         string
		input        ClassifyCategoryInput
		searchResult interfaces.CategorySearchResult
		searchErr    error
		expect       func(output ClassifyCategoryOutput, err error)
	}{
		{
			name:  "match inequivoco retorna writeDecision allowed com candidato",
			input: ClassifyCategoryInput{Term: "restaurante", Kind: "expense"},
			searchResult: interfaces.CategorySearchResult{
				Outcome: interfaces.ClassifyOutcomeMatched,
				Version: 2,
				Candidates: []interfaces.CategoryCandidate{
					{
						CategoryID:     s.leafID,
						RootCategoryID: s.rootID,
						Path:           "Alimentação > Restaurante",
						MatchedTerm:    "restaurante",
						Score:          0.95,
						SignalType:     "alias",
						Confidence:     "high",
						MatchQuality:   "exact",
						MatchReason:    "alias match",
						IsAmbiguous:    false,
					},
				},
			},
			expect: func(output ClassifyCategoryOutput, err error) {
				s.NoError(err)
				s.Equal("allowed", output.WriteDecision)
				s.Equal("", output.BlockReason)
				s.Equal("matched", output.Outcome)
				s.Equal(int64(2), output.Version)
				s.Equal(s.rootID.String(), output.CategoryID)
				s.Equal(s.leafID.String(), output.SubcategoryID)
				s.Equal("Alimentação > Restaurante", output.Path)
				s.Len(output.Candidates, 1)
				s.Equal("restaurante", output.Candidates[0].MatchedTerm)
				s.False(output.IsAmbiguous)
			},
		},
		{
			name:  "no_match bloqueia sem candidato sugerido",
			input: ClassifyCategoryInput{Term: "xyzxyz", Kind: "expense"},
			searchResult: interfaces.CategorySearchResult{
				Outcome:    interfaces.ClassifyOutcomeNoMatch,
				Version:    1,
				Candidates: []interfaces.CategoryCandidate{},
			},
			expect: func(output ClassifyCategoryOutput, err error) {
				s.NoError(err)
				s.Equal("blocked", output.WriteDecision)
				s.NotEmpty(output.BlockReason)
				s.Equal("", output.CategoryID)
				s.Equal("", output.SubcategoryID)
				s.Equal("", output.Path)
				s.True(output.IsAmbiguous)
			},
		},
		{
			name:  "multiplos candidatos bloqueia sem candidato sugerido",
			input: ClassifyCategoryInput{Term: "mercado", Kind: "expense"},
			searchResult: interfaces.CategorySearchResult{
				Outcome: interfaces.ClassifyOutcomeAmbiguous,
				Version: 1,
				Candidates: []interfaces.CategoryCandidate{
					{CategoryID: s.leafID, RootCategoryID: s.rootID, Score: 0.8, Confidence: "medium", MatchQuality: "token"},
					{CategoryID: uuid.New(), RootCategoryID: uuid.New(), Score: 0.75, Confidence: "low", MatchQuality: "fuzzy"},
				},
			},
			expect: func(output ClassifyCategoryOutput, err error) {
				s.NoError(err)
				s.Equal("blocked", output.WriteDecision)
				s.NotEmpty(output.BlockReason)
				s.Equal("", output.CategoryID)
				s.Equal("", output.SubcategoryID)
				s.Len(output.Candidates, 2)
			},
		},
		{
			name:  "candidato unico ambiguo bloqueia sem candidato sugerido",
			input: ClassifyCategoryInput{Term: "saúde", Kind: "expense"},
			searchResult: interfaces.CategorySearchResult{
				Outcome: interfaces.ClassifyOutcomeMatched,
				Version: 1,
				Candidates: []interfaces.CategoryCandidate{
					{CategoryID: s.leafID, RootCategoryID: s.rootID, Score: 0.6, IsAmbiguous: true, Confidence: "low", MatchQuality: "fuzzy"},
				},
			},
			expect: func(output ClassifyCategoryOutput, err error) {
				s.NoError(err)
				s.Equal("blocked", output.WriteDecision)
				s.Equal("candidato ambíguo", output.BlockReason)
				s.Equal("", output.CategoryID)
				s.Equal("", output.SubcategoryID)
			},
		},
		{
			name:  "root igual a leaf bloqueia",
			input: ClassifyCategoryInput{Term: "alimentação", Kind: "expense"},
			searchResult: interfaces.CategorySearchResult{
				Outcome: interfaces.ClassifyOutcomeMatched,
				Version: 1,
				Candidates: []interfaces.CategoryCandidate{
					{CategoryID: s.rootID, RootCategoryID: s.rootID, Score: 0.9, Confidence: "high", MatchQuality: "exact"},
				},
			},
			expect: func(output ClassifyCategoryOutput, err error) {
				s.NoError(err)
				s.Equal("blocked", output.WriteDecision)
				s.Equal("raiz igual à subcategoria", output.BlockReason)
				s.Equal("", output.CategoryID)
				s.Equal("", output.SubcategoryID)
			},
		},
		{
			name:  "version ausente bloqueia",
			input: ClassifyCategoryInput{Term: "salário", Kind: "income"},
			searchResult: interfaces.CategorySearchResult{
				Outcome: interfaces.ClassifyOutcomeMatched,
				Version: 0,
				Candidates: []interfaces.CategoryCandidate{
					{CategoryID: s.leafID, RootCategoryID: s.rootID, Score: 0.9, Confidence: "high", MatchQuality: "exact"},
				},
			},
			expect: func(output ClassifyCategoryOutput, err error) {
				s.NoError(err)
				s.Equal("blocked", output.WriteDecision)
				s.Equal("versão editorial ausente", output.BlockReason)
				s.Equal("", output.CategoryID)
				s.Equal("", output.SubcategoryID)
			},
		},
		{
			name:      "erro de infraestrutura propaga",
			input:     ClassifyCategoryInput{Term: "erro", Kind: "expense"},
			searchErr: errors.New("dictionary down"),
			expect: func(output ClassifyCategoryOutput, err error) {
				s.Error(err)
			},
		},
		{
			name:  "candidatos ricos incluem matchedTerm e isAmbiguous",
			input: ClassifyCategoryInput{Term: "uber", Kind: "expense"},
			searchResult: interfaces.CategorySearchResult{
				Outcome: interfaces.ClassifyOutcomeMatched,
				Version: 3,
				Candidates: []interfaces.CategoryCandidate{
					{
						CategoryID:     s.leafID,
						RootCategoryID: s.rootID,
						Path:           "Transporte > Apps",
						MatchedTerm:    "uber",
						Score:          0.99,
						SignalType:     "merchant",
						Confidence:     "high",
						MatchQuality:   "exact",
						MatchReason:    "merchant match",
						IsAmbiguous:    false,
					},
				},
			},
			expect: func(output ClassifyCategoryOutput, err error) {
				s.NoError(err)
				s.Equal("allowed", output.WriteDecision)
				s.Require().Len(output.Candidates, 1)
				s.Equal("uber", output.Candidates[0].MatchedTerm)
				s.False(output.Candidates[0].IsAmbiguous)
				s.Equal("merchant", output.Candidates[0].SignalType)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			reader := imocks.NewCategoriesReader(s.T())
			reader.EXPECT().
				SearchDictionary(s.ctx, scenario.input.Term, scenario.input.Kind).
				Return(scenario.searchResult, scenario.searchErr).
				Once()

			exec := buildClassifyCategoryExec(reader)
			output, err := exec(s.ctx, scenario.input)
			scenario.expect(output, err)
		})
	}
}
