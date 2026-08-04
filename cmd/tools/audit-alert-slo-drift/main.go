package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type ruleCondition struct {
	Evaluator struct {
		Type   string    `yaml:"type"`
		Params []float64 `yaml:"params"`
	} `yaml:"evaluator"`
}

type ruleData struct {
	RefID string `yaml:"refId"`
	Model struct {
		RefID      string          `yaml:"refId"`
		Expr       string          `yaml:"expr"`
		Conditions []ruleCondition `yaml:"conditions"`
	} `yaml:"model"`
}

type rule struct {
	UID    string            `yaml:"uid"`
	Title  string            `yaml:"title"`
	For    string            `yaml:"for"`
	Labels map[string]string `yaml:"labels"`
	Data   []ruleData        `yaml:"data"`
}

type rulesDoc struct {
	Groups []struct {
		Rules []rule `yaml:"rules"`
	} `yaml:"groups"`
}

type driftKind string

const (
	driftMissingUID driftKind = "uid_missing_in_source"
	driftFor        driftKind = "for_mismatch"
	driftLabels     driftKind = "labels_mismatch"
	driftExpr       driftKind = "expr_mismatch"
	driftThreshold  driftKind = "threshold_mismatch"
)

type drift struct {
	UID     string
	Kind    driftKind
	RefID   string
	Details string
}

func loadRules(path string) (map[string]rule, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("audit-alert-slo-drift: read %s: %w", path, err)
	}

	var doc rulesDoc
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("audit-alert-slo-drift: parse %s: %w", path, err)
	}

	out := make(map[string]rule)
	for _, group := range doc.Groups {
		for _, r := range group.Rules {
			out[r.UID] = r
		}
	}
	return out, nil
}

func labelsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func thresholdsByRefID(data []ruleData) map[string][]float64 {
	out := make(map[string][]float64, len(data))
	for _, d := range data {
		params := make([]float64, 0)
		for _, cond := range d.Model.Conditions {
			params = append(params, cond.Evaluator.Params...)
		}
		if len(params) > 0 {
			out[d.RefID] = params
		}
	}
	return out
}

func exprsByRefID(data []ruleData) map[string]string {
	out := make(map[string]string, len(data))
	for _, d := range data {
		if strings.TrimSpace(d.Model.Expr) != "" {
			out[d.RefID] = d.Model.Expr
		}
	}
	return out
}

func float64SliceEqual(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func reconcileRule(docRule, sourceRule rule) []drift {
	out := make([]drift, 0)

	if docRule.For != sourceRule.For {
		out = append(out, drift{
			UID:     docRule.UID,
			Kind:    driftFor,
			Details: fmt.Sprintf("doc=%q source=%q", docRule.For, sourceRule.For),
		})
	}

	if !labelsEqual(docRule.Labels, sourceRule.Labels) {
		out = append(out, drift{
			UID:     docRule.UID,
			Kind:    driftLabels,
			Details: fmt.Sprintf("doc=%v source=%v", docRule.Labels, sourceRule.Labels),
		})
	}

	docExprs := exprsByRefID(docRule.Data)
	sourceExprs := exprsByRefID(sourceRule.Data)
	for refID, docExpr := range docExprs {
		sourceExpr, ok := sourceExprs[refID]
		if !ok || docExpr != sourceExpr {
			out = append(out, drift{
				UID:     docRule.UID,
				Kind:    driftExpr,
				RefID:   refID,
				Details: fmt.Sprintf("doc=%q source=%q", docExpr, sourceExpr),
			})
		}
	}

	docThresholds := thresholdsByRefID(docRule.Data)
	sourceThresholds := thresholdsByRefID(sourceRule.Data)
	for refID, docParams := range docThresholds {
		sourceParams, ok := sourceThresholds[refID]
		if !ok || !float64SliceEqual(docParams, sourceParams) {
			out = append(out, drift{
				UID:     docRule.UID,
				Kind:    driftThreshold,
				RefID:   refID,
				Details: fmt.Sprintf("doc=%v source=%v", docParams, sourceParams),
			})
		}
	}

	return out
}

func reconcile(docRules, sourceRules map[string]rule) []drift {
	uids := make([]string, 0, len(docRules))
	for uid := range docRules {
		uids = append(uids, uid)
	}
	sort.Strings(uids)

	out := make([]drift, 0)
	for _, uid := range uids {
		sourceRule, ok := sourceRules[uid]
		if !ok {
			out = append(out, drift{UID: uid, Kind: driftMissingUID})
			continue
		}
		out = append(out, reconcileRule(docRules[uid], sourceRule)...)
	}
	return out
}

func audit(docPath, sourcePath string) ([]drift, error) {
	docRules, err := loadRules(docPath)
	if err != nil {
		return nil, err
	}
	sourceRules, err := loadRules(sourcePath)
	if err != nil {
		return nil, err
	}
	return reconcile(docRules, sourceRules), nil
}

func main() {
	docPath := flag.String(
		"doc",
		"observability/alert-rules-slo.yaml",
		"caminho do extrato documental de alertas SLO/RED/runtime",
	)
	sourcePath := flag.String(
		"rules",
		"deployment/telemetry/grafana/provisioning/alerting/rules.yaml",
		"caminho do rules.yaml provisionado do Grafana (fonte de verdade)",
	)
	flag.Parse()

	drifts, err := audit(*docPath, *sourcePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if len(drifts) == 0 {
		fmt.Println("audit-alert-slo-drift: OK - asset documental reconciliado com a fonte de verdade")
		return
	}

	fmt.Fprintln(os.Stderr, "audit-alert-slo-drift: FAIL - divergencia entre asset documental e fonte de verdade:")
	for _, d := range drifts {
		if d.RefID != "" {
			fmt.Fprintf(os.Stderr, "  uid=%s kind=%s refId=%s %s\n", d.UID, d.Kind, d.RefID, d.Details)
			continue
		}
		fmt.Fprintf(os.Stderr, "  uid=%s kind=%s %s\n", d.UID, d.Kind, d.Details)
	}
	os.Exit(1)
}
