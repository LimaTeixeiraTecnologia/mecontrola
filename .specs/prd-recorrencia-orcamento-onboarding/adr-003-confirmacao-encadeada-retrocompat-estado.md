# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Confirmação encadeada via campo de estado e retrocompatibilidade do estado de recorrência
- **Data:** 2026-07-15
- **Status:** Aceita
- **Decisores:** Jailton (owner), MeControla agents
- **Relacionados:** PRD (RF-10, RF-11, RF-13, RF-18, RF-20); techspec; regra `R-WF-KERNEL-001.7` (resume por merge-patch).

## Contexto

O step de recorrência hoje completa sem confirmar a decisão (nem no "sim", nem no "não"): a recorrência só aparece no resumo final com texto fixo "12 meses" (`recurrenceSummaryLine:966-971`). O RF-10 exige confirmação imediata; o RF-11 exige o resumo refletir o período real; o RF-13 exige o estado guardar a quantidade. O onboarding já tem o padrão de confirmação encadeada: `state.GoalConfirmation` é setado no step de objetivo e prefixado no prompt do step seguinte (`:1113-1118`). O estado é serializado em JSON no snapshot do kernel e o resume aplica merge-patch (RF-20). Existe a possibilidade de onboardings suspensos in-flight que já aplicaram 12 meses sob o código antigo (`Recurrence==true`, sem campo de meses) retomarem no step de conclusão após o deploy.

## Decisão

Adicionar dois campos aditivos ao `OnboardingState`: `RecurrenceMonths int` e `RecurrenceConfirmation string` (o `Recurrence bool` é preservado). O step seta `RecurrenceConfirmation` nos três desfechos que encerram o step (none/default/specific) e `RecurrenceMonths` quando há recorrência; o `BuildCardsStep` prefixa `RecurrenceConfirmation` no prompt inicial e zera o campo, espelhando exatamente o padrão `GoalConfirmation`→`BuildMonthlyBudgetStep`. O `recurrenceSummaryLine` passa a receber `(recurrence bool, months int)` e reflete N. Retrocompatibilidade (RF-20): quando `Recurrence==true && RecurrenceMonths==0` (snapshot legado), exibir 12 meses. Nenhuma migração/drain; a ausência do campo desserializa para zero-value e o merge-patch do resume preserva os demais campos.

## Alternativas Consideradas

- **Mensagem de confirmação como step próprio (suspend extra)**: adiciona um round-trip e um suspend/resume desnecessário; diverge do padrão `GoalConfirmation`. Rejeitada por custo de UX e inconsistência.
- **Só refletir no resumo final (sem confirmação imediata)**: não atende RF-10. Rejeitada.
- **Reaproveitar `Recurrence bool` sem campo de meses e recomputar N no resumo**: impossível recuperar N sem persistir. Rejeitada (RF-13).
- **Migração de snapshots legados para preencher `RecurrenceMonths=12`**: custo operacional desnecessário; o fallback em leitura resolve com segurança. Rejeitada (RF-20 pede sem migração).

## Consequências

### Benefícios Esperados

- Confirmação imediata verificável sem step extra; resumo fiel ao período real.
- Retomada transparente de onboardings in-flight; zero interrupção (RF-20).
- Estado JSON aditivo — compatível com merge-patch do kernel.

### Trade-offs e Custos

- Dois campos novos no estado. O fallback `months==0 → 12` é uma regra de leitura que precisa estar em um único ponto (`recurrenceSummaryLine`) para não divergir.

### Riscos e Mitigações

- Risco: um onboarding novo que resolve `None` tem `RecurrenceMonths==0` e `Recurrence==false` — o fallback (que só dispara com `Recurrence==true`) não o afeta; exibe "desligada". Mitigação: fallback condicionado a `Recurrence==true`. Coberto por teste.
- Risco: `RecurrenceConfirmation` órfão se o fluxo divergir. Mitigação: zerar sempre ao consumir (padrão `GoalConfirmation`), e o campo só é lido no step imediatamente seguinte.

## Plano de Implementação

1. Adicionar campos ao `OnboardingState` (tags JSON).
2. Setar confirmação/meses no `BuildRecurrenceStep`.
3. Prefixar/zerar no `BuildCardsStep`.
4. `recurrenceSummaryLine(recurrence, months)` + fallback legado; `conclusionSummaryMessage` passa `state.RecurrenceMonths`.
5. Teste de retomada legada (`Recurrence==true, RecurrenceMonths==0 → "12 meses"`).

## Monitoramento e Validação

- Sucesso: testes de resumo (N, desligada, legado) verdes; teste de prefixo no cards step verde.
- Reverter: campos aditivos podem ser ignorados; comportamento antigo é o subconjunto `Recurrence bool`.

## Impacto em Documentação e Operação

- Techspec. Sem mudança de schema de banco (snapshot JSON).

## Revisão Futura

- Após o deploy, quando não houver mais snapshots legados relevantes (todos os onboardings in-flight concluídos ou expirados pelo reaper de 7 dias), o fallback `months==0 → 12` pode ser reavaliado.
