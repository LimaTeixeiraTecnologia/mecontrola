# Tarefa 4.0: Não regressão + escopo + gate golden real-LLM

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Fase de verificação cruzada: garantir que as mudanças de copy não regridem o comportamento, que o escopo da regra de 💳 ficou restrito aos 2 fluxos determinísticos, e que o gate golden real-LLM agregado permanece verde.

<requirements>
- RF-08: verificar que o escopo do 💳 é restrito a onboarding + avulso; `mecontrola_agent.go`, tools de cartão, `pending_entry_workflow`, `destructive_confirm_workflow` e golden cases NÃO foram alterados.
- RF-16: confirmar que todas as mudanças são copy/montagem de mensagem no consumidor `internal/agents`, sem alterar ordem de etapas, suspend/resume, extração LLM, criação de cartão, ativação de orçamento, recorrência ou idempotência.
- RF-17: testes unitários e de integração determinísticos verdes com asserts de copy atualizados; gate golden real-LLM agregado (`CategoryOnboarding`, threshold ≥ 0,90) verde.
</requirements>

## Subtarefas

- [ ] 4.1 Rodar `go build ./...`, `go vet ./...`, `go test -race` (unit) e lint no módulo `internal/agents`; garantir verde.
- [ ] 4.2 Rodar a suíte de integração (`//go:build integration`) do onboarding e do avulso; garantir verde.
- [ ] 4.3 Verificar escopo por grep: `git diff --name-only` cobre apenas `onboarding_workflow.go`, `card_create_confirm_workflow.go` e arquivos de teste; nenhum diff em `mecontrola_agent.go`, tools, `pending_entry_workflow.go`, `destructive_confirm_workflow.go`, `cases_card.go`, `internal/platform/whatsapp/formatting/normalize.go`.
- [ ] 4.4 Rodar o gate golden real-LLM (`RUN_REAL_LLM=1`, `harness_realllm_test.go`) e confirmar `CategoryOnboarding` ≥ 0,90.

## Detalhes de Implementação

Ver techspec.md, seções "Abordagem de Testes / Testes E2E", "Sequenciamento de Desenvolvimento" e "Conformidade com Padrões". Requer `OPENROUTER_*` no `.env` para o gate real-LLM (feedback do projeto: validação real-LLM obrigatória em mudanças do agente).

## Critérios de Sucesso

- Build/vet/test-race/lint verdes no `internal/agents`; integração verde.
- Diff restrito aos 2 fluxos determinísticos + testes; 0 alteração no system prompt/tools/golden/normalizador.
- Gate golden `CategoryOnboarding` ≥ 0,90.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — a verificação inclui o gate golden real-LLM e o comportamento do consumidor de agente sobre o substrato Mastra.

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/golden/harness_realllm_test.go`, `internal/agents/application/golden/cases_onboarding.go`
- `internal/agents/application/workflows/onboarding_workflow.go`, `card_create_confirm_workflow.go`
- Suítes de integração do onboarding e do avulso
