# Registro de Decisão Arquitetural (ADR-004)

## Metadados

- **Título:** Ativação e recorrência como steps próprios após a confirmação; recorrência negativa/ambígua sem recorrência
- **Data:** 2026-07-10
- **Status:** Aceita
- **Decisores:** Autor da feature, usuário (decisões D-02, D-11, D-13)
- **Relacionados:** PRD (RF-22, RF-24, RF-25), techspec.md, ADR-003

## Contexto

Hoje o step de conclusão faz três coisas: ativa o orçamento, pergunta recorrência de 12 meses e grava
a WorkingMemory (`onboarding_workflow.go:837-892`). O novo fluxo exige ativação **somente após a
confirmação explícita** do resumo (RF-22) e o cadastro de cartões **depois** da ativação. Como o
resumo passou a viver no step `budget_review` (ADR-003), a ativação precisa ocorrer no step seguinte à
sua conclusão. A US é silenciosa sobre recorrência; a decisão D-02 mantém a pergunta logo após a
ativação e antes dos cartões, e D-11 define que resposta negativa ou ambígua segue sem recorrência.

## Decisão

Extrair dois steps próprios entre `budget_review` e `cards`:

- `activation`: `ActivateBudget(competence)` idempotente (tolera `ErrBudgetAlreadyActive`,
  `onboarding_workflow.go:847`) — executa apenas após `budget_review` completar com confirmação (RF-22).
- `recurrence`: suspende com a pergunta de recorrência; no resume, "sim" → `CreateRecurrence(..., 12)`;
  "não" ou resposta ambígua → segue sem recorrência, sem reprompt (D-11). Nenhuma resposta desfaz o
  orçamento ativado.

O step de conclusão fica reduzido a: upsert da WorkingMemory (objetivo financeiro, com valor quando
houver) + montagem da `FinalMessage`.

## Alternativas Consideradas

- **Ativar dentro do `budget_review` no "sim".** Vantagem: um step a menos. Desvantagem: mistura
  confirmação e efeito colateral crítico no mesmo step; dificulta idempotência e trace de ativação.
  Rejeitada.
- **Recorrência ao final (após cartões).** Vantagem: agrupa a recorrência com a conclusão.
  Desvantagem: separa a decisão de recorrência do orçamento recém-ativado; escolha do usuário foi
  logo-após-ativação (D-02). Rejeitada.
- **Reprompt em resposta ambígua de recorrência.** Vantagem: mais preciso. Desvantagem: adiciona turno;
  D-11 optou por tratar ambíguo como "não". Rejeitada.

## Consequências

### Benefícios Esperados

- Ativação isolada, idempotente e rastreável como step próprio (`step-activation`).
- Ordem clara: confirma → ativa → recorrência → cartões (D-02).
- Conclusão simples (WM + mensagem final).

### Trade-offs e Custos

- Dois steps a mais na sequência (cursor maior) — custo desprezível.
- Resposta ambígua de recorrência é tratada como "não" (pode não capturar intenção real).

### Riscos e Mitigações

- **Risco:** ativação falhar após confirmação. **Mitigação:** `failStep` tipado (sem falso sucesso);
  no resume, `ActivateBudget` idempotente; orçamento permanece consistente.
- **Risco:** recorrência falhar. **Mitigação:** `failStep` tipado; orçamento já ativado não é
  desfeito (RF-25).
- **Risco (D-13, aceito):** competência (`AAAA-MM`) recalculada inline com `time.Now()` em cada step
  (status quo, `onboarding_workflow.go:768,846`). Se o onboarding cruzar a virada do mês entre a
  criação do draft no `budget_review` e a ativação, a `ActivateBudget` da competência corrente falha
  por draft inexistente. **Mitigação:** janela rara (onboarding concluído em minutos); `failStep`
  tipado (sem falso sucesso); no resume o `budget_review` recria o draft na competência corrente antes
  da ativação. Alternativa rejeitada: calcular a competência 1x e persistir no estado — não adotada
  por decisão do usuário (menor mudança). Reavaliar se surgir incidência real em produção.
- **Rollback:** recolapsar ativação+recorrência+WM no step de conclusão (perde a ordem D-02).

## Plano de Implementação

1. `BuildActivationStep` (extrai `ActivateBudget` do conclusion atual).
2. `BuildRecurrenceStep` (extrai a pergunta e o `CreateRecurrence`); ambíguo→sem recorrência.
3. Reduzir `BuildConclusionStep` a WM + `FinalMessage`.
4. Inserir os steps na ordem `budget_review → activation → recurrence → cards → conclusion`.

Concluído quando: ativação só ocorre após confirmação; recorrência negativa/ambígua não cria
recorrência; orçamento ativado nunca é desfeito.

## Monitoramento e Validação

- `workflow_steps_total{step="step-activation"|"step-recurrence",status}`.
- Testes de step: ativação idempotente, recorrência sim/não/ambíguo.

## Impacto em Documentação e Operação

- Atualizar diagrama de sequência do onboarding.

## Revisão Futura

Revisar se o produto decidir remover ou reposicionar a recorrência, ou tornar a ativação implícita.
