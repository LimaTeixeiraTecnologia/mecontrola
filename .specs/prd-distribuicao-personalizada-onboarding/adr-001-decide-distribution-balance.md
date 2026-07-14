# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Saldo de distribuição como decisão pura + tipo fechado (Híbrido DMMF)
- **Data:** 2026-07-13
- **Status:** Aceita
- **Decisores:** Solicitante do produto (jailton.junior94), engenharia da plataforma agentiva
- **Relacionados:** `.specs/prd-distribuicao-personalizada-onboarding/prd.md` (RF-04, RF-05, RF-06, RF-15), `docs/us/us-distribuicao-personalizada-onboarding.md` (RN-02, RN-03, RN-04), `techspec.md`, ADR-003, ADR-005

## Contexto

Hoje `DecideAllocationsBP` (`internal/agents/application/workflows/onboarding_workflow.go:265-316`) retorna `(map[string]int, error)` e embute o texto de UI de "passou/faltou" dentro do `error` (linhas 284 e 302). Esse mesmo símbolo é compartilhado com `budget_creation_workflow.go` (`handleBudgetDistributionSlot`, linha 197) e há um teste que guarda o conteúdo das mensagens (`TestDecideErrorMessagesExcludeRenda`, `onboarding_workflow_test.go:2537-2549`). O PRD (RF-04/RF-05/RF-06) exige mensagens que informem exatamente o quanto passou/faltou, reafirmem o alvo e usem a unidade que o usuário empregou. Misturar cálculo de saldo com renderização de UI dentro de `error` fere o princípio DMMF de decisão pura e dificulta testar/reaproveitar a lógica nos dois fluxos.

Restrições: `domain-modeling-production` (state-as-type, `Decide*` puro e determinístico, sem IO); `go-implementation` (menor conjunto seguro de mudanças, sem regressão); `design-patterns-mandatory` (preferir a solução mais simples, aplicar padrão só com evidência).

## Decisão

Introduzir uma função pura nova `DecideDistributionBalance(kind allocationInputKind, valuesBySlug map[string]float64, monthlyBudgetCents int64) DistributionBalance`, onde `DistributionBalance` é uma struct com um tipo fechado `distributionBalanceKind` (`distributionBalanced | distributionOver | distributionUnder`), a unidade usada, o alvo, a soma bruta e o delta absoluto. O texto de UI (delta + reafirmação do alvo + eco dos valores) é renderizado na camada de workflow (`renderBalanceMessage`), não dentro da decisão. `DecideAllocationsBP` deixa de produzir as mensagens de over/under (que passam ao saldo) e passa a fechar 10000 basis points por maior-resto para percentual e reais, mantendo apenas as rejeições estruturais (negativo, confirm-com-valores, orçamento ausente). Ambos os fluxos (onboarding e `budget_creation`) consomem `DecideDistributionBalance` (RF-15).

Gate `design-patterns-mandatory` executado (`scripts/select_pattern.py`): resultado `reject` (não aplicar padrão) — a máquina de estados retomável já é fornecida pelo kernel `internal/platform/workflow`; a solução correta é código direto (funções puras + enum fechado).

## Alternativas Consideradas

- **Mínimo in-place (enriquecer strings dentro de `DecideAllocationsBP`).** Vantagens: menor churn. Desvantagens: mantém UI dentro de `error`, dificulta reuso e teste, não separa decisão de renderização (menos DMMF). Rejeitada por conflitar com `domain-modeling-production`.
- **Refactor completo (outcome fechado único cobrindo aceito/over/under/mixed/confirm em `DecideAllocationsBP`).** Vantagens: máxima pureza. Desvantagens: maior churn nos dois fluxos e em muitos testes; risco de regressão maior. Rejeitada por custo desproporcional (go-implementation: menor conjunto seguro).
- **Padrão GoF State.** Rejeitada: o kernel já modela estados/transições (`StepStatus`, `SuspendReason`, `reviewAwaitKind`); um State pattern duplicaria o mecanismo (`select_pattern.py` → `reject`).
- **Padrão GoF Strategy.** Rejeitada: não há troca de algoritmo em runtime; a unidade (percent/reais) é um dado, não uma estratégia polimórfica.

## Consequências

### Benefícios Esperados

- Decisão de saldo pura, determinística e testável sem mock; estados ilegais irrepresentáveis (tipo fechado).
- Reuso limpo nos dois fluxos sem duplicar lógica (RF-15).
- Mensagens de UI concentradas na camada de workflow, fáceis de ajustar sem tocar a decisão.

### Trade-offs e Custos

- Introduz um tipo e uma função novos e refatora `DecideAllocationsBP` (churn de testes controlado e enumerado na techspec).
- Renderização de mensagem sai da decisão e passa a viver no workflow — exige um ponto de render coerente entre os dois fluxos.

### Riscos e Mitigações

- Risco: regressão de mensagens no `budget_creation`. Mitigação: `DecideBudgetDistribution` (sum=10000) permanece como rede de segurança; suíte do `budget_creation` atualizada e mantida verde (ADR-005). Rollback: reverter para as strings embutidas em `DecideAllocationsBP` (as mensagens antigas ficam preservadas no histórico do PR).

## Plano de Implementação

1. Criar `distributionBalanceKind` e `DistributionBalance` com `String()`/`IsValid()`.
2. Implementar `DecideDistributionBalance` puro + testes unitários (over/under/balanced, unidade correta, delta).
3. Refatorar `DecideAllocationsBP` (maior-resto para ambos; remover over/under) + ajustar testes.
4. Implementar `renderBalanceMessage` no workflow e ligar nos dois handlers (onboarding e budget_creation).
5. Atualizar `TestDecideErrorMessagesExcludeRenda` para cobrir `renderBalanceMessage`.

Concluído quando: build/vet/test race/lint verdes nos dois fluxos e golden real-LLM dos comportamentos de saldo passando.

## Monitoramento e Validação

- Sinais: contador `agents_onboarding_distribution_total{outcome="over"|"under"|"tolerance_absorbed"}` (ADR-004); testes unitários e golden.
- Critério de sucesso: over/under sempre informam delta correto na unidade do usuário; nenhuma mensagem vaza "renda". Revisar se surgir necessidade de novas unidades de entrada além de % e R$.

## Impacto em Documentação e Operação

- Atualizar a US-001/PRD já refletem o comportamento; runbook de onboarding pode citar o novo contador de outcome.
- Sem mudança de runbook operacional de infra.

## Revisão Futura

Revisar se um terceiro fluxo passar a consumir o núcleo compartilhado, ou se surgir uma unidade de entrada adicional (ex.: fração), o que exigiria estender `allocationInputKind` e o render.
