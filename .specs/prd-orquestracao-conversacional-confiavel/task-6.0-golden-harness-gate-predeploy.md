# Tarefa 6.0: Golden set + harness em dois níveis + gate pré-deploy

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar o golden set versionado e o harness de avaliação em dois níveis: determinístico no CI por-PR e
real-LLM ≥ 0,90 por categoria como gate pré-deploy bloqueante.

<requirements>
- RF-35: golden set versionado cobrindo registro despesa/receita, C1–C7, cartões, orçamento,
  recorrências, onboarding, pendências, confirmações, follow-up, erro de tool, ambiguidade, formato
  WhatsApp, ausência de termos internos.
- RF-36: cada caso declara input, tool esperada, args esperados, outcome esperado e resposta/propriedade
  verificável.
- RF-37: sintéticos curados + incidentes reais reescritos/anonimizados (sem PII/WAMID/resourceId);
  nada verbatim de produção.
- RF-38: medir por versão do agente: tool-call accuracy, completude, categorização, taxa de falha,
  duração p95, truncamento.
- RF-39: gate pré-deploy = golden determinístico + testes de guard/scorer + real-LLM ≥ 0,90 por
  categoria.
- RF-40: CI por-PR só determinístico; real-LLM sob tag `realllm`/nightly + pré-deploy.
- RF-41: deploy bloqueia quando threshold do gate pré-deploy cair abaixo da baseline.
- RF-07/RF-08: golden valida roteamento C1–C7 com dados de tool e follow-up reinvocando tool.
</requirements>

## Subtarefas

- [ ] 6.1 Fixtures do golden em `internal/agents/application/golden/` (sintéticos + incidentes
  anonimizados), com o schema de caso (input/expectedTool/expectedArgs/expectedOutcome/responseProperty).
- [ ] 6.2 Harness determinístico (CI): asserts puros de guards/scorers sobre `RunSample` fixos + casos
  golden que não exigem LLM.
- [ ] 6.3 Harness real-LLM (`//go:build realllm`): dirige `BuildMeControlaAgent` com OpenRouter,
  computando ratio por categoria com invariante semântico (não keyword estreita).
- [ ] 6.4 Gate: falhar quando qualquer categoria ficar < 0,90; integrar ao passo pré-deploy.

## Detalhes de Implementação

Ver `adr-005-golden-harness-gate.md` e `techspec.md` → "Testes E2E — harness real-LLM". Requer
`OPENROUTER_*` (não roda no CI por-PR). Usa `expected_tool` (5.0) e os demais scorers. Anonimização
obrigatória (privacidade — RF-37). Gate por categoria ≥ 0,90 (registro, C1–C7, cartão, orçamento/mês,
recorrência, onboarding, pendência, confirmação, follow-up, erro, ambiguidade, formato).

## Critérios de Sucesso

- Golden versionado sem PII/WAMID; casos declaram expectativas verificáveis.
- CI por-PR verde só com determinístico; real-LLM sob tag.
- Gate real-LLM ≥ 0,90 por categoria bloqueia pré-deploy.
- Fluidez preservada (respostas válidas não regridem sob a cadeia de guardas).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — constrói harness de evals/golden sobre o agente mecontrola (Structured Output, Thread→Run, scorers).

## Testes da Tarefa

- [ ] Testes unitários: schema/carregamento dos fixtures; cálculo de ratio por categoria; determinismo do
  modo CI.
- [ ] Testes de integração: harness real-LLM (`realllm`) sobre o golden com gate ≥ 0,90 por categoria.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/golden/*` (novo)
- `internal/agents/application/agents/mecontrola_agent.go` (SUT do harness)
- `internal/agents/application/scorers/*` (reuso no gate)
