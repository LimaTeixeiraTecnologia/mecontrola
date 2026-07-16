# Tarefa 6.0: Gate real-LLM com 0 falso-sucesso e fixture full-flow

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Materializar o gate de qualidade do RF-17 estendendo o suite real-LLM dedicado ao step
(`internal/agents/application/workflows/onboarding_workflow_integration_test.go:680`
`TestRecurrenceExtractionGate`), NÃO o registry golden — que cobre as tools do agente diário,
confirmado em `internal/agents/application/golden/cases_onboarding.go:31,44`. O gate deve cobrir os
5 tipos de resposta com `ratio ≥ 0,90` e asserção explícita de 0 falso-sucesso. Atualizar as
fixtures e assinaturas de teste impactadas pela nova assinatura do step introduzida na tarefa 5.0.

Depende da tarefa 5.0.

<requirements>
- RF-17, RF-18.
- Referência: techspec "Abordagem de Testes → Testes de Integração/E2E".
- ADR-004: `.specs/prd-recorrencia-orcamento-onboarding/adr-004-gate-real-llm-zero-falso-sucesso.md`.
</requirements>

## Subtarefas

- [ ] 6.1 Estender `TestRecurrenceExtractionGate` (`onboarding_workflow_integration_test.go:680`): trocar `expected{recurrence bool}` pelo outcome fechado `recurrenceOutcomeKind`; atualizar a chamada `BuildRecurrenceStep(a, budgets, rec)` (`:716`) para a nova assinatura da tarefa 5.0.
- [ ] 6.2 Adicionar cenários dos 5 tipos (extraídos dos exemplos da US): negativa (várias formas), positiva-12, N numérico ("6 meses", "só 3", "coloca por 6"), N por extenso ("seis meses", "manter por oito meses"), inválido (">12" / "0"), ambíguo ("talvez", emoji).
- [ ] 6.3 Manter o gate `require.GreaterOrEqual(ratio, 0.90, ...)`.
- [ ] 6.4 Asserção explícita de 0 falso-sucesso: mock/spy `BudgetPlanner` garantindo que `CreateRecurrence` NÃO é chamado nos cenários negativo/inválido/ambíguo, e que o N aplicado nos cenários específicos bate exatamente (não apenas o ratio agregado).
- [ ] 6.5 Atualizar a fixture full-flow WhatsApp: `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer_integration_test.go` — o RawJSON de recorrência (`:332/342`) muda do schema antigo `{confirmed:false}` para o novo `{intent,hasMonths,months}` (manter UMA chamada `agent.Execute` no step; só o payload muda). Ajustar o stub `CreateRecurrence` (`:87`) se necessário.

## Detalhes de Implementação

Ver techspec "Abordagem de Testes → Testes de Integração/E2E" e ADR-004
(`.specs/prd-recorrencia-orcamento-onboarding/adr-004-gate-real-llm-zero-falso-sucesso.md`) —
**referenciar em vez de duplicar**.

- Build tag `//go:build integration`; ativado por `RUN_REAL_LLM=1` + `OPENROUTER_API_KEY`
  (modelo default `openai/gpt-4o-mini`).
- `internal/agents/application/postdeploy/regression_contract.go` NÃO muda (nenhum scorer novo).
- Validação real-LLM obrigatória com o `.env` do projeto (`OPENROUTER_*`); mocks não substituem o
  gate.

## Critérios de Sucesso

- `RUN_REAL_LLM=1 go test -tags integration ./internal/agents/application/workflows/ -run TestRecurrenceExtractionGate` com `ratio ≥ 0,90` e 0 falso-sucesso.
- Fixture full-flow WhatsApp verde com o novo schema `{intent,hasMonths,months}`.
- `go build -tags integration ./...` verde.
- Nenhum falso-sucesso: nenhuma aplicação de recorrência nos cenários negativo/inválido/ambíguo.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — gate real-LLM / evals do step de agente (harness `RUN_REAL_LLM`, scoring por outcome) e fixtures de integração do fluxo de onboarding.

## Testes da Tarefa

- [ ] Testes unitários
- [x] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/agents/application/workflows/onboarding_workflow_integration_test.go`
- `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer_integration_test.go`
- `internal/agents/application/workflows/onboarding_workflow.go`
- `.specs/prd-recorrencia-orcamento-onboarding/techspec.md`
- `.specs/prd-recorrencia-orcamento-onboarding/adr-004-gate-real-llm-zero-falso-sucesso.md`
