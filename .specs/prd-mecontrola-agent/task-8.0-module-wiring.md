# Tarefa 8.0: `module.go` + wiring `cmd/server`/`cmd/worker`

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Reescrever `internal/agents/module.go` para montar o `MeControlaAgent` (agente + tools + onboarding + HITL + scorers + bindings + ledger + runtime) e ajustar o wiring em `cmd/server` e `cmd/worker`: `Deps` recebe os 4 módulos de domínio na ordem correta; registrar o consumer de inbound e o embedding handler; configurar modelo `openai/gpt-4o-mini`.

<requirements>
- ADR-003 (Deps com módulos + ordem), ADR-007 (modelo + gate RUN_REAL_LLM).
- Ordem de construção: categories → card → budgets → transactions → agents.
- Cobre: RF-01, RF-37.
</requirements>

## Subtarefas

- [ ] 8.1 `agents.Deps` passa a receber `CategoriesModule`, `CardModule`, `BudgetsModule`, `TransactionsModule` (módulos já construídos).
- [ ] 8.2 `NewModule`: construir bindings (2.0), ledger (3.0), tools (4.0), HITL (5.0), onboarding (6.0), agente+scorers (7.0), runtime; manter registro do `EmbeddingIndexHandler` e `WhatsAppInboundConsumer`.
- [ ] 8.3 Wiring `cmd/server/server.go`: injetar os 4 módulos em `agents.Deps` respeitando a ordem; registrar routers/handlers necessários.
- [ ] 8.4 Wiring `cmd/worker/worker.go`: registrar consumer e handlers do agente.
- [ ] 8.5 Config: modelo `openai/gpt-4o-mini`, `maxToolRounds=12`, timezone America/Sao_Paulo; gate `RUN_REAL_LLM` documentado como pré-requisito de produção.
- [ ] 8.6 Métricas de cardinalidade controlada (`agents_inbound_total{outcome}`, `agents_onboarding_phase_total{phase}`, `agents_write_total{operation,outcome}`, `agents_destructive_confirm_total{operation,result}`); proibido `user_id`/`category_id` como label.

## Detalhes de Implementação

Ver techspec.md → "Sequenciamento" passo 8, "Pontos de Integração" e "Monitoramento". Reusa o esqueleto do `module.go` weather; troca o conteúdo de domínio.

## Critérios de Sucesso

- DI manual via construtor (sem `init()`, sem estado global); ordem de módulos correta.
- Run auditável por interação (RF-37); métricas sem alta cardinalidade (R-AGENT-WF-001.5/R-TXN-004).
- Build/gofmt/governança verdes; `cmd/server` e `cmd/worker` compilam e sobem.
- Modelo e teto configurados; gate `RUN_REAL_LLM` documentado.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — montagem do módulo consumidor do substrato (registry, runtime, memory, scorers) e wiring do agente.

## Testes da Tarefa

- [ ] Testes unitários: `NewModule` valida Deps obrigatórias; falhas de configuração.
- [ ] Testes de integração: server/worker sobem com o módulo wired; inbound percorre consumer→runtime.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/module.go` (reescrito)
- `cmd/server/server.go`, `cmd/worker/worker.go`
- `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go`
- techspec.md (Sequenciamento/Integração/Monitoramento), ADR-003/007
