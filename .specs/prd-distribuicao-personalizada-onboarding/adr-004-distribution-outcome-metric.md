# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Contador de outcome do passo de distribuição com rótulo fechado
- **Data:** 2026-07-13
- **Status:** Aceita
- **Decisores:** Solicitante do produto (jailton.junior94), engenharia da plataforma agentiva
- **Relacionados:** `.specs/prd-distribuicao-personalizada-onboarding/prd.md` (RF-16, CS-01, CS-03), `techspec.md`, ADR-001, ADR-002, ADR-003

## Contexto

O PRD define critérios de sucesso mensuráveis (CS-01 conclusão do passo, CS-03 acerto até a 2ª tentativa) e exige (RF-16) um sinal de observabilidade do resultado do passo de distribuição. Hoje o passo de distribuição não emite métrica própria; existe apenas o contador coarse de onboarding (`resolve_onboarding_or_agent.go`, outcome completed/resumed) e o reaper (`workflow_stale_suspended_reaped_total`). As regras de plataforma exigem cardinalidade controlada — proibido `user_id`/`category_id` como rótulo (R-TXN-004, R-AGENT-WF-001.5, R-WF-KERNEL-001.4).

## Decisão

Criar um contador `agents_onboarding_distribution_total`, unidade `"1"`, com um único rótulo fechado `outcome` cujos valores enumerados são: `personalize_entered`, `accepted_default`, `accepted_values`, `over`, `under`, `mixed_unit`, `tolerance_absorbed`. Precedência no aceite de valores: se houve absorção de arredondamento (ADR-003), emite `tolerance_absorbed`; caso contrário `accepted_values`. Cada caminho de retorno dos handlers emite exatamente um outcome. Criação via `o11y.Metrics().Counter(...)` e incremento com `.Add(ctx, 1, observability.String("outcome", v))`, espelhando o precedente `agents_budget_creation_total` (`internal/agents/application/usecases/budget_creation_continuer.go:36-65`). O contador é criado no wiring (`module.go`, a partir de `deps.O11y`) e injetado no passo de distribuição via `observability.Counter`; guarda nil-safe (`if c == nil { return }`) para os testes com `fake.NewProvider()`. Nenhum valor de `outcome` deriva de dado de usuário; o conjunto é fechado e definido em código.

## Alternativas Consideradas

- **Não adicionar métrica (usar só Run/reaper).** Vantagens: zero código. Desvantagens: impossível medir CS-01/CS-03 por caminho (personalizar, over, under). Rejeitada.
- **Rótulos adicionais (canal, step).** Vantagens: mais dimensões. Desvantagens: onboarding tem canal único (WhatsApp) e um único step relevante; dimensões extras sem uso aumentam custo sem ganho. Rejeitada por economia; `outcome` basta.
- **Reusar o contador coarse de onboarding.** Vantagens: um contador só. Desvantagens: o outcome coarse (completed/resumed) não distingue os caminhos do passo de distribuição. Rejeitada; contador dedicado é mais legível.

## Consequências

### Benefícios Esperados

- Medição direta de CS-01 (accepted_*) e CS-03 (razão over/under vs accepted por tentativa).
- Cardinalidade baixa e previsível (7 valores fechados).
- Padrão idêntico ao já usado no projeto (baixo risco).

### Trade-offs e Custos

- Amplia a assinatura de `BuildBudgetReviewStep`/`BuildOnboardingWorkflow` para receber `observability.Observability`/`observability.Counter` (precedente existente com reaper/continuers).

### Riscos e Mitigações

- Risco: esquecer um caminho sem incremento. Mitigação: cada retorno do handler emite exatamente um outcome; teste com `fake.FakeMetrics` valida todos os caminhos.

## Plano de Implementação

1. Criar o contador no wiring e injetá-lo no passo.
2. Incrementar em cada caminho de retorno dos handlers de distribuição/personalizar.
3. Teste de métrica por caminho via `fake.FakeMetrics`.

Concluído quando: todos os 7 outcomes são exercitados por testes e o contador aparece no scrape.

## Monitoramento e Validação

- Dashboards de onboarding existentes ganham o breakdown por `outcome`.
- Critério de sucesso: séries temporais consistentes; soma dos outcomes por run coerente com o número de turnos do passo. Revisar a lista de outcomes se um novo caminho surgir.

## Impacto em Documentação e Operação

- Atualizar `docs/dashboards`/`docs/runbooks` de onboarding com o novo contador e seus valores de `outcome`.

## Revisão Futura

Revisitar se for necessário medir latência/turnos (histograma) além de contagem por outcome.
