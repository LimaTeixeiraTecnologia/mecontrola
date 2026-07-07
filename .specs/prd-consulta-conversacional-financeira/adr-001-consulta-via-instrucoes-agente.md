# Registro de Decisão Arquitetural (ADR-001)

## Metadados

- **Título:** Roteamento de consulta C1–C7 pelo loop de tool-calling do agente (instruções), não por workflow durável
- **Data:** 2026-07-07
- **Status:** Aceita
- **Decisores:** Autor da techspec, dono do módulo `internal/agents`
- **Relacionados:** PRD `.specs/prd-consulta-conversacional-financeira/prd.md` (spec-version 3, RF-01..RF-09, RF-35), techspec desta pasta, `.claude/rules/agent-workflows-tools.md` (R-AGENT-WF-001.1)

## Contexto

As consultas C1–C7 são **read-only, stateless e idempotentes**: não mutam dados, não pedem
confirmação (HITL) e não têm passo de espera. O substrato oferece dois mecanismos de execução: o
**loop de tool-calling** do `agent.Agent.Execute` (LLM decide as ferramentas por rodada) e o **kernel
de workflow durável** (`internal/platform/workflow`, suspend/resume). O PRD trava D-03/RF-35: a
entrega deve ser instruções + testes (mais uma extensão aditiva de tool), sem novos primitivos.

## Decisão

Resolver o roteamento de consulta **exclusivamente pelas instruções** do agente
(`mecontrolaAgentInstructions`), executadas pelo loop de tool-calling existente
(`WithMaxToolRounds(12)`). Encadeamentos C4 (`resolve_card`→`query_card_invoice`) e C5
(`query_month`→`get_transaction`) ocorrem como rodadas sucessivas do mesmo loop. Não se cria
workflow, step, registry de intent nem `switch case intent.Kind` (proibido por R-AGENT-WF-001.1).

## Alternativas Consideradas

- **Workflow durável por consulta** (`Engine[S]` + steps): retomável e auditável passo-a-passo.
  Rejeitada: consultas não suspendem nem retomam; adicionaria snapshot/persistência sem valor,
  violaria D-03 e inflaria o kernel com semântica de leitura.
- **Registry/switch de intent no consumidor**: um branch por cenário. Rejeitada: proibido por
  R-AGENT-WF-001.1; reintroduz acoplamento que o loop tool-calling elimina.
- **Novas tools "fachada" por cenário** (ex.: `query_overview`): encapsularia C1 num único call.
  Rejeitada: viola D-03 (tool nova) e duplica dados já servidos por `query_month`+`query_plan`.

## Consequências

### Benefícios Esperados

- Zero mudança de arquitetura; risco de regressão mínimo (apêndice de instruções).
- Aderência a mastra (consumir substrato, agente fino) e a R-AGENT-WF-001.1.
- Encadeamento natural sem orquestração adicional.

### Trade-offs e Custos

- Correção depende da qualidade das instruções e do não-determinismo do LLM — mitigado pelo gate
  `M-04 ≥ 0.90` e cenários C1–C7 no harness real-LLM.
- Multi-tool (C1) exige asserter de conjunto no teste (não há um único `expectedTool`).

### Riscos e Mitigações

- **Risco:** agente escolhe tool errada / omite `query_plan` em C1. **Mitigação:** matriz explícita +
  cenários de gate; rollback trivial (reverter o bloco de instruções).

## Plano de Implementação

1. Adicionar bloco "Consultas Financeiras (C1–C7)" à const de instruções.
2. Adicionar cenários C1–C7 ao harness M-04.
3. Validar gate ≥ 0.90 e não-regressão dos 22 cenários existentes.

## Monitoramento e Validação

- `M-04` no log do harness ≥ 0.90; zero alucinação nas cadeias C4/C5.
- Suíte de agents (incl. `pending_entry_*`) verde — não-regressão de escrita/HITL.

## Impacto em Documentação e Operação

- Runbook do agente (se existir) menciona os novos gatilhos de consulta. Sem mudança operacional.

## Revisão Futura

- Revisitar se surgir consulta que exija estado durável (ex.: relatório paginado retomável) — aí o
  kernel de workflow passa a ser candidato.
