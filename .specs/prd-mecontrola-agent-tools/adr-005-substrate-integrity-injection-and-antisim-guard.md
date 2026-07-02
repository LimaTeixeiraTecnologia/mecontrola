# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Integridade do substrato de agent — injeção server-side de identidade/idempotência,
  guard bloqueante de anti-simulação e Run auditável com evidência de escrita real
- **Data:** 2026-07-02
- **Status:** Aceita
- **Decisores:** Autor da techspec, time de plataforma
- **Relacionados:** PRD `.specs/prd-mecontrola-agent-tools/prd.md` (RF-37, RF-38, RF-39, RF-40,
  M-05, M-07; D-07; EP-01/EP-02/EP-05); techspec; ADR-003; R-AGENT-WF-001 (.1/.2/.5);
  R-ADAPTER-001; memória `feedback_realllm_validation_required`

## Contexto

A confrontação do PRD com a conversa real do usuário `06edc407-4f63-42e8-b07c-946b9ef0a19c`
(WhatsApp +5511986896322) no ambiente remoto comprovou que o substrato de escrita/leitura assumido
como funcional na spec-version 2 está **quebrado**. Fatos verificados no código e no banco:

- **EP-01 — Sucesso alucinado (escrita perdida).** O agente respondeu "Despesa registrada com sucesso
  ✅", mas `transactions` e `agents_write_ledger` retornaram **0 linhas** para o usuário. Causa raiz:
  identidade/idempotência (`userId`/`wamid`/`itemSeq`) é exigida como **argumento do LLM** — schema
  `Strict:true` com esses campos em `required` (`internal/agents/application/tools/register_expense.go:52`)
  — e o runtime **nunca os injeta** (`internal/platform/agent/runtime.go:173-193`, `buildMessages`, não
  propaga `in.ResourceID`/`in.MessageID`; `internal/platform/agent/agent.go:198-219`, `invokeToolCall`,
  usa `tc.ArgumentsJSON` crus). O modelo não pode fornecê-los corretamente e a escrita se perde.
- **EP-02 — Leitura inoperante.** `query_plan` não foi efetivamente exercida (mesma causa raiz de
  identidade não injetada), apesar de existir budget `2026-07` ativo.
- **EP-05 — Run auditável não discrimina.** Runs em `platform_runs` marcaram `status=succeeded`/
  `outcome=routed` apesar de 0 escritas; `platform_messages` só contém `role=user`/`assistant`, nenhuma
  `role=tool`. O runtime marca sucesso por **conteúdo não-vazio** (`internal/platform/agent/runtime.go:155-162`)
  e só persiste `RoleUser`/`RoleAssistant` (`runtime.go:138-153`), embora `memory.RoleTool` esteja
  definido (`internal/platform/memory/types.go:15`) e nunca gravado. O scorer `anyFinancialToolScorer`
  roda **assíncrono** e não bloqueia a resposta
  (`internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go:163`).

Sem corrigir o substrato, cada tool nova (RF-09..RF-18e) nasce com o mesmo defeito: sucesso alucinado
e escrita perdida. Por isso o PRD absorve a correção como **P0 bloqueante** (D-07, RF-37..RF-40), não
como documento separado — é infraestrutura de plataforma de agent (`internal/platform/agent`), não
capacidade de domínio (FE-03, exceção de plataforma).

## Decisão

Tratar a correção do substrato como **pré-requisito bloqueante** de toda tool (nova ou existente),
com três frentes verificáveis, todas confinadas a `internal/platform/agent` (kernel
`internal/platform/workflow` permanece intocado; tools do consumidor permanecem adapters finos):

1. **Injeção server-side de identidade/idempotência (RF-37).** `userId`/`wamid`/`itemSeq` são
   injetados no ponto de invocação da tool (`invokeToolCall`, `internal/platform/agent`) a partir do
   `InboundRequest`/contexto do Run, e **removidos do schema exposto ao LLM**. Nenhuma tool de escrita/
   leitura por usuário confia em valor de identidade/idempotência fornecido pelo modelo (M-07 = 100%).
   A chave de idempotência `(userID, wamid, itemSeq, operation)` do `IdempotentWrite` passa a ser
   alimentada por esses valores injetados (ADR-003, emenda spec-version 3).

2. **Guard bloqueante de anti-simulação (RF-38).** O runtime NÃO marca `RunStatusSucceeded`/
   `ToolOutcomeRouted` apenas por `result.Content` não-vazio quando a intenção do usuário é uma
   **escrita** e nenhuma tool de escrita retornou um `ToolOutcome` real de sucesso
   (`routed`/`reconciled`/`replay`). O guard é síncrono no caminho da resposta — não pode depender do
   scorer assíncrono. Substitui a lógica atual de `runtime.go:155-162` que promove sucesso por qualquer
   conteúdo (M-05 = 0).

3. **Run auditável com evidência de escrita real (RF-39).** Cada execução persiste as mensagens de
   tool (`memory.RoleTool`, hoje definido e não usado) e o `resource_id` retornado pela escrita, de
   modo que o Run distinga escrita real de texto de sucesso. Estende a persistência de `runtime.go:138-153`
   para além de `RoleUser`/`RoleAssistant`, mantendo cardinalidade de métricas controlada (RF-28,
   R-AGENT-WF-001.5): labels restritos a enums fechados; sem `user_id`/`correlation_key`/`category_id`.

Os estados de fronteira (`RunStatus`/`ToolOutcome`/`MessageRole`) permanecem tipos fechados (DMMF
state-as-type), nunca string livre.

## Alternativas Consideradas

- **Manter identidade como argumento do LLM e reforçar as instruções.** Desvantagem: não corrige a
  causa raiz (o runtime não propaga `ResourceID`/`MessageID`); o modelo continua sem fonte confiável;
  viola a separação adapter fino / determinismo. Rejeitada — foi exatamente o que falhou em produção.
- **Confiar no scorer assíncrono como guard.** Desvantagem: `anyFinancialToolScorer` roda fora do
  caminho da resposta (`whatsapp_inbound_consumer.go:163`) e não bloqueia; pontua 1.0 se qualquer tool
  financeira for chamada. Não impede sucesso alucinado. Rejeitada.
- **Marcar sucesso por conteúdo não-vazio (status quo).** Desvantagem: é a origem do EP-01/EP-05.
  Rejeitada.
- **Documento/PRD separado para o fix de substrato.** Desvantagem: desacopla o gate P0 das tools que
  dependem dele; reintroduz o risco de expor tool sobre substrato quebrado. Rejeitada por D-07.

## Consequências

### Benefícios Esperados

- Elimina sucesso alucinado e escrita perdida (EP-01), fechando M-05 = 0 e M-07 = 100%.
- Leituras por usuário passam a receber identidade correta (EP-02), destravando `query_plan` e as
  novas tools de leitura.
- Run auditável passa a discriminar escrita real de texto (EP-05), sustentando RF-27/RF-29 e o assert
  de linhas no banco (ADR-002 emenda / RF-29).

### Trade-offs e Custos

- O ponto de invocação de tool (`invokeToolCall`) ganha responsabilidade de injeção — deve permanecer
  genérico (chaves opacas) e não vazar semântica de domínio para o kernel.
- O guard de anti-simulação exige distinguir intenção de escrita vs. leitura de forma determinística,
  a partir da tool efetivamente chamada (não de heurística sobre o texto).

### Riscos e Mitigações

- **Risco:** injeção acoplar o substrato a campos específicos de domínio. **Mitigação:** injetar apenas
  identidade/idempotência opaca (`resourceId`/`threadId`/`messageId`), sem tipo de domínio; kernel
  intocado.
- **Risco:** guard classificar leitura como escrita e bloquear resposta legítima. **Mitigação:** o
  guard só atua quando a tool chamada é de escrita e não retornou sucesso; leituras não são afetadas.
- **Rollback:** as três frentes são aditivas ao runtime; reverter restaura o comportamento anterior
  (não recomendado — reintroduz o defeito P0).

## Plano de Implementação

1. Propagar `ResourceID`/`ThreadID`/`MessageID` do `InboundRequest` ao contexto do Run e injetá-los
   em `invokeToolCall`; remover `userId`/`wamid`/`itemSeq` dos schemas de tool de escrita/leitura.
2. Substituir o critério de sucesso por conteúdo em `runtime.go` por guard baseado no `ToolOutcome`
   real das tools de escrita.
3. Persistir mensagens `memory.RoleTool` e `resource_id` no Run; manter labels de métrica fechados.
4. Refletir a mudança de schema na ADR-003 (idempotência alimentada por valores injetados) e no
   harness real-LLM (ADR-002, assert de linhas reais).

## Monitoramento e Validação

- M-05 (incidentes de sucesso simulado) = 0, verificado por **assert de linhas reais** no banco
  (`transactions`/`transactions_card_purchases`/`agents_write_ledger`/`transactions_recurring_templates`)
  no harness real-LLM (RF-29, ADR-002).
- M-07 (escritas com identidade injetada server-side) = 100%.
- Run auditável contém `role=tool` e `resource_id` para toda escrita; nenhum Run marca sucesso sem
  `ToolOutcome` de sucesso da tool de escrita.

## Impacto em Documentação e Operação

- Runbook do agente: documentar que identidade/idempotência é server-side e que sucesso de escrita só
  é reportado com linha real no banco.
- Este ADR é **pré-requisito de todas as tools** deste PRD: sem ele, sucesso alucinado. Nenhuma tool
  nova (RF-09..RF-18e) é considerada coberta/exercida enquanto o substrato não estiver corrigido e
  verificado por escrita real (RF-40).

## Revisão Futura

- Reavaliar o guard de anti-simulação se novas classes de operação (além de escrita/leitura) forem
  introduzidas.
- Reavaliar a injeção se surgir canal não-WhatsApp que não forneça `wamid`/`messageId`.
