# Runbook — Gate Pós-Deploy do Agente MeControla (Promoção/Rollback)

- PRD: `.specs/prd-orquestracao-conversacional-confiavel/prd.md` (RF-42, RF-43, RF-49..RF-57)
- ADR: `.specs/prd-orquestracao-conversacional-confiavel/adr-005-golden-harness-gate.md`
- Código: `internal/agents/application/postdeploy/` (cálculo puro do gate),
  `internal/agents/infrastructure/persistence/postdeploy/` (leitura Postgres)
- Dashboard: `docs/dashboards/mecontrola-agent-gate-posdeploy.json`
- Alertas: `docs/alerts/mecontrola-agent-gate-posdeploy.yaml`
- Rollout sem feature flag (onboarding): `docs/runbooks/onboarding-rollout-checklist.md`

## 1. Quando este runbook se aplica

Após um deploy da nova versão do agente MeControla que passou pelo **gate pré-deploy**
(golden set determinístico + testes de guard/scorer + harness real-LLM ≥ 0,90 por
categoria — ver `task-6.0-golden-harness-gate-predeploy.md` e o `6.0_execution_report.md`),
este runbook define o procedimento para decidir **manter, promover ou reverter** a versão
com base em evidência produtiva.

O gate pós-deploy é **consulta/observabilidade**, não código de runtime: nenhuma decisão é
tomada automaticamente pelo sistema. A decisão final é humana, documentada e rastreável por
`run_id` (RF-52).

## 2. Linha base de referência (RF-49)

Coletada em 7 dias / 23 runs, antes desta iniciativa:

| Métrica | Baseline |
|---|---|
| Runs succeeded | 19 / 23 |
| Runs failed | 4 / 23 |
| Taxa de falha | 4/23 ≈ 0,1739 |
| `tool-call-accuracy` (redefinida — RF-42) | 0,304 |
| `completeness` | 0,149 |
| `categorization` | 0,565 |

## 3. Amostra mínima e margens (RF-51, ADR-005)

O gate **não decide** sobre uma amostra pequena. Antes de promover ou reverter, exigir:

- **N ≥ 100 runs** do `agent_id` desde o deploy, **OU**
- **janela ≥ 14 dias** desde o deploy (o que ocorrer primeiro).

Se nenhuma das duas condições for satisfeita, a decisão é **adiada** (nem promove, nem
reverte) — a versão permanece em observação.

Margens absolutas exigidas para promoção (todas devem passar):

| Métrica | Baseline | Margem exigida | Threshold de promoção |
|---|---|---|---|
| Taxa de falha | 0,1739 | `<= baseline` | `<= 0,1739` |
| `tool-call-accuracy` redefinida (RF-42) | 0,304 | `+0,05` | `>= 0,354` |
| `completeness` | 0,149 | `+0,05` | `>= 0,199` |
| `categorization` | 0,565 | `+0,05` | `>= 0,615` |

Além das métricas acima, a promoção exige **zero regressão operacional** no período:

- `agent_run_truncated_total` (truncamento) — zero incrementos.
- `agent_run_update_errors_total` (falha de `RunStore.Update`) — zero incrementos.
- `agent_message_append_errors_total` (falha de `MessageStore.Append`) — zero incrementos.
- `no_duplicate_write` — zero violações (nenhum `score < 1` no scorer no período).

Estas constantes estão codificadas em `internal/agents/application/postdeploy/gate.go`
(`MinimumSampleRuns`, `MinimumSampleWindowDays`, `ScorerImprovementMargin`,
`BaselineFailureRate`, `RequiredToolCallAccuracy`, `RequiredCompleteness`,
`RequiredCategorization`) — não redigite os números manualmente; qualquer revisão de
threshold deve alterar o código e os testes correspondentes em `gate_test.go`.

## 4. `tool-call-accuracy` redefinida (RF-42)

A métrica publicada pelo scorer `tool-call-accuracy` mede "alguma tool financeira foi
chamada" em qualquer run, incluindo runs onde nenhuma tool era esperada (saudação,
clarify, replay). Isso dilui o denominador com ruído.

O **gate pós-deploy** redefine a métrica na camada de agregação (não no scorer):

```text
tool_call_accuracy_redefinida = hits / runs_onde_tool_era_esperada
runs_onde_tool_era_esperada   = runs com status=succeeded E outcome NOT IN (clarify, replay)
hits                          = subconjunto acima onde a tool esperada foi de fato chamada
```

A baseline 0,304 é **reinterpretada** sob esta definição — não é recomputada
retroativamente (não há dados históricos com outcome discriminado o suficiente); o
threshold de promoção (`>= 0,354`) usa a baseline 0,304 como ponto de partida por decisão
explícita do PRD (RF-42), documentando a limitação.

## 5. Como computar o gate (evidência rastreável por `run_id`)

O gate é uma função pura (`postdeploy.EvaluateGate`) alimentada por 3 fontes:

1. **Postgres** (`platform_runs` + `platform_scorer_results`) via
   `postdeploy.NewAggregateReader(db)` — runs, outcomes, truncamento, scores por scorer,
   violações de `no_duplicate_write`.
2. **Prometheus** (`agent_run_update_errors_total`, `agent_message_append_errors_total`) —
   passados manualmente como `postdeploy.PrometheusCounters` (não há write-path Postgres
   para esses dois contadores; ver seção 7 "Limitação conhecida").
3. **Janela temporal** (`since time.Time`) — normalmente o timestamp do deploy da versão
   sob avaliação.

### 5.1 Consulta manual (operador, via `psql` ou client SQL)

Sempre filtrar por `agent_id` (constante `mecontrola-agent`, definida em
`internal/agents/application/agents/mecontrola_agent.go` como `MecontrolaAgentID`) e por
`started_at >= <timestamp do deploy>`.

**Runs e outcomes:**

```sql
SELECT
    count(*) AS total_runs,
    count(*) FILTER (WHERE status = 'succeeded') AS succeeded_runs,
    count(*) FILTER (WHERE status = 'failed') AS failed_runs,
    count(*) FILTER (WHERE outcome NOT IN ('clarify', 'replay') OR outcome = '') AS expected_tool_runs,
    count(*) FILTER (WHERE outcome = 'truncated') AS truncated_runs,
    min(started_at) AS window_start,
    max(started_at) AS window_end
FROM mecontrola.platform_runs
WHERE agent_id = 'mecontrola-agent' AND started_at >= '<timestamp-do-deploy>';
```

**Scorers (média por `scorer_id`):**

```sql
SELECT sr.scorer_id, count(*) AS sample_n, avg(sr.score) AS mean_score
FROM mecontrola.platform_scorer_results sr
JOIN mecontrola.platform_runs pr ON pr.id = sr.run_id
WHERE pr.agent_id = 'mecontrola-agent' AND pr.started_at >= '<timestamp-do-deploy>'
GROUP BY sr.scorer_id;
```

**Escrita duplicada (`no_duplicate_write` com score < 1 = violação):**

```sql
SELECT count(*) AS violations
FROM mecontrola.platform_scorer_results sr
JOIN mecontrola.platform_runs pr ON pr.id = sr.run_id
WHERE pr.agent_id = 'mecontrola-agent'
  AND pr.started_at >= '<timestamp-do-deploy>'
  AND sr.scorer_id = 'no_duplicate_write'
  AND sr.score < 1;
```

**Rastreabilidade por `run_id` (evidência para a decisão — RF-52):** todo run failed,
truncado ou com violação de `no_duplicate_write` deve ser listado individualmente antes da
decisão:

```sql
SELECT id AS run_id, status, outcome, error, started_at, ended_at
FROM mecontrola.platform_runs
WHERE agent_id = 'mecontrola-agent'
  AND started_at >= '<timestamp-do-deploy>'
  AND (status = 'failed' OR outcome = 'truncated')
ORDER BY started_at DESC;
```

Nenhuma dessas consultas expõe conteúdo de mensagem do usuário (RF-32/RF-34) — apenas
`run_id`, `status`, `outcome`, `error` sanitizado e timestamps.

### 5.2 Via código (preferencial para automação futura)

```go
reader := postdeploy.NewAggregateReader(db)
verdict, err := postdeploy.ComputeGate(ctx, reader, agents.MecontrolaAgentID, deployTimestamp, postdeploy.PrometheusCounters{
    RunUpdateErrors:     <valor lido de agent_run_update_errors_total no Prometheus>,
    MessageAppendErrors: <valor lido de agent_message_append_errors_total no Prometheus>,
})
// verdict.Promote == true  -> promover
// verdict.Reasons          -> lista de motivos de bloqueio, se houver
```

## 6. Decisão: promover, manter em observação, ou reverter

| Situação | Decisão |
|---|---|
| Amostra insuficiente (`verdict.SampleSufficient == false`) | Manter em observação; não promover nem reverter. Reavaliar ao atingir N≥100 ou 14 dias. |
| Amostra suficiente + `verdict.Promote == true` | **Promover**: versão vira baseline; arquivar evidência (query + `run_id`s + timestamp) em anexo à decisão. |
| Amostra suficiente + `verdict.Promote == false` + `NoRegressionOperational == false` | **Reverter imediatamente** — há regressão operacional ativa (truncamento, update/append de erro, escrita duplicada). Não esperar a janela completa. |
| Amostra suficiente + `verdict.Promote == false` + métricas abaixo da margem, mas sem regressão operacional | Registrar decisão de **não promover** (manter versão anterior como baseline oficial); investigar causa antes de nova tentativa. |

Toda decisão (promover, reverter, manter em observação) DEVE ser registrada com:

1. Timestamp da decisão.
2. `verdict` completo (JSON serializável de `postdeploy.GateVerdict`).
3. Lista de `run_id`s usados como evidência (seção 5.1, última query).
4. Nome/identificador do operador que decidiu.

Nunca decidir por "impressão subjetiva" ou por leitura de mensagens de usuário — a decisão
é sempre por agregado + `run_id` (RF-52).

## 7. Limitação conhecida — contadores Prometheus-only

`agent_run_update_errors_total` e `agent_message_append_errors_total` são emitidos apenas
como métricas Prometheus (`internal/platform/agent/runtime.go`); não há persistência
equivalente em `platform_runs` para esses dois eventos especificamente (o run em si é
marcado, mas o motivo "falha de update" não decorre de uma coluna dedicada). Por isso
`postdeploy.ComputeGate` recebe esses dois valores via `PrometheusCounters` (fonte manual
ou por integração futura com a API do Prometheus), não via `AggregateReader`.

Consulta PromQL equivalente para o período do deploy:

```promql
increase(agent_run_update_errors_total{agent_id="mecontrola-agent"}[<janela-desde-deploy>])
increase(agent_message_append_errors_total{agent_id="mecontrola-agent"}[<janela-desde-deploy>])
```

Truncamento (`agent_run_truncated_total`), por contraste, **é** persistido em
`platform_runs.outcome = 'truncated'` e por isso entra em `RunAggregate.TruncatedRuns` via
Postgres — preferir a leitura Postgres para truncamento por já estar coberta pelo
`AggregateReader`.

## 8. Contrato de regressão (RF-54..RF-57)

Antes de qualquer deploy que sirva de candidato ao gate pós-deploy, confirmar:

1. **Nenhuma tool removida/renomeada/oculta/com contrato alterado sem história própria**
   (RF-54): `go test ./internal/agents/application/postdeploy/... -run
   TestRegressionContractSuite -v` compara o inventário versionado
   (`postdeploy.RegisteredTools`, `RegisteredWorkflows`, `RegisteredScorers`) contra:
   - a lista real de chamadas `agenttools.BuildXTool` em `internal/agents/module.go`
     (parseada via `go/ast`, não hardcoded);
   - as constantes `WorkflowID` reais dos pacotes `workflows`;
   - os construtores reais de scorer em `internal/agents/application/scorers`.
   Qualquer divergência falha o teste — é o gate de regressão executável.
2. **Contrato público preservado** (RF-55): `BuildMeControlaAgent`, `AgentRuntime`,
   `RunStore`, `ThreadGateway`, `MessageStore`, `WorkingMemory`, schemas strict das tools e
   workflows duráveis — verificado por `go build ./...` sem quebra de assinatura (mudança
   de assinatura pública quebraria todo o `internal/agents` na compilação) e pela suíte
   completa de testes (`go test ./... -count=1`).
3. **Fluxos existentes cobertos** (RF-56/RF-57): os 18 fluxos listados em
   `postdeploy.CoveredExistingFlows` (registro despesa/receita, consulta mensal, orçamento,
   fatura, última transação, busca de transações, cartões, recorrências, categorias,
   onboarding, pendências, confirmação destrutiva, criação de cartão, criação de orçamento,
   memória, scorers, entrega WhatsApp) devem continuar cobertos por
   `internal/agents/application/golden/` (34 casos, tarefa 6.0) + testes unitários/integração
   existentes — sem novo gap conhecido.

Se qualquer um dos 3 pontos falhar, **o deploy não é elegível ao gate pós-deploy** —
corrigir a regressão antes.

## 9. Procedimento passo a passo (checklist operacional)

1. Confirmar que o gate pré-deploy passou (golden determinístico verde + harness real-LLM
   ≥ 0,90 por categoria) antes do deploy — ver `6.0_execution_report.md`.
2. Anotar o timestamp do deploy no dashboard (`docs/dashboards/mecontrola-agent-gate-posdeploy.json`,
   anotação "Deploys do agente") e no `docs/runbooks/deploy-anti-storm.md` se aplicável ao
   processo de deploy do serviço.
3. Rodar o contrato de regressão (seção 8) — bloquear se falhar.
4. Monitorar continuamente via dashboard + alertas (`docs/alerts/mecontrola-agent-gate-posdeploy.yaml`)
   durante a janela de observação.
5. Ao atingir N≥100 runs OU 14 dias (o que ocorrer primeiro), computar o gate (seção 5).
6. Se `NoRegressionOperational == false` a qualquer momento antes disso, **não esperar a
   janela**: reverter imediatamente e registrar a decisão (seção 6).
7. Ao final da janela, aplicar a matriz de decisão (seção 6) e registrar a evidência.
8. Se promovido: a versão vira a nova baseline; atualizar a seção 2 deste runbook com os
   novos números medidos (não os thresholds codificados, que continuam fixos até revisão
   formal do PRD/ADR-005).

## 11. Troubleshooting de falso sucesso financeiro (RF-31/RF-34)

Alerta: `PendingEntryFalseSuccess` (`agents_pending_entry_false_success_total` > 0).

### Quando este runbook se aplica

O alerta dispara quando o workflow `pending-entry` registra uma confirmação positiva do
usuário (resposta "sim"/"confirmar"/"ok"/"pode") mas não obtém um `resourceID` de transação
ativa como resultado. Isso caracteriza **falso sucesso financeiro**: o usuário aceitou a
operação, mas não há transação durável rastreável.

### Investigação imediata (sem ler mensagens do usuário)

1. **Confirmar o incremento da métrica** no Prometheus:
   ```promql
   increase(agents_pending_entry_false_success_total{workflow="pending-entry"}[5m])
   ```
   Qualquer valor > 0 é crítico.

2. **Correlacionar com spans** no Tempo:
   - `agents.usecase.pending_entry_continuer`
   - `agents.usecase.idempotent_write`
   - `transactions.usecase.create_transaction` (ou `.update_transaction`)
   Use `trace_id` para ligar o evento ao run correspondente.

3. **Verificar rastros duráveis** (sem expor texto de mensagem como label):
   ```sql
   SELECT id, workflow, status, stage, error, started_at, ended_at
   FROM mecontrola.workflow_runs
   WHERE workflow = 'pending-entry'
     AND status = 'failed'
     AND started_at >= now() - interval '30 minutes'
   ORDER BY started_at DESC;
   ```

4. **Confirmar ausência de transação ativa** pelo `origin_wamid` (obtido do span ou do run
   de continuer; não use como label de métrica):
   ```sql
   SELECT id, amount_cents, description, payment_method, category_path, created_at
   FROM mecontrola.transactions
   WHERE origin_wamid = '<wamid-do-evento>'
     AND deleted_at IS NULL;
   ```
   Resultado esperado em caso de falso sucesso: **0 linhas**.

5. **Verificar se o ledger de escrita idempotente registrou a operação**:
   ```sql
   SELECT id, operation, resource_id, resource_kind, created_at
   FROM mecontrola.agents_write_ledger
   WHERE wamid = '<wamid-do-evento>';
   ```
   `resource_id` nulo ou ausente confirma que a escrita foi aceita sem recurso.

### Ações

- Se confirmado falso sucesso em produção: **reverter o deploy imediatamente** e abrir
  incidente. Não aguardar a janela do gate pós-deploy.
- Se o alerta for falso positivo (ex.: métrica incrementada por retry idempotente já
  resolvido): documentar o `trace_id`/run_id e ajustar o threshold somente após revisão do
  PRD.
- Após correção, garantir que testes de regressão cobrem confirmação positiva com e sem
  transação ativa (`pending_entry_no_false_success_test.go`).

### Métricas e labels permitidos

- `agents_pending_entry_false_success_total`: labels `workflow`, `step`.
- Proibido como label: `user_id`, telefone, `wamid`, categoria, `run_id`, `thread_id`,
  `resource_id` ou IDs de entidade.

## 12. Referências

- `internal/agents/application/postdeploy/gate.go` — cálculo puro (amostra mínima, margem,
  `tool-call-accuracy` redefinida, veredito de promoção).
- `internal/agents/application/postdeploy/reader.go` — porta `AggregateReader` +
  `ComputeGate`.
- `internal/agents/infrastructure/persistence/postdeploy/aggregate_reader.go` — adaptador
  Postgres (consultas de agregação, sem regra de negócio).
- `internal/agents/application/postdeploy/regression_contract.go` — inventário versionado
  de tools/workflows/scorers/fluxos cobertos (RF-54..RF-57).
- `internal/agents/application/postdeploy/regression_contract_test.go` +
  `module_wiring_source_test.go` — verificação executável do contrato de regressão.
- `internal/agents/application/workflows/pending_entry_workflow.go` — métrica de falso
  sucesso financeiro.
- `.specs/prd-orquestracao-conversacional-confiavel/adr-005-golden-harness-gate.md`.
- `.specs/prd-onboarding-sem-friccao-ate-primeiro-lancamento/prd.md` (RF-31..RF-34).
- `docs/runbooks/onboarding-rollout-checklist.md` — checklist de rollout sem feature flag,
  validação pós-deploy, SLO e procedimento de rollback para o onboarding sem fricção.
