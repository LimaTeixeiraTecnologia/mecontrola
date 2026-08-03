package interfaces_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
)

type DiscriminatorsSuite struct {
	suite.Suite
}

func TestDiscriminatorsSuite(t *testing.T) {
	suite.Run(t, new(DiscriminatorsSuite))
}

func (s *DiscriminatorsSuite) TestParseEntryKind() {
	scenarios := []struct {
		name    string
		input   string
		want    interfaces.EntryKind
		wantErr bool
	}{
		{name: "transaction", input: "transaction", want: interfaces.EntryKindTransaction},
		{name: "recurring_template", input: "recurring_template", want: interfaces.EntryKindRecurringTemplate},
		{name: "card", input: "card", want: interfaces.EntryKindCard},
		{name: "card_invoice_item", input: "card_invoice_item", want: interfaces.EntryKindCardInvoiceItem},
		{name: "invalido", input: "algo_invalido", wantErr: true},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			got, err := interfaces.ParseEntryKind(scenario.input)
			if scenario.wantErr {
				s.Error(err)
				s.ErrorIs(err, interfaces.ErrInvalidEntryKind)
				return
			}
			s.NoError(err)
			s.Equal(scenario.want, got)
		})
	}
}

func (s *DiscriminatorsSuite) TestEntryKindStringRoundTrip() {
	kinds := []interfaces.EntryKind{
		interfaces.EntryKindTransaction,
		interfaces.EntryKindRecurringTemplate,
		interfaces.EntryKindCard,
		interfaces.EntryKindCardInvoiceItem,
	}

	for _, kind := range kinds {
		s.Run(kind.String(), func() {
			s.True(kind.IsValid())
			parsed, err := interfaces.ParseEntryKind(kind.String())
			s.NoError(err)
			s.Equal(kind, parsed)
		})
	}
}

func (s *DiscriminatorsSuite) TestEntryKindInvalidZeroValue() {
	var zero interfaces.EntryKind
	s.False(zero.IsValid())
	s.Equal("", zero.String())
}
