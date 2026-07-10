# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Idempotência por pendência/operação e retry controlado de confirmação
- **Data:** 2026-07-10
- **Status:** Aceita
- **Decisores:** time de plataforma / agente financeiro
- **Relacionados:** PRD (RF-09, RF-30), `techspec.md`, ADR-001, US-001

## Contexto

Na jornada financeira via WhatsApp, quando o usuário responde "Sim" para confirmar
uma pendência (`PendingEntryState` em `AwaitingSlotConfirmation`), a escrita é
efetivada via `IdempotentWrite.Execute`. A idempotência da escrita é ancorada na
chave do ledger `agents_write_ledger` = `(wamid, item_seq, operation)`
(`write_ledger_repository.go`, `Insert` com `ON CONFLICT (wamid, item_seq, operation) DO NOTHING`).

Observou-se em produção que **dois "Sim" para a mesma pendência, mas com WAMIDs
diferentes, poderiam duplicar a transação**. O risco decorre de uma leitura
equivocada do fluxo: se a chave usasse o WAMID de cada resposta de confirmação, dois
"Sim" distintos gerariam duas linhas distintas no ledger e, portanto, duas mutações.

A verificação do código real mostra que o defeito **não está** na estrutura da chave:
`pending_entry_workflow.go` L540 passa `state.MessageID` — o **WAMID ORIGINAL** da
mensagem que criou a pendência — como `wamid` da escrita, não o WAMID da resposta de
confirmação. Como `state.MessageID`, `state.ItemSeq` e `state.OperationKind` são
estáveis por pendência, a chave `(wamid_original, item_seq, operation)` já é
efetivamente **por-pendência/operação**. O gap real está em dois pontos:

1. Falta de dedupe explícito na **camada de confirmação**: a decisão pura
   `DecideConfirmation` (`pending_entry_decisions.go` L241-265) já trata replay por
   `ProcessedMessageID` (L246) e expiração (L242), mas o **ramo ACCEPT**
   (`pending_entry_workflow.go` L368) **não grava** `state.ProcessedMessageID` —
   somente o ramo de reprompt o faz (L389). Isso deixa a dedupe por-mensagem da
   confirmação incompleta.
2. Semântica de retry frouxa: o contador `maxFailedWriteResumes = 2`
   (`pending_entry_workflow.go` L409) permite duas tentativas adicionais, divergindo
   da decisão de produto D-10 ("máx 1 retry por confirmação repetida").

Decisões de produto relevantes:
- **D-03:** idempotência por pendência/operação (não por WAMID de cada resposta).
- **D-10:** no máximo 1 retry por confirmação repetida, dentro de TTL de 30 min.

Contratos reais confirmados no código:
- `idempotent_write.go` `Execute` L43-57; replay detection L76-85 (`FindByKey` acha
  linha → `agent.ToolOutcomeReplay` reusando o `resourceID` existente, sem 2ª mutação).
- `write_ledger_repository.go`: chave `(wamid, item_seq, operation)`, `Insert` com
  `ON CONFLICT DO NOTHING`.
- `pending_entry_workflow.go`: L540 usa `state.MessageID` como `wamid`;
  `PendingEntryState` fechado com `ProcessedMessageID`, `ItemSeq`,
  `FailedWriteResumeCount`, `SuspendedAt`; `maxFailedWriteResumes = 2` (L409);
  `IsResumableAfterFailedWrite` (L411) exige `Status==Active`,
  `Awaiting==AwaitingSlotConfirmation`, `len(Candidates)>0`, `!isExpired`,
  `FailedWriteResumeCount<2`; `pendingTTL = 30min`; `PendingEntryStaleAfter = 35min`.
- `pending_entry_decisions.go` `DecideConfirmation` (puro, L241-265): trata replay por
  `ProcessedMessageID` (L246) e expiração via `isExpired` (L242).

## Decisão

1. **NÃO mudar a chave do ledger.** `(wamid_original, item_seq, operation)` já entrega
   semântica por-pendência/operação porque `wamid = state.MessageID` (original e
   estável ao longo de toda a pendência) e `operation`/`item_seq` são estáveis por
   pendência. Um segundo "Sim" com WAMID diferente **reusa a mesma chave** do ledger,
   pois a escrita continua ancorada no WAMID original — não no WAMID da resposta.

2. **Separar replay de retry conforme o resultado da 1ª escrita:**
   - Se a **1ª escrita PERSISTIU** (linha presente no ledger), o segundo "Sim" →
     `FindByKey` acha a linha → `agent.ToolOutcomeReplay` no `IdempotentWrite`,
     retornando o `resourceID` existente → pendência transita para
     `PendingStatusCompleted` **sem 2ª mutação**.
   - Se a **1ª escrita FALHOU antes de persistir** (`run=failed`, estado `Active`
     preservado pelo ADR-001, sem linha no ledger), o segundo "Sim" → **retry
     controlado** via `tryResumeFailedWrite` + `SeedResumeAfterFailedWrite`.

3. **Gravar `state.ProcessedMessageID = state.IncomingMessageID` NO RAMO ACCEPT**
   (`pending_entry_workflow.go` L368). Hoje esse campo só é gravado no reprompt
   (L389). Com isso a dedupe por-mensagem na camada de confirmação passa a valer
   também para o ACCEPT, espelhando `card_create_confirm_workflow.go` L151.

4. **D-10 — reduzir `maxFailedWriteResumes` de 2 para 1** (`pending_entry_workflow.go`
   L409). "1 retry" significa **1 tentativa adicional** (total de até 2 escritas: a
   original + 1 retry). O contador em `IsResumableAfterFailedWrite` e
   `SeedResumeAfterFailedWrite` passa a bloquear a partir da 1ª tentativa adicional.

5. **TTL 30min com transição EXPLÍCITA na expiração.** O TTL continua avaliado no
   resume via `isExpired` (`pending_entry_decisions.go` L126, `now - SuspendedAt > pendingTTL`).
   Quando expirado, transitar explicitamente `Active → PendingStatusExpired` (em vez
   de apenas retornar `false` em `IsResumableAfterFailedWrite`), tornando a expiração
   **visível ao usuário** com mensagem determinística. O reaper
   (`PendingEntryStaleAfter = 35min`, `NewStaleSuspendedReaper`) permanece como rede
   de segurança para snapshots órfãos.

6. **Preservar merge-patch no resume (R-WF-KERNEL-001.7).** O payload de resume
   permanece um delta JSON merge-patch aplicado sobre `snap.State`:
   `{"resumeText":...,"incomingMessageId":...}`. Nenhuma substituição de estado
   inteiro; nenhum tipo de domínio exposto no kernel.

**Impacto:** ajustes localizados em `pending_entry_workflow.go` (ramo ACCEPT,
constante `maxFailedWriteResumes`, transição de expiração explícita). A chave do
ledger, o `IdempotentWrite` e o adapter Postgres permanecem intactos.

## Alternativas Consideradas

### (a) Mudar a chave do ledger para `pending_id`

- **Descrição:** trocar `(wamid, item_seq, operation)` por `(pending_id, item_seq, operation)`.
- **Vantagens:** amarra a idempotência diretamente ao identificador da pendência.
- **Desvantagens:** quebra a idempotência histórica de todas as linhas já persistidas
  em produção (o histórico continua chaveado por WAMID); exige migração de schema e
  backfill; introduz risco operacional desproporcional ao benefício.
- **Motivo de rejeição:** desnecessária — `wamid = state.MessageID` (original) já dá
  semântica por-pendência. O custo de migração e a perda de compatibilidade histórica
  superam qualquer ganho.

### (b) Sem retry — apenas replay ou falha terminal

- **Descrição:** tratar toda 2ª confirmação como replay (se persistiu) ou falha
  terminal (se não persistiu), sem retry controlado.
- **Vantagens:** modelo mais simples, um único caminho.
- **Desvantagens:** contraria RF-09 e a US, que exigem "retry limitado se a primeira
  escrita falhou antes de persistir"; deixa o usuário sem recuperação em falhas
  transitórias (ex.: indisponibilidade momentânea do módulo de transações).
- **Motivo de rejeição:** viola o requisito explícito de retry limitado.

## Consequências

### Benefícios Esperados

- Elimina o risco de duplicação por dois "Sim" com WAMIDs distintos — a escrita
  permanece ancorada no WAMID original e a camada de confirmação passa a deduplicar
  também no ACCEPT.
- Dedupe por-mensagem consistente entre ACCEPT e reprompt, espelhando o padrão já
  validado em `card_create_confirm_workflow.go`.
- Semântica de retry alinhada a D-10 (1 tentativa adicional), reduzindo escritas
  redundantes e evitando o efeito de "martelar" o módulo de transações.
- Expiração visível ao usuário (transição explícita para `PendingStatusExpired`),
  eliminando pendências que "somem" silenciosamente.
- Zero mudança de schema; compatibilidade total com o ledger histórico de produção.

### Trade-offs e Custos

- A correção exige tocar **ambos os caminhos de escrita** (ACCEPT direto e resume após
  falha) para gravar `ProcessedMessageID` de forma consistente — atenção a não deixar
  um caminho sem dedupe.
- Reduzir `maxFailedWriteResumes` diminui a tolerância a falhas transitórias
  consecutivas dentro de uma mesma pendência (mitigado pelo TTL de 30 min e pela
  possibilidade de o usuário reenviar a informação completa).

### Riscos e Mitigações

- **Risco:** habilitar retry após confirmação repetida pode reativar uma pendência
  antes considerada inativa (transição `Cancelled → Active`).
  - **Impacto:** desejado e explícito — permite recuperar uma escrita que falhou antes
    de persistir.
  - **Mitigação:** o retry é **limitado por contador** (`FailedWriteResumeCount < maxFailedWriteResumes = 1`)
    **e por TTL** (`isExpired`, 30 min). Não há loop: após 1 tentativa adicional ou
    após o TTL, a pendência é finalizada (`Expired`/terminal). `IsResumableAfterFailedWrite`
    só permite resume com `Status==Active`, `Awaiting==AwaitingSlotConfirmation` e
    `Candidates>0`.
- **Risco:** interpretação divergente de "1 retry".
  - **Mitigação:** confirmar D-10 como **1 tentativa adicional** (total de até 2
    escritas: original + 1 retry). O teste de contador deve assertar exatamente esse
    limite.
- **Risco:** corrigir apenas um dos caminhos de escrita deixa dedupe parcial.
  - **Mitigação:** o plano cobre **ambos** os caminhos (ACCEPT e resume-após-falha) e o
    teste de regressão exercita os dois com WAMIDs distintos.
- **Plano de rollback:** reverter as três mudanças pontuais (gravação de
  `ProcessedMessageID` no ACCEPT, constante `maxFailedWriteResumes`, transição
  explícita de expiração). Como não há mudança de schema nem de contrato do ledger, o
  rollback é um simples revert de código sem migração reversa.

## Plano de Implementação

1. **ACCEPT dedupe:** no ramo `case ConfirmActionAccept` (`pending_entry_workflow.go`
   L368), gravar `state.ProcessedMessageID = state.IncomingMessageID` antes de invocar
   `executeWrite`, espelhando `card_create_confirm_workflow.go` L151.
2. **Constante D-10:** alterar `const maxFailedWriteResumes = 2` → `= 1`
   (`pending_entry_workflow.go` L409). Renomear o identificador não é necessário
   (mantém `maxFailedWriteResumes`).
3. **Expiração explícita:** no ponto de resume, quando `isExpired(state, now)` for
   verdadeiro, transitar `state.Status = PendingStatusExpired` com `ResponseText`
   determinístico, em vez de retornar `false` silenciosamente. Garantir que o run
   complete (`StepStatusCompleted`), sem permanecer suspenso.
4. **Merge-patch preservado:** manter o payload de resume como
   `{"resumeText":...,"incomingMessageId":...}` aplicado sobre `snap.State` (nenhuma
   substituição de estado inteiro; R-WF-KERNEL-001.7 intacta).
5. **Reaper:** confirmar que `PendingEntryStaleAfter = 35min` continua ativo como rede
   de segurança para snapshots suspensos órfãos.

**Sequência recomendada:** (1) → (2) → (3), com (4) e (5) verificados como invariantes
não-regressivas. **Dependências:** ADR-001 (preservação de estado `Active` após
`run=failed`), do qual o caminho de retry depende.

**Critérios de conclusão:** ambos os caminhos de escrita gravam `ProcessedMessageID`;
contador de retry limita a 1 tentativa adicional; expiração transita explicitamente
para `PendingStatusExpired`; testes de regressão verdes.

## Monitoramento e Validação

- **Métrica:** `agents_pending_entry_total` segmentada por `outcome` (ex.: `completed`,
  `replay`, `retry`, `expired`, `cancelled`) — cardinalidade controlada, sem `user_id`
  como label (R-TXN-004 / R-AGENT-WF-001.5). Acompanhar razão `replay`/`completed` e
  volume de `expired`.
- **Métrica de escrita:** `IdempotentWrite` já emite `total` com labels
  `operation`/`outcome` (`replay` vs `usecase_error`); acompanhar `replay` como sinal
  de dedupe funcionando.
- **Teste de regressão obrigatório:** dois WAMIDs distintos confirmando a **mesma**
  pendência **não** duplicam a transação — assertar exatamente **uma** linha no
  `agents_write_ledger` e **um** `resourceID` retornado (2º "Sim" → `ToolOutcomeReplay`).
- **Teste de retry:** 1ª escrita falha antes de persistir → 2º "Sim" faz **1** retry;
  3º "Sim" além do limite não escreve novamente; `FailedWriteResumeCount` assertado.
- **Teste de expiração:** resume após `pendingTTL` → `PendingStatusExpired` com
  mensagem visível; run completa, não fica suspenso.
- **Critério de sucesso:** zero duplicações observadas em produção para pendências
  confirmadas com múltiplos WAMIDs; retentativas limitadas a 1 por pendência.
- **Critério de reversão:** aumento inesperado de escritas duplicadas ou de pendências
  presas em `Active` sem terminar dentro do TTL.

## Impacto em Documentação e Operação

- **Documentação técnica:** `techspec.md` (seção de idempotência da pendência e retry).
- **Runbook:** `docs/runbooks/transactions.md` — nota sobre semântica replay-vs-retry
  na confirmação de pendências.
- **Observabilidade:** dashboard do agente financeiro — painel de `outcome` de
  `agents_pending_entry_total` e razão de replay.
- **Onboarding:** nenhuma mudança de fluxo de onboarding.

## Revisão Futura

- **Marco de revisão:** revisar caso surja novo tipo de operação com WAMID de escrita
  distinto do WAMID original da pendência (invalidaria a premissa de estabilidade da
  chave), ou caso o volume de `expired` indique TTL de 30 min inadequado.
- **Condições de substituição:** decisão de migrar a chave do ledger para `pending_id`
  (exigiria nova ADR com plano de migração), ou revisão de D-10 para permitir mais de
  1 retry.

## Conformidade

- **Sem novo GoF pattern:** a mudança é ajuste de constante (`maxFailedWriteResumes`) +
  gravação de campo existente (`ProcessedMessageID`) + reuso do mecanismo de
  idempotência já existente (`IdempotentWrite`/`FindByKey`/ledger). Nenhum padrão
  estrutural ou comportamental novo é introduzido.
- **Zero comentários** nos snippets Go (R-ADAPTER-001.1).
- **Merge-patch preservado** no resume (R-WF-KERNEL-001.7); estados permanecem tipos
  fechados (`PendingEntryState`/`ConfirmAction`/`agent.ToolOutcome`).
