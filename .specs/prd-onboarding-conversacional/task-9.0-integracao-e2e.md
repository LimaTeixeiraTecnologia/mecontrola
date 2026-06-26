# Tarefa 9.0: Testes de integração e E2E

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Provar a jornada completa e as fronteiras de IO com testes de integração (testcontainers) e E2E fiéis ao oficial.

<requirements>
- Integração (testcontainers-go, `//go:build integration`): resume durável (suspende etapa → resume aplica merge-patch → avança; reinício de processo preserva estado via snapshot) (RF-23).
- Integração: propagação por evento idempotente — `splits_calculated`→orçamento ativo; `card_registered`→cartão com vencimento + fechamento derivado; `completed`→working memory; sem duplicidade em replay (RF-27, RF-28).
- Integração: migração-reset de sessão `in_progress` com fase legada.
- E2E: jornada completa das 8 etapas (feliz), validando ordem, persistência e `onboarding.completed`.
- E2E: bordas — correção no resumo, "não uso cartão", comando diário no meio (redireciona sem registrar), retomada após interrupção.
- Reaproveitar a suíte e2e existente de onboarding quando possível.
</requirements>

## Subtarefas

- [ ] 9.1 Integração: resume durável + reinício de processo.
- [ ] 9.2 Integração: propagação por evento (card/budgets/agent) + idempotência.
- [ ] 9.3 Integração: migração-reset.
- [ ] 9.4 E2E: jornada feliz das 8 etapas.
- [ ] 9.5 E2E: casos de borda (correção, "não uso", desvio diário, retomada).

## Detalhes de Implementação

Ver `techspec.md` → "Abordagem de Testes" (Integração/E2E). Usar `//go:build integration`; podem usar `package <X>_test` (exceção documentada a R-TESTING-001).

## Critérios de Sucesso

- Resume durável comprovado entre turnos e através de reinício de processo.
- Eventos propagam e são idempotentes; cartão criado com fechamento derivado; orçamento ativado; WM consolidada.
- Jornada completa e bordas passam fiéis ao Cap. 08.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — validação do ciclo Thread→Run, suspend/resume e jornada do agent ponta a ponta.

## Testes da Tarefa

- [ ] Testes unitários — N/A (esta tarefa é de integração/e2e).
- [ ] Testes de integração + E2E conforme requisitos acima.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/onboarding/e2e/` (suíte existente)
- `internal/agent/.../*_integration_test.go` (novos)
- `internal/platform/workflow/` (resume durável)
