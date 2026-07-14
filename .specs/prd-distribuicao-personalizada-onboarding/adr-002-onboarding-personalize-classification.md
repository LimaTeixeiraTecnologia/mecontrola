# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Sub-estado fechado `reviewAwaitPersonalize` e classificação de intenção onboarding-only
- **Data:** 2026-07-13
- **Status:** Aceita
- **Decisores:** Solicitante do produto (jailton.junior94), engenharia da plataforma agentiva
- **Relacionados:** `.specs/prd-distribuicao-personalizada-onboarding/prd.md` (RF-01, RF-02, RF-03, RF-08, RF-10, RF-14, RF-15), `docs/us/us-distribuicao-personalizada-onboarding.md` (RN-01, RN-05, RN-13, RN-14), `techspec.md`, ADR-001, ADR-005

## Contexto

No passo de distribuição, responder apenas "não" não abre personalização: o enum de ação compartilhado é `{confirm, percent, reais}` (`allocationInputSchema`, `onboarding_workflow.go:499-511`) e uma recusa colapsa para `confirm` com zeros, aplicando a distribuição padrão à revelia (`DecideAllocationKind` retorna `confirm` quando `sum<=0`, linhas 250-263). O PRD exige (RF-01) que a recusa abra um modo personalizar exclusivo do onboarding (RF-15: `budget_creation` NÃO tem esse modo), pedindo valores por categoria com âncora do orçamento e regra do ZERO, além de detectar unidades misturadas (RF-10) e aceitar valores por extenso (RF-08).

Restrições: `mastra` (LLM só nas call-sites sancionadas, estado durável via suspend/resume, sem `switch` de domínio); `domain-modeling-production` (state-as-type); `go-implementation` (menor conjunto seguro, eficiência — evitar chamadas LLM redundantes); R-AGENT-WF-001.7 (persistir estado de espera antes de pedir input).

## Decisão

1. Adicionar a constante `reviewAwaitPersonalize` ao enum fechado `reviewAwaitKind` (`onboarding_workflow.go:132-154`), com `String()`/`IsValid()` atualizados. O sub-estado persiste automaticamente no `Snapshot.State` (é campo de `OnboardingState`, o `S` de `Engine[S]`) e é retomado por merge-patch antes do parse (`codec.go:31-46`) — sem side-store.
2. Introduzir uma classificação de intenção **onboarding-only em dois passos** (garantia de zero-regressão): (1) pré-classificador de intenção — tipo fechado `distributionIntentKind` (`accept | personalize | values`), `distributionIntentSchema` **apenas** com `action` + `mixed_unit` (não extrai valores) — roda ANTES da extração de valores; (2) quando `action=values`, a extração de valores usa o `allocationInputSchema`/`allocationInputSystemPrompt` **compartilhado e inalterado** (comportamento idêntico ao atual), seguida de `DecideAllocationKind`. O schema/prompt compartilhado NÃO ganha o valor `personalize`; `budget_creation` permanece intocado (RF-15).
3. Roteamento no passo por intenção fechada (não por `intent.Kind` de domínio), com precedência **`values` > `personalize` > `accept`** (se há números utilizáveis, é `values` mesmo com a palavra "não"): `accept`→distribuição padrão→confirmar; `personalize`→`reviewAwaitPersonalize` + `personalizePrompt` (âncora do orçamento + 5 categorias + ZERO); `values`→extração compartilhada→`DecideAllocationKind`→`DecideDistributionBalance` (ADR-001); `mixed_unit=true`→pedir unidade única (curto-circuito antes da extração). O `methodologyPrompt` passa a anunciar a opção "não → personalizar" mantendo o texto "Aceita esta sugestão" (RF-02, preserva o teste `onboarding_workflow_test.go:1386`). Após personalizar e chegar ao resumo, "não" no resumo reabre a distribuição na sugestão padrão (não em personalizar), preservando o comportamento atual (NR-05).

## Alternativas Consideradas

- **Estender o enum de ação compartilhado com `personalize`.** Vantagens: uma classificação só. Desvantagens: altera o schema compartilhado e obriga `budget_creation` a tratar `personalize`, contrariando RF-15 (personalizar é onboarding-only). Rejeitada.
- **Schema superset onboarding-only (uma chamada que também extrai os 5 valores, substituindo a extração compartilhada).** Vantagens: uma chamada LLM no caminho `values`. Desvantagens: a extração de valores do onboarding deixaria de ser a compartilhada, criando risco de divergência de comportamento no caminho `values` existente — inaceitável sob a diretriz de zero-regressão. Rejeitada em favor do design em dois passos, que preserva a extração compartilhada intacta (NR-02); custo aceito: segunda chamada LLM apenas no caminho `values`.
- **Heurística determinística para `mixed_unit`.** Vantagens: sem LLM. Desvantagens: frágil (misturas ambíguas). Rejeitada: a detecção fuzzy fica na call-site LLM sancionada (Structured Output), com branch determinístico a jusante.

## Consequências

### Benefícios Esperados

- Modo personalizar isolado no onboarding, sem tocar `budget_creation` (RF-15).
- Estado de espera fechado e durável; retomada correta por merge-patch (R-AGENT-WF-001.7).
- Extração de valores do onboarding preservada intacta (reutiliza o schema/prompt compartilhado), garantindo zero-regressão no caminho `values` (NR-02).

### Trade-offs e Custos

- Um novo schema/prompt de intenção onboarding-only a manter (apenas `action`+`mixed_unit`); a extração de valores continua sendo a compartilhada (com exemplos por extenso adicionados — RF-08, beneficia ambos os fluxos).
- Segunda chamada LLM no caminho `values` (intenção + extração). Custo aceito deliberadamente em favor de zero-regressão do caminho de valores existente (NR-02); nos caminhos `accept`/`personalize` há apenas uma chamada.

### Riscos e Mitigações

- Risco: classificação de intenção errada (accept vs personalize). Mitigação: enum fechado + Structured Output estrito; ambiguidade re-suspende com orientação (nunca ativa); golden real-LLM obrigatório. Rollback: reverter para o roteamento compartilhado anterior.

## Plano de Implementação

1. Adicionar `reviewAwaitPersonalize` + atualizar `String()`/`IsValid()`.
2. Criar `distributionIntentKind`, `distributionIntentExtract`, `distributionIntentSchema`, `distributionIntentSystemPrompt` (com exemplos por extenso).
3. `handleReviewAwaitDistribution` roteia por intenção; novo `handleReviewAwaitPersonalize`.
4. `personalizePrompt(monthlyBudgetCents)` (âncora + 5 rótulos `categoryLabels` + ZERO) e copy de `methodologyPrompt` (anúncio).
5. Teste de resume Postgres cobrindo persistência do novo sub-estado.

Concluído quando: os cenários de personalizar/mixed-unit passam em unit + golden real-LLM, e `budget_creation` permanece byte-idêntico no comportamento de personalizar (não existe).

## Monitoramento e Validação

- Sinais: `agents_onboarding_distribution_total{outcome="personalize_entered"|"mixed_unit"}` (ADR-004).
- Critério: "não" no passo de distribuição sempre entra em personalizar (nunca aplica default); mixed-unit sempre pede unidade única. Revisar se a taxa de reprompt de personalizar indicar prompt confuso.

## Impacto em Documentação e Operação

- Runbook de onboarding: descrever o novo sub-estado e o outcome `personalize_entered`.
- Sem impacto em infra/observabilidade além do contador de outcome.

## Revisão Futura

Revisitar se `budget_creation` (ou outro consumidor) precisar de modo personalizar — nesse caso, promover a classificação de intenção a um primitivo reutilizável em vez de onboarding-only.
