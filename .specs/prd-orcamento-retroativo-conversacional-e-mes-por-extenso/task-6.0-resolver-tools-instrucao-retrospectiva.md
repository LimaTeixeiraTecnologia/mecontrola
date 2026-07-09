# Tarefa 6.0: Resolver nas tools de leitura + instrução do agente + composição da retrospectiva

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Aplicar o resolvedor determinístico de mês (`DecideCompetence`) às tools de competência existentes e atualizar a instrução do agente: remover a "REGRA ABSOLUTA DE DATA" que rejeita mês relativo, instruir a emitir `MonthReference` estruturado, citar meses por extenso e compor a retrospectiva planejado vs realizado a partir das tools de leitura existentes (sem tool nova).

<requirements>
- RF-17: resolvedor aplicado a todos os tools que recebem competência (query_month, query_plan, create_budget, retrospectiva), preservando fallback para mês corrente.
- RF-18 (aplicação): toda saída ao usuário que cite competência usa mês por extenso.
- RF-20: retrospectiva planejado vs realizado por categoria e total, com % de execução, mês por extenso.
- RF-21: reutiliza query_plan (planejado+realizado) e query_month (realizado), sem nova fonte de verdade.
- RF-22: mês com orçamento sem lançamentos → comparativo com realizado 0 e execução 0%.
- RF-23: mês sem orçamento com lançamentos → oferece criar E mostra realizado.
- RF-24: mês sem orçamento sem lançamentos → oferece criar.
</requirements>

## Subtarefas

- [ ] 6.1 `query_month.go`/`query_plan.go`: aceitar `MonthReference` estruturado e resolver via `DecideCompetence(ref, time.Now().In(loc))`; manter fallback para mês corrente quando `ref` ausente.
- [ ] 6.2 Instrução (`mecontrola_agent.go`): remover/substituir a "REGRA ABSOLUTA DE DATA" (linha ~60) que rejeita mês relativo; instruir classificação em `MonthReference` (inclui relativo futuro).
- [ ] 6.3 Instrução: citar competência por extenso (via `FormatCompetencePtBR` na saída); mensagem de orçamento não encontrado por extenso; oferta de criação apenas via `create_budget`.
- [ ] 6.4 Instrução: composição da retrospectiva (com orçamento → query_plan; sem orçamento com lançamentos → query_month + oferta; sem nada → oferta), por exemplo (instrução-por-exemplo).
- [ ] 6.5 Testes das tools (whitebox) para resolução e fallback.

## Detalhes de Implementação

Ver techspec.md → "Arquitetura > Modificados", ADR-002 (resolver), ADR-003 (extenso), ADR-004 (retrospectiva por composição). `query_plan` já retorna planejado+realizado+% por raiz; `query_month` cobre realizado. Instrução-por-exemplo mitiga single-shot mascarar acurácia (lição de reviews anteriores). Sem tool nova; sem SQL/branching nas tools além da resolução.

## Critérios de Sucesso

- `go build`, `go vet`, `go test -race`, lint verdes.
- "Mês passado"/"mês que vem"/nomeado resolvem corretamente; sem ano → clarifica; toda menção de mês por extenso.
- Retrospectiva compõe as tools existentes sem nova fonte de verdade; casos com/sem orçamento e sem lançamentos cobertos.
- Zero comentários; tools permanecem finas.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — instrução do agente, contrato das tools de leitura e composição de capacidades read-only sobre o substrato, sem novo switch de intenção.

## Testes da Tarefa

- [ ] Testes unitários das tools (resolução via MonthReference, fallback mês corrente).
- [ ] Testes E2E de instrução cobertos na tarefa 7.0 (gate real-LLM).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/tools/query_month.go` (modificado)
- `internal/agents/application/tools/query_plan.go` (modificado)
- `internal/agents/application/agents/mecontrola_agent.go` (modificado — instrução)
- `internal/budgets/domain/valueobjects/{month_reference.go, competence.go}` (referência — resolver/extenso)
