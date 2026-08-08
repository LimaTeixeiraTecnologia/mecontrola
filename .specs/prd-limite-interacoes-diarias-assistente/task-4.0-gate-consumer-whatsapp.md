# Tarefa 4.0: Gate do limite diário no consumer de WhatsApp

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Ligar o resolvedor de limite diário ao consumer `whatsapp_inbound` via nova `ConsumerOption` e método `tryDailyLimit`, posicionado após `tryResume` e antes de `handleAgentInbound` nos dois pontos de dispatch (texto e áudio transcrito), mantendo o consumer como adapter fino e a cadeia dedup, resume, limite, agente nessa ordem exata.

<requirements>
- RF-01: verificação do limite antes do dispatch ao agente, antes da abertura de Run e antes de qualquer chamada ao OpenRouter.
- RF-06: decisão na camada de aplicação; consumer apenas consulta o resolvedor e envia a resposta.
- RF-07: retomadas de workflows suspensos e onboarding não são contados nem bloqueados, pois o gate vem depois de `tryResume`.
- RF-10: mensagem bloqueada não abre Run, não chama o LLM e não incrementa a contagem do dia.
- RF-12: falha na contagem segue o fluxo de erro existente do consumer, incluindo compensação de dedup, sem resposta inventada.
- RF-13: pipeline de áudio inalterado; o gate cobre também o dispatch de áudio transcrito.
- RF-14: mensagens duplicadas tratadas pela deduplicação por WAMID não passam pelo gate.
</requirements>

## Subtarefas

- [ ] 4.1 Declarar a interface local `dailyLimitResolver` com `Execute(ctx context.Context, userID string) (usecases.DailyLimitResult, error)`, padrão de `onboardingResolver` (`whatsapp_inbound_consumer.go:34-36`)
- [ ] 4.2 Adicionar `WithDailyLimitResolver` como `ConsumerOption`, padrão de `WithOnboardingResolver` (`whatsapp_inbound_consumer.go:89-93`)
- [ ] 4.3 Implementar `tryDailyLimit` retornando `(bool, error)`: resolvedor nil retorna não tratado; bloqueado envia `result.Message` via `sendReply` existente; erro propaga para o fluxo de erro com compensação de dedup
- [ ] 4.4 Posicionar a chamada após `tryResume` em `Handle` (`whatsapp_inbound_consumer.go:226-230`) e no closure de dispatch de áudio (`whatsapp_inbound_consumer.go:281-286`)
- [ ] 4.5 Cobrir com cenários novos na suite existente usando mock manual, padrão de `whatsapp_inbound_consumer_test.go:33-103`

## Detalhes de Implementação

- ADR-001 (`adr-001-gate-como-resolvedor-no-consumer.md`) registra posicionamento, alternativas rejeitadas e comportamento fail safe da option ausente.
- Seção `Visão Geral dos Componentes` da techspec.md define o fluxo de dados e a ordem da cadeia.
- R-ADAPTER-001: zero SQL, zero regra de negócio e zero branching de domínio no consumer; seleção de mensagem já vem pronta do use case.

## Critérios de Sucesso

- Bloqueio envia a mensagem estática e prova que `handleInbound` não foi chamado (RF-01, RF-10).
- Mensagem tratada por `tryResume` nunca consulta o resolvedor de limite (RF-07).
- Gate cobre o dispatch de áudio transcrito (RF-13) e fica depois da deduplicação (RF-14).
- Erro do resolvedor passa por `compensateDedup` e retorna erro ao runner, sem resposta ao usuário (RF-12).
- Teste de ordenação prova a cadeia dedup, resume, limite, agente, no padrão de `TestResumeDispatcherPrecedesOnboarding` (`whatsapp_inbound_consumer_test.go:953-988`).
- Todos os cenários preexistentes da suite do consumer permanecem verdes sem alteração de asserção.
- `go build ./...`, `go vet ./...`, `go test -race -count=1 ./internal/agents/...`, `task ci:agent-boundary` e `task ci:zero-comments` verdes.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — alteração no fluxo canônico de inbound do consumidor agentivo, com o gate posicionado antes de `AgentRuntime.Execute` e sem tocar o substrato
- `design-patterns-mandatory` — introdução de nova Functional Option no consumer exige o gate de desenho, com expectativa de reuso do padrão já existente em vez de pattern novo

## Testes da Tarefa

- [ ] Testes unitários: bloqueio com mensagem estática sem chamar `handleInbound`; permitido segue ao agente; resume tem precedência sobre o gate; gate no dispatch de áudio; erro do resolvedor com compensação de dedup; ordenação da cadeia
- [ ] Testes de integração: não aplicável (sem fronteira de IO nova nesta tarefa)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go`
- `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer_test.go`
- `internal/agents/application/usecases/daily_interaction_limit.go` (contrato consumido, tarefa 3.0)
