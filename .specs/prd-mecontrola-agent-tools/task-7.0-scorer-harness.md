# Tarefa 7.0: Scorer de tool esperada + harness real-LLM + observabilidade

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar um scorer code-based de tool esperada por cenário, um harness E2E com LLM real cobrindo
todas as 24 tools e a observabilidade correspondente (Run auditável + métricas com cardinalidade
controlada), fechando M-03/M-04/RF-29 com gate anti-falso-positivo. Depende da 6.0 e é
paralelizável com a 8.0. Ver techspec.md, "Abordagem de Testes" (E2E) e ADR-002.

<requirements>
- RF-27, RF-28, RF-29, RF-30, RF-33, RF-34.
- Dependência: 6.0. Paralelizável com 8.0.
</requirements>

## Subtarefas

- [ ] 7.1 Implementar um scorer code-based de tool esperada por cenário em
  `internal/agents/application/scorers/mecontrola_scorers.go` (e, se preciso, expor
  `ExpectedTool`/`Args` em `internal/platform/scorer` `RunSample`/`ToolCallRecord`, mantendo tipos
  fechados) — ADR-002.
- [ ] 7.2 Atualizar a lista `mecontrolaFinancialTools` para as 24 tools e `BuildMeControlaScorers`.
- [ ] 7.3 Criar harness E2E `*_realllm_test.go` gated por `RUN_REAL_LLM=1` + `OPENROUTER_*` do `.env`,
  com conjunto canônico determinístico (1 tool esperada por cenário) cobrindo TODAS as 24 tools,
  medindo M-04 (acerto ≥ 0.90) e RF-29 (cada tool exercida ≥ 1 vez).
- [ ] 7.4 Garantir Run auditável por execução (RF-27) e labels de métrica com cardinalidade
  controlada (enums fechados; sem `user_id`/`category_id` — RF-28); gate anti-falso-positivo:
  aceite bloqueado se alguma tool registrada não for exercida ou M-03 < 100% (RF-30/RF-33/RF-34).

## Detalhes de Implementação

Ver techspec.md, seções "Abordagem de Testes" (E2E), "Monitoramento e Observabilidade" e o ADR-002
(`adr-002-expected-tool-scorer-and-realllm-harness.md`). O scorer eleva a barra do scorer coarse
atual (match de qualquer tool financeira) para tool esperada por cenário. Mocks não contam como
evidência (validação com LLM real obrigatória).

## Critérios de Sucesso

- O harness passa com LLM real; relatório lista 0 tools não exercidas; M-04 ≥ 0.90.
- Métricas com cardinalidade controlada (enums fechados; sem `user_id`/`category_id`).
- Cada execução é um Run auditável.
- Mocks não contam como evidência.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — registro de tools, instruções do agente, scorers e verificação da superfície seguem o molde internal/agents sobre internal/platform.

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

Harness real-LLM (`RUN_REAL_LLM=1`) + teste unitário do scorer. Integração via LLM real é
obrigatória.

## Arquivos Relevantes
- `internal/agents/application/scorers/mecontrola_scorers.go`
- `internal/platform/scorer/{scorer,types}.go` (se necessário)
- `internal/agents/.../*_realllm_test.go`
