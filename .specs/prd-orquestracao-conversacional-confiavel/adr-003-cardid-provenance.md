# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Proveniência de cardId — validação de existência na tool + PostGuard de cadeia
- **Data:** 2026-07-09
- **Status:** Aceita
- **Decisores:** Plataforma / dono do agente MeControla
- **Relacionados:** `prd.md` (RF-16, RF-17, RF-18), `techspec.md`, `adr-001-guard-chain-cor.md`, US-001

## Contexto

RF-17 exige garantia **determinística** de que o `cardId` usado em escrita/consulta de fatura veio de
`resolve_card`/`list_cards` — nunca fabricado pelo texto do usuário. Hoje as tools consumidoras de
cartão (`register_expense`, `create_recurrence`, `query_card_invoice`) recebem `cardId` como string,
parseiam como UUID e delegam ao usecase; a instrução de "cardId só de resolve_card/list_cards" vive
apenas no prompt (`mecontrola_agent.go:94-95, 230`). As tools leem identidade via
`agent.InboundIdentityFromContext(ctx)` e resolvem `resourceId → userID`.

`agent.Result.ToolCalls` expõe `ToolCallRecord{Tool, Outcome, Content}` — nomes e ordem das tools
chamadas, sem os args. A decisão do usuário: **validação de existência na tool + handler de cadeia**.

## Decisão

Garantir proveniência de cartão em **duas camadas determinísticas**:

1. **Tool-level (existência):** as tools consumidoras de `cardId` validam que o `cardId` resolve para um
   cartão real do `resourceId` (usecase de leitura já existente). UUID fabricado/inexistente →
   `card não encontrado` como **erro de domínio limpo** (não crash, não `usecaseError` genérico),
   convertido em `clarify`/fallback pedindo escolha de cartão (RF-18).
2. **Chain-level (`PostGuard` `card_provenance`):** se uma tool consumidora de cartão aparece em
   `Result.ToolCalls` **sem** que `resolve_card` ou `list_cards` a preceda na mesma sequência de
   chamadas → sobrescreve o `Result` para pedir escolha de cartão. Usa apenas `ToolCallRecord.Tool` e a
   ordem — não depende de args (RF-16/17).

As duas camadas são complementares: a (1) barra o UUID inexistente pelo valor; a (2) barra o caso em que
o LLM pula `resolve_card` e inventa um UUID plausível, respondendo genericamente. Nenhuma regra de
domínio vive no guard (R-AGENT-WF-001.2): a resolução/validação é do usecase; o guard só inspeciona a
sequência de tools.

## Alternativas Consideradas

- **Proveniência rastreada no context (set de cardIds resolvidos):** o runtime registraria no context os
  cardIds retornados por `resolve_card`/`list_cards` e a tool rejeitaria qualquer `cardId` fora do set.
  Vantagem: determinismo máximo de proveniência por valor. Desvantagem: exige threading de estado no
  context da plataforma e tocar o `exec` de tool → mais acoplamento/plumbing no substrato. Rejeitada por
  custo desproporcional frente ao ganho, já que a validação de existência + guard de sequência cobrem os
  casos reais.
- **Só validação de existência na tool:** mais simples, mas não cobre o caso "citou cartão, pulou
  resolve_card e alucinou UUID inexistente com mensagem genérica" de forma conversacionalmente correta
  (pedir escolha). Rejeitada por deixar lacuna de UX/segurança.

## Consequências

### Benefícios Esperados

- `cardId` fabricado nunca vira lançamento/consulta na fatura errada (RF-16/17).
- `resolve_card` com `found=false` sempre leva a pedir escolha (RF-18).
- Sem plumbing novo no substrato; aproveita usecase e `ToolCalls` existentes.

### Trade-offs e Custos

- A camada (2) depende de `resolve_card`/`list_cards` aparecerem antes na sequência — se o modelo chamar
  a tool consumidora isoladamente, o guard pede escolha (comportamento seguro, mas pode custar um turno
  extra).

### Riscos e Mitigações

- **Risco:** falso positivo do guard quando `list_cards` foi chamado em turno anterior (não no run
  atual). **Mitigação:** o guard considera a sequência do run; para follow-up, o fluxo reinvoca tool
  (RF-08), então `resolve_card`/`list_cards` reaparece. Golden cobre o cenário multi-turno.
- **Rollback:** remover o guard e manter só a validação de existência (degrada UX, não segurança).

## Plano de Implementação

1. Adicionar validação de existência de `cardId` nas três tools consumidoras (via usecase de leitura),
   com erro de domínio limpo → `clarify`.
2. Implementar `guards/card_provenance.go` como `PostGuard` (sequência de `ToolCalls`).
3. Testes: UUID inexistente → clarify; consumidora sem resolve prévio → pede escolha; caminho feliz.

Concluído quando: as duas camadas cobrem os cenários do golden de cartão.

## Monitoramento e Validação

- `agent_guard_decisions_total{guard="card_provenance", decision="handled"}`.
- Golden de cartão no harness real-LLM (ADR-005): resolve→registro e resolve `found=false`→escolha.

## Impacto em Documentação e Operação

- Runbook do agente: descrever a política de proveniência de cartão.

## Revisão Futura

Reavaliar a camada rastreada-no-context se surgir caso real de proveniência por valor não coberto pela
validação de existência + guard de sequência.
