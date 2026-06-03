# Prompt mandatório — execute-all-tasks para `.specs/prd-outbox-event-driven`

```text
[PAPEL OU POSTURA]
Atue como orquestrador determinístico do fluxo `ai-spec` em PT-BR, com foco máximo em governança, rastreabilidade e zero degradação silenciosa. Você NÃO é o implementador — você é o coordenador que delega cada tarefa a um subagent fresh via `execute-task` e consolida o relatório agregado.

[OBJETIVO]
Use a skill `execute-all-tasks` (`.agents/skills/execute-all-tasks/SKILL.md`, fonte de verdade canônica; espelhos em `.claude/skills/`, `.github/skills/`, `.gemini/skills/`, `.codex/skills/`) para executar de ponta a ponta TODAS as 10 tarefas do PRD da fundação Outbox transacional em `.specs/prd-outbox-event-driven/`, respeitando o DAG já declarado, paralelizando apenas onde explicitamente permitido em `tasks.md`, com halt-first no primeiro retorno não-`done`, retomada idempotente a partir de checkpoints e relatório agregado final em `_orchestration_report.md`.

[ENTRADAS OBRIGATÓRIAS]
- Slug do PRD: `outbox-event-driven`.
- PRD: `.specs/prd-outbox-event-driven/prd.md` (spec-hash atual no header de `tasks.md`: `d87cd9fb18697e38e526c80ad8f8a6f474ff01306f5bbf0bbe3c127c79d225f2`).
- Tech spec: `.specs/prd-outbox-event-driven/techspec.md` (spec-hash atual: `ae6b6ca92511186afd50cd37d775c289f4e8294fc903286bb77f8c5803209f18`).
- Plano de tarefas: `.specs/prd-outbox-event-driven/tasks.md` (10 tarefas; cobertura RF-01…RF-40 já validada via `ai-spec check-spec-drift`).
- Arquivos por tarefa (caminho relativo à raiz): `.specs/prd-outbox-event-driven/task-1.0-foundation-migration-mockery-config.md` … `task-10.0-dashboard-runbook-adr-agents-pr-template.md`.
- ADRs locais: `.specs/prd-outbox-event-driven/adr-001-schema-two-table.md` até `adr-005-mockery-yml-creation.md`.
- Descobertas (somente leitura, não modificar): `docs/discoveries/brainstorm-event-driven-outbox-foundation/`, `docs/discoveries/technical-outbox-event-driven-foundation/`.
- Governança transversal: `AGENTS.md` raiz, `.claude/rules/governance.md`, `.agents/skills/agent-governance/`, `.agents/skills/go-implementation/`.

[GRAFO DECLARADO EM tasks.md — RESPEITAR LITERALMENTE]
- Caminho crítico: 1.0 → 2.0 → 3.0/4.0 → 5.0/6.0/7.0 → 8.0 → 9.0 → 10.0.
- Paralelismo permitido (coluna `Paralelizável`): 3.0 ‖ 4.0; 5.0 ‖ 6.0 ‖ 7.0 (apenas após 3.0 e 4.0 estarem `done`).
- Skills processuais declaradas: 1.0 → `taskfile-production`; 10.0 → `otel-grafana-dashboards`; demais → auto-carregadas (`agent-governance` + `go-implementation`) inferidas pelo diff.

[GATES DE PRÉ-VOO MANDATÓRIOS — NÃO SE PODE PULAR]
1. Invocar `bash .claude/hooks/pre-execute-all-tasks.sh outbox-event-driven` (cascata portátil `.agents/hooks/` → `.claude/hooks/` → `.gemini/hooks/` → `.codex/hooks/` → `.github/hooks/`). Exit ≠ 0 → `failed` repassando stderr. Hook ausente em todos os caminhos → `failed: hook de governança 'pre-execute-all-tasks.sh' ausente — reinstale via 'ai-spec install'`.
2. `unset AI_PREFLIGHT_DONE` antes de qualquer outro comando.
3. Resolver lib de profundidade `check-invocation-depth.sh` na cascata `.agents/lib/` → `scripts/lib/` e `source` no shell do orquestrador. Ausente nas duas → `failed: depth lib missing — vendor a lib ou rode 'ai-spec install'`.
4. Validar binário `ai-spec` no PATH (`command -v ai-spec`). Ausente → `needs_input` com instrução de instalação (`brew install ai-spec-harness` neste host). **Proibido degradar silenciosamente para "modo legado".**
5. `ai-spec skills --verify` — `blocked` se houver drift de skill.
6. `ai-spec check-spec-drift .specs/prd-outbox-event-driven` — `blocked` se algum RF não estiver coberto ou se spec-hash divergir.
7. Confirmar existência de `prd.md`, `techspec.md`, `tasks.md` e dos 10 `task-X.0-*.md`. Qualquer ausência → `needs_input`.

[CONTRATO YAML ESTRITO DO SUBAGENT — TODA TAREFA RETORNA EXATAMENTE]
- ```yaml
  status: done | blocked | failed | needs_input
  report_path: .specs/prd-outbox-event-driven/<id>_execution_report.md
  summary: <uma linha, sem diffs/código/logs>
  ```
- `report_path` DEVE ser relativo à raiz do repositório. Absoluto ou relativo ao subdir do subagent é rejeitado.
- Campos extras, duplicados, comentários ou texto livre fora do bloco → `failed: contract violation`.
- Para `status: done`: o `report_path` deve existir, não ser vazio e o status em `tasks.md` ter migrado para `done`. Qualquer divergência → `failed: missing evidence` ou `failed: status drift`.
- YAML ausente/corrompido: tentar recuperar `.specs/prd-outbox-event-driven/.checkpoints/<id>.yaml` (escrito pelo `execute-task` Stage 5.3). Ausente lá também → `failed: no return and no checkpoint`.

[PROMPT PADRÃO INJETADO EM CADA SUBAGENT]
- Paths absolutos dos 3 documentos (task file, prd, techspec) + caminho do tasks.md.
- "Invoque `execute-task`. Carregue APENAS o necessário. Não saia do escopo da tarefa."
- "`export AI_INVOCATION_DEPTH=0`; resolver `check-invocation-depth.sh` em cascata e `source`."
- "`export AI_PREFLIGHT_DONE=1` — orquestrador já validou; pule os gates duplicados."
- Sem diffs, sem código, sem logs no retorno — somente o YAML descrito acima.

[LOOP TOPOLÓGICO — WAIT-ALL-THEN-HALT]
- Repetir até `pending == 0` ou halt:
  1. Re-ler `tasks.md` do disco a cada iteração (subagents podem ter mutado).
  2. `ready = { t : t.status == "pending" E todas dep(t).status == "done" }`.
  3. `ready == ∅` com `pending > 0` → `failed: cycle or orphan dependency`.
  4. Compor wave: se houver alguma `Paralelizável == Não` em `ready`, executar só ela; senão, executar todas as `Com <ids>` em paralelo.
  5. Verificar suporte do tool a paralelismo nativo. Sem suporte → degradar para sequencial e registrar `degradado: tool sem spawn paralelo nativo` no relatório agregado.
  6. Spawnar TODOS os subagents da wave; aguardar TODOS concluírem antes de decidir halt (evita race em writes concorrentes de `tasks.md`).
  7. Aplicar cadeia de validação a cada YAML retornado.
  8. Qualquer `status ≠ done` após validação → halt imediato; consolidar wave atual; reportar.

[CHECKPOINT E RELATÓRIO]
- Ao final de cada wave validada: `bash .claude/hooks/post-wave.sh outbox-event-driven <wave-id> <results-yaml-file>` (mesma cascata). Hook escreve `.specs/prd-outbox-event-driven/_orchestration_report.partial.md` append-only.
- Próxima invocação detecta `.partial.md` na Etapa 1; consolida com `tasks.md` atual; usa como ponto de partida (retomada idempotente).
- Ao concluir todas as waves sem halt: rename atômico `.partial.md` → `_orchestration_report.md`. Renderizar com template em `.agents/skills/execute-all-tasks/assets/` contendo:
  - Snapshot inicial vs final de status por tarefa.
  - Lista de waves executadas e duração de cada.
  - Tarefas executadas, puladas (já `done`) e bloqueadas.
  - Próximos passos sugeridos quando `partial`.
  - Para Codex/Gemini/Copilot CLI: registrar se kill no timeout foi efetivo (`killed`) ou apenas soft-discard (`discarded`).

[RESTRIÇÕES INVIOLÁVEIS]
- Toda tarefa em subagent fresh — orquestrador NUNCA executa `execute-task` inline.
- Orquestrador NUNCA muta `tasks.md` diretamente — só subagents via `execute-task` Stage 5.
- Não coordenar arquivos entre subagents paralelos — confiar literalmente na coluna `Paralelizável`.
- Não re-executar tarefas com status `failed`, `blocked` ou `needs_input` — respeitar o retorno do subagent.
- Não pular `ai-spec check-spec-drift` mesmo que a sessão anterior tenha rodado — drift pode ter surgido por edição manual do PRD ou techspec.
- Não usar `git` destrutivo em nenhum momento (sem `reset --hard`, sem `push --force`, sem `--no-verify`).
- Não introduzir skills fora das declaradas em `tasks.md` (`taskfile-production` em 1.0, `otel-grafana-dashboards` em 10.0). Skills auto-carregadas (`agent-governance`, `go-implementation`) entram via `execute-task` Stage 2 por inferência do diff.
- Não criar broker externo (RabbitMQ, Kafka, NATS) — o escopo é Postgres-only conforme R-03 do PRD.
- Não revogar o `events.Bus` (ADR-003) — Publisher do Outbox coexiste como caminho alternativo opt-in (RF-05).
- Não alterar `cmd/server` — escopo de execução é exclusivamente `cmd/worker` (FC-03).
- Não baixar dependência diferente de `github.com/robfig/cron/v3@v3.0.1` (D-04).

[SAÍDA ESPERADA]
- Status final agregado: `done` (todas as 10 tarefas em `done`) | `partial` (alguma não-done com motivo explícito) | `failed` (pré-voo abortou) | `needs_input` (algum dado obrigatório faltando).
- Arquivo `.specs/prd-outbox-event-driven/_orchestration_report.md` materializado no caminho relativo correto.
- `tasks.md` com `Status` de cada linha refletindo o estado real após a execução; spec-hashes preservados no header (não tocar).
- Para cada tarefa concluída: arquivo `.specs/prd-outbox-event-driven/<id>_execution_report.md` existindo, não-vazio, referenciando os arquivos alterados, comandos de validação executados e link para a entrada correspondente no `tasks.md`.

[VARIÁVEIS DE AMBIENTE OPCIONAIS — REGISTRAR NO REPORT SE USADAS]
- `AI_TASK_TIMEOUT_SECONDS` (default 1800s; override por tarefa via `<!-- task-timeout-seconds: N -->`).
- `AI_TASK_TOKEN_BUDGET` (default 0 = ilimitado; zero preserva F1).
- `AI_VALIDATE_GIT_HISTORY=1` (F35: para cada `done`, validar `DiffSHA` no `git cat-file`).
- `AI_TASKS_ROOT` / `AI_PRD_PREFIX` — manter defaults `.specs` / `prd-` (caminho do PRD é `prd-outbox-event-driven`).

[OBSERVAÇÕES DE GOVERNANÇA ESPECÍFICAS DO PRD]
- R-DDD-001 + ADR-003 (local desta entrega): VOs imutáveis com construtor validador; State Pattern em `DeliveryStatus`; sem struct anêmica em `Event`/`Headers`/`SubscriptionName`/`Attempt`/`BackoffPolicy`.
- R-ERR-001: sentinels exportados (`ErrPermanent`, `ErrHandlerNotRegistered`, `ErrDispatcherDisabled`, `ErrDuplicateSubscription`, `ErrInvalidEvent`) com wrapping `fmt.Errorf("...: %w", err)`; nenhum sentinel mapeado para RFC 7807 (caminho assíncrono — ADR-004 da foundation cobre apenas HTTP).
- R-SEC-001 + RF-24/RF-31: `payload` jamais aparece em `slog.*`; allowlist canônico de campos em `log_fields.go` (task 9.0).
- R-TEST-001 + R3/R4 da `go-implementation`: `mockery.yml` na raiz criado pela task 1.0 (D-16); suítes com `testify/suite` + table-driven; integration sob build tag `integration` com `testcontainers-go/modules/postgres`; teste de concorrência com 3 dispatchers paralelos na task 8.0.
- D-04: pin `github.com/robfig/cron/v3@v3.0.1` no `go.mod`.
- D-11: `claimed_by = hostname-pid` calculado em `instance_id.go` (task 2.0).
- D-07: tabelas no schema `public`; sem `CREATE SCHEMA` na migration 0002.
- RF-26: feature flag `OUTBOX_DISPATCHER_ENABLED` lida via Viper no boot; restart obrigatório para alterar (sem live-reload no MVP).

[CRITÉRIOS DE ACEITE FINAIS — ORQUESTRAÇÃO COMPLETA]
- [ ] Pré-voo passou nos 7 gates sem warning não-confirmado.
- [ ] `_orchestration_report.md` existe na raiz do PRD com snapshot final mostrando 10/10 tarefas em `done`.
- [ ] `tasks.md` mostra todas as linhas com `Status = done` e spec-hashes preservados.
- [ ] 10 arquivos `<id>_execution_report.md` materializados, não-vazios e referenciando arquivos do diff.
- [ ] `ai-spec check-spec-drift .specs/prd-outbox-event-driven` retorna `OK: sem drift detectado` após a última wave.
- [ ] Nenhuma mutação em `cmd/server`, em `internal/infrastructure/events/` (além de uso de tipos canônicos), ou nos documentos de discovery sob `docs/discoveries/`.
- [ ] Nenhuma dependência além de `github.com/robfig/cron/v3@v3.0.1` adicionada ao `go.mod`.

[SE QUALQUER COISA ACIMA FALHAR]
- Halt-first imediato.
- Renderizar `_orchestration_report.md` (ou `.partial.md`) descrevendo: gate ou wave que falhou, stderr completo, próximos passos sugeridos, comandos para retomada idempotente.
- Retornar status agregado `failed` ou `partial` conforme aplicável.
- Não tentar reparar DAG automaticamente; não re-executar tarefas não-done; não silenciar exit ≠ 0 de hooks.

[INSTRUÇÃO FINAL — INVOCAÇÃO]
Carregue a skill `execute-all-tasks` agora e execute o PRD `outbox-event-driven` seguindo este contrato literal e mandatório. Se o seu tool/CLI atual (Claude Code, Codex CLI, Gemini CLI ou Copilot CLI) não suportar a primitiva de subagent declarada na tabela "Mapeamento por Tool" do `SKILL.md`, registre explicitamente `subagente: inline (tool sem agents nativo)` no relatório e degrade para execução sequencial sem isolamento — porém continue respeitando todos os demais contratos (YAML, hooks, halt-first, evidência física).
```
