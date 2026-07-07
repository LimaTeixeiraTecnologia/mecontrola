# Tarefa 8.0: Validação — testes unit + harness + real-LLM (M-04 ≥ 0,90)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Fatia de validação end-to-end. Consolidar a cobertura de testes das mudanças e provar o comportamento com LLM real, cumprindo as métricas do PRD (M-01..M-05). Cobre a verificação de todos os RFs de comportamento.

<requirements>
- Verificar RF-01..RF-23 no comportamento integrado (registro, datas, categoria, cartão, confirmação, idempotência, valor, multi-item).
- M-01 completude de campos; M-02 zero duplicidade (write-ledger); M-03 zero alucinação; M-04 ≥ 0,90; M-05 confirmação universal.
- Validação real-LLM obrigatória (`feedback_realllm_validation_required`): `RUN_REAL_LLM=1` + `OPENROUTER_*` do `.env`.
</requirements>

## Subtarefas

- [ ] 8.1 Harness: cenário de **idempotência durável de write-ledger** (mesma `(wamid_original, itemSeq, operation)` 2× → 1 INSERT, 2ª replay), distinto do replay de mensagem existente (`TestG7_09`).
- [ ] 8.2 Harness: cenário de **data por dia da semana** (estado com `OccurredAt` resolvido de "terça") e de **teto de valor** (exec rejeita acima do teto sem `engine.Start`).
- [ ] 8.3 Real-LLM (`//go:build integration`): R1–R7 + data por dia da semana + rejeição de "semana/mês passado" + ambiguidade de categoria e de cartão + confirmação/cancelamento + valor inválido + fronteira multi-item (RF-16).
- [ ] 8.4 Rodar toda a suíte de validação do PRD (build/vet/race/lint) e a real-LLM com `.env`; registrar evidência (M-04 ≥ 0,90; M-03/M-05 = 0 violações).

## Detalhes de Implementação

Ver `techspec.md` › **Abordagem de Testes** (unit/harness/E2E real-LLM) e `prd.md` › **Critérios de Validação** e **Métricas de sucesso**. Reaproveitar `pendingEntryHarness`, `hNewExpenseState`, os scorers `BuildMeControlaScorers` e os arquivos `*_realllm_test.go`.

## Critérios de Sucesso

- Cenário de write-ledger prova 1 INSERT para 2 execuções da mesma chave.
- Real-LLM verde com M-04 ≥ 0,90; nenhuma alucinação de campo (M-03) e nenhuma escrita sem confirmação (M-05).
- `go test -race` das suítes de tools/usecases/workflows verdes; `golangci-lint run` limpo (v2).
- Testes seguem R-TESTING-001 (testify/suite whitebox, `fake.NewProvider()`) onde aplicável.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — validação do runtime agentivo com scorers/evals e testes real-LLM do stack Mastra Go.

## Testes da Tarefa

- [ ] Testes unitários (harness: write-ledger replay, dia da semana, teto)
- [ ] Testes de integração (real-LLM `RUN_REAL_LLM=1`, R1–R7 + bordas; scorers M-04)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/agents/pending_entry_harness_test.go` — cenários de harness.
- `internal/agents/application/agents/pending_entry_realllm_test.go`, `mecontrola_agent_realllm_test.go` — real-LLM.
- `internal/agents/application/scorers/mecontrola_scorers.go` — scorers (M-04).
