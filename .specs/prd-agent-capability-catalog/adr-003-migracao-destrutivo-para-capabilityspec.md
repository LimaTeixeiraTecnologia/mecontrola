# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Classificação destrutivo/sensível derivada do CapabilitySpec sem regressão HITL
- **Data:** 2026-06-25
- **Status:** Aceita
- **Decisores:** Time de plataforma (agent)
- **Relacionados:** PRD (RF-12), techspec, ADR-001, ADR-002, `.claude/rules/agent-workflows-tools.md` (R-AGENT-WF-001.1 / .7-A).

## Contexto

A decisão "este kind exige confirmação humana?" vive hoje num mapa ad hoc `intentToOperationKind` (`daily_ledger_agent.go:648-654`) consultado por `isDestructiveKind` (linha 656) e `resolveOperationKind` (linha 661). Cobre 5 kinds: `KindDeleteLastTransaction`, `KindEditLastTransaction`, `KindDeleteCard`, `KindDeleteTransactionByRef`, `KindEditTransactionByRef`. É conhecimento operacional espalhado, desconectado do catálogo, e duplica a noção de "capability sensível" que o `CapabilitySpec.RequiresConfirmation` passa a representar.

Restrições críticas: os gates HITL (R-AGENT-WF-001.7-A) **não podem regredir** — operação destrutiva sem confirmação humana explícita é proibida; `OperationKind` é tipo fechado; proibido novo `case intent.Kind` de domínio no switch (R-AGENT-WF-001.1).

## Decisão

`isDestructiveKind(kind)` passa a derivar de `catalog.Lookup(kind).RequiresConfirmation` — o catálogo é a fonte única do "é destrutivo/sensível?". As 5 capabilities destrutivas são declaradas no catálogo com `RequiresConfirmation: true` e `Mode: ModeWrite`.

O mapa `intentToOperationKind` **permanece** como a tradução `intent.Kind → confirmation.OperationKind` (tipo fechado), consumida por `resolveOperationKind` no fluxo de confirmação. Essa tradução **não é duplicada** no catálogo — o catálogo responde "exige confirmação?" (booleano), o mapa responde "qual OperationKind?" (enum fechado). Separação de responsabilidades: classificação (catálogo) vs. roteamento HITL (mapa de `OperationKind`).

Um teste dedicado garante: para todo kind em `intentToOperationKind`, `catalog.Lookup(kind).RequiresConfirmation == true` — fechando divergência entre as duas estruturas.

## Alternativas Consideradas

- **Mover `OperationKind` para dentro do `CapabilitySpec`:** acoplaria a semântica de confirmação (tipo fechado de operação) ao catálogo de classificação, inflando a struct e misturando responsabilidades; e os kinds destrutivos têm campos específicos do fluxo HITL. Rejeitada — mantida a separação.
- **Manter `isDestructiveKind` consultando só o mapa (status quo):** perpetua conhecimento espalhado e não permite que o catálogo seja fonte única de classificação. Rejeitada.
- **Derivar `RequiresConfirmation` automaticamente da presença no mapa:** inverte a fonte de verdade (mapa → catálogo) em vez de tornar o catálogo canônico; aceitável como implementação interna do `BuildCatalog`, desde que o catálogo seja o ponto consultado pelo runtime. Aceita como detalhe de construção.

## Consequências

### Benefícios Esperados
- "É destrutivo?" passa a ter resposta única e introspectável (catálogo).
- Adicionar capability sensível nova = declarar `RequiresConfirmation: true` + registrar `OperationKind`, num fluxo guiado pelo checklist (RF-15).

### Trade-offs e Custos
- Duas estruturas relacionadas (catálogo + mapa `OperationKind`) mantidas em sincronia; mitigado por teste de consistência.

### Riscos e Mitigações
- **Risco:** regressão de gate HITL (operação destrutiva escapando confirmação). **Mitigação:** teste de consistência catálogo↔`intentToOperationKind`; suíte HITL existente como rede de não-regressão. **Rollback:** `isDestructiveKind` volta a consultar só o mapa (mudança isolada).
- **Risco:** `RequiresConfirmation` divergir do mapa ao adicionar kind. **Mitigação:** o teste falha o build se um kind do mapa não tiver `RequiresConfirmation: true`.

## Plano de Implementação
1. Declarar as 5 capabilities destrutivas no `BuildCatalog` com `RequiresConfirmation: true`, `Mode: ModeWrite`.
2. `isDestructiveKind` consulta `catalog.Lookup(kind).RequiresConfirmation`.
3. Manter `intentToOperationKind`/`resolveOperationKind` inalterados.
4. Teste de consistência catálogo↔mapa + validação dos gates HITL.

## Monitoramento e Validação
- Teste de consistência catálogo↔`intentToOperationKind` verde.
- Suíte de confirmação/HITL verde (sem regressão).
- Nenhuma operação destrutiva executa sem confirmação (invariante preservada).

## Impacto em Documentação e Operação
- Checklist de extensão (RF-15) inclui "novo gate de confirmação": declarar `RequiresConfirmation: true` + registrar `OperationKind`.
- Sem mudança de runbook.

## Revisão Futura
- Revisitar se um terceiro eixo de sensibilidade surgir (ex.: operações que exigem 2FA), avaliando se vira campo do `CapabilitySpec` ou estrutura própria.
