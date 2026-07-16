# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Topologia de workflows por forma de interação
- **Data:** 2026-07-15
- **Status:** Aceita
- **Decisores:** Solicitante do produto, engenharia de plataforma
- **Relacionados:** PRD `.specs/prd-operacao-conversacional-diaria/prd.md`; techspec `techspec.md`; US `docs/us/us-operacao-conversacional-diaria.md`; R-AGENT-WF-001

## Contexto

A reescrita do dia a dia cobre 13 fluxos. Hoje existem 5 workflows duráveis separados (pending-entry, destructive-confirm, card-create-confirm, budget-creation, onboarding), com estados e continuers próprios. Precisamos organizar os workflows da reescrita minimizando superfície e duplicação, sem cair em branching de domínio proibido por R-AGENT-WF-001, e preservando as invariantes de produção (idempotência, guarda de falso-sucesso, TTL, reprompt).

## Decisão

Organizar em **poucos workflows por forma de interação**, cada um uma `workflow.Definition[S]` com estado fechado próprio:

- `transaction-write`: registro de despesa/receita/recorrência e edição, com slot-filling e confirmação universal antes de gravar.
- `destructive-confirm`: excluir cartão/recorrência com aviso de impacto.
- `budget-manage`: criação retroativa, alterar valor total e alterar distribuição do orçamento (OperationKind `create_retroactive`/`edit_total`/`edit_distribution`).
- `card-manage`: cadastrar e editar cartão.
- `goal-edit`: alterar objetivo na WorkingMemory.

Consultas, resumos e informacionais são tools de leitura sem máquina de estado além da confirmação quando aplicável. A discriminação interna de cada workflow usa `OperationKind` fechado com mapa `map[OperationKind]decideFn`, nunca `switch case intent.Kind`.

## Alternativas Consideradas

- **Um workflow por operação**: máxima isolação; rejeitada por gerar muitos workflows quase idênticos, mais wiring/continuers e reapers, sem ganho de clareza.
- **Um workflow genérico parametrizado por OperationKind**: menos arquivos; rejeitada por concentrar estados heterogêneos num único `S` e tender a um "god-workflow" com branching interno difícil de testar.

## Consequências

### Benefícios Esperados

- Superfície mínima com invariantes agrupadas por família de interação.
- Reuso do padrão durável existente (suspend/resume, merge-patch, reaper).
- Estados fechados por workflow facilitam testes de `Decide*` puros.

### Trade-offs e Custos

- Cada workflow agrega mais de uma operação, exigindo `OperationKind` fechado bem modelado para não virar branching.

### Riscos e Mitigações

- Risco: acoplamento de operações distintas num mesmo `S`. Mitigação: `OperationKind` enumerado + mapa de `Decide*`; cobertura unitária por operação. Rollback: dividir um workflow em dois é local e não afeta os demais.

## Plano de Implementação

1. Definir os estados fechados (`TransactionWriteState`, `DestructiveConfirmState`, `BudgetManageState`, `CardManageState`, `GoalEditState`).
2. Implementar `Decide*` puros por operação.
3. Montar cada `Definition[S]` e registrar engine/def/reaper no `module.go`.

## Monitoramento e Validação

- Métricas por `workflow`/`status`/`outcome`; testes unitários de `Decide*`; gate golden por fluxo.
- Sucesso: 13 fluxos cobertos por 5 workflows + tools de leitura, sem `switch intent.Kind`.

## Impacto em Documentação e Operação

- Atualizar runbook de agentes com a nova topologia e os reapers por workflow.

## Revisão Futura

- Revisitar se um workflow acumular operações com invariantes divergentes o bastante para justificar divisão.
