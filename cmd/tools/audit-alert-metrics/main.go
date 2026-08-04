package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type rulesDoc struct {
	Groups []struct {
		Rules []struct {
			UID   string `yaml:"uid"`
			Title string `yaml:"title"`
			Data  []struct {
				DatasourceUID string `yaml:"datasourceUid"`
				Model         struct {
					Expr string `yaml:"expr"`
				} `yaml:"model"`
			} `yaml:"data"`
		} `yaml:"rules"`
	} `yaml:"groups"`
}

type ruleMetric struct {
	Source string
	UID    string
	Title  string
	Metric string
}

type registration struct {
	Name       string
	Unit       string
	Instrument string
}

var (
	identifierRE         = regexp.MustCompile(`[A-Za-z_:][A-Za-z0-9_:]*`)
	quotedSpanRE         = regexp.MustCompile(`"(?:[^"\\]|\\.)*"`)
	groupingSpanRE       = regexp.MustCompile(`(?:\bby\b|\bwithout\b|\bon\b|\bignoring\b|\bgroup_left\b|\bgroup_right\b)\s*\(([^)]*)\)`)
	metricRegistrationRE = regexp.MustCompile(
		`o11y\.Metrics\(\)\.(Counter|Histogram|HistogramWithBuckets|Gauge|UpDownCounter)\(\s*"([A-Za-z0-9_:]+)"\s*,\s*"(?:[^"\\]|\\.)*"\s*,\s*"([^"]*)"`,
	)

	exporterPrefixes = []string{
		"node_",
		"pg_stat_",
		"pgbackrest_",
		"otelcol_",
		"probe_",
		"scrape_",
		"up",
	}

	runtimeMetrics = []string{
		"go_goroutine_count",
		"go_memory_used_bytes",
		"go_memory_limit_bytes",
		"go_memory_allocated_bytes_total",
		"go_memory_allocations_total",
		"go_memory_gc_goal_bytes",
		"go_processor_limit",
		"go_config_gogc_percent",
	}

	devkitMetrics = []string{
		"http_server_request_duration_seconds",
		"http_server_request_count_total",
		"http_server_request_error_count_total",
		"http_server_request_active",
		"http_client_request_duration_milliseconds",
		"http_client_request_count_total",
		"http_client_request_errors_total",
		"database_pool_connections_open",
		"database_pool_connections_idle",
		"database_pool_wait_count_total",
		"database_pool_wait_duration_ms_milliseconds",
		"database_query_duration_ms_milliseconds",
		"database_tx_committed_total",
		"database_tx_rolledback_total",
		"database_tx_duration_ms_milliseconds",
	}

	unitSuffixes = map[string]string{
		"s":    "_seconds",
		"ms":   "_milliseconds",
		"By":   "_bytes",
		"KiBy": "_kibibytes",
		"MiBy": "_mebibytes",
		"GiBy": "_gibibytes",
		"%":    "_percent",
	}

	histogramSuffixes = []string{"_bucket", "_sum", "_count"}
)

func withinAnySpan(pos int, spans [][]int) bool {
	for _, span := range spans {
		if pos >= span[0] && pos < span[1] {
			return true
		}
	}
	return false
}

func extractPromQLIdentifiers(expr string) []string {
	skipSpans := quotedSpanRE.FindAllStringIndex(expr, -1)
	for _, span := range groupingSpanRE.FindAllStringSubmatchIndex(expr, -1) {
		skipSpans = append(skipSpans, []int{span[2], span[3]})
	}
	matches := identifierRE.FindAllStringIndex(expr, -1)
	seen := make(map[string]struct{}, len(matches))
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		start, end := m[0], m[1]
		if withinAnySpan(start, skipSpans) {
			continue
		}
		token := expr[start:end]
		if !strings.Contains(token, "_") {
			continue
		}
		if strings.HasPrefix(token, "__") {
			continue
		}
		if start > 0 && (expr[start-1] == '$' || expr[start-1] == '.') {
			continue
		}
		var next byte
		if end < len(expr) {
			next = expr[end]
		}
		if next == '(' || next == '=' || next == '~' || next == '!' {
			continue
		}
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		out = append(out, token)
	}
	sort.Strings(out)
	return out
}

func renderedNames(reg registration) []string {
	base := reg.Name
	unit := strings.TrimSpace(reg.Unit)

	isAnnotation := strings.HasPrefix(unit, "{") && strings.HasSuffix(unit, "}")
	monotonic := reg.Instrument == "Counter"
	gauge := reg.Instrument == "Gauge"
	histogram := reg.Instrument == "Histogram" || reg.Instrument == "HistogramWithBuckets"

	if !isAnnotation {
		if suffix, ok := unitSuffixes[unit]; ok {
			if !strings.HasSuffix(base, suffix) {
				base += suffix
			}
		} else if unit == "1" && gauge {
			if !strings.HasSuffix(base, "_ratio") {
				base += "_ratio"
			}
		}
	}

	if monotonic && !strings.HasSuffix(base, "_total") {
		base += "_total"
	}

	if histogram {
		out := make([]string, 0, len(histogramSuffixes))
		for _, suffix := range histogramSuffixes {
			out = append(out, base+suffix)
		}
		return append(out, base)
	}

	return []string{base}
}

func loadRuleMetrics(path string) ([]ruleMetric, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("audit-alert-metrics: read rules file: %w", err)
	}

	var doc rulesDoc
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("audit-alert-metrics: parse rules yaml: %w", err)
	}

	out := make([]ruleMetric, 0)
	for _, group := range doc.Groups {
		for _, rule := range group.Rules {
			for _, data := range rule.Data {
				if data.DatasourceUID != "prometheus" {
					continue
				}
				for _, metric := range extractPromQLIdentifiers(data.Model.Expr) {
					out = append(out, ruleMetric{
						Source: path,
						UID:    rule.UID,
						Title:  rule.Title,
						Metric: metric,
					})
				}
			}
		}
	}
	return out, nil
}

func collectPanelExprs(node any, out *[]string) {
	switch value := node.(type) {
	case map[string]any:
		if expr, ok := value["expr"].(string); ok && expr != "" {
			*out = append(*out, expr)
		}
		for _, nested := range value {
			collectPanelExprs(nested, out)
		}
	case []any:
		for _, nested := range value {
			collectPanelExprs(nested, out)
		}
	}
}

func loadDashboardMetrics(dir string) ([]ruleMetric, error) {
	out := make([]ruleMetric, 0)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, fmt.Errorf("audit-alert-metrics: read dashboards dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil, fmt.Errorf("audit-alert-metrics: read %s: %w", path, readErr)
		}
		var doc any
		if unmarshalErr := json.Unmarshal(raw, &doc); unmarshalErr != nil {
			return nil, fmt.Errorf("audit-alert-metrics: parse %s: %w", path, unmarshalErr)
		}
		exprs := make([]string, 0)
		collectPanelExprs(doc, &exprs)
		for _, expr := range exprs {
			for _, metric := range extractPromQLIdentifiers(expr) {
				out = append(out, ruleMetric{
					Source: path,
					UID:    entry.Name(),
					Title:  "painel",
					Metric: metric,
				})
			}
		}
	}
	return out, nil
}

func scanCodeMetrics(root string) (map[string]struct{}, error) {
	found := make(map[string]struct{})
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("audit-alert-metrics: walk %s: %w", path, err)
		}
		if d.IsDir() {
			if d.Name() == "mocks" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("audit-alert-metrics: read %s: %w", path, readErr)
		}
		for _, m := range metricRegistrationRE.FindAllStringSubmatch(string(content), -1) {
			reg := registration{Instrument: m[1], Name: m[2], Unit: m[3]}
			for _, rendered := range renderedNames(reg) {
				found[rendered] = struct{}{}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return found, nil
}

func isKnownMetric(metric string, code map[string]struct{}) bool {
	if _, ok := code[metric]; ok {
		return true
	}
	for _, suffix := range histogramSuffixes {
		if base, cut := strings.CutSuffix(metric, suffix); cut {
			if _, ok := code[base]; ok {
				return true
			}
		}
	}
	if slices.Contains(runtimeMetrics, metric) {
		return true
	}
	for _, known := range devkitMetrics {
		if metric == known {
			return true
		}
		for _, suffix := range histogramSuffixes {
			if metric == known+suffix {
				return true
			}
		}
	}
	for _, prefix := range exporterPrefixes {
		if strings.HasPrefix(metric, prefix) {
			return true
		}
	}
	return false
}

func audit(rulesPath, dashboardsDir string, codeRoots ...string) ([]ruleMetric, error) {
	metrics, err := loadRuleMetrics(rulesPath)
	if err != nil {
		return nil, err
	}

	if dashboardsDir != "" {
		panelMetrics, dashErr := loadDashboardMetrics(dashboardsDir)
		if dashErr != nil {
			return nil, dashErr
		}
		metrics = append(metrics, panelMetrics...)
	}

	code := make(map[string]struct{})
	for _, root := range codeRoots {
		rootMetrics, scanErr := scanCodeMetrics(root)
		if scanErr != nil {
			return nil, scanErr
		}
		for metric := range rootMetrics {
			code[metric] = struct{}{}
		}
	}

	violations := make([]ruleMetric, 0)
	for _, rm := range metrics {
		if !isKnownMetric(rm.Metric, code) {
			violations = append(violations, rm)
		}
	}
	return violations, nil
}

func main() {
	rulesPath := flag.String(
		"rules",
		"deployment/telemetry/grafana/provisioning/alerting/rules.yaml",
		"caminho do rules.yaml provisionado do Grafana",
	)
	dashboardsDir := flag.String(
		"dashboards",
		"deployment/dashboards",
		"diretorio dos dashboards provisionados do Grafana",
	)
	codeRoots := flag.String(
		"code-root",
		"internal,cmd",
		"raizes de codigo Go (separadas por virgula) a varrer por registros o11y.Metrics()",
	)
	flag.Parse()

	violations, err := audit(*rulesPath, *dashboardsDir, strings.Split(*codeRoots, ",")...)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if len(violations) == 0 {
		fmt.Println("audit-alert-metrics: OK - nenhum alerta ou painel referencia serie morta")
		return
	}

	fmt.Fprintln(os.Stderr, "audit-alert-metrics: FAIL - alertas/paineis referenciam metricas nao emitidas:")
	for _, v := range violations {
		fmt.Fprintf(os.Stderr, "  source=%s uid=%s title=%q metric=%s\n", v.Source, v.UID, v.Title, v.Metric)
	}
	os.Exit(1)
}
