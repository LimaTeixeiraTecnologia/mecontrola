package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
)

type ReadinessSuite struct {
	suite.Suite
}

func TestReadinessSuite(t *testing.T) {
	suite.Run(t, new(ReadinessSuite))
}

func (s *ReadinessSuite) TestParsesRealSpecDocument() {
	raw, err := os.ReadFile("../../../.specs/prd-alertas-proativos/meta-templates-status.md")
	s.Require().NoError(err)

	rows := ParseTemplateRows(string(raw))
	s.Require().NotEmpty(rows)

	byKind := map[string]TemplateRow{}
	for _, row := range rows {
		byKind[row.Kind] = row
	}

	for _, kind := range []string{
		"category_threshold_80",
		"category_threshold_100",
		"category_threshold_90",
		"budget_missing_month_start",
		"budget_not_reviewed_day_3",
		"weekly_motivation",
		"usage_reactivation_3d",
		"abandonment_risk_7d",
	} {
		_, ok := byKind[kind]
		s.True(ok, "kind %s ausente do quadro", kind)
	}

	s.Equal("MARKETING", byKind["budget_not_reviewed_day_3"].Category)
	s.Equal("UTILITY", byKind["category_threshold_80"].Category)
	s.Equal("PENDING", byKind["abandonment_risk_7d"].MetaStatus)
}

func (s *ReadinessSuite) TestDecideReadiness() {
	scenarios := []struct {
		name            string
		row             TemplateRow
		allowlist       string
		consentSource   bool
		expectDelivery  bool
		expectReasonHas string
	}{
		{
			name:            "utility aprovado e allowlisted e entregavel",
			row:             TemplateRow{Kind: "category_threshold_80", Category: "UTILITY", MetaStatus: "APPROVED"},
			allowlist:       "category_threshold_80",
			expectDelivery:  true,
			expectReasonHas: "liberado",
		},
		{
			name:            "kind fora do release 1 e bloqueado",
			row:             TemplateRow{Kind: "weekly_motivation", Category: "MARKETING", MetaStatus: "APPROVED"},
			allowlist:       "weekly_motivation",
			consentSource:   true,
			expectDelivery:  false,
			expectReasonHas: "fora do Release 1",
		},
		{
			name:            "threshold 90 permanece bloqueado mesmo aprovado na Meta",
			row:             TemplateRow{Kind: "category_threshold_90", Category: "UTILITY", MetaStatus: "APPROVED"},
			allowlist:       "category_threshold_90",
			expectDelivery:  false,
			expectReasonHas: "fora do Release 1",
		},
		{
			name:            "template pendente bloqueia",
			row:             TemplateRow{Kind: "category_threshold_80", Category: "UTILITY", MetaStatus: "PENDING"},
			allowlist:       "category_threshold_80",
			expectDelivery:  false,
			expectReasonHas: "PENDING",
		},
		{
			name:            "sem allowlist bloqueia",
			row:             TemplateRow{Kind: "category_threshold_100", Category: "UTILITY", MetaStatus: "APPROVED"},
			allowlist:       "",
			expectDelivery:  false,
			expectReasonHas: "ausente de BUDGETS_THRESHOLD_TEMPLATES_APPROVED_KINDS",
		},
		{
			name:            "marketing sem fonte de consentimento bloqueia",
			row:             TemplateRow{Kind: "budget_not_reviewed_day_3", Category: "MARKETING", MetaStatus: "APPROVED"},
			allowlist:       "budget_not_reviewed_day_3",
			consentSource:   false,
			expectDelivery:  false,
			expectReasonHas: "sem fonte de consentimento",
		},
		{
			name:            "marketing com fonte de consentimento libera",
			row:             TemplateRow{Kind: "budget_not_reviewed_day_3", Category: "MARKETING", MetaStatus: "APPROVED"},
			allowlist:       "budget_not_reviewed_day_3",
			consentSource:   true,
			expectDelivery:  true,
			expectReasonHas: "liberado",
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			result := DecideReadiness(scenario.row, ParseAllowlist(scenario.allowlist), scenario.consentSource)
			s.Equal(scenario.expectDelivery, result.Deliverable)
			s.Contains(result.Reason, scenario.expectReasonHas)
		})
	}
}

func (s *ReadinessSuite) TestDefaultConfigurationDeliversNothing() {
	raw, err := os.ReadFile("../../../.specs/prd-alertas-proativos/meta-templates-status.md")
	s.Require().NoError(err)

	report := DecideAll(ParseTemplateRows(string(raw)), ParseAllowlist(""), false)
	s.Require().NotEmpty(report)
	for _, entry := range report {
		s.False(entry.Deliverable, "kind %s entregavel com allowlist vazia", entry.Kind)
	}
	s.Empty(Inconsistencies(report))
}

func (s *ReadinessSuite) TestInconsistenciesFlagMisconfiguration() {
	rows := []TemplateRow{
		{Kind: "weekly_motivation", Category: "MARKETING", MetaStatus: "APPROVED"},
		{Kind: "abandonment_risk_7d", Category: "MARKETING", MetaStatus: "PENDING"},
		{Kind: "budget_not_reviewed_day_3", Category: "MARKETING", MetaStatus: "APPROVED"},
		{Kind: "category_threshold_80", Category: "UTILITY", MetaStatus: "APPROVED"},
	}
	allowlist := ParseAllowlist("weekly_motivation,abandonment_risk_7d,budget_not_reviewed_day_3,category_threshold_80")

	problems := Inconsistencies(DecideAll(rows, allowlist, false))

	s.Require().NotEmpty(problems)
	joined := ""
	for _, problem := range problems {
		joined += problem + "\n"
	}
	s.Contains(joined, "weekly_motivation: allowlisted mas fora do Release 1")
	s.Contains(joined, "abandonment_risk_7d: allowlisted mas fora do Release 1")
	s.Contains(joined, "budget_not_reviewed_day_3: allowlisted e MARKETING, mas nao ha fonte de consentimento")
	s.NotContains(joined, "category_threshold_80:")
}

func (s *ReadinessSuite) TestParseAllowlistTrimsAndIgnoresEmpty() {
	allowlist := ParseAllowlist(" category_threshold_80 , ,category_threshold_100,")
	s.Len(allowlist, 2)
	_, ok := allowlist["category_threshold_80"]
	s.True(ok)
	_, ok = allowlist["category_threshold_100"]
	s.True(ok)
}
