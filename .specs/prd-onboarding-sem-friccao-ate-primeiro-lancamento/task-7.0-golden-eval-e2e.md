# Tarefa 7.0: Atualizar golden/eval e E2E de primeiro lançamento

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Incluir casos de golden/eval e testes E2E para onboarding sem "Oi", cadastro/recusa de cartão, despesa pix sem pergunta de cartão e receita com separador de milhar sem falso múltiplo lançamento.

<requirements>
- RF-20: receita simples com separador de milhar não vira múltiplos lançamentos.
- RF-21: termo literal da receita preservado como descrição.
- RF-22: receita simples inicia apenas confirmação mínima necessária.
- RF-23: após confirmação positiva, transação ativa de receita com valor e descrição corretos.
- RF-35 a RF-39: cobertura por golden/eval e E2E.
</requirements>

## Subtarefas

- [ ] 7.1 Adicionar caso golden/eval de onboarding inicial sem "Oi".
- [ ] 7.2 Adicionar caso golden/eval de 💳 válido com banco/apelido único.
- [ ] 7.3 Adicionar caso golden/eval de recusa de 💳.
- [ ] 7.4 Adicionar caso golden/eval de despesa pix sem pergunta de 💳.
- [ ] 7.5 Adicionar caso golden/eval de receita com separador de milhar sem falso multi-lançamento.
- [ ] 7.6 Garantir que scorer de `verbatim_required` continue passando para confirmações retornadas por tools.
- [ ] 7.7 Atualizar testes E2E existentes com os novos cenários.

## Detalhes de Implementação

Ver `techspec.md` — seção **Testes E2E, Golden e Pós-Deploy**. Os casos devem usar o mecanismo existente em `internal/agents/application/golden/*` e `internal/agents/application/postdeploy/*`.

## Critérios de Sucesso

- Golden/eval passam para todos os novos casos.
- Scorer `verbatim_required` continua passando para confirmações verbatim.
- E2E cobre jornada completa de pix e receita.
- `task test:golden:gate` passa quando disponível.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — golden/eval e E2E no consumidor agentivo.

## Testes da Tarefa

- [ ] Execução do suite golden/eval.
- [ ] Execução dos testes E2E.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/golden/*`
- `internal/agents/application/postdeploy/*`
- `internal/agents/application/agents/mecontrola_agent_gherkin_e2e_test.go`
