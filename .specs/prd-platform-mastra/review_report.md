# Review Report — prd-platform-mastra

- **Data:** 2026-06-29
- **Escopo:** `.specs/prd-platform-mastra` (PRD spec-version 3, RF-01..RF-46), working tree como fonte da verdade
- **Método:** 6 reviewers paralelos por cluster de RF + verificação pessoal de cada finding borderline (zero falso positivo) + ciclo review→bugfix→review
- **Veredito final:** **APPROVED**

## Resultado

Plataforma `internal/platform` (agent, memory, workflow kernel, tool, scorer, llm) + consumidor de
referência `internal/agents` (weather Mastra) conformes a RF-01..RF-46. Todas as 9 tarefas validadas
contra o working tree. `internal/agent` (singular) removido; sem imports/SQL legados.

## Findings remediados nesta sessão (todos com teste de regressão, `-race` verde)

| # | Sev | Arquivo | Correção |
|---|-----|---------|----------|
| tool | medium | `internal/platform/tool/tool.go` | erro de compilação de schema deixava de validar silenciosamente → aflora explícito; +teste enum |
| scorer | medium | `internal/platform/scorer/runner.go` | `WithWorkers` no-op (8 fixo) → honra contagem; +métrica `outcome="dropped"` |
| agent | low | `internal/platform/agent/stream.go` | leak de goroutine no cancel mid-stream → drain seleciona `ctx.Done()` |
| agent | low | `internal/platform/agent/runtime.go` | runs falhos não contavam `agent_runs_total` → métrica exatamente-uma-vez em `closeRun` |
| agent | low | `internal/platform/agent/agent.go` | `AfterExecute` não disparava no streaming → corrigido |
| conformance | medium | `test/conformance/weather/conformance_test.go` | cenários ausentes "structured" + working-memory (8.5) → adicionados, determinísticos |
| docs | low | `.env.example` | `RUN_REAL_LLM` documentado |

Prior bugfixes confirmados reais no código (não regredidos): B1, B3, B4, B5, L1, M1, M3, M4.

## Migrations — decisão do usuário: unificar TODAS (produção 0 usuários)

`000001`+`000002`+`000003` colapsados em um único `000001_initial_schema.{up,down}.sql`. Zero `agent_*`,
`platform_*` + pgvector + HNSW + seeds. **Zero-loss provado por schema-diff vazio** vs o estado final
das 3 migrations; paridade de seeds 3/139/530/1. Corrigido bug pré-existente de bookkeeping
(`schema_migrations` relocado para `public`), destravando `TestBaselineUpDownUp`. Ver
`docs/runs/2026-06-29-unificar-migrations.md`.

## Reds pré-existentes (módulos NÃO tocados por este branch) — corrigidos a pedido do usuário

- `internal/categories` `TestListPagination`: bug real de collation (WHERE default vs ORDER BY
  `pt-BR-x-icu`) → keyset agora collation-consistente; teste reforçado (sem duplicata E sem lacuna).
- `internal/transactions` `TestMaterializeRecurringForDaySuite` + `TestRecurringMaterializerJobIntegrationSuite`:
  date-rot (hoje dia 29 > `DayOfMonth` 1..28) → testes ancorados a data fixa (dia 15), date-robust.
  Produção já correta (R-TXN-001: `now` é parâmetro).

## Validações finais (reais)

- `go build ./...` → OK
- `gofmt -l` (tracked) → vazio
- `task gates:platform` → 5/5 PASS · `task gates:no-internal-agent` → PASS
- `go test -tags=integration ./...` (CI merge gate completo) → **0 FAIL / 0 build-failed**
- Conformance + persistência real (pgvector Postgres testcontainers) → verde

## spec_coverage

RF-01..RF-46: atendidos com evidência file:line (ver relatórios dos 6 reviewers). Critérios de
sucesso das 9 tasks: atendidos. DoD platform-mastra: 100% (suite verde, gates verdes, migration
reversível, `internal/agent` ausente, workflow kernel preservado).

## Riscos residuais (low, não bloqueiam)

- Streaming standalone não roteado pelo Run auditável de agent (coberto como Run de workflow quando
  usado como step) — deferral documentado.
- Scorer in-flight não cancelável no shutdown (fire-and-forget por design).
- `platform_embeddings.thread_id` guarda o PK do thread; dimensão do vetor não validada na fronteira.

## Validação RUN_REAL_LLM (OpenRouter real + Postgres real)

Executada com credenciais de `.env` (`RUN_REAL_LLM=1`, google/gemini-2.5-flash-lite,
openai/text-embedding-3-small). **Expôs um bug real que TODOS os testes com mock não pegaram:**

- **Bug (high — RF-01/RF-03/RF-09):** `agent.Execute` fazia um único round-trip; quando o modelo
  retornava tool call, invocava a tool mas descartava o resultado e devolvia `Content` vazio. Loop
  agêntico não implementado. `TestWeatherAgent_Execute_Sync` falhava com o modelo real.
- **Correção (causa raiz):** `llm.Message` estendida (`ToolCalls`/`ToolCallID`/`Name`); serialização
  OpenRouter de mensagem `role:"tool"` + assistant `tool_calls`; loop agêntico em `agent.Execute`
  (`completeWithTools`, cap `maxToolRounds=5`, `ErrMaxToolRounds`); hooks/métricas/decoder
  preservados. Teste de regressão **determinístico** (mock: tool call → resultado → resposta final)
  para o CI pegar sem rede. Stream mantido single round-trip (consumidor planActivities não usa tool
  no stream) — residual documentado.

Resultado final real-LLM: `TestWeatherConformanceSuite` **PASS** completo (22s) — sync, stream,
ThreadRun, workflow agent-como-step, structured output, scorers, working memory injection.

## next_action

APPROVED. Merge gate determinístico verde (145 ok / 0 fail) + RUN_REAL_LLM verde + lint/vet/gofmt/
gates verdes. Commit não realizado (sem pedido explícito). Residual único: Stream sem loop de tool
(não exercitado pelo consumidor; sync corrigido).
