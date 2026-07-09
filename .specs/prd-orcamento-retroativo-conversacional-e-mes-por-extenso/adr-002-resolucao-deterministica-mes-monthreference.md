# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Resolução determinística de mês via tipo fechado `MonthReference` e `Decide*` puro
- **Data:** 2026-07-08
- **Status:** Aceita
- **Decisores:** Time de plataforma / agente financeiro
- **Relacionados:** PRD (RF-13..RF-17), techspec.md, DMMF state-as-type, R-AGENT-WF-001.4 (LLM só nas call-sites sancionadas), feedback "proibido abstrair tempo"

## Contexto

Não existe utilitário de resolução de mês relativo/nomeado (busca por `ParseMonth`/`ResolveMonth`/"mês passado" em `internal/agents` e `internal/platform` sem correspondência). A instrução delega a conversão ao LLM (`mecontrola_agent.go:197`) e, pior, a "REGRA ABSOLUTA DE DATA" (`mecontrola_agent.go:60`) manda **rejeitar** "mês passado"/"semana passada". No incidente, "mês passado" resolveu para "setembro de 2023" (mês errado). As tools `query_month.go`/`query_plan.go` só têm fallback para mês corrente quando o parâmetro chega vazio (`time.Now().In("America/Sao_Paulo").Format("2006-01")`), não corrigem mês incorreto injetado pelo LLM.

`Competence` valida apenas formato `YYYY-MM` (`NewCompetence`, `competence.go:25`), tem `Next()` (`AddDate +1`) mas **não** `Prev()`. O projeto proíbe abstrair tempo (sem `Clock`); usa `time.Now().In(loc)` inline na borda; `Decide*` puros recebem `now time.Time` (ex.: `DecidePendingResume`).

## Decisão

1. Introduzir tipo fechado `MonthReference` (união discriminada) em `internal/budgets/domain/valueobjects/month_reference.go`, com `MonthRefKind` ∈ {`Current`, `Previous`, `Next`, `Explicit(year,month)`, `NamedWithoutYear(month)`, `Unknown`}. Torna estados ilegais irrepresentáveis (DMMF).
2. Adicionar função pura `DecideCompetence(ref MonthReference, now time.Time) (Competence, ClarifyReason, error)`: `Current→CompetenceFromTime(now)`, `Previous→.Prev()`, `Next→.Next()`, `Explicit→NewCompetence(YYYY-MM)`, `NamedWithoutYear→(zero, ClarifyMissingYear, nil)`, `Unknown→(zero, ClarifyUnrecognized, nil)`. Adicionar `Competence.Prev()` (simétrico a `Next()`). `now` chega já convertido para `America/Sao_Paulo` na borda — sem `time.Now()` interno.
3. O LLM passa a **classificar** o texto do usuário em `MonthReference` estruturado (campos no schema das tools de competência), nunca a inventar o `YYYY-MM` de expressões relativas. A autoridade da resolução é `DecideCompetence`. `ClarifyMissingYear`/`ClarifyUnrecognized` disparam pedido de esclarecimento (RF-15/RF-16).
4. Aplicar a `query_month`, `query_plan`, `create_budget` e à retrospectiva (RF-17): as tools resolvem via `DecideCompetence(ref, time.Now().In(loc))`; mantêm fallback para mês corrente quando `ref` ausente. Inclui referências relativas futuras (`Next`) — RF-13/RF-14.
5. Substituir na instrução (`mecontrola_agent.go`) a "REGRA ABSOLUTA DE DATA" que rejeita mês relativo por instrução de classificação em `MonthReference`.

## Alternativas Consideradas

- **Enum relativo leve + `competence string`:** menor mudança de schema, mas "junho sem ano" dependeria de instrução (LLM deixar `competence` vazio) em vez de um estado tipado; risco de o LLM adivinhar o ano. Rejeitada em favor da união discriminada (decisão do usuário; DMMF puro).
- **Parsing NL de português em Go puro:** moveria interpretação linguística para o domínio, frágil e fora do papel do LLM sancionado. Rejeitada.
- **Continuar delegando ao LLM:** é a causa raiz do bug. Rejeitada.
- **Abstrair relógio (`Clock`):** viola feedback do projeto. Rejeitada; `now` injetado na borda.

## Consequências

### Benefícios Esperados

- "Mês passado"/"mês que vem"/"mês atual" resolvem deterministicamente; elimina a classe do bug "setembro de 2023" em todas as tools de competência.
- "Junho" sem ano vira estado explícito → clarificação, sem adivinhação.
- Testabilidade pura por tabela (viradas de ano), sem mock.

### Trade-offs e Custos

- Mudança de schema em `query_month`/`query_plan` (blast radius controlado; fallback preservado).
- Regressão possível na classificação do LLM → mitigada por gate.

### Riscos e Mitigações

- **LLM classifica errado:** `DecideCompetence` é autoridade; instrução por exemplo; gate real-LLM ≥0.90.
- **Virada de ano dez/jan:** `Prev()`/`Next()` via `AddDate` sobre `CompetenceFromTime(now_saopaulo)`; testes de tabela nas viradas.
- **Fuso:** conversão `time.Now().In(LoadLocation("America/Sao_Paulo"))` na borda, fallback UTC (padrão vigente). Rollback: reverter instrução e schema desativa o resolver sem quebrar os demais fluxos.

## Plano de Implementação

1. `MonthReference` + `ClarifyReason` + `DecideCompetence` + `Competence.Prev()` com testes puros.
2. Mapeamento payload LLM → `MonthReference`.
3. Integrar em `query_month`/`query_plan`/`create_budget`/retrospectiva.
4. Atualizar instrução do agente.
5. E2E real-LLM.

## Monitoramento e Validação

- Cenário E2E "mês passado"→junho/2026; "junho" sem ano→clarifica; gate ≥0.90.
- Ausência de reincidência de mês incorreto em produção (amostragem de conversas).

## Impacto em Documentação e Operação

- Instrução do agente atualizada; documentação de contrato das tools de competência.

## Revisão Futura

- Revisar se surgir necessidade de intervalos/trimestres (hoje fora de escopo) ou i18n além de pt-BR.
