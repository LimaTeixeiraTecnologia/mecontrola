# Tarefa 4.0: Confirmação honesta no runtime do agente

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Eliminar o sucesso alucinado e a mensagem vazia: `invokeToolCall` deixa de engolir erro de tool,
`runtime.Execute` deriva `RunStatus`/`ToolOutcome` do resultado real (não hardcoda), as write tools
propagam o `ToolOutcome` tipado no output e `sendReply` ganha guarda contra envio vazio (RF-06, RF-07,
RF-08; ADR-002).

<requirements>
- RF-06: o agente só confirma sucesso após a tool retornar resultado tipado indicando persistência efetiva; em falha responde honestamente, nunca sucesso.
- RF-07: a write tool é adapter fino que propaga o resultado real (persistido/duplicado/erro) ao agente, sem regra de negócio (R-AGENT-WF-001.2, R-ADAPTER-001).
- RF-08: NUNCA enviar outbound vazio; `content==""` (saída vazia do LLM) → fallback honesto ("não consegui concluir agora, pode repetir?"), nunca `SendTextMessage("")`; resultado observável (métrica `no_reply`), jamais envio em branco.
- `invokeToolCall`: status fechado `toolExecOK|toolExecError` (nunca silencioso); em erro, entregar ao LLM um tool message com erro **estruturado** preservando `%w` (não `content==""`).
- `runtime.Execute`: incluir `Outcome ToolOutcome` no `Outcome` retornado; `RunStatus`/`ToolOutcome` derivados do resultado real (parar de hardcodar `RunStatusSucceeded`/`ToolOutcomeRouted`).
- write tools expõem `ToolOutcome` no output (não apenas `IsReplay`); `usecaseError`/`missingResolver` nunca viram confirmação de sucesso.
- `agent.ToolOutcome` e `RunStatus` permanecem tipos fechados (DMMF state-as-type); LLM só nas call-sites sancionadas; zero comentários.
- Validação com LLM real obrigatória (`RUN_REAL_LLM=1` + `OPENROUTER_*` do `.env`) — memória `feedback_realllm_validation_required`.
</requirements>

## Subtarefas

- [ ] 4.1 `invokeToolCall`/`completeWithTools`: introduzir `toolExecStatus` fechado e tool message de erro estruturado; parar de zerar `content`.
- [ ] 4.2 `runtime.Execute`: derivar `RunStatus`/`ToolOutcome` do resultado; adicionar `Outcome ToolOutcome` ao struct `Outcome`.
- [ ] 4.3 Write tools (`register_expense/income/card_purchase`): output tipado carrega `ToolOutcome`.
- [ ] 4.4 `sendReply`: guarda de envio vazio → fallback honesto + métrica `no_reply`; nunca `return nil` silencioso nem envio em branco.
- [ ] 4.5 Testes unitários (testify/suite) por cenário: ok/replay/usecaseError/missingResolver/erro de IO; `content==""` → fallback.
- [ ] 4.6 Validação com LLM real do loop de tool-calling (RUN_REAL_LLM=1).

## Detalhes de Implementação

Ver ADR-002 §Contexto/§Decisão (itens 1–4) e techspec §Interfaces Chave (blocos `Outcome`,
`invokeToolCall`, `toolExecStatus`) e §Abordagem de Testes. NÃO alterar o provider (OpenRouter
inalterado); muda apenas o tratamento de erro de tool no loop. A remoção do gate de idempotência e o
mapa `reconciled` ficam na tarefa 5.0 (dependente desta).

## Critérios de Sucesso

- Erro de tool → `toolExecError` + tool message estruturado (não `content==""`).
- `Outcome.Outcome` preenchido; conteúdo vazio nunca vira sucesso silencioso.
- `sendReply` com `content==""` → fallback honesto + métrica, gateway nunca chamado com vazio.
- `usecaseError`/`missingResolver` nunca produzem confirmação de sucesso.
- Validação com LLM real passa (resposta honesta em falha).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

- `mastra` — altera o loop de tool-calling do Agent, o `runtime.Execute` (ciclo Run) e as write tools; a skill é a base canônica para agente/tool/runtime sobre `internal/platform`/`internal/agents`.

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

Unitários testify/suite (whitebox, `fake.NewProvider()`, dependencies+IIFE) para `invokeToolCall`,
`runtime.Execute`, write tools e `sendReply`. Validação com LLM real do loop. A verificação end-to-end
(erro de persistência → sem sucesso/sem vazio) é a CA-03 na tarefa 8.0.

## Rollback

Os 4 pontos (invokeToolCall, runtime, tools, sendReply) são isolados por função; reverter cada um
restaura o comportamento anterior (ADR-002 §Riscos/Rollback).

## Done-when

- Suites unitárias verdes em todos os cenários.
- Validação com LLM real registrada como evidência.
- Zero caminho que produza `SendTextMessage("")` ou sucesso sem persistência.

## Arquivos Relevantes
- `internal/platform/agent/agent.go` (`invokeToolCall`, `completeWithTools`)
- `internal/platform/agent/runtime.go` (`Execute`), `internal/platform/agent/types.go` (`Outcome`, `ToolOutcome`, `RunStatus`)
- `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go` (`sendReply`)
- `internal/agents/application/tools/{register_expense,register_income,register_card_purchase}.go`
