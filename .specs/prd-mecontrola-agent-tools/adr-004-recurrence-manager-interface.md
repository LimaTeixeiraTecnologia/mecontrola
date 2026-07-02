# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Interface coesa `RecurrenceManager` para os 5 fluxos de recorrência
- **Data:** 2026-07-02
- **Status:** Aceita
- **Decisores:** Autor da techspec, time de plataforma
- **Relacionados:** PRD (RF-14, RF-15, RF-16, RF-17); techspec; R-AGENT-WF-001.2 / R-ADAPTER-001

## Contexto

As tools de recorrência (`list_recurrences`, `create_recurrence`, `update_recurrence`,
`delete_recurrence`) delegam a use cases de `internal/transactions` (Create/Update/Delete/List
RecurringTemplate). A interface de consumidor `TransactionsLedger` já tem 8 métodos; acrescentar mais
4–5 de recorrência a tornaria larga e de baixa coesão (mistura lançamentos e templates recorrentes).

## Decisão

Criar uma interface de consumidor dedicada `RecurrenceManager` em
`internal/agents/application/interfaces/recurrence_manager.go` com os métodos de recorrência, e um
adapter próprio `recurrence_manager_adapter.go` que injeta os use cases
`Create/Update/Delete/List RecurringTemplate`. `TransactionsLedger` recebe apenas os métodos de
**leitura** de lançamentos/faturas (get/search/list), preservando coesão.

## Alternativas Consideradas

- **Inflar `TransactionsLedger`.** Vantagem: menos arquivos. Desvantagem: baixa coesão (Interface
  Segregation), interface grande e difícil de mockar. Rejeitada.
- **Uma interface por use case.** Desvantagem: fragmentação excessiva, mais wiring. Rejeitada.
- **Colocar recorrência em `CardManager`/`BudgetPlanner`.** Desvantagem: recorrência é de
  `transactions`, não de cartão/orçamento. Rejeitada por fronteira de domínio.

## Consequências

### Benefícios Esperados

- Coesão: cada interface de consumidor mapeia uma responsabilidade clara.
- Mocks menores e testes de tool mais focados.
- Segue o idioma já existente (uma interface + um adapter por área).

### Trade-offs e Custos

- Um arquivo de interface + um de adapter + wiring adicional em `module.go`.

### Riscos e Mitigações

- **Risco:** duplicação de conceitos de mapeamento (Raw/EntryRef). **Mitigação:** reutilizar tipos
  agent-owned já existentes (`EntryRef`) e criar apenas `RawRecurrence`/`Recurrence` novos.
- **Rollback:** fundir em `TransactionsLedger` é mecânico se a segregação se provar desnecessária.

## Plano de Implementação

1. Definir `RecurrenceManager` + tipos `RawRecurrence`/`RawUpdateRecurrence`/`Recurrence`.
2. Implementar `recurrence_manager_adapter.go` injetando os 4 use cases de template recorrente.
3. Wiring em `module.go`; passar `RecurrenceManager` a `buildFinancialTools`.

## Monitoramento e Validação

- Spans `agents.binding.recurrence_manager.*`; testes de mapeamento args↔DTO.
- Cobertura das 4 tools de recorrência no harness real-LLM (ADR-002).

## Impacto em Documentação e Operação

- Registrar a nova interface no mapa capacidade→tool do PRD.

## Revisão Futura

- Reavaliar se surgir sobreposição forte entre recorrência e lançamentos que justifique reunificação.
