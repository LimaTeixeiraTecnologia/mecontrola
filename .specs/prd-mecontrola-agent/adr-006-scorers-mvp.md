# Registro de Decisão Arquitetural (ADR-006)

## Metadados

- **Título:** Conjunto mínimo de scorers/evals do MVP
- **Data:** 2026-06-30
- **Status:** Aceita
- **Decisores:** Time de plataforma
- **Relacionados:** PRD (RF-39, D-24), techspec.md; `internal/platform/scorer/*`; molde `internal/agents/application/scorers/scorers.go`

## Contexto

O PRD exige avaliação de qualidade observável já no MVP, como prova contínua ("production-proof") (RF-39/D-24). O substrato `internal/platform/scorer` já oferece scorers code-based (`NewToolCallAccuracyScorer`, `NewCompletenessScorer`) e LLM-judged (`NewLLMJudgedScorer`), com `ScorerRunner` assíncrono, amostragem (`AlwaysSample`) e persistência (`platform_scorer_results`). O weather usava 3 scorers com `AlwaysSample`.

## Decisão

Definir `BuildMeControlaScorers(provider)` com um conjunto mínimo:

1. **tool-call-accuracy** (code-based) — a intenção do usuário resultou na tool esperada (ex.: "gastei..." → `register_expense`; consulta → `query_month`). Lista de tools válidas por classe de intenção.
2. **completeness** (code-based) — o lançamento/resposta contém os campos essenciais (valor, categoria, meio de pagamento na confirmação de despesa; itens do resumo nas consultas).
3. **categorization** (LLM-judged) — a categoria inferida é plausível para o texto do usuário (ex.: "mercado" → Custo Fixo/Supermercado), com `score` 0–1 + `reason`.

Amostragem configurável (default `AlwaysSample` em dev; ratio reduzido em produção via `RunnerOption`). Resultados persistidos; scorers **fora do caminho principal** (assíncronos), não bloqueiam a resposta. LLM-judged usa o provider único (OpenRouter), call-site sancionada (R-AGENT-WF-001.4).

## Alternativas Consideradas

- **Sem scorers no MVP (só métricas operacionais)** — Vantagem: menos escopo. Desvantagem: sem prova contínua de qualidade exigida pelo PRD. Rejeitada (D-24).
- **Scorers pesados/muitos no MVP** — custo de tokens/latência sem retorno proporcional. Rejeitada; começar mínimo.

## Consequências

### Benefícios Esperados

- Sinal contínuo de qualidade (seleção de tool, completude, categorização) reaproveitando primitivo existente.

### Trade-offs e Custos

- Scorer LLM-judged consome tokens; mitigado por amostragem em produção.

### Riscos e Mitigações

- **Custo/latência** → ratio de amostragem configurável; scorers assíncronos.
- **Falso negativo de categorização** → tratado como sinal, não bloqueio; revisão periódica do dicionário.

## Plano de Implementação

1. Implementar os 3 scorers (2 code-based + 1 LLM-judged) e `BuildMeControlaScorers`.
2. Wire no `module.go` via `ScorerRunner` + `ScoringHooks` (como o weather).
3. Testes dos scorers code-based; LLM-judged atrás de `RUN_REAL_LLM`.

## Monitoramento e Validação

- `scorer_runs_total`, `scorer_duration_seconds`; dashboards de score médio por scorer.
- Sucesso: scores persistidos e correlacionáveis ao Run.

## Impacto em Documentação e Operação

- Dashboard de qualidade do agente; documentar limiares de atenção.

## Revisão Futura

- Ampliar scorers (ex.: aderência ao tom/identidade, segurança de operação destrutiva) após o MVP.
