# Tarefa 9.0: Paridade/não regressão + validação de gates

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Provar zero regressão do fluxo migrado e validar todos os gates de governança. Suíte de paridade
dirigida por tabela compara `Reply`/`Outcome`/`Kind` entre caminho atual (flag off) e caminho kernel
(flag on) para os mesmos inputs, mais E2E do `record-expense`, encerrando com os gates `R-*` e o
checklist `R0–R7`.

<requirements>
- RF-24: comportamento do fluxo migrado idêntico ao atual (não regressão verificável).
- RF-32: gates `R-ADAPTER-001`/`R-AGENT-WF-001`/`R-TESTING-001`/`R-WF-KERNEL-001` + checklist `R0–R7` verdes.
</requirements>

## Subtarefas

- [ ] 9.1 Suíte de paridade (tabela): auto-log, ambiguous→choice→resume, needs_confirm→confirm/cancel→
  resume, authz_denied, replay, policy_blocked, usecase_error, missing_resolver — flag off vs on com
  saída idêntica.
- [ ] 9.2 E2E inbound→reply do `record-expense` (consumer fake, flag on): auto-log e ciclo
  ambiguous→escolha→persistência, reply final idêntico ao atual.
- [ ] 9.3 Executar e registrar evidência dos gates: `R-ADAPTER-001` (zero comentários/SQL em adapter),
  `R-AGENT-WF-001` (sem novo case/LLM só no parse/estados fechados), `R-TESTING-001` (testify/suite),
  `R-WF-KERNEL-001` (kernel sem domínio) e checklist `R0–R7` de `go-implementation`.

## Detalhes de Implementação

Ver techspec.md → "Abordagem de Testes" e "Conformidade com Padrões". Carregar `mastra`. A suíte de
paridade é o critério de aceite de não regressão (ADR-005). Default da flag permanece off no merge.

## Critérios de Sucesso

- 0 divergência de `Reply`/`Outcome`/`Kind` entre flag off e on em todos os cenários.
- E2E verde; gates `R-*` e `R0–R7` com evidência registrada (saída dos comandos).
- DoD/critérios de aceite das tarefas 1.0–8.0 confirmados com evidência.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — validação de paridade comportamental do `internal/agent` sob R-AGENT-WF-001.

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agent/application/workflow/parity_test.go` (novo)
- `internal/agent/.../*_e2e_test.go` (novo)
- Saídas dos gates `R-*` e `R0–R7` (evidência)
