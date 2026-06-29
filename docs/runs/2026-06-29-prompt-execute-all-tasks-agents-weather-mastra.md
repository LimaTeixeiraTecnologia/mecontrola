# Prompt PRONTO PARA USO — execute-all-tasks (prd-agents-weather-mastra)

Cole o bloco abaixo como prompt. Ele orquestra a execução completa do PRD
`.specs/prd-agents-weather-mastra` via a skill `execute-all-tasks`, sem desvios,
0 gaps, 0 lacunas, 0 falso positivo, e com prova real de production-ready.

```text
Execute @.claude/skills/execute-all-tasks/ (Copilot: @.github/skills/execute-all-tasks/) com rigor máximo, sem flexibilização, orquestrando a entrega COMPLETA do PRD `.specs/prd-agents-weather-mastra` (slug: agents-weather-mastra).

Fonte da verdade (carregar e obedecer, sem desvio):
- `.specs/prd-agents-weather-mastra/prd.md` (RF-01..RF-30)
- `.specs/prd-agents-weather-mastra/techspec.md` (spec-hash do PRD; ADR-001..005)
- `.specs/prd-agents-weather-mastra/tasks.md` (7 tarefas, DAG)
- `.specs/prd-agents-weather-mastra/task-1.0..7.0-*.md`
- `.specs/prd-agents-weather-mastra/adr-001..005-*.md`
- `AGENTS.md` como governança canônica; `.claude/rules/*` (go-implementation R0–R7, R-ADAPTER-001 zero comentários, R-TESTING-001 testify/suite whitebox, R-DTO-VALIDATE-001, R-WF-KERNEL-001, R-AGENT-WF-001, DMMF state-as-type).

Objetivo inegociável:
- Encerrar somente com TODAS as 7 tarefas em `done`, com evidência física (`<id>_execution_report.md` não-vazio) por tarefa e `_orchestration_report.md` final.
- 0 gaps, 0 lacunas, 0 falso positivo, conformidade total com PRD/techspec/tasks/ADRs.
- Realmente production-ready/proof — sem claims vagos; cada RF comprovado por código + teste + artefato objetivo.

Regras de orquestração (da skill, obrigatórias):
1. Rodar o pré-voo: hook `pre-execute-all-tasks.sh`, `ai-spec skills --verify`, `ai-spec check-spec-drift .specs/prd-agents-weather-mastra/tasks.md`. Abortar (`blocked`/`needs_input`/`failed`) se qualquer gate falhar — sem modo legado silencioso.
2. Toda tarefa roda em SUBAGENT FRESH via `execute-task` (orquestrador nunca executa inline). Contrato YAML estrito `{status, report_path, summary}`; violação = `failed: contract violation`.
3. Respeitar o DAG e a coluna `Paralelizável`: 1.0 ‖ 2.0; depois 3.0 ← 2.0; 4.0 ← 1.0,3.0; 5.0 ← 4.0; 6.0 ← 5.0; 7.0 ← 6.0. Halt-first: qualquer tarefa ≠ `done` após validação interrompe e reporta.
4. Validação de evidência física para `done` (realpath + arquivo não-vazio + status atualizado em tasks.md). Sem evidência = `failed`.
5. Não mutar `tasks.md` no orquestrador; apenas os subagents via `execute-task`. Checkpoint por wave; retomada idempotente.

Definição de pronto por tarefa (cada subagent DEVE comprovar, sem flexibilizar):
- `go build ./...` EXIT 0; `go vet ./...` EXIT 0; `go test` das áreas tocadas verde; `gofmt -l` (arquivos rastreados) vazio (gate `lint:fmt:check`).
- Zero comentários em Go de produção nas camadas novas (R-ADAPTER-001.1); testify/suite whitebox (R-TESTING-001); DTOs com `Validate()` (R-DTO-VALIDATE-001); tipos fechados/state-as-type (DMMF).
- Gates de governança verdes (`task gates:platform`): kernel sem domínio/LLM, cardinalidade controlada, tipos fechados.
- Testes de regressão/prova adicionados e EXECUTADOS antes de `done` (não aceitar "não verificável" como sucesso).

Restrições de sequência críticas:
- A tarefa 6.0 é a ELIMINAÇÃO FÍSICA e IRREVERSÍVEL de `internal/agent` + desligamento do onboarding conversacional. NÃO executar 6.0 antes de 1.0–5.0 estarem `done` com `internal/agents` totalmente wired e build/CI verdes. Executar 6.0 em commit isolado (rollback via revert).
- Critério de pronto da 6.0 (gate bloqueante): `grep -rn "internal/agent\"" internal/ cmd/ test/ | grep -v internal/platform/agent` retorna vazio; `test -d internal/agent` é falso; `go build ./...`, `go vet`, `go test` e gates verdes.

Prova de production-ready (RF-44/45/46 equivalentes deste PRD — RF-18,19,22,28,29,30 + conformidade):
- Indexação assíncrona de embeddings conectada e idempotente por `event_id` (gap B3 resolvido); `platform_embeddings` populada; semantic recall retorna itens escopados por `resourceId` em teste de integração testcontainers (Postgres + pgvector, migrations 000001..000003).
- Run auditável persistido em `platform_runs` (status fechado, duração); scorers persistidos em `platform_scorer_results`.
- Canal WhatsApp end-to-end: mensagem de clima → resposta (clima + atividades) enviada via gateway; thread/message/run/embedding/scorer_results persistidos.
- CI padrão determinístico (provider fake + testcontainers pgvector) verde; variante real atrás de `RUN_REAL_LLM=1` disponível e executável sob demanda.

Condições de encerramento:
1. Só `done` global quando as 7 tarefas estiverem `done` com evidência física, `_orchestration_report.md` consolidado, e os gates finais verdes (build/vet/test/gofmt/governança + ausência total de `internal/agent`).
2. Se qualquer RF não puder ser comprovado por código/teste/artefato, NÃO marcar `done` — reportar `partial`/`failed`/`blocked` com a lacuna exata e a evidência faltante.
3. Não normalizar falso positivo, risco residual ou "deixa para depois". Sem flexibilização por estilo, conveniência ou prazo.
```

## Notas

- A skill `execute-all-tasks` spawna um subagent fresh por tarefa (`execute-task`), respeita o DAG (1.0‖2.0 → 3.0 → 4.0 → 5.0 → 6.0 → 7.0), aplica halt-first e retomada idempotente via checkpoints.
- A tarefa **6.0 é destrutiva e irreversível** (apaga `internal/agent/**` e desliga o onboarding conversacional do WhatsApp). O prompt impõe que ela só rode após a fundação (1.0–5.0) estar `done` e verde, em commit isolado.
- Pré-condições já satisfeitas: `ai-spec check-spec-drift .specs/prd-agents-weather-mastra` → sem drift; cobertura RF-01..RF-30 completa; spec-hashes sincronizados.
- Resultado agregado esperado em `.specs/prd-agents-weather-mastra/_orchestration_report.md`.
