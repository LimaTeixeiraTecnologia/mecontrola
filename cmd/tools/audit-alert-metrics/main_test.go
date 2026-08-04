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

const fixtureCodeGo = `package example

func wire(o11y observability.Observability) {
	total := o11y.Metrics().Counter(
		"agents_write_total",
		"Total de escritas do agente",
		"1",
	)
	latency := o11y.Metrics().HistogramWithBuckets(
		"agent_llm_provider_latency_seconds",
		"Latencia",
		"s",
		[]float64{0.1, 0.5, 1},
	)
	_ = total
	_ = latency
}
`

func TestAuditDetectsDeadMetric(t *testing.T) {
	dir := t.TempDir()
	rulesYAML := `
apiVersion: 1
groups:
  - orgId: 1
    name: agente
    rules:
      - uid: mc-agent-idempotency-replay
        title: Replays idempotentes acima do esperado
        data:
          - refId: A
            datasourceUid: prometheus
            model:
              refId: A
              expr: 'sum(rate(agent_idempotency_replay_total{job="mecontrola-api"}[5m]))'
          - refId: C
            datasourceUid: __expr__
            model:
              refId: C
              type: threshold
`
	rulesPath := writeFixture(t, dir, "rules.yaml", rulesYAML)
	codeRoot := filepath.Join(dir, "internal")
	writeFixture(t, codeRoot, "example/wire.go", fixtureCodeGo)

	violations, err := audit(rulesPath, "", codeRoot)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d: %+v", len(violations), violations)
	}
	if violations[0].Metric != "agent_idempotency_replay_total" {
		t.Fatalf("unexpected violation metric: %s", violations[0].Metric)
	}
	if violations[0].UID != "mc-agent-idempotency-replay" {
		t.Fatalf("unexpected violation uid: %s", violations[0].UID)
	}
}

func TestAuditPassesOnReconciledRules(t *testing.T) {
	dir := t.TempDir()
	rulesYAML := `
apiVersion: 1
groups:
  - orgId: 1
    name: agente
    rules:
      - uid: mc-agent-write-replay
        title: Replays idempotentes acima do esperado
        data:
          - refId: A
            datasourceUid: prometheus
            model:
              refId: A
              expr: 'sum(rate(agents_write_total{job="mecontrola-api", outcome="replay"}[5m]))'
          - refId: C
            datasourceUid: __expr__
            model:
              refId: C
              type: threshold
      - uid: mc-api-down
        title: API sem metricas
        data:
          - refId: A
            datasourceUid: prometheus
            model:
              refId: A
              expr: 'absent(http_server_request_active{job="mecontrola-api"})'
      - uid: mc-db-pool-wait
        title: Espera por conexao no pool
        data:
          - refId: A
            datasourceUid: prometheus
            model:
              refId: A
              expr: 'sum(rate(database_pool_wait_count_total{job=~"mecontrola-.+"}[5m]))'
      - uid: mc-disk-usage-high
        title: Uso de disco alto
        data:
          - refId: A
            datasourceUid: prometheus
            model:
              refId: A
              expr: '100 - ((node_filesystem_avail_bytes{mountpoint="/"} * 100) / node_filesystem_size_bytes{mountpoint="/"})'
      - uid: mc-cpu-usage-high
        title: Uso de CPU alto
        data:
          - refId: A
            datasourceUid: prometheus
            model:
              refId: A
              expr: '100 - (avg by (instance) (irate(node_cpu_seconds_total{mode="idle"}[5m])) * 100)'
      - uid: mc-wal-lag-high
        title: WAL lag alto
        data:
          - refId: A
            datasourceUid: prometheus
            model:
              refId: A
              expr: '(
  pg_stat_archiver_last_archive_age > 900
  and on()
  (
    rate(pg_stat_database_xact_commit[10m]) > 0
    or
    rate(pg_stat_database_blks_written[10m]) > 0
  )
)'
      - uid: mc-agent-llm-error-rate
        title: Taxa de erro dos providers LLM alta
        data:
          - refId: A
            datasourceUid: prometheus
            model:
              refId: A
              expr: 'histogram_quantile(0.99, sum by (le) (rate(agent_llm_provider_latency_seconds_bucket{job="mecontrola-api"}[5m])))'
`
	rulesPath := writeFixture(t, dir, "rules.yaml", rulesYAML)
	codeRoot := filepath.Join(dir, "internal")
	writeFixture(t, codeRoot, "example/wire.go", fixtureCodeGo)

	violations, err := audit(rulesPath, "", codeRoot)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("expected 0 violations, got %d: %+v", len(violations), violations)
	}
}

func TestExtractPromQLIdentifiersSkipsLabelsAndFunctions(t *testing.T) {
	expr := `100 * sum(rate(http_server_request_count_total{job="mecontrola-api", http_response_status_code=~"5.."}[5m])) / clamp_min(sum(rate(http_server_request_count_total{job="mecontrola-api"}[5m])), 1)`
	got := extractPromQLIdentifiers(expr)
	want := []string{"http_server_request_count_total"}
	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("unexpected identifiers: %+v", got)
	}
}

func TestAuditRealRepositoryHasNoDeadMetrics(t *testing.T) {
	rulesPath := filepath.Join("..", "..", "..", "deployment", "telemetry", "grafana", "provisioning", "alerting", "rules.yaml")
	dashboardsDir := filepath.Join("..", "..", "..", "deployment", "dashboards")
	internalRoot := filepath.Join("..", "..", "..", "internal")
	cmdRoot := filepath.Join("..", "..", "..", "cmd")

	if _, err := os.Stat(rulesPath); err != nil {
		t.Skipf("rules.yaml not found at %s, skipping repository-wide audit: %v", rulesPath, err)
	}

	violations, err := audit(rulesPath, dashboardsDir, internalRoot, cmdRoot)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("rules.yaml e dashboards devem estar reconciliados; violacoes encontradas: %+v", violations)
	}
}

func TestRenderedNamesModelsOTLPToPrometheusSuffixes(t *testing.T) {
	scenarios := []struct {
		name string
		reg  registration
		want string
	}{
		{
			name: "gauge com unit 1 ganha sufixo _ratio",
			reg:  registration{Instrument: "Gauge", Name: "worker_heartbeat", Unit: "1"},
			want: "worker_heartbeat_ratio",
		},
		{
			name: "gauge com unit de anotacao preserva o nome",
			reg:  registration{Instrument: "Gauge", Name: "worker_heartbeat", Unit: "{beat}"},
			want: "worker_heartbeat",
		},
		{
			name: "counter monotonico ganha sufixo _total",
			reg:  registration{Instrument: "Counter", Name: "outbox_events_inserted", Unit: "{event}"},
			want: "outbox_events_inserted_total",
		},
		{
			name: "unit de bytes vira sufixo _bytes",
			reg:  registration{Instrument: "UpDownCounter", Name: "cache_size", Unit: "By"},
			want: "cache_size_bytes",
		},
		{
			name: "counter com unit 1 e sufixo _total nao duplica nem vira _ratio",
			reg:  registration{Instrument: "Counter", Name: "agents_write_total", Unit: "1"},
			want: "agents_write_total",
		},
		{
			name: "counter com unit 1 sem sufixo ganha _total, nunca _ratio",
			reg:  registration{Instrument: "Counter", Name: "probe_write_count", Unit: "1"},
			want: "probe_write_count_total",
		},
		{
			name: "updowncounter com unit 1 NAO ganha _ratio (so gauge observavel ganha)",
			reg:  registration{Instrument: "UpDownCounter", Name: "probe_inflight", Unit: "1"},
			want: "probe_inflight",
		},
		{
			name: "unit de segundos nao duplica sufixo existente",
			reg:  registration{Instrument: "Gauge", Name: "outbox_lag_seconds", Unit: "s"},
			want: "outbox_lag_seconds",
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			got := renderedNames(scenario.reg)
			if len(got) != 1 || got[0] != scenario.want {
				t.Fatalf("renderedNames(%+v) = %+v, quer %q", scenario.reg, got, scenario.want)
			}
		})
	}
}

func TestAuditDetectsGaugeRenamedByUnitRatio(t *testing.T) {
	dir := t.TempDir()
	rulesYAML := `
apiVersion: 1
groups:
  - orgId: 1
    name: worker
    rules:
      - uid: mc-worker-down
        title: Worker sem heartbeat
        data:
          - refId: A
            datasourceUid: prometheus
            model:
              refId: A
              expr: 'absent(worker_heartbeat{job="mecontrola-worker"})'
`
	codeGo := `package example

func wire(o11y observability.Observability) {
	_ = o11y.Metrics().Gauge(
		"worker_heartbeat",
		"Liveness heartbeat do worker",
		"1",
		nil,
	)
}
`
	rulesPath := writeFixture(t, dir, "rules.yaml", rulesYAML)
	codeRoot := filepath.Join(dir, "cmd")
	writeFixture(t, codeRoot, "worker/worker.go", codeGo)

	violations, err := audit(rulesPath, "", codeRoot)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(violations) != 1 || violations[0].Metric != "worker_heartbeat" {
		t.Fatalf("gauge com unit \"1\" renderiza worker_heartbeat_ratio; o alerta deve ser sinalizado, got %+v", violations)
	}
}

func TestAuditScansDashboardPanels(t *testing.T) {
	dir := t.TempDir()
	rulesYAML := `
apiVersion: 1
groups:
  - orgId: 1
    name: vazio
    rules: []
`
	dashboardJSON := `{
  "panels": [
    {
      "title": "Row",
      "panels": [
        {
          "title": "Fila outbox",
          "targets": [
            { "refId": "A", "expr": "sum(metrica_morta_total)" }
          ]
        }
      ]
    }
  ]
}`
	rulesPath := writeFixture(t, dir, "rules.yaml", rulesYAML)
	dashboardsDir := filepath.Join(dir, "dashboards")
	writeFixture(t, dashboardsDir, "infra.json", dashboardJSON)
	codeRoot := filepath.Join(dir, "internal")
	writeFixture(t, codeRoot, "example/wire.go", fixtureCodeGo)

	violations, err := audit(rulesPath, dashboardsDir, codeRoot)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(violations) != 1 || violations[0].Metric != "metrica_morta_total" {
		t.Fatalf("painel com serie morta deve ser sinalizado, got %+v", violations)
	}
}

func TestExtractPromQLIdentifiersSkipsGrafanaVarsAndGroupingLabels(t *testing.T) {
	expr := `sum by (agent_id) (rate(agents_write_total[$__rate_interval]))`
	got := extractPromQLIdentifiers(expr)
	if len(got) != 1 || got[0] != "agents_write_total" {
		t.Fatalf("variaveis Grafana e labels de agrupamento nao sao metricas: %+v", got)
	}
}
