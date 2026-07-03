# Relatorio de Auditoria de Producao

Auditoria read-only, orientada a evidencias, executada em 2026-07-03T08:57:42Z.
Alvo: usuario `06edc407-4f63-42e8-b07c-946b9ef0a19c` / `jailton.junior94@outlook.com`.
Metodo: 5 subagentes especializados (DB, Logs/Loki, Tracing/Tempo, Metrics/Prometheus, Reconciliacao codebase) + verificacao de codigo no root.

---

## 1. Escopo auditado

- **Alvo**: user `06edc407-4f63-42e8-b07c-946b9ef0a19c`, email `jailton.junior94@outlook.com` (confirmado: id + email batem; criado 2026-07-01T19:59:31Z).
- **Janela**: 2026-06-26T08:57:42Z .. 2026-07-03T08:57:42Z (168h, UTC; servidor `srv1761537` em Etc/UTC).
- **Fontes**: Postgres 16.14 (schema `mecontrola`, 50 tabelas), Loki, Tempo, Prometheus (otel-lgtm 0.7.5), codebase root local + deploy remoto.
- **Deploy em execucao**: imagem `mecontrola:571425f` nos 4 app containers (server-1/2, worker-1/2) + migrate. DB migrado ate `000001` apenas.
- **Codebase root (main)**: HEAD `8bd5ad6`.

## 2. Metodologia aplicada

- Acesso: `ssh root@187.77.45.48`; psql via `docker exec` (trust); observabilidade via proxy anonimo do Grafana (`/api/datasources/proxy/uid/{loki,prometheus,tempo}`).
- Validacao cruzada minima de 2 fontes por afirmacao quando tecnicamente possivel.
- Limites: prod = 1 usuario real / ledger vazio; logs sem `user_id`; traceparent nao propagado no handoff assincrono (em prod); ~32h finais sem metrica de request-path (sistema ocioso).

## 3. Identificacao do usuario e correlacoes

- `select * from users where id=...` -> 1 linha, email confere, criado 2026-07-01T19:59:31Z.
- Correlatos: 1 `billing_subscriptions` ACTIVE, 1 `identity_entitlements` ACTIVE (period_end 2026-07-31), token onboarding CONSUMED, 1 budget `2026-07` ACTIVE (R$8.000, 5 alocacoes = 100%), 1 thread + 16 turnos de agente (16 user + 16 assistant), workflow de onboarding `succeeded` (8 steps completos).

## 4. Evidencias por fonte

### 4.1 Banco de dados — anomalies-found
- **Infra limpa**: outbox 118/118 publicados (0 pending/dead-letter), dedup 42/42 message_ids distintos, 0 auth failures, todos runs/workflows terminais, budget math exato.
- **A1 (HIGH)**: agente respondeu "Despesa registrada com sucesso R$150,00" (21:15:08Z) e "R$1.500,00 ... registrada com sucesso" (21:55:12Z) e tratou salario de R$13.500, mas `transactions`=0, `agents_write_ledger`=0, `budgets_expenses`=0. Confirmacao de escrita alucinada — em producao, no usuario-alvo.
- **A2 (MEDIUM)**: agente disse "nao encontrei plano orcamentario para julho de 2026" (21:15:59Z) mas o budget `2026-07` existe e esta ACTIVE. Read path do app falho (em prod).
- **A3 (MEDIUM)**: 13 webhooks Kiwify validos recebidos, apenas 1 aplicado; reconciliacao nunca rodou (`billing_reconciliation_checkpoints`=0).
- **A4 (LOW)**: entitlement pendente orfao nao limpo pos-apply. **A5 (LOW)**: skew paid_at/consumed_at (+3h BRT-as-UTC).

### 4.2 Logs (Loki) — problems-found (nenhum bloqueante para o usuario)
- Retencao cobre 168h. Sem `user_id`/`email`/`message_id` em nenhuma linha (gap de observabilidade).
- Usuario-alvo: unico erro correlacionado = "card: nickname already in use" (validacao benigna, 20:32:20Z). 0 panics/fatal/5xx apos criacao.
- **P1 (HIGH, ongoing)**: `billing.reconciliation` falha de hora em hora com **Kiwify HTTP 401** durante toda a janela, ainda falhando no fim (2026-07-03T08:00Z, 91 ocorrencias). Credencial invalida/expirada. Nao especifico do usuario.
- P2/P3/P4 (migrations ausentes, DB em recovery) reais mas auto-resolvidos ate 2026-06-28, antes do usuario existir.

### 4.3 Tracing (Tempo) — problems-found
- Instrumentacao do worker e boa (Thread->Run->WorkingMemory->LLM->binding->usecase->repo DB->workflow store). Usuario atribuivel no span `auth.resolve_principal` (ingress).
- **Circuito causal quebrado (em prod)**: ingress HTTP e consumer do worker sao traces disjuntos — traceparent nao propagado no handoff do outbox. Spans orfaos de `llm.complete`/`llm.embed`. `user_id` ausente nos spans do worker.
- Unico erro real: 7 traces de 401 Kiwify (mesma causa do P1). Latencia agente p50 3.07s / p95 4.35s.

### 4.4 Metrics (Prometheus) — healthy-but-partially-observable
- **Higiene de labels: PASS** (sem `user_id`/`category_id`; `outbox_events_inserted_total` usa `has_user_id` booleano). 0 provider errors, 0 fallback exhausted, 0 persistence-failure series, 0 deadlocks, sem saturacao infra.
- **1 dead-letter** `agents.whatsapp.inbound.v1` no redeploy de 2026-07-01T20:00Z (nao sistemico).
- Gaps: ~32h finais sem metrica de request-path (sistema ocioso); `increase()` inutil por 228 resets/7d; **sem metrica de consumer-lag / idempotency-replay / ordering/claim** — o proprio caminho de ordenacao/idempotencia nao e comprovavel por metrica.

### 4.5 Codebase / deploy — producao ATRAS do root
- `git merge-base --is-ancestor 38b64f1 571425f` -> **false**. Deploy `571425f` esta **5 commits atras** do `main` (`8bd5ad6`).
- Commits no root ausentes em prod: `8bd5ad6` (tools do agente), `01ebc94` (docs), **`38b64f1` (whatsapp: ordenacao + idempotencia + confirmacao honesta)**, `64fdd8c` (docs), `18da009` (card).
- DB de prod aplicou so `000001`; faltam `000002` (card) e **`000003` (indice parcial `outbox_events_user_inflight_uidx` — backstop de serializacao)**.
- Todas as 9 tarefas do PRD whatsapp-ordenacao estao `done` no root, codigo verificado.

## 5. Matriz exaustiva de bugs, gaps, lacunas e ressalvas

| item | descricao | fonte_historica_ou_sintoma | evidencia_no_codigo (root) | evidencia_em_producao | contradicoes | status_final |
|---|---|---|---|---|---|---|
| B1 | WhatsApp fora de ordem (sem serializacao por usuario) | PRD whatsapp-ordenacao | `storage_postgres.go:63-93` claim particionado + ordem `(occurred_at,created_at,id)` | ausente em `571425f` | — | **nao resolvido em prod / resolvido no root** |
| B2 | Unicidade in-flight (23505) | idem | `storage_postgres.go:95-100` + migration `000003` | indice `000003` nao aplicado no DB de prod | — | **nao resolvido em prod / resolvido no root** |
| B3 | Idempotencia default-on (natural key + reconciled/replay) | idem | mapeamento `ToolOutcomeReconciled`/`Replay` nas write-tools | ausente em prod | — | **nao resolvido em prod / resolvido no root** |
| B4 | Advisory lock de sessao quebrado sob pgbouncer | idem | lock de sessao removido (task 5.0) | remocao nao aplicada em prod | — | **nao resolvido em prod / resolvido no root** |
| B5 | TOCTOU onboarding | idem | `engine.go:111-171` Start idempotente->resume | ausente em prod | — | **nao resolvido em prod / resolvido no root** |
| B6 | Sucesso de escrita alucinado (confirmacao honesta) | A1 (prod, usuario-alvo) | guard `writeToolGuardFailed` so virava status de auditoria; consumer enviava `outcome.Content` cru sem gate | A1 confirmado em prod | guard nao cobria reply ao usuario | **CORRIGIDO no root (2026-07-03); nao resolvido em prod ate deploy** |
| B7 | Read path de budget falho | A2 (prod) | tool `query_plan` (registrada em `module.go:282`), ausente em prod | A2 confirmado em prod | — | **resolvido no root; nao resolvido em prod ate deploy** |
| B8 | Reconciliacao Kiwify falha (401) | P1/traces (ongoing) | credencial, nao codigo | 91 falhas na janela, ativa no fim | deploy do root NAO corrige credencial | **nao resolvido (operacional)** |
| B9 | 12/13 webhooks validos sem efeito | A3 (prod) | — | so 1 aplicado; reconciliacao nunca rodou | pode ser ruido de sandbox | **nao comprovado** |
| B10 | Traceparent nao propagado (circuito causal) | Tempo | `outbox/publisher.go` injeta traceparent no Metadata; consumer extrai (ambos testados) | traces disjuntos em prod | — | **resolvido no root; nao resolvido em prod ate deploy** |
| B11 | Sem metrica de ordering/idempotency/consumer-lag | Metrics | — | ausente | mesmo pos-deploy nao comprova ordenacao | **lacuna de observabilidade** |
| B12 | Sem `user_id` em logs; user-creation nao logada | Logs | — | ausente | — | **lacuna de observabilidade** |

## 6. Achados negativos (o que impede declarar 100%)

1. **Producao roda codigo anterior as correcoes** (`571425f`, 5 commits atras). Nenhum dos fixes B1-B5/B7/B10 esta em prod; DB de prod sem `000002`/`000003`. Zero evidencia operacional de que os mecanismos novos funcionam sob carga real — nunca rodaram la.
2. **A1/B6** era gap de codigo genuino do root (confirmacao honesta nao-estrutural); **corrigido em 2026-07-03** (ver secao seguinte). Residuo consciente: cenario em que o LLM alega sucesso sem chamar a write-tool nao e deterministicamente detectavel por texto livre.
3. **B8 — 401 Kiwify** e operacional (credencial); deploy do root nao corrige.
4. **Comprovacao impossivel hoje**: sem metrica de ordering/idempotency/consumer-lag (B11), logs sem `user_id` (B12), 32h finais sem metrica de request-path. O gate de carga sintetica (task 9.0 do PRD) nao foi executado em prod.
5. **A3/A4/A5** com causa-raiz nao totalmente disambiguada apenas por DB.

## 7. Veredito final

**`nao foi possivel comprovar 100%`.**

- O **codebase root corrige as causas-raiz** de ordenacao, unicidade in-flight, idempotencia, advisory-lock-sob-pgbouncer, TOCTOU onboarding, read de budget e propagacao de traceparent (B1-B7, B10). Com a correcao de 2026-07-03, tambem a confirmacao honesta (B6).
- Porem: (a) **producao nao esta rodando esse codigo** — esta 5 commits atras, sem as migrations `000002`/`000003`; (b) ha **defeito operacional ativo (401 Kiwify, B8)** independente de deploy; (c) **nao ha observabilidade** para comprovar o caminho de ordenacao/idempotencia mesmo apos deploy.

## 8. Criterios de aceite

- **0 gaps comprovados?** NAO. B8 (401 Kiwify) e gap operacional ativo; B6 corrigido mas ainda nao em prod.
- **0 lacunas comprovadas?** NAO. B11/B12 (sem metrica de ordering/idempotency; sem user_id em logs) sao lacunas de observabilidade.
- **0 desvios comprovados?** NAO. Producao esta 5 commits + 2 migrations atras do root.
- **0 falso positivo nesta analise?** SIM — cada afirmacao tem SQL/PromQL/LogQL/TraceQL ou `file:line`; incertezas marcadas como "nao comprovado".
- **Production-ready com prova suficiente?** NAO. O root corrige B1-B7/B10 e agora B6, mas B8 e operacional e a comprovacao exige deploy do `main` + migrations `000002`/`000003` + carga sintetica (task 9.0) + metricas de ordering/idempotency inexistentes.

## Correcao aplicada no codebase root (2026-07-03, pos-auditoria)

Fechado o unico gap de codigo genuino do root: **B6 — confirmacao honesta ao usuario**
(go-implementation Etapas 1-5 + DMMF state-as-type). Defesa em profundidade:

1. **Plataforma (`internal/platform/agent`)** — invariante de honestidade na fonte:
   - `runtime.go` `finishRun`: run nao-`RunStatusSucceeded` nunca vaza o texto do modelo
     (`content = ""` quando `runStatus != RunStatusSucceeded`). Protege todo consumidor.
   - `ports.go`: predicado puro `Outcome.Succeeded()` (derivado do estado fechado `RunStatus`).
2. **Consumidor (`internal/agents/.../consumers/whatsapp_inbound_consumer.go`)** — gate observavel:
   - `handleAgentInbound` so envia `outcome.Content` quando `outcome.Succeeded()`; caso contrario
     envia o fallback honesto (`fallbackReply`) com label de metrica `outcome="not_confirmed"`.
   - `sendReply` passa a rotular a entrega (`success`/`no_reply`/`not_confirmed`), tornando a supressao
     de confirmacao alucinada observavel.
3. **Testes (regressao)**:
   - `runtime_test.go` RF38: run com write-tool que falhou + texto "Despesa registrada com sucesso"
     agora exige `outcome.Content` vazio e `!Succeeded()`.
   - `whatsapp_inbound_consumer_test.go`: cenario "suprimir confirmacao alucinada quando run falhou"
     exige que o gateway receba o fallback honesto, nunca o texto de sucesso alucinado.

Validacao: `go build ./...` ok; `go vet` ok; `go test -race` nos pacotes tocados ok; gofmt limpo;
zero comentarios (R-ADAPTER-001.1) ok; sem `init()` (R0) ok. Residuo consciente: cenario (a) — LLM
alega sucesso **sem** invocar a write-tool — nao e deterministicamente detectavel por texto livre;
mitigado (audit honesto + supressao no gate quando o run falha), mas o fechamento total exige
confirmacao de escrita derivada do outcome (reply templado) + hardening de prompt/eval.

Itens ja corrigidos no root (so faltam em prod por defasagem de deploy), confirmados nesta rodada:
A2 read de budget via `query_plan` tool (registrada em `module.go:282`); B10 propagacao de traceparent
no outbox (`publisher.go` injeta, consumer extrai, ambos testados). B8 (401 Kiwify) e operacional
(credencial), nao codigo. B11/B12 (metricas de ordering/idempotency/consumer-lag; `user_id` em logs)
permanecem como enhancements de observabilidade.

## Recomendacao objetiva (nao implementada — auditoria read-only)

1. Deployar `main` (`8bd5ad6`) e rodar migrations `000002`/`000003`.
2. Corrigir credencial Kiwify (B8) — operacional.
3. Adicionar metricas de ordering/claim/idempotency-replay/consumer-lag para tornar a resolucao comprovavel.
4. Executar o gate de carga sintetica (task 9.0) em staging/prod antes de declarar comprovado.
5. Fechar o residuo do cenario (a) do B6 com confirmacao de escrita derivada do outcome (reply templado).
