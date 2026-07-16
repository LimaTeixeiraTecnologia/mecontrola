# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Regra de recorrência como decisão pura `DecideRecurrence` com tipos-estado fechados
- **Data:** 2026-07-15
- **Status:** Aceita
- **Decisores:** Jailton (owner), MeControla agents
- **Relacionados:** PRD `.specs/prd-recorrencia-orcamento-onboarding/prd.md` (RF-01, RF-02, RF-03, RF-06, RF-07, RF-08, RF-13); techspec `.specs/prd-recorrencia-orcamento-onboarding/techspec.md`; regra `R-AGENT-WF-001`; skills `domain-modeling-production`, `design-patterns-mandatory`.

## Contexto

O step de recorrência precisa resolver três decisões (sem recorrência / 12 meses / N meses 1–12) mais dois desfechos de repergunta (inválido, ambíguo) a partir de uma resposta em linguagem natural. Hoje a regra está implícita num `if !extract.Confirmed` binário (`onboarding_workflow.go:1539`). O arquivo já adota o padrão DMMF: funções `Decide*` puras (`DecideMonthlyBudgetCents:342`, `DecideDistribution:349`) e enums fechados com `String/IsValid/Parse` (`distributionIntentKind:184-222`). A governança exige regra de negócio em ponto puro e estados de fronteira como tipos fechados (state-as-type), nunca string livre.

## Decisão

Concentrar toda a regra de resolução numa função pura `DecideRecurrence(intent recurrenceIntentKind, hasMonths bool, months int) recurrenceDecision`, sem IO nem `context.Context`, determinística e testável sem mock. Modelar dois enums fechados: `recurrenceIntentKind` (`negative|positive|unclear`) para a intenção extraída, e `recurrenceOutcomeKind` (`none|default|specific|invalid|ambiguous`) para a decisão resolvida — que também é a fonte do rótulo de métrica. `recurrenceDecision{Outcome, Months}` carrega o resultado.

Gate `design-patterns-mandatory`: **não aplicar padrão** GoF (estrutural ou comportamental). A solução direta — função pura + enums fechados + mapa/switch de despacho de outcome no step — é a mais simples, barata e robusta; nenhum sinal justifica Strategy, State, Command ou Chain of Responsibility (não há família de algoritmos intercambiáveis em runtime, nem máquina de estados persistida além do já provido pelo kernel de workflow).

## Alternativas Consideradas

- **Manter branching binário `confirmed bool` e adicionar campos soltos**: simples, porém não representa N meses nem os desfechos de repergunta; espalha regra por `if`s e viola state-as-type. Rejeitada por não atender RF-04/07/08 e por fragilidade.
- **Pattern State/Strategy (GoF)**: encapsular cada outcome numa estratégia. Introduz tipos e indireção sem ganho — o despacho é um `switch` de 5 casos puros. Rejeitada por overengineering (custo estrutural > problema).
- **Resolver a decisão dentro do prompt/LLM (sem função pura)**: delegar a "decisão" ao modelo. Rejeitada: a regra de prioridade e limites 1–12 precisam ser determinísticos e testáveis sem LLM; LLM só extrai sinais.

## Consequências

### Benefícios Esperados

- Regra determinística, coberta por testes unitários sem mock (gate (b) do RF-17).
- Estados ilegais irrepresentáveis (enums fechados); rótulo de métrica derivado do mesmo enum, sem string solta.
- Aderência direta a `R-AGENT-WF-001` (state-as-type) e DMMF (Decide puro).

### Trade-offs e Custos

- Dois enums + uma struct + uma função novos no arquivo (baixo custo; espelham padrões já presentes).

### Riscos e Mitigações

- Risco: divergência entre o rótulo de métrica e o enum. Mitigação: `recurrenceOutcomeKind.String()` é a única fonte do rótulo; teste asserta os 5 valores exatos do RF-16.

## Plano de Implementação

1. Declarar `recurrenceIntentKind`/`recurrenceOutcomeKind` com `String/IsValid/Parse`.
2. Implementar `DecideRecurrence` conforme a precedência RF-06 (quantidade válida vence; fora de 1–12 → invalid; sem quantidade → positiva=12/negativa=none/unclear=ambiguous).
3. Testar `DecideRecurrence` e os enums (round-trip, zero-value, fronteiras 0/1/12/13).

## Monitoramento e Validação

- Sucesso: testes unitários da decisão verdes; `recurrenceOutcomeKind.String()` cobre RF-16.
- Revisão: se surgir necessidade de N>12 ou recorrência por categoria, revisar o enum e a decisão.

## Impacto em Documentação e Operação

- Techspec (este PRD). Sem runbook novo.

## Revisão Futura

- Revisitar se o domínio de budgets ampliar o intervalo de meses ou se a recorrência passar a ter variantes (parcial/por categoria) — hoje explicitamente fora de escopo.
