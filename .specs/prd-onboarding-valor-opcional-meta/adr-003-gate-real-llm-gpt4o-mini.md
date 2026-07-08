# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Gate de merge via harness real-LLM ≥ 0.90 medido em `openai/gpt-4o-mini`
- **Data:** 2026-07-08
- **Status:** Aceita
- **Decisores:** Jailton (owner), técnica via subagentes
- **Relacionados:** [prd.md](prd.md) (RF-14, Objetivos/Métrica de sucesso), [techspec.md](techspec.md), [adr-001-extracao-combinada-sentinela.md](adr-001-extracao-combinada-sentinela.md)

## Contexto

A qualidade da extração combinada meta+valor só se comprova com LLM real — mocks não garantem correção de linguagem natural, e o projeto já teve falso-verde onde teste com modelo mais forte mascarava a falha real do modelo de produção (memória C4). A extração roda em produção no `openai/gpt-4o-mini`; modelos mais fortes podem inclusive **piorar** certos casos. O PRD fixa acerto ≥ 0.90 como gate de merge (RF-14) e exige política anti-falso-positivo.

## Decisão

1. **Harness real-LLM** em `internal/agents/application/agents/onboarding_goal_value_realllm_test.go`, `//go:build integration`, gate `RUN_REAL_LLM=1` + `OPENROUTER_API_KEY` (reuso de `buildRealLLMProvider`, `mecontrola_agent_realllm_test.go:26`).
2. **Modelo do gate fixado em `openai/gpt-4o-mini`** (default do harness via `AGENT_HARNESS_MODEL`). Modelos mais fortes **não** satisfazem o gate — a medição precisa refletir a experiência de produção.
3. **Casos rotulados** exercitando o `BuildGoalStep` single-shot (extração combinada), cobrindo os 3 cenários (valor junto / ausente / inválido-recusa) e os 5 formatos de RF-09; acerto conjunto (meta E valor) por caso.
4. **Assert `require.GreaterOrEqual(ratio, 0.90)`** sobre `hits/total`. Este é o gate de merge; merge só permitido com o gate verde.

Composição de casos (mínimo; ampliável na implementação sem afrouxar o gate):

| caso | mensagem | meta esperada | centavos esperados |
|---|---|---|---|
| junto-mascarado-virgula | "comprar uma casa, meta de R$ 400.000,00" | presente | 40000000 |
| junto-digitos-puros | "montar uma reserva de 400000" | presente | 40000000 |
| junto-coloquial-mil | "reserva de emergência de 10 mil reais" | presente | 1000000 |
| junto-coloquial-abrev | "meta de viagem, uns 400 mil" | presente | 40000000 |
| junto-coloquial-milhao | "liberdade financeira, 1,5 milhão" | presente | 150000000 |
| so-meta-sem-valor | "quero quitar minhas dívidas" | presente | 0 |
| meta-com-recusa-valor | "quero viajar, mas não sei quanto vou gastar" | presente | 0 |
| valor-invalido-zero | "economizar, uns R$ 0" | presente | 0 |

## Alternativas Consideradas

- **Gate em modelo mais forte (ex.: gpt-4o)**: rejeitada — facilita passar o gate mas não reflete produção; reintroduz o falso-verde já observado.
- **Só testes unitários com LLM mockado**: rejeitada — mock não valida extração NL; não fecha RF-14.
- **Assert tolerante em casos coloquiais marginais**: rejeitada — mascara falha; política anti-falso-positivo exige assert estrito (ex.: "1,5 milhão" → 150000000 exato).

## Consequências

### Benefícios Esperados

- Gate honesto sobre o modelo de produção; bloqueia regressão de extração antes do merge.
- Cobertura explícita dos 3 cenários + 5 formatos.

### Trade-offs e Custos

- Custo de execução real-LLM no CI/local (gated por env; não roda no build padrão).
- Sensível a variação do modelo — mitigado por instruction-by-example (ADR-001).

### Riscos e Mitigações

- **Gate marginal por instabilidade do `hasAmount`** → reforçar instruction-by-example no prompt (ADR-001, R3); ampliar casos sem baixar o limiar. Nunca subir o modelo para "passar".

## Plano de Implementação

1. Implementar o harness após os passos 1–5 do sequenciamento da techspec.
2. Rodar `RUN_REAL_LLM=1 OPENROUTER_API_KEY=... go test -tags integration ./internal/agents/application/agents/ -run GoalValue`.
3. Iterar prompt/exemplos até ratio ≥ 0.90 estável; então liberar merge.

## Monitoramento e Validação

- Critério de sucesso: `ratio ≥ 0.90` reproduzível no gpt-4o-mini.
- Log do harness reporta `hits/total`, o modelo (`AGENT_HARNESS_MODEL`) e o detalhe por caso para diagnóstico.

## Impacto em Documentação e Operação

- Nenhum runbook novo. O comando do harness pode entrar no Taskfile de testes de integração, se desejado (fora do escopo desta entrega).

## Revisão Futura

- Revisitar o limiar/casos se o modelo de produção mudar, ou se novos formatos monetários entrarem em escopo (hoje "por extenso" é fora de escopo, PRD).
