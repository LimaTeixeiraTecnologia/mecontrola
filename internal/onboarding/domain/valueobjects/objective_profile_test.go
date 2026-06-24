package valueobjects

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type ObjectiveProfileSuite struct {
	suite.Suite
}

func TestObjectiveProfileSuite(t *testing.T) {
	suite.Run(t, new(ObjectiveProfileSuite))
}

func (s *ObjectiveProfileSuite) TestParseObjectiveProfile_KnownValues() {
	cases := []struct {
		raw      string
		expected ObjectiveProfile
	}{
		{"organize_spending", ProfileOrganizeSpending},
		{"payoff_debt", ProfilePayoffDebt},
		{"emergency_fund", ProfileEmergencyFund},
		{"invest", ProfileInvest},
		{"specific_goal", ProfileSpecificGoal},
	}

	for _, c := range cases {
		got, ok := ParseObjectiveProfile(c.raw)
		s.True(ok, "expected ok=true for %q", c.raw)
		s.Equal(c.expected, got)
	}
}

func (s *ObjectiveProfileSuite) TestParseObjectiveProfile_UnknownReturnsNotOk() {
	cases := []string{"", "unknown", "random", "débito", "organizar"}

	for _, raw := range cases {
		_, ok := ParseObjectiveProfile(raw)
		s.False(ok, "expected ok=false for %q", raw)
	}
}

func (s *ObjectiveProfileSuite) TestParseObjectiveProfile_CaseInsensitiveAndTrimmed() {
	got, ok := ParseObjectiveProfile("  ORGANIZE_SPENDING  ")
	s.True(ok)
	s.Equal(ProfileOrganizeSpending, got)
}

func (s *ObjectiveProfileSuite) TestObjectiveProfileString() {
	cases := []struct {
		profile  ObjectiveProfile
		expected string
	}{
		{ProfileOrganizeSpending, "organize_spending"},
		{ProfilePayoffDebt, "payoff_debt"},
		{ProfileEmergencyFund, "emergency_fund"},
		{ProfileInvest, "invest"},
		{ProfileSpecificGoal, "specific_goal"},
		{ObjectiveProfile(99), "unknown"},
	}

	for _, c := range cases {
		s.Equal(c.expected, c.profile.String())
	}
}

func (s *ObjectiveProfileSuite) TestclassifyByKeyword_DebtProfile() {
	cases := []string{
		"quitar dívidas",
		"pagar divida do cartão",
		"tenho crédito para quitar",
		"empréstimo pessoal",
	}
	for _, text := range cases {
		got, ok := classifyByKeyword(text)
		s.True(ok, "expected match for %q", text)
		s.Equal(ProfilePayoffDebt, got, "expected PayoffDebt for %q", text)
	}
}

func (s *ObjectiveProfileSuite) TestclassifyByKeyword_EmergencyFundProfile() {
	cases := []string{
		"montar reserva de emergência",
		"criar fundo de emergencia",
		"segurança financeira",
	}
	for _, text := range cases {
		got, ok := classifyByKeyword(text)
		s.True(ok, "expected match for %q", text)
		s.Equal(ProfileEmergencyFund, got, "expected EmergencyFund for %q", text)
	}
}

func (s *ObjectiveProfileSuite) TestclassifyByKeyword_InvestProfile() {
	cases := []string{
		"quero investir meu dinheiro",
		"construir patrimônio",
		"independência financeira",
		"aposentadoria tranquila",
	}
	for _, text := range cases {
		got, ok := classifyByKeyword(text)
		s.True(ok, "expected match for %q", text)
		s.Equal(ProfileInvest, got, "expected Invest for %q", text)
	}
}

func (s *ObjectiveProfileSuite) TestclassifyByKeyword_SpecificGoalProfile() {
	cases := []string{
		"juntar para uma viagem",
		"comprar uma casa",
		"casamento no fim do ano",
		"poupar para o carro",
	}
	for _, text := range cases {
		got, ok := classifyByKeyword(text)
		s.True(ok, "expected match for %q", text)
		s.Equal(ProfileSpecificGoal, got, "expected SpecificGoal for %q", text)
	}
}

func (s *ObjectiveProfileSuite) TestclassifyByKeyword_AmbiguousReturnsFalse() {
	cases := []string{
		"organizar minhas finanças",
		"controlar gastos",
		"",
		"não sei bem",
	}
	for _, text := range cases {
		_, ok := classifyByKeyword(text)
		s.False(ok, "expected ok=false (no match) for %q", text)
	}
}

func (s *ObjectiveProfileSuite) TestSplitTemplate_SumsTo10000ForAllProfiles() {
	profiles := []ObjectiveProfile{
		ProfileOrganizeSpending,
		ProfilePayoffDebt,
		ProfileEmergencyFund,
		ProfileInvest,
		ProfileSpecificGoal,
	}

	for _, p := range profiles {
		entries := SplitTemplate(p)
		total := 0
		for _, e := range entries {
			total += e.BasisPoints
		}
		s.Equal(10000, total, "SplitTemplate(%s) basis points sum must be 10000, got %d", p, total)
	}
}

func (s *ObjectiveProfileSuite) TestSplitTemplate_PayoffDebt_Values() {
	entries := SplitTemplate(ProfilePayoffDebt)
	s.Require().Len(entries, 5)
	s.Equal(4500, entries[0].BasisPoints)
	s.Equal(500, entries[1].BasisPoints)
	s.Equal(1000, entries[2].BasisPoints)
	s.Equal(2500, entries[3].BasisPoints)
	s.Equal(1500, entries[4].BasisPoints)
}

func (s *ObjectiveProfileSuite) TestSplitTemplate_EmergencyFund_Values() {
	entries := SplitTemplate(ProfileEmergencyFund)
	s.Require().Len(entries, 5)
	s.Equal(4000, entries[0].BasisPoints)
	s.Equal(500, entries[1].BasisPoints)
	s.Equal(1000, entries[2].BasisPoints)
	s.Equal(1500, entries[3].BasisPoints)
	s.Equal(3000, entries[4].BasisPoints)
}

func (s *ObjectiveProfileSuite) TestSplitTemplate_Invest_Values() {
	entries := SplitTemplate(ProfileInvest)
	s.Require().Len(entries, 5)
	s.Equal(4000, entries[0].BasisPoints)
	s.Equal(1000, entries[1].BasisPoints)
	s.Equal(1000, entries[2].BasisPoints)
	s.Equal(1000, entries[3].BasisPoints)
	s.Equal(3000, entries[4].BasisPoints)
}

func (s *ObjectiveProfileSuite) TestSplitTemplate_SpecificGoal_Values() {
	entries := SplitTemplate(ProfileSpecificGoal)
	s.Require().Len(entries, 5)
	s.Equal(4000, entries[0].BasisPoints)
	s.Equal(500, entries[1].BasisPoints)
	s.Equal(1000, entries[2].BasisPoints)
	s.Equal(3000, entries[3].BasisPoints)
	s.Equal(1500, entries[4].BasisPoints)
}

func (s *ObjectiveProfileSuite) TestSplitTemplate_OrganizeSpending_Values() {
	entries := SplitTemplate(ProfileOrganizeSpending)
	s.Require().Len(entries, 5)
	s.Equal(4000, entries[0].BasisPoints)
	s.Equal(1000, entries[1].BasisPoints)
	s.Equal(1500, entries[2].BasisPoints)
	s.Equal(2000, entries[3].BasisPoints)
	s.Equal(1500, entries[4].BasisPoints)
}

func (s *ObjectiveProfileSuite) TestSplitTemplate_UnknownProfileUsesDefaultOrganizeSpending() {
	entries := SplitTemplate(ObjectiveProfile(99))
	total := 0
	for _, e := range entries {
		total += e.BasisPoints
	}
	s.Equal(10000, total)
}

func (s *ObjectiveProfileSuite) TestSplitTemplate_HasFiveEntries() {
	profiles := []ObjectiveProfile{
		ProfileOrganizeSpending,
		ProfilePayoffDebt,
		ProfileEmergencyFund,
		ProfileInvest,
		ProfileSpecificGoal,
	}

	for _, p := range profiles {
		entries := SplitTemplate(p)
		s.Len(entries, 5, "expected 5 entries for profile %s", p)
	}
}

func (s *ObjectiveProfileSuite) TestSplitTemplate_NoCentsMathPresent() {
	for _, p := range []ObjectiveProfile{ProfilePayoffDebt, ProfileEmergencyFund, ProfileInvest, ProfileSpecificGoal, ProfileOrganizeSpending} {
		entries := SplitTemplate(p)
		for _, e := range entries {
			s.NotEmpty(e.RootSlug)
			s.Greater(e.BasisPoints, 0)
		}
	}
}
