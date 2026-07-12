# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Passo de conclusão monta o resumo lendo as fontes de verdade (SuggestAllocation + ListCards)
- **Data:** 2026-07-12
- **Status:** Aceita
- **Decisores:** Time de plataforma (agents), solicitante do produto
- **Relacionados:** `.specs/prd-onboarding-cartao-resumo-conclusao/prd.md` (RF-10..RF-16), `techspec.md`, `internal/agents/application/workflows/onboarding_workflow.go`

## Contexto

O passo de conclusão do onboarding (`BuildConclusionStep`, `onboarding_workflow.go:1036`) hoje recebe apenas `memory.WorkingMemory` e emite uma frase final que cita só o objetivo. O PRD exige um "Resumo de Onboarding" completo — objetivo, meta, orçamento, distribuição por categoria, cartões cadastrados (ou "nenhum cartão 💳") e recorrência — refletindo exatamente o estado persistido (RF-14). O passo é o último da `Sequence` (`:1110`), executado após a etapa de cartões, cobrindo os desfechos com e sem cartão.

Restrições: não alterar o kernel de workflow (R-WF-KERNEL-001); não introduzir estado de domínio novo; reutilizar helpers existentes (`renderAllocationLines`, `categoryLabels`, `money`); manter consistência com a distribuição já mostrada na revisão pré-ativação (que usa `BudgetPlanner.SuggestAllocation`).

## Decisão

`BuildConclusionStep` passa a receber também `interfaces.BudgetPlanner` e `interfaces.CardManager` — ambos já construídos e passados a `BuildOnboardingWorkflow` no wiring (`module.go:231`), portanto sem nova construção. No passo, após o upsert de working memory, ele:

1. chama `budgets.SuggestAllocation(ctx, state.MonthlyBudgetCents, allocationBPList(state.Allocations))` para obter `[]AllocationCents` e renderizar a distribuição com o mesmo helper e a mesma aritmética da revisão pré-ativação;
2. chama `cards.ListCards(ctx, userUUID)` para listar os cartões reais do usuário;
3. compõe `state.FinalMessage` via a função pura `conclusionSummaryMessage(state, items, cards)`, que antecede a cauda inalterada de `conclusionFinalMessage`.

Falhas de qualquer das duas chamadas retornam `failStep` com erro embrulhado, coerente com o tratamento de IO já existente no passo (`WorkingMemory.Upsert`).

## Alternativas Consideradas

- **Recomputar a distribuição localmente (puro, sem BudgetPlanner):** `PlannedCents = MonthlyBudgetCents * bp / 10000`. Vantagem: uma dependência a menos e zero IO extra. Desvantagem: risco de divergência de 1 centavo vs. a revisão pré-ativação (que usa `SuggestAllocation` com distribuição de resto), violando o reflexo exato de RF-14. Rejeitada por consistência/correção.
- **Carregar cartões e distribuição no `OnboardingState` durante os passos anteriores:** Vantagem: conclusão sem IO. Desvantagem: incha o estado durável (persistido em cada snapshot), duplica dados que já têm fonte de verdade e abre janela de inconsistência se o cartão mudar entre passos. Rejeitada por robustez e simplicidade.
- **Manter conclusão só com objetivo (status quo):** Rejeitada: não atende ao PRD.

## Consequências

### Benefícios Esperados

- Resumo exato e consistente com o que o usuário já viu na revisão (mesma fonte de distribuição).
- Cartões sempre atualizados (fonte de verdade), inclusive múltiplos cartões.
- Mudança mínima de assinatura, sem tocar `module.go`, kernel ou estado de domínio.

### Trade-offs e Custos

- Duas chamadas de IO adicionais no passo final do onboarding (uma vez por onboarding). Impacto desprezível.
- `BuildConclusionStep` passa de 1 para 3 dependências injetadas.

### Riscos e Mitigações

- Risco: IO falha e bloqueia a conclusão. Impacto: usuário não recebe o encerramento naquela tentativa. Mitigação: `MaxAttempts: 3` do workflow durável + observabilidade `status="failed"`; rollback trivial (reverter assinatura). Sem degradação silenciosa para não emitir resumo incompleto (RF-14).

## Plano de Implementação

1. Ampliar a assinatura de `BuildConclusionStep` e o registro em `BuildOnboardingWorkflow` (`:1110`).
2. Implementar `conclusionSummaryMessage` (pura) reutilizando `renderAllocationLines`.
3. Testes unitários dos três desfechos (0, 1, ≥2 cartões) com `BudgetPlanner`/`CardManager` mockados.
4. Considerar concluído quando os testes exact-copy do resumo passarem e o gate real-LLM de extração estiver verde.

## Monitoramento e Validação

- `workflow_steps_total{step="step-conclusion",status}` e `onboarding_workflow_total{status}` sem regressão de taxa de sucesso.
- Validação em produção: inspecionar a mensagem final de um onboarding real (Postgres/`otel-lgtm` em `root@187.77.45.48`) confirmando o bloco "📊 Resumo de Onboarding" com distribuição e cartões corretos.
- Revisar se a taxa de `status="failed"` na conclusão subir após o deploy.

## Impacto em Documentação e Operação

- Runbook de onboarding: registrar que a conclusão agora depende de `SuggestAllocation` e `ListCards`.
- Sem mudança em playbooks de alerta (métricas inalteradas).

## Revisão Futura

- Revisitar se o passo de conclusão passar a exigir mais fontes (ex.: metas detalhadas) ou se a latência das duas chamadas se tornar relevante sob carga.
</content>
