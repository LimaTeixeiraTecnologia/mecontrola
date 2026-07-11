# Registro de Decisão Arquitetural (ADR)

## Metadados
- **Título:** Não aplicar padrão GoF — workflow direto espelhando `budget-creation`
- **Data:** 2026-07-10
- **Status:** Aceita
- **Decisores:** Plataforma / autor da techspec
- **Relacionados:** PRD `.specs/prd-editar-orcamento-conversacional/prd.md`; techspec.md; selector `design-patterns-mandatory`

## Contexto
A edição conversacional de orçamento precisa orquestrar 3 operações (editar total, ajustar % de categoria, refazer distribuição) num fluxo durável com coleta, confirmação HITL, suspend/resume, TTL/reaper e replay. Já existe um molde direto — `budget_creation_workflow.go:32-308` — que resolve o caso análogo com um único `Step` e `switch` sobre `state.Awaiting`, sem nenhum design pattern GoF. A governança exige rodar o seletor determinístico antes de aplicar qualquer padrão.

## Decisão
Não aplicar padrão GoF. Implementar a edição como código direto/composição, espelhando o `budget-creation`: um `Step[BudgetEditState]` com `switch` sobre `(Operation, Awaiting)`, funções puras `Decide*`, e o kernel `Engine[S]` provendo suspend/resume. O seletor `scripts/select_pattern.py` retornou `status: reject` para os sinais `state_transition_driven_behavior`, `fixed_workflow_with_variable_steps`, `prefer_direct_solution`, `low_change_frequency`, `snapshot_and_restore` com as restrições `minimize_class_count`, `minimize_indirection`, `preserve_public_contract`, `team_needs_low_cognitive_load`, `avoid_inheritance` — recomendando solução direta.

## Alternativas Consideradas
- **State (GoF) com tipos por estado:** vantagem de encapsular transições; desvantagem: multiplica tipos/indireção para uma máquina de 2 slots já modelada como dado fechado; rejeitado por `minimize_class_count`/`minimize_indirection` e por divergir do molde existente.
- **Strategy por operação:** vantagem de isolar a coleta de cada operação; desvantagem: coordenação de estado/confirmação é comum e governada por transições — Strategy não modela transição; a variação cabe num `switch` sobre `Operation`. Rejeitado (matriz Strategy vs State favorece State quando há transições; e aqui o custo estrutural não se paga).
- **Template Method:** exigiria herança (`avoid_inheritance`). Rejeitado.

## Consequências
### Benefícios Esperados
- Simetria total com `budget-creation` (menor custo cognitivo, revisão trivial).
- Menos tipos e indireção → menor superfície de falha.
- Estados permanecem tipos fechados (DMMF) sem classes por estado.

### Trade-offs e Custos
- O `switch (Operation, Awaiting)` cresce se surgirem muitas operações novas; aceitável dado `low_change_frequency` (3 operações fixas).

### Riscos e Mitigações
- Risco: duplicação acidental de código entre create e edit. Mitigação: extrair helpers comuns (regex de confirmação já compartilhado em `pending_entry_decisions.go:121-122`), reuso de `Decide*`.

## Plano de Implementação
1. Criar `budget_edit_state.go`/`decisions.go`/`workflow.go` espelhando os equivalentes de `budget_creation`.
2. Reutilizar `reConfirmYes`/`reConfirmNo`/`isCancelMessage` e `DecideAllocationKind`/`DecideAllocationsBP`.
3. Não criar novos tipos de estado por operação.

## Monitoramento e Validação
- Critério de sucesso: revisão confirma ausência de padrão GoF e paridade estrutural; `select_pattern.py` reproduzível com `status: reject`.

## Impacto em Documentação e Operação
- Techspec e tasks referenciam esta decisão; sem impacto operacional.

## Revisão Futura
- Revisitar se o número de operações de edição crescer além de ~5 ou se surgir variação combinatória de dimensões (aí reavaliar Strategy/State).
