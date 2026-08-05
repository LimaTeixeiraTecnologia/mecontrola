# Tarefa 6.0: Integração consumer, outbox e wiring Mastra

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Integrar áudio no fluxo real do consumidor `internal/agents` antes de `HandleInbound`, preservando o agente `mecontrola`, tools, workflows, memory e confirmação destrutiva existentes. O objetivo é permitir dispatch apenas de texto canônico aprovado.

<requirements>
- Cobrir RF-18, RF-20, RF-39 e RF-43.
- Atualizar payload publicado em `agents.whatsapp.inbound.v1` sem quebrar texto atual.
- Bloquear `tryResume`, onboarding, `HandleInbound` e tool financeira para áudio não aprovado.
- Manter `AudioEnabled=false` por default.
- Não criar agente paralelo, workflow kernel de áudio, fallback chain ou tool financeira duplicada.
</requirements>

## Subtarefas

- [x] 6.1 Atualizar rota WhatsApp do módulo de agentes para publicar inbound tipado com modalidade e metadados de áudio.
- [x] 6.2 Atualizar consumer para branch textual preservar comportamento atual e branch de áudio seguir `validate -> audit -> download -> duration/cost preflight -> STT -> decide -> persist -> dispatch`.
- [x] 6.3 Garantir retorno antes de `tryResume`, onboarding e `HandleInbound` para qualquer outcome não aprovado.
- [x] 6.4 Integrar Media API client, transcriber, audit repository e replies via DI manual no `module.go`.
- [x] 6.5 Garantir que áudio aprovado chama o mesmo `HandleInbound` textual com texto canônico sem enriquecimento semântico.
- [x] 6.6 Criar testes de consumer para texto, áudio aprovado, áudio incerto, falha STT, falha download, duplicado e feature flag off.

## Detalhes de Implementação

Referenciar `techspec.md` nas seções `Fluxo de Dados`, `Fronteiras`, `Pontos de Integracao` e `Conformidade com Skills e Regras`.

Evidências de codebase a respeitar:
- `internal/agents/module.go:505`
- `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go:154`
- `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go:183`
- `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go:200`
- `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go:256`
- `internal/platform/agent/runtime.go:213`

## Critérios de Sucesso

- Fluxo textual atual continua verde.
- Áudio aprovado entra no mesmo agente textual.
- Áudio incerto/rejeitado/falho cria outcome terminal e resposta segura, sem run/thread/message de agente.
- WAMID duplicado não duplica lançamento.
- `AudioEnabled=false` mantém comportamento atual.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — A tarefa altera o consumidor agentivo real e precisa preservar Thread/Run/tools/workflows.
- `domain-modeling-production` — O fluxo de áudio usa estados fechados, invariantes e outcome terminal.
- `design-patterns-mandatory` — A tarefa deve confirmar fluxo direto/adapters finos sem pattern formal novo.

## Testes da Tarefa

- [x] `go test -race -count=1 ./internal/agents/infrastructure/messaging/database/consumers/...`
- [x] `go test -race -count=1 ./internal/agents/...`
- [x] Teste de 0 `HandleInbound` em `TranscriptionUncertain`.
- [x] Teste de 0 tool call em áudio não aprovado.
- [x] Teste de compatibilidade textual.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/module.go`
- `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go`
- `internal/agents/application/usecases/handle_inbound.go`
- `internal/platform/agent/runtime.go`
