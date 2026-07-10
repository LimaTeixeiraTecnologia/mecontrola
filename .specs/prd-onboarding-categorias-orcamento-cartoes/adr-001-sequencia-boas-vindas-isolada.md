# Registro de Decisão Arquitetural (ADR-001)

## Metadados

- **Título:** Reordenação da sequência de onboarding, boas-vindas isolada e categorias+orçamento em mensagem única
- **Data:** 2026-07-10
- **Status:** Aceita
- **Decisores:** Autor da feature, usuário (decisões D-01, D-07)
- **Relacionados:** PRD `prd-onboarding-categorias-orcamento-cartoes/prd.md` (RF-01..RF-14, RF-03, RF-12), techspec.md, ADR-002, ADR-006

## Contexto

O onboarding atual combina boas-vindas com a pergunta de objetivo num único prompt
(`onboarding_workflow.go:455` `welcomeGoalPrompt`) e apresenta as categorias apenas no step de
metodologia, depois de coletar renda. O fluxo desejado exige boas-vindas isolada como primeiro passo,
meta no segundo, apresentação das 5 categorias no terceiro com avanço imediato para a coleta de
orçamento mensal (sem "Faz sentido?").

Restrição técnica dominante: `Engine.Resume` retoma sempre no `snap.Cursor` do step suspenso
(`engine.go:263,306`; `combinators.go:31`). Cada step suspende na primeira entrada (`ResumeText==""`)
e processa no resume. O cadastro do onboarding parte de `PhaseWelcome`
(`resolve_onboarding_or_agent.go:83,124`), mas hoje nenhum step suspende em boas-vindas isolada.

## Decisão

Refatorar a `Sequence` de steps para a ordem: `welcome → goal → monthly_budget → budget_review →
activation → recurrence → cards → conclusion`. Introduzir `BuildWelcomeStep` como step isolado que
suspende com a apresentação do MeControla e, no resume, **completa ignorando o texto** (D-07: a
resposta é apenas gatilho de avanço; a meta é sempre perguntada no step seguinte). Remover o preâmbulo
de boas-vindas do prompt do step de meta. No step `monthly_budget`, entregar a apresentação das 5
categorias (texto exato RF-11) e a pergunta de orçamento mensal numa **única mensagem** com um único
suspend (D-01), sem confirmação intermediária.

Escopo: apenas `internal/agents/application/workflows/onboarding_workflow.go`. Wiring
(`module.go:231`) e usecase de start/resume permanecem inalterados.

## Alternativas Consideradas

- **Duas mensagens para categorias + orçamento (um wait).** Enviar apresentação como mensagem
  informativa e a pergunta separada. Vantagem: apresentação como mensagem distinta. Desvantagem: exige
  emitir mensagem não-suspensa fora do fluxo de prompt do kernel (o append hoje ocorre no prompt de
  suspend, `onboarding_workflow.go:923`). Rejeitada: maior complexidade sem ganho de produto (D-01).
- **Interpretar a resposta da boas-vindas como possível objetivo.** Extrair meta já na resposta ao
  welcome. Vantagem: menos turnos. Desvantagem: mistura passos, adiciona risco de extração precoce e
  quebra a previsibilidade. Rejeitada (D-07).

## Consequências

### Benefícios Esperados

- Primeira mensagem limpa e previsível (M-01).
- Apresentação das categorias antes da coleta de valores, com avanço imediato (RF-09..RF-12).
- Menor superfície de mudança: reordenar slice de steps + um step novo, sem tocar o kernel.

### Trade-offs e Custos

- Um turno adicional (welcome isolado) antes da meta — aceito pela clareza.
- O step de boas-vindas descarta o texto da resposta (D-07); qualquer objetivo adiantado é re-perguntado.

### Riscos e Mitigações

- **Risco:** ordem/cursor inconsistente após reordenar. **Mitigação:** cada step preserva o gate
  `ResumeText==""` e define sua `Phase`; testes de step cobrem a cadeia welcome→goal→monthly_budget.
- **Rollback:** reverter o slice de `BuildOnboardingWorkflow` e o prompt combinado.

## Plano de Implementação

1. Redefinir o enum `OnboardingPhase` (`PhaseWelcome`/`PhaseGoal`/`PhaseCards`/`PhaseConclusion` já
   existem; adicionar `PhaseMonthlyBudget`/`PhaseBudgetReview`/`PhaseActivation`/`PhaseRecurrence` e
   remover `PhaseMonthlyIncome`/`PhaseMethodology`/`PhaseDistribution`/`PhaseSummary`) e criar `BuildWelcomeStep`.
2. Reescrever prompts (welcome isolado, goal sem preâmbulo, monthly_budget = categorias RF-11 + orçamento).
3. Reordenar a `Sequence` em `BuildOnboardingWorkflow`.
4. Testes de step da nova cadeia inicial.

Concluído quando: welcome suspende isolado; resposta ao welcome leva à pergunta de meta; step de
orçamento apresenta as 5 categorias e pergunta o valor num único suspend.

## Monitoramento e Validação

- `workflow_suspend_total{workflow="onboarding-workflow"}` mostra suspends em `step-welcome` e
  `step-monthly-budget`.
- Gate M-01 (primeira mensagem só boas-vindas) e M-02 (sem "renda líquida").
- Teste de step + harness real-LLM (RF-42).

## Impacto em Documentação e Operação

- Atualizar runbook de onboarding (se houver) com a nova sequência.
- Sem mudança operacional, migration ou config.

## Revisão Futura

Revisar se a taxonomia de categorias mudar, se o gatilho de início do onboarding mudar, ou se o
kernel passar a suportar retorno a step anterior (o que reabriria a discussão de ADR-003).
