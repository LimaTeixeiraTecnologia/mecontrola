package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

type TemplateRow struct {
	Kind       string
	Template   string
	Category   string
	MetaStatus string
}

type Readiness struct {
	Kind          string
	Template      string
	Category      string
	MetaStatus    string
	Allowlisted   bool
	ReleaseOne    bool
	RequiresOptIn bool
	ConsentSource bool
	Deliverable   bool
	Reason        string
}

var releaseOneKinds = map[string]struct{}{
	"category_threshold_80":      {},
	"category_threshold_100":     {},
	"budget_missing_month_start": {},
	"budget_not_reviewed_day_3":  {},
}

var tableRowPattern = regexp.MustCompile(`^\|\s*` + "`" + `([a-z0-9_]+)` + "`" + `\s*\|\s*` + "`" + `([a-z0-9_]+)` + "`" + `\s*\|[^|]*\|\s*` + "`" + `([A-Z]+)` + "`" + `\s*\|[^|]*\|\s*` + "`" + `([A-Z]+)` + "`" + `\s*\|`)

func ParseTemplateRows(doc string) []TemplateRow {
	var rows []TemplateRow
	for _, line := range strings.Split(doc, "\n") {
		match := tableRowPattern.FindStringSubmatch(strings.TrimSpace(line))
		if match == nil {
			continue
		}
		rows = append(rows, TemplateRow{
			Kind:       match[1],
			Template:   match[2],
			Category:   match[3],
			MetaStatus: match[4],
		})
	}
	return rows
}

func DecideReadiness(row TemplateRow, allowlist map[string]struct{}, consentSource bool) Readiness {
	_, allowlisted := allowlist[row.Kind]
	_, releaseOne := releaseOneKinds[row.Kind]
	requiresOptIn := row.Category == "MARKETING"

	result := Readiness{
		Kind:          row.Kind,
		Template:      row.Template,
		Category:      row.Category,
		MetaStatus:    row.MetaStatus,
		Allowlisted:   allowlisted,
		ReleaseOne:    releaseOne,
		RequiresOptIn: requiresOptIn,
		ConsentSource: consentSource,
	}

	switch {
	case !releaseOne:
		result.Reason = "bloqueado: kind fora do Release 1"
	case row.MetaStatus != "APPROVED":
		result.Reason = fmt.Sprintf("bloqueado: template Meta em %s", row.MetaStatus)
	case !allowlisted:
		result.Reason = "bloqueado: kind ausente de BUDGETS_THRESHOLD_TEMPLATES_APPROVED_KINDS"
	case requiresOptIn && !consentSource:
		result.Reason = "bloqueado: template MARKETING sem fonte de consentimento"
	default:
		result.Deliverable = true
		result.Reason = "liberado"
	}

	return result
}

func DecideAll(rows []TemplateRow, allowlist map[string]struct{}, consentSource bool) []Readiness {
	out := make([]Readiness, 0, len(rows))
	for _, row := range rows {
		out = append(out, DecideReadiness(row, allowlist, consentSource))
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Kind < out[j].Kind })
	return out
}

func Inconsistencies(report []Readiness) []string {
	var problems []string
	for _, entry := range report {
		if !entry.Allowlisted {
			continue
		}
		if !entry.ReleaseOne {
			problems = append(problems, fmt.Sprintf("%s: allowlisted mas fora do Release 1", entry.Kind))
		}
		if entry.MetaStatus != "APPROVED" {
			problems = append(problems, fmt.Sprintf("%s: allowlisted mas template Meta em %s", entry.Kind, entry.MetaStatus))
		}
		if entry.RequiresOptIn && !entry.ConsentSource {
			problems = append(problems, fmt.Sprintf("%s: allowlisted e MARKETING, mas nao ha fonte de consentimento", entry.Kind))
		}
	}
	return problems
}

func ParseAllowlist(raw string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, part := range strings.Split(raw, ",") {
		kind := strings.TrimSpace(part)
		if kind == "" {
			continue
		}
		out[kind] = struct{}{}
	}
	return out
}
