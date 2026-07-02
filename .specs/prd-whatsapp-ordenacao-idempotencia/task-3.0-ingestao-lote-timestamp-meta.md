# Tarefa 3.0: Ingestão em lote do webhook + timestamp da Meta no OccurredAt

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Processar todas as mensagens de um webhook (hoje só a primeira via `ExtractFirstMessage`) e emitir
1 evento outbox por mensagem, cada um com seu `wamid` e o `OccurredAt = msg.Timestamp` da Meta
(epoch → `time.Time`), preservando a ordem real do usuário (RF-17, RF-18; ADR-005).

<requirements>
- RF-17: extrair todas as mensagens de `Entry[].Changes[].Value.Messages[]` (não só `Messages[0]`); emitir **1 evento outbox por mensagem** (cada com seu `wamid` como `aggregate_id` e `aggregate_user_id` do principal resolvido).
- RF-18: `OccurredAt = msg.Timestamp` da Meta (epoch string → `time.Time`) como critério primário do FIFO; `created_at` do outbox como desempate (D-08).
- `item_seq` permanece como índice de escrita dentro do turno de uma mensagem (chave `(wamid, item_seq, operation)`); não confundir com número da mensagem no lote.
- Fallback: `msg.Timestamp` ausente/zero/inválido → usar `time.Now().UTC()` como `occurred_at` e registrar métrica (não falhar a ingestão) — ADR-005 §Riscos.
- Handler/produtor permanece adapter fino (R-ADAPTER-001): parseia, resolve principal, publica N eventos; sem regra de negócio nem branching de domínio.
- Sem abstrair tempo: `time.Now().UTC()` inline; sem `init()`; `errors.Join`/`%w`.
- Mensagem única mantém comportamento equivalente ao atual.
</requirements>

## Subtarefas

- [ ] 3.1 Substituir `ExtractFirstMessage` por extração de todas as mensagens (nova função retornando `[]Message`), preservando `msg.Timestamp` (string epoch).
- [ ] 3.2 Em `buildWhatsAppAgentRoute`/`dispatcher.go`, publicar 1 evento por mensagem com `OccurredAt` convertido do timestamp da Meta; fallback para `now()` quando ausente.
- [ ] 3.3 Converter epoch string → `time.Time` de forma segura (tratar vazio/zero/inválido).
- [ ] 3.4 Testes unitários: webhook com N mensagens → N eventos ordenados por `occurred_at`; timestamp ausente → fallback; mensagem única mantém comportamento.

## Detalhes de Implementação

Ver ADR-005 §Decisão/§Plano de Implementação e techspec §Modelos de Dados (bloco
`whatsapp_inbound_payload`). O `OccurredAt` hoje é `time.Now().UTC()` em `module.go`
`buildWhatsAppAgentRoute`/`dispatcher.go`; a ordem entre as N mensagens do mesmo usuário é garantida
pelo claim particionado da tarefa 2.0 (não por processá-las no mesmo Run).

## Critérios de Sucesso

- Webhook multi-mensagem gera N eventos, um por `wamid`, sem descarte silencioso.
- `OccurredAt` reflete o timestamp da Meta; empates dentro do mesmo segundo desempatam por `created_at`.
- Timestamp ausente não falha a ingestão (fallback + métrica).
- Produtor permanece fino (sem regra de negócio).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

- `mastra` — a rota `buildWhatsAppAgentRoute` (wiring do agente/ingestão inbound sobre `internal/agents`/`internal/platform`) é alterada; a skill cobre a montagem do ciclo inbound do agente.

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

Unitários no parser e na publicação (N mensagens → N eventos; fallback de timestamp; caso single).
A verificação end-to-end (webhook multi-mensagem processado na ordem da Meta sob claim) é a CA-07 na
tarefa 8.0.

## Rollback

Reverter para `ExtractFirstMessage` e `OccurredAt = time.Now().UTC()`; o claim continua funcional (só
perde o FIFO por timestamp da Meta e volta a descartar mensagens além da primeira).

## Done-when

- Suite unitária verde (multi, fallback, single).
- Nenhuma mensagem descartada em webhook multi-mensagem.
- `OccurredAt` = timestamp da Meta comprovado no evento publicado.

## Arquivos Relevantes
- `internal/platform/whatsapp/payload/parser.go` (`ExtractFirstMessage` → extração total)
- `internal/platform/whatsapp/payload/types.go`
- `internal/agents/module.go` (`buildWhatsAppAgentRoute`)
- `internal/platform/outbox/dispatcher.go` (publicação com `OccurredAt`)
