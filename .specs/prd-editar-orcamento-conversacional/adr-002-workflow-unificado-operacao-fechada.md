# Registro de Decisão Arquitetural (ADR)

## Metadados
- **Título:** Workflow unificado `budget-edit` com `Operation` fechado; uma tool `edit_budget`; remoção da `adjust_allocation` imediata
- **Data:** 2026-07-10
- **Status:** Aceita
- **Decisores:** Plataforma / autor da techspec
- **Relacionados:** PRD (RF-01..RF-03, RF-14..RF-23, D-02, E2); techspec.md; ADR-001

## Contexto
O PRD define 3 operações de edição, cada uma coletando um valor diferente, mas compartilhando confirmação HITL, resolução de mês, robustez e persistência. Hoje a única edição conversacional é a tool `adjust_allocation` (`internal/agents/application/tools/adjust_allocation.go:58`), que aplica a mudança **imediatamente, sem confirmação** e só em orçamento Ativo (G-03). A decisão E2 do PRD determina "uma operação por fluxo" e a D-02 exige confirmação.

## Decisão
1. Modelar um **único workflow durável `budget-edit`** com estado fechado `BudgetEditState` cujo campo `Operation BudgetEditOperation` (`EditTotal | AdjustCategory | Redistribute`, tipo fechado com `String()/Parse/IsValid`) seleciona a coleta. O step faz `switch (Operation, Awaiting{AwaitingEditValue, AwaitingEditConfirm})`.
2. Expor **uma única tool starter `edit_budget`** cujo input traz `operation` (enum obrigatório) + referência de mês + valores opcionais; a desambiguação de operação (RF-02/A1) acontece na camada do agente/LLM antes de chamar a tool com uma operação concreta ("uma operação por fluxo").
3. **Remover a tool `adjust_allocation`** (mutação sem confirmação). O ajuste de categoria passa a ser `edit_budget` com `operation=AdjustCategory`, sujeito ao mesmo gate HITL.
4. **Prefill (eficiência):** o input da tool carrega os valores opcionais já ditos na mensagem inicial (`newTotalCents` para `EditTotal`; `targetRootSlug`+`targetPercentage` para `AdjustCategory`). Quando o valor é completo e válido, o workflow inicia direto em `AwaitingEditConfirm` (pula a coleta); senão inicia em `AwaitingEditValue`. `Redistribute` sempre coleta. A confirmação HITL nunca é pulada.

## Alternativas Consideradas
- **Três tools separadas (edit_budget_total, adjust_allocation com HITL, redistribute_budget):** vantagem de schemas menores; desvantagem: triplica wiring/continuer/reaper e o roteamento LLM entre 3 tools é mais frágil que 1 tool + enum. Rejeitada por economia e por multiplicar superfície de teste.
- **Manter `adjust_allocation` imediata + adicionar as outras com HITL:** cria dois caminhos de escrita (um sem confirmação) — viola D-02 e mantém G-03. Rejeitada.
- **Um workflow por operação (3 definições irmãs):** mais isolamento, porém 3× wiring/reaper e duplicação da máquina de confirmação. Rejeitada (ADR-001 favorece composição mínima).

## Consequências
### Benefícios Esperados
- Um único gate HITL cobre todas as edições (fecha G-03).
- Um wiring, um reaper, um continuer, um golden set.
- `Operation` fechado torna estados ilegais irrepresentáveis (DMMF).

### Trade-offs e Custos
- O schema de input da tool cobre campos usados por operações distintas (alguns opcionais). Mitigação: `Validate()` por operação.
- Remover `adjust_allocation` é mudança de comportamento (deixa de aplicar imediato). Aceito e desejado pelo PRD (D-02).

### Riscos e Mitigações
- Risco: LLM roteia operação errada. Mitigação: gate real-LLM ≥0,90 por categoria; golden cases por operação; descrição/enum explícitos.
- Risco: remoção da tool quebra testes existentes de `adjust_allocation`. Mitigação: migrar/retirar esses testes na mesma task.

## Plano de Implementação
1. Definir `BudgetEditOperation`/`BudgetEditAwaiting` fechados.
2. `edit_budget` tool com `operation` obrigatório + `Validate()` por operação.
3. Remover `adjust_allocation.go` e seu registro em `buildFinancialTools`; ajustar testes/golden.
4. Instrução do agente: quando o pedido de edição for vago, perguntar a operação antes de chamar a tool.

## Monitoramento e Validação
- Métrica `agents_budget_edit_total{operation,outcome}`.
- Gate real-LLM cobre roteamento das 3 operações + caso vago (desambiguação).

## Impacto em Documentação e Operação
- Remover referências a `adjust_allocation` na documentação do agente; atualizar golden.

## Revisão Futura
- Revisitar se surgir necessidade de editar total e distribuição na mesma confirmação (hoje fora de escopo, E2).
