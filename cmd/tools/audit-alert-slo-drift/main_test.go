package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFixture(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

const fixtureSourceRules = `
apiVersion: 1
groups:
  - orgId: 1
    name: slo
    rules:
      - uid: mc-slo-latency-p95
        title: Latencia P95 acima do SLO
        for: 10m
        labels: { severity: warning }
        data:
          - refId: A
            datasourceUid: prometheus
            model:
              refId: A
              expr: 'histogram_quantile(0.95, sum by (le) (rate(http_server_request_duration_seconds_bucket{job="mecontrola-api"}[5m])))'
          - refId: C
            datasourceUid: __expr__
            model:
              refId: C
              type: threshold
              conditions:
                - type: query
                  evaluator: { type: gt, params: [0.5] }
      - uid: mc-runtime-goroutine-growth
        title: Crescimento sustentado de goroutines
        for: 15m
        labels: { severity: warning }
        data:
          - refId: A
            datasourceUid: prometheus
            model:
              refId: A
              expr: 'max(go_goroutine_count{job=~"mecontrola-.+"})'
          - refId: C
            datasourceUid: __expr__
            model:
              refId: C
              type: threshold
              conditions:
                - type: query
                  evaluator: { type: gt, params: [3000] }
`

func fixtureDocRulesMatching() string {
	return fixtureSourceRules
}

func TestReconcilePassesOnMatchingRules(t *testing.T) {
	dir := t.TempDir()
	sourcePath := writeFixture(t, dir, "rules.yaml", fixtureSourceRules)
	docPath := writeFixture(t, dir, "alert-rules-slo.yaml", fixtureDocRulesMatching())

	drifts, err := audit(docPath, sourcePath)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(drifts) != 0 {
		t.Fatalf("expected 0 drifts, got %d: %+v", len(drifts), drifts)
	}
}

func TestReconcileDetectsMissingUID(t *testing.T) {
	dir := t.TempDir()
	sourcePath := writeFixture(t, dir, "rules.yaml", fixtureSourceRules)

	docYAML := `
apiVersion: 1
groups:
  - orgId: 1
    name: slo
    rules:
      - uid: mc-slo-latency-p95-renamed
        title: Renamed
        for: 10m
        labels: { severity: warning }
        data:
          - refId: A
            datasourceUid: prometheus
            model:
              refId: A
              expr: 'histogram_quantile(0.95, sum by (le) (rate(http_server_request_duration_seconds_bucket{job="mecontrola-api"}[5m])))'
`
	docPath := writeFixture(t, dir, "alert-rules-slo.yaml", docYAML)

	drifts, err := audit(docPath, sourcePath)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(drifts) != 1 {
		t.Fatalf("expected 1 drift, got %d: %+v", len(drifts), drifts)
	}
	if drifts[0].Kind != driftMissingUID {
		t.Fatalf("expected driftMissingUID, got %s", drifts[0].Kind)
	}
}

func TestReconcileDetectsExprDrift(t *testing.T) {
	dir := t.TempDir()
	sourcePath := writeFixture(t, dir, "rules.yaml", fixtureSourceRules)

	docYAML := `
apiVersion: 1
groups:
  - orgId: 1
    name: runtime
    rules:
      - uid: mc-runtime-goroutine-growth
        title: Crescimento sustentado de goroutines
        for: 15m
        labels: { severity: warning }
        data:
          - refId: A
            datasourceUid: prometheus
            model:
              refId: A
              expr: 'max(go_goroutine_count{job="mecontrola-api"})'
          - refId: C
            datasourceUid: __expr__
            model:
              refId: C
              type: threshold
              conditions:
                - type: query
                  evaluator: { type: gt, params: [3000] }
`
	docPath := writeFixture(t, dir, "alert-rules-slo.yaml", docYAML)

	drifts, err := audit(docPath, sourcePath)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(drifts) != 1 {
		t.Fatalf("expected 1 drift, got %d: %+v", len(drifts), drifts)
	}
	if drifts[0].Kind != driftExpr {
		t.Fatalf("expected driftExpr, got %s", drifts[0].Kind)
	}
	if drifts[0].RefID != "A" {
		t.Fatalf("expected refId A, got %s", drifts[0].RefID)
	}
}

func TestReconcileDetectsThresholdDrift(t *testing.T) {
	dir := t.TempDir()
	sourcePath := writeFixture(t, dir, "rules.yaml", fixtureSourceRules)

	docYAML := `
apiVersion: 1
groups:
  - orgId: 1
    name: runtime
    rules:
      - uid: mc-runtime-goroutine-growth
        title: Crescimento sustentado de goroutines
        for: 15m
        labels: { severity: warning }
        data:
          - refId: A
            datasourceUid: prometheus
            model:
              refId: A
              expr: 'max(go_goroutine_count{job=~"mecontrola-.+"})'
          - refId: C
            datasourceUid: __expr__
            model:
              refId: C
              type: threshold
              conditions:
                - type: query
                  evaluator: { type: gt, params: [5000] }
`
	docPath := writeFixture(t, dir, "alert-rules-slo.yaml", docYAML)

	drifts, err := audit(docPath, sourcePath)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(drifts) != 1 {
		t.Fatalf("expected 1 drift, got %d: %+v", len(drifts), drifts)
	}
	if drifts[0].Kind != driftThreshold {
		t.Fatalf("expected driftThreshold, got %s", drifts[0].Kind)
	}
}

func TestReconcileDetectsForAndLabelsDrift(t *testing.T) {
	dir := t.TempDir()
	sourcePath := writeFixture(t, dir, "rules.yaml", fixtureSourceRules)

	docYAML := `
apiVersion: 1
groups:
  - orgId: 1
    name: runtime
    rules:
      - uid: mc-runtime-goroutine-growth
        title: Crescimento sustentado de goroutines
        for: 5m
        labels: { severity: critical }
        data:
          - refId: A
            datasourceUid: prometheus
            model:
              refId: A
              expr: 'max(go_goroutine_count{job=~"mecontrola-.+"})'
          - refId: C
            datasourceUid: __expr__
            model:
              refId: C
              type: threshold
              conditions:
                - type: query
                  evaluator: { type: gt, params: [3000] }
`
	docPath := writeFixture(t, dir, "alert-rules-slo.yaml", docYAML)

	drifts, err := audit(docPath, sourcePath)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(drifts) != 2 {
		t.Fatalf("expected 2 drifts (for + labels), got %d: %+v", len(drifts), drifts)
	}
	kinds := map[driftKind]bool{}
	for _, d := range drifts {
		kinds[d.Kind] = true
	}
	if !kinds[driftFor] || !kinds[driftLabels] {
		t.Fatalf("expected driftFor and driftLabels, got %+v", drifts)
	}
}

func TestAuditRealRepositoryHasNoSLOAssetDrift(t *testing.T) {
	docPath := filepath.Join("..", "..", "..", "observability", "alert-rules-slo.yaml")
	sourcePath := filepath.Join("..", "..", "..", "deployment", "telemetry", "grafana", "provisioning", "alerting", "rules.yaml")

	if _, err := os.Stat(docPath); err != nil {
		t.Skipf("alert-rules-slo.yaml not found at %s, skipping repository-wide audit: %v", docPath, err)
	}
	if _, err := os.Stat(sourcePath); err != nil {
		t.Skipf("rules.yaml not found at %s, skipping repository-wide audit: %v", sourcePath, err)
	}

	drifts, err := audit(docPath, sourcePath)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(drifts) != 0 {
		t.Fatalf("alert-rules-slo.yaml deve estar reconciliado com rules.yaml; drifts: %+v", drifts)
	}
}
