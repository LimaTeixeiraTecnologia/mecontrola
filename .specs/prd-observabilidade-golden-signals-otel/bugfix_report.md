# Relatório de Bugfix — Ciclo review → bugfix → review

- Data: 2026-08-04
- Origem: `docs/reviews/2026-08-04-review-prd-observabilidade-golden-signals-otel.md`
- Spec: `.specs/prd-observabilidade-golden-signals-otel/`
- Veredito da rodada 1: `REJECTED` (1 critical, 3 high, 3 medium, 2 low)

## Nota de contexto sobre a rodada 1

A revisão iniciou com a orquestração `execute-all-tasks` **ainda em voo**: `.checkpoints/` mostra
`6.0.yaml` às 11:41 e a Tarefa 8.0 concluindo por volta de 11:50–11:55, durante a auditoria.
`observability/STANDARD.md` passou de 159 para 343 linhas e o validador passou de FAIL (5 erros)
para SUCCESS no meio da revisão. Os achados abaixo foram todos reconfirmados contra o estado final.

## Defeito raiz — uma única causa, quatro alertas mortos

Gauges registrados com unidade OTel `"1"` são renomeados pelo tradutor OTLP→Prometheus
(`prometheus/otlptranslator`) com sufixo `_ratio`. O nome usado nos alertas era o nome de registro,
não o renderizado. Como todos usam `noDataState: OK`, a falha era silenciosa.

**Confirmação empírica** (não inferência), harness `grafana/otel-lgtm:0.7.5` + OTLP HTTP real:

| Unidade no código | Nome renderizado no Prometheus |
| --- | --- |
| `"1"` | `worker_heartbeat_ratio`, `outbox_pending_jobs_ratio` |
| `{beat}` / `{job}` | `worker_heartbeat`, `outbox_pending_jobs` |

### B-01 `critical` — `mc-worker-down` disparava permanentemente; liveness do worker cego (RF-11)

`absent(worker_heartbeat{...})` nunca casava a série real → alerta `severity: critical` firing 24/7
para o Telegram, e nenhuma cobertura real de liveness.

- Correção: `cmd/worker/worker.go` — unidade `"1"` → `{beat}`.
- Reforço: expr passou a cobrir dois modos de falha —
  `absent(...) or (changes(...[10m]) == bool 0)` (processo morto **ou** ticker congelado com
  processo vivo). O `== bool 0` é necessário: `== 0` devolveria valor 0 e nunca cruzaria o threshold.

### B-02 `high` — `mc-outbox-queue-high` removido sob premissa falsa (RF-20)

A justificativa em `rules.yaml` afirmava que `outbox_pending_jobs` "nunca foi emitida"; ela **é**
emitida em `cmd/worker/worker.go` (`registerOutboxMetrics`). O `grep` da auditoria foi restrito a
`internal/platform/outbox/`. O sintoma real era o mesmo sufixo `_ratio`.

- Correção: alerta restaurado com a definição original; unidade `"1"` → `{job}`; comentário de
  reconciliação reescrito com a causa verdadeira.

### B-03 `high` — `mc-onboarding-unconsumed` também morto (achado novo, encontrado pelo gate reforçado)

`onboarding_tokens_paid_unconsumed{,_overdue}` tinham o mesmo defeito. Não estava em nenhum relatório
de execução — foi o gate corrigido que o revelou.

- Correção: `internal/onboarding/module.go` e `internal/platform/whatsapp/ratelimit/limiter.go` —
  unidades `"1"` → `{token}` / `{bucket}`.

### B-04 `high` — `go.mod` violava RF-01 (otel v1.45.0 em vez de v1.44.0)

`prd.md:62` fixa v1.44.0 e a task 2.0 proíbe explicitamente o upgrade;
`contrib/instrumentation/runtime@v0.69.0` requer exatamente v1.44.0. O bump arrastava
`kin-openapi`, `gopsutil`, `moby/go-archive`, `plan9stats` e `genproto`.

- Correção: família otel inteira revertida para v1.44.0 e bumps não relacionados revertidos.
  Delta final vs. `origin/main`: apenas `contrib/instrumentation/runtime v0.69.0` e duas promoções
  indirect→direct (`otel/sdk/metric`, `gopkg.in/yaml.v3`), ambas legítimas.

### B-05 `medium` — o gate de CI era estruturalmente incapaz de detectar B-01/B-02/B-03

`cmd/tools/audit-alert-metrics` comparava o nome de registro (ignorando a transformação de sufixo),
allowlistava prefixos inteiros (`go_`, `http_server_request_`, `database_pool_`) e **não lia
dashboards**, embora RF-07 fale em "alerta/painel".

- Correção: o tool agora **renderiza** o nome Prometheus a partir de nome + unidade + tipo de
  instrumento (`_ratio`, `_total`, `_bytes`, `_seconds`, `_percent`, sufixos de histograma), varre
  `deployment/dashboards/*.json` recursivamente, e substitui os prefixos de código próprio por
  listas explícitas de nomes. Prefixos permanecem apenas para exporters de terceiros.
- Prova de regressão: reintroduzir a unidade `"1"` faz o gate falhar apontando `mc-worker-down`.
- Testes novos: `TestRenderedNamesModelsOTLPToPrometheusSuffixes`,
  `TestAuditDetectsGaugeRenamedByUnitRatio`, `TestAuditScansDashboardPanels`,
  `TestExtractPromQLIdentifiersSkipsGrafanaVarsAndGroupingLabels`.

### B-06 `medium` — RF-08 não aplicado aos dashboards

RF-08 exige a série canônica em "dashboards/alertas"; 6 pontos de `mecontrola-api.json` ainda
derivavam tráfego e erro de `http_server_request_count_total`.

- Correção: migrados para `http_server_request_duration_seconds_count`. `http_server_request_active`
  permanece — é gauge de concorrência, não tráfego nem erro.

### B-07 `high` — mitigação do ADR-001 não existia de fato (RF-02)

O critério "nenhuma métrica de instância regride" não tinha teste algum.

- Correção: `TestStartDoesNotRegressInstanceMetrics` prova que, com o MeterProvider global ativo,
  métricas de instância continuam no provider próprio e não há vazamento nem duplicação em nenhuma
  direção.

### B-08 `low` — `runtimemetrics.Start` recebia `o11y` e ignorava

Assinatura sugeria um binding por instância que não existe.

- Correção: parâmetro removido e substituído por guarda real — `Start` retorna
  `ErrGlobalMeterProviderNotRegistered` se o provider global for o noop, transformando um no-op
  silencioso em erro explícito de bootstrap. Coberto por `TestStartFailsWithoutGlobalMeterProvider`.
- Também corrigido: testes gravavam `otel.SetMeterProvider(nil)` no global; agora restauram noop.

### B-09 `low` — alerta de causa sem runbook (ADR-001)

- Correção: `mc-runtime-goroutine-growth` passou a referenciar
  `docs/runbooks/infra-saturation-triage.md`, e o runbook ganhou a seção 4 com queries de
  goroutines/heap/GC, critério para distinguir leak de carga e procedimento de dump.

### B-10 `medium` — token Telegram ausente derrubava TODO o alerting (descoberto durante a validação)

`setup-grafana-alerts.sh` saía com `exit 0` sem gerar `contact-points.rendered.yaml`; o compose faz
bind-mount desse caminho, o Docker cria um diretório vazio e o Grafana **aborta o provisionamento de
todo o alerting**. Provado no harness: `"Failed to provision alerting: could not find Bot Token in
settings"` → **0 alert rules carregadas**, incluindo o tier de e-mail novo e todos os alertas de SLO.
O aviso original dizia apenas "alertas do Telegram nao serao ativados", subestimando o efeito.

- Correção: o script agora falha com `exit 1` e explica o efeito real. Bloquear o deploy é preferível
  a subir a stack sem nenhum alerta.

## Validações executadas

| Validação | Resultado |
| --- | --- |
| `go build ./...` | pass |
| `go vet ./...` (+ `-tags integration`) | pass |
| `go test -race ./internal/platform/observability/... ./cmd/...` | pass |
| `task lint:run` (golangci-lint) | `0 issues` |
| `lint:auth-bypass`, `lint:outbox-user-id`, `lint:deadcode` | 7 pass, 0 fail |
| Gate zero-comentários (R-ADAPTER-001.1) | vazio (pass) |
| `task ci:audit-alert-metrics` | OK — nenhum alerta **ou painel** referencia série morta |
| `audit-alert-slo-drift` | OK — asset documental reconciliado |
| `validate-standard.py` | `SUCCESS` |
| Provisionamento Grafana ao vivo (`otel-lgtm:0.7.5`) | `finished to provision alerting`; **32 regras**, **0 erros de avaliação** |
| Nome renderizado das séries (OTLP→Prometheus) | confirmado empiricamente, antes e depois da correção |

## Riscos residuais

- **Cardinalidade do `otelsql` sob `RegisterGlobal: true`.** `XSAM/otelsql` resolve
  `otel.GetTracerProvider()`/`GetMeterProvider()` no `Open`; antes da mudança o global era noop
  permanente, agora é o provider real. Efeito: spans por query (com `db.statement`) e séries
  `db_client_*` passam a ser emitidos. Não quebra nada e o `tail_sampling` amortece, mas o volume
  não foi dimensionado. Recomendo medir o delta de spans/séries no primeiro deploy.
- **`clamp_min(denominador, 1)`** nas queries de RED e burn-rate dessensibiliza a razão abaixo de
  1 req/s (100% de erro a 0,3 req/s lê como 30%). Pré-existente, mas agora afeta os limiares novos
  de burn-rate (0,0144). Não alterado — mudança de comportamento fora do escopo dos achados.
- **`GF_SMTP_PASSWORD` como env var** em `compose.swarm.yml:524`, enquanto a aplicação consome o
  mesmo segredo como Docker secret. Legível via `docker service inspect`. Higiene, não defeito ativo.
- **Thresholds não calibrados**: `3000` goroutines e `1000` jobs de outbox são baselines
  operacionais sem histórico de produção. Recalibrar pós-deploy.
- Confirmação ao vivo foi feita em harness local, não no Prometheus de produção.
