# Tarefa 4.0: `module.go` + usecase `HandleInbound` (DI completo + AgentRuntime Thread→Run)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Montar o módulo `internal/agents` por DI manual sobre `internal/platform`: provider, tool, agent, workflow, scorers, memória (com `MessageStore` decorado da tarefa 1.0) e `AgentRuntime`; e o usecase `HandleInbound` que executa o agente Thread→Run a partir de um texto de entrada.

<requirements>
- RF-01: módulo em `internal/agents` consumindo a plataforma; sem importar `internal/agent`.
- RF-02: construtor de módulo (DI via construtor, sem `init()`, sem estado global).
- RF-03: aderência R0–R7, zero comentários, testify/suite whitebox, DTOs `Validate()`.
- RF-17: persistência thread/mensagens/working memory em Postgres (chaves opacas).
- ADR-001, ADR-002 (MessageStore decorado), ADR-003 (Thread→Run).
</requirements>

## Subtarefas

- [ ] 4.1 `module.go`: `NewModule(deps) (Module, error)` instanciando provider OpenRouter, tool, agent, workflow, scorers, repos de memória (thread/message/working/semantic), `RunStore`, e `AgentRuntime` com `MessageStore` decorado (publicador).
- [ ] 4.2 `application/usecases/handle_inbound.go`: recebe `InboundRequest` (resource/thread opacos, message, messageID) e chama `AgentRuntime.Execute`; DTO com `Validate()`.
- [ ] 4.3 Expor no `Module` a callback de rota WhatsApp, os EventHandlers e os Jobs para `cmd/*` (preenchidos na tarefa 5.0/cutover).

## Detalhes de Implementação

Ver techspec.md §"Interfaces Chave" (Module, Deps), §"Fluxo de dados", ADR-001/002/003. Mapear `peer/user`→`resourceId/threadId` opacos.

## Critérios de Sucesso

- `NewModule` monta o grafo sem `init()`/estado global; `go build` verde.
- `HandleInbound` executa o agente e produz Run auditável (status fechado, duração) em `platform_runs`.
- DTOs com `Validate()`; testes whitebox; zero comentários; gofmt limpo.

## Skills Necessárias

<!-- MANDATÓRIO -->

- `mastra` — o ciclo Thread→Run, AgentRuntime e wiring de agent/memory seguem o modelo Mastra mapeado ao `internal/platform`.

## Testes da Tarefa

- [ ] Testes unitários (HandleInbound: sucesso/erro/validação com mocks de AgentRuntime/portas; suite whitebox)
- [ ] Testes de integração (Run auditável persistido; opcional/integração com 7.0)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/module.go`, `internal/agents/application/usecases/handle_inbound.go`, `internal/agents/application/dtos/input/*`.
