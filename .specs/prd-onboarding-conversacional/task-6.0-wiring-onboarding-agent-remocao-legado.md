# Tarefa 6.0: Wiring OnboardingAgent e remoção do legado

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Integrar o novo workflow ao runtime do agent (resume antes do parse) e remover por completo o fluxo legado de onboarding.

<requirements>
- `OnboardingAgent.Handle` resolve a sessão de onboarding e faz `Engine.Resume` (ou `Start` idempotente) ANTES do `ParseInbound` do agente diário; sem novo `case intent.Kind` (R-AGENT-WF-001.1, RF-01, RF-02).
- Resume idempotente por `messageID` (RF-03); thread/run resolvidos pelo runtime (RF-02).
- Resume durável: retoma a etapa onde parou; snapshot é fonte única (RF-23).
- Remoção do legado: `run_onboarding_turn.go` (loop de fases), `OnbPhaseFirstTx`/`firstTxPhase`, `buildAutoSplitPreview`/`suggest_budget_split` no caminho oficial, headers "Etapa X/4", schema `onboarding_first_tx`, `mark_first_transaction_recorded` no caminho de conclusão.
- Wiring do módulo do agent atualizado; nenhum import morto; build/lint limpos.
</requirements>

## Subtarefas

- [ ] 6.1 Reescrever `OnboardingAgent.Handle` (resume-antes-do-parse) consumindo o `Engine`.
- [ ] 6.2 Registrar a `Definition` e dependências no `module.go` do agent.
- [ ] 6.3 Remover arquivos/trechos legados e atualizar wiring/imports.
- [ ] 6.4 Garantir ordem determinística de resolução inbound (onboarding antes do agente diário).

## Detalhes de Implementação

Ver `techspec.md` → "Resolução inbound", "Arquivos Relevantes e Dependentes" e ADR-001. Não remover o legado antes de 4.0/5.0 prontos (ordem de dependência).

## Critérios de Sucesso

- Mensagem em onboarding é resolvida pelo workflow antes do parse diário; fora do onboarding, segue o agente diário.
- Reprocessamento de `messageID` não avança etapa duas vezes.
- Zero referência a "Etapa X/4"/`first_tx`/auto-sugestão no código; build e lint limpos.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — wiring do runtime do agent (Thread→Run, resolução inbound, registro do workflow) e remoção do roteamento legado.

## Testes da Tarefa

- [ ] Testes unitários: `OnboardingAgent.Handle` (em onboarding → resume; fora → handled=false; replay idempotente).
- [ ] Testes de integração — jornada e resume durável cobertos na 9.0.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agent/application/services/onboarding_agent.go`
- `internal/agent/module.go`
- Removidos: `internal/agent/application/usecases/run_onboarding_turn.go`, `onboarding_scripts.go` (partes), `onboarding_structured_schema.go` (`onboarding_first_tx`)
