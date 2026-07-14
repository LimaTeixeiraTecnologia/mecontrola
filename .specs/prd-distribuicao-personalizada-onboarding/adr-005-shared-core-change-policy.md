# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Política de mudança do núcleo compartilhado de distribuição (onboarding + budget_creation)
- **Data:** 2026-07-13
- **Status:** Aceita
- **Decisores:** Solicitante do produto (jailton.junior94), engenharia da plataforma agentiva
- **Relacionados:** `.specs/prd-distribuicao-personalizada-onboarding/prd.md` (RF-12, RF-15), `docs/us/us-distribuicao-personalizada-onboarding.md` (RN-08, RN-09), `techspec.md`, ADR-001, ADR-002, ADR-003

## Contexto

A investigação de blast radius confirmou que 13 símbolos de distribuição são compartilhados entre `onboarding_workflow.go` e `budget_creation_workflow.go`. O crítico é `DecideAllocationsBP` (`onboarding_workflow.go:265-316`), chamado por `handleReviewAwaitDistribution` (onboarding:978) e por `handleBudgetDistributionSlot` (`budget_creation_workflow.go:197`). Mudar esse núcleo afeta os dois fluxos inevitavelmente. O PRD decidiu (RF-15) que as melhorias de núcleo (saldo passou/faltou, tolerância, extração por extenso) valem para ambos, enquanto o modo personalizar e a copy de anúncio ficam exclusivos do onboarding; e exige (RF-12/RN-09) zero regressão.

## Decisão

1. As mudanças no núcleo compartilhado (`DecideDistributionBalance` novo consumido por ambos, `DecideAllocationsBP` refatorado, `allocationInputSystemPrompt` com exemplos por extenso) são deliberadamente comuns aos dois fluxos. Nenhuma função de decisão é duplicada/forkada para isolar fluxos (fere DRY/DMMF).
2. O modo personalizar (`reviewAwaitPersonalize`, classificação de intenção onboarding-only) NÃO é levado ao `budget_creation` (ADR-002). O aviso de categoria zerada (RF-07) também é onboarding-only (não está na lista de melhorias compartilhadas): `budget_creation` não recebe o aviso nesta entrega.
3. `DecideBudgetDistribution` (`budget_creation_decisions.go:50-59`, valida sum=10000) permanece como rede de segurança após `DecideAllocationsBP`.
4. As suítes dos dois fluxos são atualizadas e mantidas verdes como gate de merge. Testes de baseline impactados (enumerados): `onboarding_workflow_test.go:442,482,508,537,545,553,1331,2537,2541,2545,2549`; `budget_creation_workflow_test.go:163,183,235,254`; `budget_creation_decisions_test.go:88,111`; `budget_creation_workflow_integration_test.go:181`. As asserções de over/under migram de `DecideAllocationsBP` para `DecideDistributionBalance`/`renderBalanceMessage`; os prefixos de reprompt e o invariante 10000 permanecem.

## Alternativas Consideradas

- **Forkar as funções compartilhadas para isolar `budget_creation`.** Vantagens: `budget_creation` byte-idêntico. Desvantagens: duplicação de `Decide*` (fere DRY/DMMF), duas fontes de verdade divergindo no tempo. Rejeitada.
- **Aplicar personalizar também no `budget_creation` (paridade total).** Vantagens: comportamento idêntico. Desvantagens: fora do escopo do PRD (RF-15), aumenta superfície e risco nesta entrega. Rejeitada.

## Consequências

### Benefícios Esperados

- Uma única fonte de verdade para a decisão de distribuição; consistência entre os dois fluxos.
- `budget_creation` ganha as melhorias de saldo/tolerância/extenso sem esforço extra (RF-15).

### Trade-offs e Custos

- Churn de testes no `budget_creation` nesta entrega (enumerado e limitado).
- Qualquer mudança futura no núcleo continua sendo dois-fluxos por construção — exige rodar ambas as suítes.

### Riscos e Mitigações

- Risco: regressão silenciosa no `budget_creation`. Mitigação: `DecideBudgetDistribution` como rede de segurança; ambas as suítes obrigatórias no gate; golden real-LLM dos dois fluxos. Rollback: reverter o núcleo para a versão anterior (mensagens embutidas), restaurando os dois fluxos juntos.

## Plano de Implementação

1. Implementar o núcleo (ADR-001/003).
2. Ligar `budget_creation` ao `DecideDistributionBalance` e ao render de saldo no seu reprompt.
3. Atualizar as suítes dos dois fluxos e rodar `go test ./... -race` nos pacotes afetados.

Concluído quando: as duas suítes passam, incluindo os testes enumerados atualizados, e os golden real-LLM dos dois fluxos.

## Monitoramento e Validação

- Sinais: `agents_onboarding_distribution_total` (onboarding) e `agents_budget_creation_total` (budget_creation) coerentes após a mudança.
- Critério: nenhuma regressão nos caminhos de aceite/valores válidos dos dois fluxos; over/under com delta correto em ambos.

## Impacto em Documentação e Operação

- Nota nos runbooks de onboarding e de criação de orçamento de que a decisão de distribuição é compartilhada.

## Revisão Futura

Se um terceiro consumidor surgir, promover o núcleo compartilhado a um pacote de decisão dedicado (ex.: `internal/agents/application/workflows/distribution`) para tornar a fronteira explícita.
