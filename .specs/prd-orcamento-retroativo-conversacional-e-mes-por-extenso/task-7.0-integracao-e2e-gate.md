# Tarefa 7.0: Testes de integração (Postgres) + E2E real-LLM gate estatístico ≥0.90

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Fechar a fatia com o gate de aceitação: suíte de integração Postgres (persistência/unicidade/limpeza) e suíte E2E real-LLM (`RUN_REAL_LLM=1`) com **gate estatístico agregado ≥ 0.90** sobre os cenários-chave. Este gate não existe no código hoje — é criado aqui, com N execuções por cenário e asserção sobre a taxa de acerto.

<requirements>
- RF-04, RF-05, RF-08, RF-09: criação com distribuição, retroativo, sucesso e negação verificados fim-a-fim.
- RF-15, RF-16: mês sem ano / sem referência pedem esclarecimento.
- RF-22, RF-23, RF-24: retrospectiva com/sem orçamento e sem lançamentos.
- RF-26: falha ao persistir devolve mensagem específica e auditável.
- Gate de sucesso do PRD: taxa de conclusão + zero fallback genérico neste caminho + eval real-LLM ≥ 0.90.
</requirements>

## Subtarefas

- [ ] 7.1 Integração Postgres (`//go:build integration`): criação retroativa persistida e ativada; unicidade não duplica; draft futuro tratado como existente; confirmação negada limpa estado; TTL/reaper não deixa run suspenso.
- [ ] 7.2 Suíte E2E real-LLM espelhando `onboarding_workflow_integration_test.go`/`mecontrola_agent_integration_test.go` (modelo `AGENT_HARNESS_MODEL`, default `openai/gpt-4o-mini`).
- [ ] 7.3 Gate estatístico: N execuções por cenário, asserção sobre taxa agregada ≥ 0.90. Cenários: criação com distribuição; retroativo junho/2026; antigo jan/2025; "mês passado"→junho/2026 por extenso; mês sem ano→clarifica; retrospectiva com/sem orçamento e sem lançamentos; competência já existente; confirmação negada; falha de persistência → mensagem específica.
- [ ] 7.4 Verificar ausência do fallback genérico neste caminho quando o use case retorna sucesso.

## Detalhes de Implementação

Ver techspec.md → "Abordagem de Testes" (integração + E2E) e ADR-004/ADR-005. Validação real-LLM obrigatória (`RUN_REAL_LLM=1` com `.env`/`OPENROUTER_*`); mocks não bastam. Instrução-por-exemplo e desc de tool para elevar acurácia (lição de reviews sobre single-shot). Cuidado com falso-verde: o harness deve exercitar o caminho real de tools/workflow, não apenas o workflow diretamente.

## Critérios de Sucesso

- Suíte de integração verde (`-tags integration`) com Postgres real.
- Gate real-LLM agregado ≥ 0.90 nos cenários-chave; evidência registrada.
- Zero fallback genérico neste caminho em sucesso; run falho auditável em falha.
- `go build`, `go vet`, lint verdes; sem falso positivo (harness dirige tools/workflow reais).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — harness de avaliação real-LLM (scorers, gate estatístico) e testes E2E do runtime Thread→Run sobre o substrato de agente.

## Testes da Tarefa

- [ ] Testes de integração Postgres (`//go:build integration`).
- [ ] Testes E2E real-LLM com gate estatístico ≥0.90 (`RUN_REAL_LLM=1`).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/budget_creation_integration_test.go` (novo)
- `internal/agents/application/agents/mecontrola_agent_integration_test.go` (estendido — cenários de orçamento/mês/retrospectiva)
- `internal/agents/application/scorers/mecontrola_scorers.go` (referência — scorers)
