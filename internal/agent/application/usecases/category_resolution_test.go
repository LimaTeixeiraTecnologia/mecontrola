package usecases

import (
	"context"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	categoriesoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/dtos/output"
	categoriesvo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

type CategoryResolutionSuite struct {
	suite.Suite
	ctx context.Context
}

func TestCategoryResolutionSuite(t *testing.T) {
	suite.Run(t, new(CategoryResolutionSuite))
}

func (s *CategoryResolutionSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *CategoryResolutionSuite) TestAmbiguousProducesTypedError() {
	obs := fake.NewProvider()
	resolveBad := obs.Metrics().Counter("agent_test_resolve_bad", "teste", "1")
	rootID := uuid.New()
	resolver := &fakeResolver{out: &categoriesoutput.DictionarySearchOutput{
		Candidates: []categoriesoutput.CandidateOutput{
			{CategoryID: uuid.New(), RootCategoryID: rootID, Path: "Prazeres", IsAmbiguous: true},
			{CategoryID: uuid.New(), RootCategoryID: rootID, Path: "Custo Fixo"},
		},
	}}

	_, _, err := resolveCategoryCandidate(s.ctx, resolver, resolveBad, newMatchScoreHistogram(obs), "ifood", categoriesvo.KindExpense)
	s.Require().ErrorIs(err, ErrLogTransactionCategoryAmbiguous)

	var ambiguous *CategoryAmbiguousError
	s.Require().ErrorAs(err, &ambiguous)
	s.Equal("ifood", ambiguous.Hint)
	s.Equal([]string{"Prazeres", "Custo Fixo"}, ambiguous.Candidates)
}

func (s *CategoryResolutionSuite) TestNoMatchProducesNotFound() {
	obs := fake.NewProvider()
	resolveBad := obs.Metrics().Counter("agent_test_resolve_bad_nf", "teste", "1")
	resolver := &fakeResolver{out: &categoriesoutput.DictionarySearchOutput{Candidates: nil}}

	_, _, err := resolveCategoryCandidate(s.ctx, resolver, resolveBad, newMatchScoreHistogram(obs), "xyz", categoriesvo.KindExpense)
	s.Require().ErrorIs(err, ErrLogTransactionCategoryNotFound)
}

func (s *CategoryResolutionSuite) TestAutoScorePassesThrough() {
	obs := fake.NewProvider()
	resolveBad := obs.Metrics().Counter("agent_test_resolve_bad_auto", "teste", "1")
	id := uuid.New()
	resolver := &fakeResolver{out: &categoriesoutput.DictionarySearchOutput{
		Candidates: []categoriesoutput.CandidateOutput{
			{CategoryID: id, RootCategoryID: id, Path: "Custo Fixo > Supermercado", Score: 0.95},
		},
	}}

	candidate, path, err := resolveCategoryCandidate(s.ctx, resolver, resolveBad, newMatchScoreHistogram(obs), "mercado", categoriesvo.KindExpense)
	s.Require().NoError(err)
	s.Equal("Custo Fixo > Supermercado", path)
	s.Equal(0.95, candidate.Score)
}

func (s *CategoryResolutionSuite) TestMediumScoreProducesNeedsConfirmation() {
	obs := fake.NewProvider()
	resolveBad := obs.Metrics().Counter("agent_test_resolve_bad_conf", "teste", "1")
	id := uuid.New()
	resolver := &fakeResolver{out: &categoriesoutput.DictionarySearchOutput{
		Candidates: []categoriesoutput.CandidateOutput{
			{CategoryID: id, RootCategoryID: id, Path: "Prazeres > Delivery", Score: 0.62},
		},
	}}

	_, _, err := resolveCategoryCandidate(s.ctx, resolver, resolveBad, newMatchScoreHistogram(obs), "ifood", categoriesvo.KindExpense)
	s.Require().ErrorIs(err, ErrLogTransactionCategoryNeedsConfirmation)

	var needsConfirmation *CategoryNeedsConfirmationError
	s.Require().ErrorAs(err, &needsConfirmation)
	s.Equal("ifood", needsConfirmation.Hint)
	s.Equal([]string{"Prazeres > Delivery"}, needsConfirmation.Candidates)
}

func (s *CategoryResolutionSuite) TestLowScoreProducesNotFound() {
	obs := fake.NewProvider()
	resolveBad := obs.Metrics().Counter("agent_test_resolve_bad_low", "teste", "1")
	id := uuid.New()
	resolver := &fakeResolver{out: &categoriesoutput.DictionarySearchOutput{
		Candidates: []categoriesoutput.CandidateOutput{
			{CategoryID: id, RootCategoryID: id, Path: "Prazeres", Score: 0.30},
		},
	}}

	_, _, err := resolveCategoryCandidate(s.ctx, resolver, resolveBad, newMatchScoreHistogram(obs), "xpto", categoriesvo.KindExpense)
	s.Require().ErrorIs(err, ErrLogTransactionCategoryNotFound)
}
