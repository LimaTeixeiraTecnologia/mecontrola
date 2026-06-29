# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Scorers/Evals (code-based + LLM-judged) com sampling e persistência
- **Data:** 2026-06-29
- **Status:** Aceita
- **Decisores:** Time de plataforma
- **Relacionados:** PRD (RF-40..RF-43), techspec, ADR-002, ADR-003

## Contexto

O exemplo de referência weather (`weather-scorer.ts`) usa scorers code-based (`toolCallAccuracy`, `completeness`) e um scorer LLM-judged com structured output (`translationScorer`, judge via OpenRouter), anexados ao agente com sampling por ratio. O PRD adiciona Scorers/Evals como primitivo (paridade `@mastra/core/evals`), com resultados persistidos em Postgres. Hoje não existe esse conceito.

## Decisão

Implementar `internal/platform/scorer` com um contrato `Scorer` (tipo fechado `ScorerKind`: `codeBased | llmJudged`). Scorers code-based avaliam deterministicamente o `RunSample` (input/output/tool-calls); scorers LLM-judged usam `llm.Complete` com `StructuredContract` (ADR-003) e judge configurável (modelo via OpenRouter). Um `ScorerRunner` é anexado ao agente com `Sampling{Type, Rate}` (tipo fechado `SamplingType`: `ratio | always | never`); a avaliação roda **assincronamente fora do caminho crítico** do agente (RF-41) e persiste `platform_scorer_results` vinculado ao `Run`. Falha de scorer nunca afeta o resultado do agente.

## Alternativas Consideradas

- **Só code-based.** Desvantagem: deixa lacuna vs Mastra (weather usa LLM-judged). Rejeitada.
- **Scorer síncrono no caminho do agente.** Vantagem: simplicidade. Desvantagem: adiciona latência/custo ao caminho crítico e acopla falha de eval à execução. Rejeitada.
- **Sem persistência (só métrica).** Desvantagem: perde auditabilidade do score/reason exigida pelo PRD. Rejeitada.

## Consequências

### Benefícios Esperados

- Paridade com evals do Mastra; avaliação contínua amostrada.
- Auditabilidade (score + reason + metadata) por run.
- Isolamento: eval não impacta latência nem confiabilidade do agente.

### Trade-offs e Custos

- Execução assíncrona exige cuidado com ciclo de vida/cancelamento (sem leak de goroutine).
- LLM-judged consome tokens/custo adicional, controlado por sampling.

### Riscos e Mitigações

- **Risco:** goroutine leak no runner assíncrono. **Mitigação:** worker com `context` cancelável e shutdown cooperativo (R6); sem `init()`/`panic` em produção.
- **Risco:** custo de LLM-judge. **Mitigação:** `Sampling` (ratio) configurável; default conservador.

## Plano de Implementação

1. Definir `Scorer`/`ScoreResult`/`Sampling`/tipos fechados.
2. Implementar code-based (tool-call accuracy, completeness) e LLM-judged via `llm.Complete`+contract.
3. Implementar `ScorerRunner` assíncrono + adapter Postgres `platform_scorer_results`.
4. Unit (sampling, decode), integração (persistência), E2E (weather scorers).

## Monitoramento e Validação

- Métricas `scorer_runs_total`, `scorer_duration_seconds` (labels `scorer_id`, `kind`, `outcome`).
- Critério de sucesso: resultados persistidos e auditáveis; sampling respeitado em teste.

## Impacto em Documentação e Operação

- Runbook descreve configuração de scorers e sampling por agente.

## Revisão Futura

- Revisitar conjunto de scorers prebuilt conforme necessidade de avaliação.
