package services_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

type CandidateResolverSuite struct {
	suite.Suite
}

func TestCandidateResolverSuite(t *testing.T) {
	suite.Run(t, new(CandidateResolverSuite))
}

func (s *CandidateResolverSuite) newCategory(id uuid.UUID, name string, kind valueobjects.Kind, parentID *uuid.UUID) entities.Category {
	return entities.Category{
		ID:             id,
		Slug:           name,
		Name:           name,
		Kind:           kind,
		ParentID:       parentID,
		AllocationType: valueobjects.AllocationTypeConsumption,
	}
}

func (s *CandidateResolverSuite) newEntry(categoryID uuid.UUID, kind valueobjects.Kind, term string, signalType valueobjects.SignalType, confidence valueobjects.Confidence, isAmbiguous bool) entities.DictionaryEntry {
	return entities.DictionaryEntry{
		ID:          uuid.New(),
		CategoryID:  categoryID,
		Kind:        kind,
		Term:        term,
		SignalType:  signalType,
		Confidence:  confidence,
		IsAmbiguous: isAmbiguous,
	}
}

func (s *CandidateResolverSuite) TestResolveEmptyEntries() {
	resolver := services.NewCandidateResolver()
	candidates, hasMore := resolver.Resolve([]entities.DictionaryEntry{}, nil)

	s.Nil(candidates)
	s.False(hasMore)
}

func (s *CandidateResolverSuite) TestResolveHighUnequivocal() {
	resolver := services.NewCandidateResolver()

	rootID := uuid.New()
	subID := uuid.New()

	categories := map[uuid.UUID]entities.Category{
		rootID: s.newCategory(rootID, "Salario", valueobjects.KindIncome, nil),
		subID:  s.newCategory(subID, "Decimo Terceiro", valueobjects.KindIncome, &rootID),
	}

	entries := []entities.DictionaryEntry{
		s.newEntry(subID, valueobjects.KindIncome, "13 salario", valueobjects.SignalTypeAlias, valueobjects.ConfidenceHigh, false),
	}

	candidates, hasMore := resolver.Resolve(entries, categories)

	s.Len(candidates, 1)
	s.False(hasMore)
	s.Equal(subID, candidates[0].CategoryID)
	s.Equal(rootID, candidates[0].RootCategoryID)
	s.Equal("Salario > Decimo Terceiro", candidates[0].Path)
	s.Equal("13 salario", candidates[0].MatchedTerm)
	s.Equal(valueobjects.SignalTypeAlias, candidates[0].SignalType)
	s.Equal(valueobjects.ConfidenceHigh, candidates[0].Confidence)
	s.False(candidates[0].IsAmbiguous)
	s.Equal("alias inequivoco", candidates[0].MatchReason)
}

func (s *CandidateResolverSuite) TestResolveMerchantAmbiguous() {
	resolver := services.NewCandidateResolver()

	rootID1 := uuid.New()
	rootID2 := uuid.New()
	subID1 := uuid.New()
	subID2 := uuid.New()

	categories := map[uuid.UUID]entities.Category{
		rootID1: s.newCategory(rootID1, "Custo Fixo", valueobjects.KindExpense, nil),
		rootID2: s.newCategory(rootID2, "Prazeres", valueobjects.KindExpense, nil),
		subID1:  s.newCategory(subID1, "Transporte por Aplicativo Recorrente", valueobjects.KindExpense, &rootID1),
		subID2:  s.newCategory(subID2, "Transporte de Lazer", valueobjects.KindExpense, &rootID2),
	}

	entries := []entities.DictionaryEntry{
		s.newEntry(subID1, valueobjects.KindExpense, "uber", valueobjects.SignalTypeMerchant, valueobjects.ConfidenceMedium, true),
		s.newEntry(subID2, valueobjects.KindExpense, "uber", valueobjects.SignalTypeMerchant, valueobjects.ConfidenceMedium, true),
	}

	candidates, hasMore := resolver.Resolve(entries, categories)

	s.Len(candidates, 2)
	s.False(hasMore)

	for _, c := range candidates {
		s.True(c.IsAmbiguous, "todos candidatos devem ser ambiguos quando >1")
		s.Equal(valueobjects.SignalTypeMerchant, c.SignalType)
	}
}

func (s *CandidateResolverSuite) TestResolveNoMatch() {
	resolver := services.NewCandidateResolver()

	candidates, hasMore := resolver.Resolve([]entities.DictionaryEntry{}, nil)

	s.Nil(candidates)
	s.False(hasMore)
}

func (s *CandidateResolverSuite) TestResolveTieHighConfidence() {
	resolver := services.NewCandidateResolver()

	rootID1 := uuid.New()
	rootID2 := uuid.New()
	subID1 := uuid.New()
	subID2 := uuid.New()

	categories := map[uuid.UUID]entities.Category{
		rootID1: s.newCategory(rootID1, "Root A", valueobjects.KindIncome, nil),
		rootID2: s.newCategory(rootID2, "Root B", valueobjects.KindIncome, nil),
		subID1:  s.newCategory(subID1, "Sub A", valueobjects.KindIncome, &rootID1),
		subID2:  s.newCategory(subID2, "Sub B", valueobjects.KindIncome, &rootID2),
	}

	entries := []entities.DictionaryEntry{
		s.newEntry(subID1, valueobjects.KindIncome, "termo", valueobjects.SignalTypeCanonicalName, valueobjects.ConfidenceHigh, false),
		s.newEntry(subID2, valueobjects.KindIncome, "termo", valueobjects.SignalTypeCanonicalName, valueobjects.ConfidenceHigh, false),
	}

	candidates, hasMore := resolver.Resolve(entries, categories)

	s.Len(candidates, 2)
	s.False(hasMore)

	for _, c := range candidates {
		s.True(c.IsAmbiguous, "todos candidatos devem ser ambiguos em empate de alta confianca")
	}
}

func (s *CandidateResolverSuite) TestResolveDeduplicationByPrecedence() {
	resolver := services.NewCandidateResolver()

	rootID := uuid.New()
	subID := uuid.New()

	categories := map[uuid.UUID]entities.Category{
		rootID: s.newCategory(rootID, "Custo Fixo", valueobjects.KindExpense, nil),
		subID:  s.newCategory(subID, "Aluguel", valueobjects.KindExpense, &rootID),
	}

	entries := []entities.DictionaryEntry{
		s.newEntry(subID, valueobjects.KindExpense, "aluguel", valueobjects.SignalTypeCanonicalName, valueobjects.ConfidenceHigh, false),
		s.newEntry(subID, valueobjects.KindExpense, "locacao", valueobjects.SignalTypeAlias, valueobjects.ConfidenceHigh, false),
		s.newEntry(subID, valueobjects.KindExpense, "alug", valueobjects.SignalTypeAlias, valueobjects.ConfidenceHigh, false),
	}

	candidates, hasMore := resolver.Resolve(entries, categories)

	s.Len(candidates, 1)
	s.False(hasMore)
	s.Equal(valueobjects.SignalTypeCanonicalName, candidates[0].SignalType)
	s.Equal("aluguel", candidates[0].MatchedTerm)
	s.False(candidates[0].IsAmbiguous)
}

func (s *CandidateResolverSuite) TestResolveHasMoreWhenMoreThan3() {
	resolver := services.NewCandidateResolver()

	rootID := uuid.New()
	subIDs := make([]uuid.UUID, 5)
	categories := map[uuid.UUID]entities.Category{
		rootID: s.newCategory(rootID, "Root", valueobjects.KindExpense, nil),
	}

	entries := make([]entities.DictionaryEntry, 5)
	for i := 0; i < 5; i++ {
		subIDs[i] = uuid.New()
		categories[subIDs[i]] = s.newCategory(subIDs[i], "Sub "+string(rune('A'+i)), valueobjects.KindExpense, &rootID)
		entries[i] = s.newEntry(subIDs[i], valueobjects.KindExpense, "termo", valueobjects.SignalTypeAlias, valueobjects.ConfidenceHigh, false)
	}

	candidates, hasMore := resolver.Resolve(entries, categories)

	s.Len(candidates, 3)
	s.True(hasMore)

	for _, c := range candidates {
		s.True(c.IsAmbiguous)
	}
}

func (s *CandidateResolverSuite) TestResolveOrderingByPath() {
	resolver := services.NewCandidateResolver()

	rootID := uuid.New()
	subID1 := uuid.New()
	subID2 := uuid.New()

	categories := map[uuid.UUID]entities.Category{
		rootID: s.newCategory(rootID, "Root", valueobjects.KindExpense, nil),
		subID1: s.newCategory(subID1, "Aluguel", valueobjects.KindExpense, &rootID),
		subID2: s.newCategory(subID2, "Condominio", valueobjects.KindExpense, &rootID),
	}

	entries := []entities.DictionaryEntry{
		s.newEntry(subID2, valueobjects.KindExpense, "termo", valueobjects.SignalTypeAlias, valueobjects.ConfidenceHigh, false),
		s.newEntry(subID1, valueobjects.KindExpense, "termo", valueobjects.SignalTypeAlias, valueobjects.ConfidenceHigh, false),
	}

	candidates, _ := resolver.Resolve(entries, categories)

	s.Len(candidates, 2)
	s.Equal("Root > Aluguel", candidates[0].Path)
	s.Equal("Root > Condominio", candidates[1].Path)
}

func (s *CandidateResolverSuite) TestResolveOrderingBySignalType() {
	resolver := services.NewCandidateResolver()

	rootID := uuid.New()
	subID1 := uuid.New()
	subID2 := uuid.New()

	categories := map[uuid.UUID]entities.Category{
		rootID: s.newCategory(rootID, "Root", valueobjects.KindExpense, nil),
		subID1: s.newCategory(subID1, "Sub A", valueobjects.KindExpense, &rootID),
		subID2: s.newCategory(subID2, "Sub B", valueobjects.KindExpense, &rootID),
	}

	entries := []entities.DictionaryEntry{
		s.newEntry(subID2, valueobjects.KindExpense, "termo phrase", valueobjects.SignalTypePhrase, valueobjects.ConfidenceHigh, false),
		s.newEntry(subID1, valueobjects.KindExpense, "termo alias", valueobjects.SignalTypeAlias, valueobjects.ConfidenceHigh, false),
	}

	candidates, _ := resolver.Resolve(entries, categories)

	s.Len(candidates, 2)
	s.Equal(valueobjects.SignalTypeAlias, candidates[0].SignalType)
	s.Equal(valueobjects.SignalTypePhrase, candidates[1].SignalType)
}

func (s *CandidateResolverSuite) TestResolveMatchReasons() {
	resolver := services.NewCandidateResolver()

	rootID := uuid.New()
	subID := uuid.New()

	categories := map[uuid.UUID]entities.Category{
		rootID: s.newCategory(rootID, "Root", valueobjects.KindExpense, nil),
		subID:  s.newCategory(subID, "Sub", valueobjects.KindExpense, &rootID),
	}

	scenarios := []struct {
		signalType valueobjects.SignalType
		expected   string
	}{
		{signalType: valueobjects.SignalTypeCanonicalName, expected: "canonical name"},
		{signalType: valueobjects.SignalTypeAlias, expected: "alias inequivoco"},
		{signalType: valueobjects.SignalTypePhrase, expected: "phrase match"},
		{signalType: valueobjects.SignalTypeMerchant, expected: "merchant match"},
		{signalType: valueobjects.SignalTypeSegment, expected: "segment match"},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.signalType.String(), func() {
			entries := []entities.DictionaryEntry{
				s.newEntry(subID, valueobjects.KindExpense, "termo", scenario.signalType, valueobjects.ConfidenceHigh, false),
			}

			candidates, _ := resolver.Resolve(entries, categories)

			s.Len(candidates, 1)
			s.Equal(scenario.expected, candidates[0].MatchReason)
		})
	}
}
