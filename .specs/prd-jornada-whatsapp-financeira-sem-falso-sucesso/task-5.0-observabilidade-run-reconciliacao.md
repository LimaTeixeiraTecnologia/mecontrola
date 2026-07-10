# Tarefa 5.0: Observabilidade de run — continuers, reconciliação e status distinguível

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Fechar as lacunas de observabilidade da jornada financeira: o erro de fechamento de run engolido nos três continuers de resume passa a ser tratado uma única vez (log estruturado + métrica de cardinalidade controlada), a concordância de estado (RF-16) vira invariante testável via query de reconciliação read-only, e a ausência de status de entrega outbound do WhatsApp torna-se consultável e distinguível de falha de envio/persistência. Nenhum falso sucesso de observabilidade: falha de `Update` nunca fica invisível nem contamina o resultado de negócio já entregue. Detalhes em `techspec.md` e `adr-005-correlacao-wamid-e-run-update-observavel.md` — não duplicar aqui.

<requirements>
- RF-13 (correlation_key = WAMID na retomada de pendência), RF-14, RF-15, RF-16, RF-19, RF-22, RF-23, RF-24 conforme `prd.md`.
- Decisões firmes do ADR-005 (fechamento observável nos 3 continuers, reconciliação por invariante, padronização de campos) e D-04.
- Sem novo design pattern GoF — validação de fronteira + tratamento de erro único + query de auditoria (refactor local).
- Métrica `agents_run_update_errors_total` com labels FECHADOS `workflow`/`stage`/`status`; NUNCA `user_id`/`wamid`/`correlation_key` como label (herda R-TXN-004 / R-AGENT-WF-001.5).
- `MessageDeliveryState` como tipo fechado enumerado (DMMF state-as-type); predicado puro no use case.
- Query de reconciliação read-only como leitura invocada por use case — adapter fino (RF-28, R-ADAPTER-001.2); SQL apenas no adapter postgres.
- Zero comentários em Go de produção (R-ADAPTER-001.1); `errors.Join`/wrapping `%w` preservados.
- Manter os comentários HTML guard-rail deste arquivo (não são código Go).
</requirements>

## Subtarefas

- [ ] 5.1 (Continuers — fechamento observável) Substituir o swallow `_ = c.runs.Update(...)` por tratamento ÚNICO do erro nos três continuers: `internal/agents/application/usecases/pending_entry_continuer.go` (`closeRun` ~L297-307), `card_create_confirm_continuer.go` (~L167), `budget_creation_continuer.go` (~L171). Cada caminho emite log estruturado (`run_id`, `wamid`, `workflow`, `stage`) + incrementa a nova métrica `agents_run_update_errors_total` com labels fechados `workflow`/`stage`/`status`. Espelhar o `closeRun` central do runtime; NÃO propagar o erro ao usuário (run é telemetria; resultado de negócio já entregue).
- [ ] 5.2 (Métrica) Declarar `agents_run_update_errors_total` (Counter) com os três labels fechados; garantir que nenhum valor sensível (`user_id`, telefone, email, WAMID, `correlation_key`, texto de mensagem) seja usado como label. Manter o conjunto de `workflow` como valores fechados (R4 do ADR-005).
- [ ] 5.3 (RF-16 — reconciliação) Criar `internal/agents/infrastructure/postgres/audit_reconciliation.go` (adapter fino, leitura read-only) + use case que o invoca. Cruzar `platform_runs × workflow_runs × agents_write_ledger × transactions × platform_scorer_results` por `correlation_key`/`wamid`/`run_id` usando as colunas confirmadas (R3 do ADR-005): `workflow_runs.correlation_key`+`status`+`state->>'status'`; `agents_write_ledger.wamid = platform_runs.correlation_key`; `platform_scorer_results.run_id → platform_runs.id`; `transactions` via `agents_write_ledger.resource_id = transactions.id`. Verificar as invariantes: `correlation_key != ''`; `succeeded`/`routed` ⇒ efeito no ledger OU workflow `succeeded`; `failed` ⇒ sem write órfão; `wf_status` (kernel) e `run_status` (agent) não divergem (comparar por mapeamento `String()` explícito, R5). Materializar como TESTE de invariante de integração (0 violações), NÃO como dashboard.
- [ ] 5.4 (RF-19 — status outbound) Adicionar `LookupDeliveryState(ctx, messageID) (MessageDeliveryState, error)` e o tipo fechado `MessageDeliveryState` (`not_received`/`failed`/`delivered`) no pacote `internal/platform/whatsapp/status`. Query read-only (total + count filtrado por `failed`); predicado puro no use case decide o estado a partir das contagens. Ausência de status ⇒ `not_received`, distinguível de falha de envio/persistência. Mecanismo de `record_message_status` existente permanece funcional e observável.
- [ ] 5.5 (RF-22/23/24 — padronização) Padronizar os campos de log/span `run_id`/`wamid`/`workflow`/`stage`/`status` nos três continuers (e coerentes com o runtime central); rodar o gate `grep` confirmando que nenhum label de métrica carrega dado sensível.
- [ ] 5.6 (Testes) Ver seção "Testes da Tarefa".

## Detalhes de Implementação

Ver `techspec.md` (seções "Estado de entrega distinguível (RF-19)", "Testes de Integração", "Monitoramento e Observabilidade") e `adr-005-correlacao-wamid-e-run-update-observavel.md` desta pasta — **referenciar em vez de duplicar**. O ADR-005 fixa: o fechamento observável nos 3 continuers (Decisão 2, espelhando o `closeRun` central do runtime que já trata o erro), a query de reconciliação como invariante testável e não dashboard (Decisão 3, com colunas de join confirmadas em R3 e mapeamento de status por `String()` em R5), e a padronização de campos + gate de labels sensíveis (Decisão 5). Escopo desta tarefa exclui a validação de fronteira `InboundRequest.Validate`/`ErrEmptyMessageID` e a migration/backfill/CHECK (tarefas 3.0 e 7.0). RISCO herdado do ADR-005: manter o conjunto de `workflow` IDs fechado no label (R4) e comparar estados do kernel vs. agent por mapeamento explícito (R5), nunca por igualdade crua.

## Critérios de Sucesso

- Falha de `Update` injetada no fechamento ⇒ incrementa `agents_run_update_errors_total` E emite log com `run_id`/`wamid`/`workflow`/`stage`, sem silenciar e sem quebrar o resultado de negócio já entregue ao usuário.
- Query de reconciliação retorna 0 violações após um fluxo financeiro bem-sucedido E detecta o caso negativo (run `failed` com write pré-existente / `correlation_key` vazio / divergência de status).
- `LookupDeliveryState` distingue os três estados: ausência de status ⇒ `not_received`; status `failed` ⇒ `failed`; status de entrega ⇒ `delivered`.
- `agents_run_update_errors_total` usa apenas labels fechados `workflow`/`stage`/`status`; gate `grep` de labels sensíveis retorna limpo.
- Gates Go verdes no escopo alterado (build, vet, `test -race`, lint quando disponível) e gates de governança limpos (R-ADAPTER-001, R-AGENT-WF-001.5, DMMF state-as-type).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — Run auditável, continuers de resume, métricas de cardinalidade controlada do substrato.
- `postgresql-production-standards` — query de reconciliação read-only e leitura de status conforme documentação oficial PostgreSQL.

## Testes da Tarefa

- [ ] Testes unitários — predicado puro de `MessageDeliveryState` (contagens ⇒ `not_received`/`failed`/`delivered`, `IsValid`); continuers com `fake.NewProvider()` e suíte `testify/suite` table-driven (R-TESTING-001): erro de `Update` injetado ⇒ log + incremento da métrica, sem propagar erro de negócio; caminho feliz sem incremento.
- [ ] Testes de integração — Postgres (`//go:build integration`, testcontainers): `correlation_key` preenchido com WAMID após routed e resume; falha de `Update` observada (incrementa métrica + log, sem silenciar); reconciliação retorna 0 violações após fluxo ok e detecta o caso negativo; status `LookupDeliveryState` distingue `0`/`failed`/`sent` ⇒ `not_received`/`failed`/`delivered`.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/usecases/pending_entry_continuer.go` — `closeRun` (~L297-307), swallow `_ = c.runs.Update(...)` → tratamento único (log + métrica).
- `internal/agents/application/usecases/card_create_confirm_continuer.go` — fechamento de run (~L167), mesmo tratamento.
- `internal/agents/application/usecases/budget_creation_continuer.go` — fechamento de run (~L171), mesmo tratamento.
- `internal/agents/infrastructure/postgres/audit_reconciliation.go` — NOVO adapter fino read-only da query de reconciliação (RF-16).
- `internal/platform/whatsapp/status/record_message_status.go` — use case de status; adicionar `LookupDeliveryState` + predicado puro (RF-19).
- `internal/platform/whatsapp/status/postgres/repository.go` — leitura read-only (total + count `failed`) para o estado de entrega.
- `internal/platform/whatsapp/status/types.go` — tipo fechado `MessageDeliveryState` (`not_received`/`failed`/`delivered`).
