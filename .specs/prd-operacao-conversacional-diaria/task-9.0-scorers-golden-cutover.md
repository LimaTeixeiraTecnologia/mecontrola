# Tarefa 9.0: Scorers, golden 13 fluxos, gate real-LLM e cutover do legado

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Fechar a jornada: adicionar scorers de aderência verbatim/tom, casos golden para os 13 fluxos, estender o `TestGoldenSetGate` (piso 0,90/fluxo, 0 falso-sucesso) e o gate real-LLM; e executar o CUTOVER com remoção TOTAL do legado conforme a seção "Remoção Total do Legado" da techspec, com drenagem dos runs suspensos por janela de graça (ADR-005).

<requirements>
- RF-27: substituição da camada conversacional do dia a dia; domínio aditivo.
- RF-28: drenagem dos runs suspensos no cutover (ADR-005).
- RF-29: gate real-LLM 0,90/fluxo + 0 falso-sucesso + gates de governança.
- RF-30: KPIs por scorers/métricas.
- ADR-005: cutover com drenagem de runs suspensos por janela de graça.
- Dependência: task 8.0 (fluxo novo completo — tools, wiring e prompt).
</requirements>

## Subtarefas

- [ ] 9.1 Scorers de aderência verbatim/tom (code-based e, se aplicável, LLM-judged) adicionados a `BuildMeControlaScorers` em `internal/agents/application/scorers/mecontrola_scorers.go`.
- [ ] 9.2 Casos golden para os 13 fluxos em `internal/agents/application/golden/` com `ResponseProperty`/`ResponseDescribe` e as categorias novas registradas em `AllCases()`.
- [ ] 9.3 Estender `TestGoldenSetGate` (piso 0,90/fluxo, 3x por caso) e o gate real-LLM (`//go:build integration`, `RUN_REAL_LLM=1`, `OPENROUTER_API_KEY`/`AGENT_HARNESS_MODEL` do ambiente).
- [ ] 9.4 CUTOVER: remoção TOTAL do legado conforme a seção "Remoção Total do Legado (executada no cutover)" da techspec (deletar os arquivos listados; modificar consumer/`module.go`/prompt; preservar onboarding/idempotência/leitura/guards) + drenagem dos runs suspensos por janela de graça (ADR-005).
- [ ] 9.5 Verificação de conclusão: `grep` sem referência aos símbolos removidos (`PendingEntry*`, `CardCreate*`, `BudgetCreation*`, `DestructiveConfirm*` antigos, `register_attempt`) fora dos testes + build/vet/test/lint verdes.

## Detalhes de Implementação

Ver `techspec.md` (RF-27, RF-28, RF-29, RF-30 e, sobretudo, a seção "Remoção Total do Legado (executada no cutover)" com o inventário exato de deletar/modificar/preservar e o critério de conclusão) e `adr-005-cutover-drain.md` desta pasta — **referenciar em vez de duplicar**.

Pontos-chave do ADR-005:
- Cutover com drenagem por janela de graça: novos inbounds já entram no fluxo novo; os runs suspensos existentes continuam podendo concluir ou expiram pelo TTL/reaper de cada workflow.
- O legado só é desativado após a janela de graça (dimensionada pelo maior TTL dos workflows suspensos); nenhuma confirmação em aberto é encerrada à força.
- Reaper ativo por workflow suspenso; métrica de runs suspensos por workflow deve chegar a zero antes da remoção.

Cutover (techspec — "Remoção Total do Legado"):
- Deletar: workflows/estados/decisões de `pending_entry`, `destructive_confirm`, `card_create_confirm`, `budget_creation` e os continuers correspondentes + `register_attempt.go` (reencarnados/substituídos nos novos artefatos, não copiados).
- Modificar: `whatsapp_inbound_consumer.go`, `module.go`, `mecontrola_agent.go` (já cobertos por tasks 7.0/8.0).
- Preservar: onboarding, idempotência/utilitários, runtime, tools de leitura e guards.

## Critérios de Sucesso

- Golden ≥ 0,90 por fluxo + 0 falso-sucesso.
- Gate real-LLM verde.
- Legado removido totalmente sem referências pendentes aos símbolos removidos.
- build/vet/test/lint verdes.
- Gates de governança verdes: R-AGENT-WF-001, R-ADAPTER-001, R-DTO-VALIDATE-001, R-TESTING-001.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — scorers/evals e gate de jornada do agente.
- `postgresql-production-standards` — limpeza/verificação de runs suspensos na drenagem do cutover.

## Testes da Tarefa

- [ ] Testes unitários (golden `TestGoldenSetGate`: piso 0,90/fluxo, 3x por caso, 0 falso-sucesso)
- [ ] Testes de integração (gate real-LLM `RUN_REAL_LLM=1`; verificação de remoção do legado via `grep` + build/vet/test/lint)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/scorers/mecontrola_scorers.go`
- `internal/agents/application/scorers/*.go`
- `internal/agents/application/golden/*.go`
- Arquivos legados a remover (ver seção "Remoção Total do Legado" da `techspec.md`)
- `internal/agents/module.go`
</content>
