# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Validação simétrica de distribuição de orçamento e não sobrescrita da personalização
- **Data:** 2026-07-10
- **Status:** Aceita
- **Decisores:** time de plataforma / agente financeiro
- **Relacionados:** PRD (RF-01, RF-02, RF-03, RF-04, RF-29), `techspec.md`, US-001

## Contexto

Na jornada real de onboarding via WhatsApp, a cliente informou renda de R$5.000 (500000 cents) e enviou uma distribuição personalizada:

- Custo Fixo R$2.500 (5000 bp)
- Conhecimento R$0 (0 bp)
- Prazeres R$500 (1000 bp)
- Metas R$0 (0 bp)
- Liberdade R$2.000 (4000 bp)

A soma é 5000 + 0 + 1000 + 0 + 4000 = 10000 basis points — ou seja, FECHA 100%. Mesmo assim o orçamento foi ativado com a distribuição PADRÃO (4000/1000/1000/1000/3000), descartando a personalização válida da cliente. Isso configura falso sucesso: o sistema confirmou e persistiu um estado que a usuária não pediu.

A análise do código isolou três vetores independentes que, combinados, produzem o defeito:

- **Vetor (a) — extração LLM ambígua:** o `allocationInputSchema` (`onboarding_workflow.go` L404-416) expõe `action ∈ {confirm, percent, reais}` com 5 valores obrigatórios, mas o `allocationInputSystemPrompt` (L446-450) não desambigua valores em reais (R$2.500) de percentuais. Valores altos podem ser lidos como `percent` ou o LLM pode escolher `confirm` mesmo tendo recebido 5 números.
- **Vetor (b) — confirmação indevida do padrão:** em `DecideAllocationsBP` (`onboarding_workflow.go` L209-257, puro), o ramo `allocationInputConfirm` (L212) SEMPRE clona `defaultDistributionBP`, sem checar se a usuária enviou valores. Se o LLM classificar como `confirm` com valores presentes, o padrão sobrescreve a personalização.
- **Vetor (c) — validação assimétrica no domínio (DEFEITO REAL):** em `internal/budgets/domain/commands/create_budget.go`, `NewCreateBudgetCommand` (L69) usa `if sumBP > 10000`, ou seja, ACEITA `sumBP < 10000`. Isso permite gravar um command com distribuição parcial. As camadas de workflow já são simétricas — `DecideDistribution` (L186-207) e `DecideBudgetDistribution` (`budget_creation_decisions.go` L50-59) exigem `total == 10000` — e o domínio de ativação também: `Budget.Activate` (`budget.go` L143) e `RebalanceAllocations` já exigem `== 10000`. O único ponto com gap era o smart constructor do command.

O invariante de negócio é único e claro: a soma dos basis points de uma distribuição DEVE ser exatamente 10000. A ausência de simetria em uma única camada quebra a garantia de fonte única de verdade.

## Decisão

Impor o invariante `Σ basisPoints == 10000` de forma simétrica em profundidade, com o domínio como autoridade única, e impedir que o padrão sobrescreva personalização válida. Escopo: domínio de `budgets` e os dois workflows (`onboarding` e `budget-creation`) que compartilham `DecideAllocationsBP`.

**1. Vetor (c) — fonte única de verdade no domínio.** Alterar `NewCreateBudgetCommand` (`create_budget.go` L69) de:

```go
if sumBP > 10000 {
```

para:

```go
if sumBP != 10000 {
```

Mantendo `errors.Join` e a sentinela `ErrCommandInvalidAllocation`. O smart constructor do command passa a ser simétrico, alinhado a `Activate` e `RebalanceAllocations`; nenhum draft parcial fica gravável.

**2. Vetor (b) — não sobrescrever personalização.** No ramo `allocationInputConfirm` de `DecideAllocationsBP`, aplicar `defaultDistributionBP` apenas quando `sum(valuesBySlug) == 0`. Se `action == "confirm"` mas houver valores não-nulos, tratar como classificação errada e retornar erro de reprompt (RF-02), impedindo o padrão de descartar a distribuição enviada.

**3. Vetor (a) — endurecer a extração.** Reforçar `allocationInputSystemPrompt` com regra de desambiguação ancorada no caso real:

- valores ≫ 100 ou acompanhados de `R$`/`reais` ⇒ `reais`;
- números pequenos que somam ~100 ⇒ `percent`;
- nunca `confirm` quando vierem 5 números.

Decisão firme (confirmada 2026-07-10): adotar a função pura local `DecideAllocationKind(raw, incomeCents) allocationInputKind` que reclassifica por invariante numérica quando o `action` do LLM vier incompatível (soma≈renda ⇒ `reais`; soma≈100 ⇒ `percent`), sem coagir uma categoria na outra — apenas corrige a classificação; `DecideAllocationsBP` segue rejeitando o que não fecha 100%. Como a classificação LLM é não-determinística e foi o vetor real da falha em produção, essa guarda determinística é obrigatória (não apenas o prompt), complementando o `allocationInputSystemPrompt` endurecido.

**4. Estado de espera tipado para correção (RF-02) — reuso.** O estado de espera já existe e é reutilizado: `AwaitingBudgetDistribution` (budget-creation) e `methodologyReprompt` (onboarding) mantêm o run suspenso via `workflow.SuspendAwaitingInput`. Nenhum `SuspendReason` ou `PendingStatus` novo é introduzido.

**Invariante:** `Σ basisPoints == 10000`, imposto simetricamente em profundidade — domínio (`NewCreateBudgetCommand` + `Activate` + `RebalanceAllocations`) autoritativo; workflow (`DecideDistribution` / `DecideBudgetDistribution`) como guarda de UX antecipada. Nunca em adapter/handler.

## Alternativas Consideradas

- **Corrigir apenas no workflow.**
  - *Descrição:* enrijecer só `DecideDistribution`/`DecideBudgetDistribution` e o ramo de confirmação, sem tocar o domínio.
  - *Vantagens:* mudança concentrada nos consumidores; não mexe no módulo `budgets`.
  - *Desvantagens:* o domínio continua aceitando draft parcial (`sumBP < 10000`); o invariante deixa de ser único e qualquer outro chamador de `NewCreateBudgetCommand` reabre o gap.
  - *Motivo da rejeição:* viola fonte única de verdade — a invariante de domínio precisa viver no domínio.

- **Corrigir apenas o prompt.**
  - *Descrição:* tratar tudo como problema de extração e melhorar só `allocationInputSystemPrompt`.
  - *Vantagens:* nenhuma alteração de código de domínio.
  - *Desvantagens:* a classificação do LLM é não-determinística; sem guarda determinística no domínio, o defeito volta em qualquer regressão do modelo.
  - *Motivo da rejeição:* prompt reduz probabilidade mas não garante o invariante; guarda determinística é obrigatória.

## Consequências

### Benefícios Esperados

- Invariante `Σ bp == 10000` imposto em todas as camadas, com o domínio como autoridade única.
- Personalização válida da cliente nunca mais é sobrescrita pelo padrão (elimina o falso sucesso do caso real).
- Nenhum draft parcial gravável no módulo `budgets`.
- Correção alinhada a `Activate`/`RebalanceAllocations`, sem divergência entre criação e ativação.
- Zero novos estados de espera — reuso de `AwaitingBudgetDistribution` e `methodologyReprompt`.

### Trade-offs e Custos

- `DecideAllocationsBP` e helpers são COMPARTILHADOS entre onboarding e budget-creation: a mudança do ramo `confirm` afeta ambos os fluxos e exige teste dos dois.
- O reprompt de correção adiciona uma volta a mais de conversa quando o LLM classifica errado — trade-off aceito em favor de correção.
- `sumBP != 10000` é mais restritivo: qualquer chamador que dependesse de soma parcial legítima passa a falhar (avaliado como risco baixo, ver abaixo).

### Riscos e Mitigações

- **Risco:** `DecideAllocationsBP` e helpers compartilhados entre onboarding e budget-creation — regressão em um fluxo pode passar despercebida no outro.
  - *Impacto:* médio.
  - *Mitigação:* teste unitário do ramo `confirm` cobrindo os dois consumidores; executar as suites de onboarding e budget-creation.
- **Risco:** vetor (c) pode quebrar chamadores que dependiam de soma parcial legítima ao construir o command.
  - *Impacto:* baixo — `Activate` e `RebalanceAllocations` já exigem `== 10000`, então nenhum orçamento parcial jamais chegou a produção por essas rotas.
  - *Mitigação:* rodar a suite completa de `budgets`; auditar chamadores de `NewCreateBudgetCommand`.
- **Risco:** `BuildDistributionStep` não revalida antes de `CreateBudget`; o erro do command precisa ser propagado corretamente.
  - *Impacto:* médio — erro engolido resultaria em run suspenso ou falha silenciosa.
  - *Mitigação:* garantir tratamento explícito do erro de `NewCreateBudgetCommand` no step, mapeando para reprompt/`StepStatusFailed` observável.
- **Rollback:** reverter os quatro pontos é trivial (troca de operador, condição do ramo, texto do prompt e função pura local); reverter apenas o vetor (c) para `sumBP > 10000` restaura o comportamento anterior sem migração de dados.

## Plano de Implementação

1. Domínio: alterar `NewCreateBudgetCommand` L69 de `sumBP > 10000` para `sumBP != 10000`, preservando `errors.Join` e `ErrCommandInvalidAllocation`.
2. Onboarding: no ramo `allocationInputConfirm` de `DecideAllocationsBP`, aplicar `defaultDistributionBP` só quando `sum(valuesBySlug) == 0`; caso contrário, retornar erro de reprompt.
3. Extração: endurecer `allocationInputSystemPrompt` com a regra de desambiguação e adicionar `DecideAllocationKind` como função pura local determinística (obrigatória).
4. Step: garantir que `BuildDistributionStep` trate e propague o erro de `NewCreateBudgetCommand`.
5. Testes: unit do caso real (renda 500000, distribuição 5000/0/1000/0/4000) nos dois fluxos; integração assertando `budgets_allocations`.
6. Validação: build, vet, test race, lint e gates de governança; suite completa de `budgets`.

- *Dependências:* nenhuma externa; usa `workflow.SuspendAwaitingInput` já existente.
- *Sequência recomendada:* 1 → 2 → 3 → 4 → 5 → 6.
- *Conclusão:* considerada adotada quando os testes unit e de integração passam e a distribuição persistida bate com a enviada.

## Monitoramento e Validação

- **Teste unitário do caso real:** renda 500000, entrada 5000/0/1000/0/4000 ⇒ command válido e distribuição preservada; entrada `confirm` com valores não-nulos ⇒ erro de reprompt.
- **Teste de integração:** assert `budgets_allocations == distribuição enviada`; nunca `4000/1000/1000/1000/3000` quando a cliente personalizou.
- **Sinais:** taxa de reprompt de distribuição no onboarding; ausência de commands com `sumBP != 10000` chegando ao domínio.
- **Critério de sucesso:** nenhum orçamento ativado com distribuição diferente da confirmada pela usuária.
- **Critério de revisão/reversão:** aumento anômalo de reprompts (indício de prompt over-restritivo) ou regressão de extração no modelo.

## Impacto em Documentação e Operação

- Techspec `prd-jornada-whatsapp-financeira-sem-falso-sucesso`: registrar a simetria do invariante e a regra de desambiguação.
- Documentação do fluxo de onboarding/budget-creation: refletir o reprompt de correção (RF-02).
- Sem mudança de runbook operacional; sem nova configuração de observabilidade além dos sinais acima.

## Revisão Futura

- Revisar quando um novo consumidor de `DecideAllocationsBP` for adicionado ou quando o modelo LLM de extração for trocado.
- Premissa que invalida a decisão: introdução de distribuição não-fechada (soma ≠ 100%) como caso de negócio legítimo — exigiria nova ADR.
- **Conformidade:** SEM novo GoF pattern — a solução é troca de operador (`!=`), guarda de ramo (`confirm` só com soma zero), reforço de prompt e função pura local determinística (`DecideAllocationKind`). Zero comentários em código Go.
