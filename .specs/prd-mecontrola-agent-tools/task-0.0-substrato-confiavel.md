# Tarefa 0.0: Substrato de escrita/leitura confiável (P0 bloqueante)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

<critical>P0 BLOQUEANTE: esta é a raiz do DAG. Enquanto esta tarefa não estiver concluída e
verificada por escrita real no banco (RF-40), NENHUMA tool nova (RF-09..RF-18e) é considerada
"coberta"/"exercida" — todas herdam o defeito de sucesso alucinado comprovado em produção
(seção `Evidência de Produção`, EP-01..EP-05). Todas as demais tarefas de implementação de tools
(1.0..9.0) dependem desta.</critical>

## Visão Geral

Corrigir o substrato de agent/runtime (`internal/platform/agent`) que hoje reporta "sucesso de
escrita" sem gravar linha alguma. A confrontação do PRD com a conversa real do usuário
`06edc407-4f63-42e8-b07c-946b9ef0a19c` (WhatsApp +5511986896322) provou que o agente respondeu
"Despesa registrada com sucesso ✅" com **0 linhas** em `transactions`/`agents_write_ledger`, e não
encontrou um `budget 2026-07` que existe e está ativo — porque a identidade/idempotência
(`userId`/`wamid`/`itemSeq`) é exigida do LLM (que não a fornece) e o Run marca sucesso por conteúdo
não-vazio, sem nunca persistir mensagens `role=tool`.

Esta tarefa corrige três defeitos de plataforma (RF-37/RF-38/RF-39) e corrige a premissa da
spec-version 2 (RF-40): o bucket 1 (tools de escrita/leitura já existentes) NÃO é assumido funcional.
É **exceção de plataforma** explicitamente dentro de escopo (FE-03), pois é infraestrutura de agent
(`internal/platform/agent`), não capacidade de domínio novo. O kernel `internal/platform/workflow`
permanece intocado (R-WF-KERNEL-001).

<requirements>
- RF-37 — Injeção server-side de identidade/idempotência. As tools de escrita e de leitura por usuário
  (`register_expense`, `register_income`, `register_card_purchase`, `query_month`, `query_plan`,
  `create_recurrence`, e toda tool nova que precise de `userId`/`wamid`/`itemSeq`) NÃO recebem esses
  valores como argumentos do LLM. Devem ser injetados **server-side** a partir do `InboundRequest`/
  contexto do Run no ponto de invocação (`internal/platform/agent`, `invokeToolCall`) e **removidos do
  schema** exposto ao modelo. (Defeitos: `internal/agents/application/tools/register_expense.go:52`
  marca `wamid`/`itemSeq`/`userId` como `required` do LLM com `Strict:true`;
  `internal/platform/agent/runtime.go:173-193` — `buildMessages` — nunca injeta
  `in.ResourceID`/`in.MessageID`.)
- RF-38 — Guard bloqueante de anti-simulação. O runtime NÃO reporta sucesso de escrita ao usuário sem
  que a tool de escrita correspondente tenha retornado um `ToolOutcome` real de sucesso
  (`routed`/`reconciled`/`replay`). PROIBIDO marcar `RunStatusSucceeded`/`ToolOutcomeRouted` apenas por
  `result.Content` não-vazio quando a intenção era escrita e nenhuma tool de escrita retornou sucesso.
  (Defeito: `internal/platform/agent/runtime.go:155-162`.)
- RF-39 — Run auditável com evidência de escrita real. Cada execução persiste as mensagens de tool
  (`memory.RoleTool`) e o `resource_id` retornado pela escrita, distinguindo escrita real de texto de
  sucesso. (Defeito: `internal/platform/agent/runtime.go:138-153` só persiste `RoleUser`/`RoleAssistant`;
  `RoleTool` é definido mas nunca gravado.)
- RF-40 — Premissa corrigida. A correção RF-37..RF-39 é pré-requisito bloqueante; nenhuma tool nova é
  "coberta"/"exercida" enquanto o substrato não for verificado por escrita real no banco (RF-29/RF-33).
- RTA-08 — Identidade/idempotência injetada server-side no ponto de invocação; PROIBIDO expô-la no
  schema de tool ou confiar em valor do LLM.
- RTA-03 — `ToolOutcome`/`RunStatus`/`AwaitingKind` como tipos fechados (DMMF state-as-type); nunca
  string livre.
- R-WF-KERNEL-001 — kernel `internal/platform/workflow` intocado (sem domínio/LLM/SQL fora do adapter).
- R-AGENT-WF-001.5 — Run auditável; R-AGENT-WF-001.8 — WorkingMemory no system prompt (preservar).
- R-ADAPTER-001.1 — zero comentários em Go de produção.
- R-DTO-VALIDATE-001 — input DTO alterado mantém `Validate()` (`errors.Join`, campo nomeado).
- Proibido abstração de tempo (`time.Now().UTC()` inline); proibido asserção `var _ I = (*T)(nil)`.
</requirements>

## Subtarefas

- [ ] 0.1 Injeção server-side (RF-37/RTA-08): no ponto de invocação de tool em `internal/platform/agent`
  (`invokeToolCall`/`buildMessages`), popular `userId`/`wamid`/`itemSeq` a partir do
  `InboundRequest`/contexto do Run (`in.ResourceID`/`in.MessageID`) antes de chamar o `exec` da tool;
  remover esses campos do schema exposto ao modelo nas tools de escrita/leitura por usuário
  (`register_expense`, `register_income`, `register_card_purchase`, `query_month`, `query_plan`).
- [ ] 0.2 Guard bloqueante (RF-38): em `runtime.go:155-162`, não derivar `RunStatusSucceeded`/
  `ToolOutcomeRouted` de `result.Content` não-vazio quando a intenção é escrita; exigir `ToolOutcome`
  real de sucesso (`routed`/`reconciled`/`replay`) retornado pela tool de escrita para confirmar
  sucesso ao usuário.
- [ ] 0.3 Persistência de tool messages (RF-39): em `runtime.go:138-153`, gravar `memory.RoleTool` com o
  `resource_id` retornado pela escrita, além de `RoleUser`/`RoleAssistant`.
- [ ] 0.4 Testes: unitários do runtime (injeção server-side, guard anti-simulação, persistência de
  `RoleTool`) e integração (escrita real → linha no banco → Run com `role=tool` persistido), validando a
  correção contra o cenário EP-01/EP-05.

## Detalhes de Implementação

Ver `prd.md` seções `Substrato de escrita/leitura confiável (P0 — bloqueante)` (RF-37..RF-40),
`Evidência de Produção` (EP-01..EP-05) e RTA-08; `techspec.md` seções "Arquitetura do Sistema"
(fluxo `InboundRequest → AgentRuntime.Execute → invokeToolCall → exec → binding → usecase`) e
"Considerações Técnicas". A correção é **exclusivamente** no substrato `internal/platform/agent` e no
schema das tools de escrita/leitura já existentes; não altera o kernel de workflow nem cria capacidade
de domínio. Os arquivos e linhas de defeito estão nomeados nos `<requirements>` acima — batê-los
contra o código real antes de editar. Trechos ilustrativos, se houver, com zero comentários.

## Critérios de Sucesso

- Nenhuma tool de escrita/leitura por usuário expõe `userId`/`wamid`/`itemSeq` no schema ao LLM; esses
  valores vêm do `InboundRequest`/contexto (RF-37/RTA-08). M-07 = 100% (identidade server-side).
- O runtime não confirma sucesso de escrita sem `ToolOutcome` real de sucesso da tool de escrita
  (RF-38); M-05 = 0 no cenário de escrita.
- Cada execução com escrita persiste `memory.RoleTool` + `resource_id` (RF-39); o Run distingue escrita
  real de texto de sucesso (EP-05 corrigido).
- Teste de integração comprova escrita real: `select count(*)` na tabela de destino > 0 após a operação,
  e `platform_messages` contém `role=tool` (EP-01/EP-05 não reproduzem mais).
- Kernel `internal/platform/workflow` inalterado; estados de fronteira permanecem tipos fechados.
- Zero comentários em Go de produção; sem abstração de tempo; sem `var _ I = (*T)(nil)`.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — correção do substrato de agent (`internal/platform/agent`: `invokeToolCall`, `runtime.go`,
  schema de tools) segue o molde do consumidor de referência `internal/agents` sobre `internal/platform`.
- `go-implementation` — mandato do repositório (CLAUDE.md): alteração de plataforma em Go exige Etapas
  1–5 + checklist R0–R7; declarada explicitamente por ser correção de infraestrutura crítica.

## Testes da Tarefa

- [ ] Testes unitários — runtime: injeção server-side de identidade, guard anti-simulação (sucesso só
  com `ToolOutcome` real de escrita), persistência de `memory.RoleTool` com `resource_id`.
- [ ] Testes de integração — escrita real end-to-end via `testcontainers` (`//go:build integration`):
  operação de escrita → assert de linha na tabela de destino (`transactions`/`agents_write_ledger`) e de
  `role=tool` em `platform_messages`; reprodução negativa de EP-01/EP-05.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/platform/agent/runtime.go` (`invokeToolCall`, `buildMessages`, linhas 138-153 e 155-162)
- `internal/platform/agent/ports.go` (contrato de invocação/identidade, se necessário)
- `internal/agents/application/tools/register_expense.go` (remoção de `userId`/`wamid`/`itemSeq` do schema)
- `internal/agents/application/tools/register_income.go`
- `internal/agents/application/tools/register_card_purchase.go`
- `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go` (fluxo de resposta)
