package entities_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

type DictionaryEntrySuite struct {
	suite.Suite
}

func TestDictionaryEntrySuite(t *testing.T) {
	suite.Run(t, new(DictionaryEntrySuite))
}

func (s *DictionaryEntrySuite) TestDictionaryEntryFields() {
	id := uuid.New()
	categoryID := uuid.New()
	now := time.Now()

	entry := entities.DictionaryEntry{
		ID:           id,
		CategoryID:   categoryID,
		Kind:         valueobjects.KindExpense,
		Term:         "aluguel",
		SignalType:   valueobjects.SignalTypeCanonicalName,
		Confidence:   valueobjects.ConfidenceHigh,
		IsAmbiguous:  false,
		DeprecatedAt: &now,
	}

	s.Equal(id, entry.ID)
	s.Equal(categoryID, entry.CategoryID)
	s.Equal(valueobjects.KindExpense, entry.Kind)
	s.Equal("aluguel", entry.Term)
	s.Equal(valueobjects.SignalTypeCanonicalName, entry.SignalType)
	s.Equal(valueobjects.ConfidenceHigh, entry.Confidence)
	s.False(entry.IsAmbiguous)
	s.Equal(&now, entry.DeprecatedAt)
}

func (s *DictionaryEntrySuite) TestDictionaryEntryAmbiguous() {
	scenarios := []struct {
		name        string
		isAmbiguous bool
	}{
		{name: "entrada nao ambigua", isAmbiguous: false},
		{name: "entrada ambigua", isAmbiguous: true},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			entry := entities.DictionaryEntry{
				ID:          uuid.New(),
				CategoryID:  uuid.New(),
				Kind:        valueobjects.KindExpense,
				Term:        "test",
				SignalType:  valueobjects.SignalTypeMerchant,
				Confidence:  valueobjects.ConfidenceMedium,
				IsAmbiguous: scenario.isAmbiguous,
			}

			s.Equal(scenario.isAmbiguous, entry.IsAmbiguous)
		})
	}
}
