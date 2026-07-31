# Execução Completa — PRD Lançamento e Edição de Receitas em Linguagem Natural

- Data: 31-07-2026
- Fonte única: `.specs/prd-lancamento-edicao-receitas`
- Skill: `execute-all-tasks`
- Status final: **done** — 5/5 tarefas concluídas, 0 desvios, 0 lacunas, 0 pendências

## Snapshot

| # | Tarefa | Status inicial | Status final | RFs cobertos |
|---|--------|-----------------|--------------|---------------|
| 1.0 | Datas determinísticas: estender ParseInputDate + helpers puros | done (execução anterior) | done | RF-05, RF-06, RF-07, RF-22 |
| 2.0 | Contrato de data verbatim nos schemas de register_income e edit_entry | done (execução anterior) | done | RF-16 |
| 3.0 | Confirmação de receita com linha Data condicional | done (execução anterior) | done | RF-10, RF-26 |
| 4.0 | Golden real-LLM de lançamento (grupos + valor + data) + gate | done (execução anterior) | done | RF-01, RF-02, RF-03, RF-04, RF-08, RF-09, RF-11, RF-12, RF-13, RF-14, RF-15, RF-25, RF-27 |
| 5.0 | Golden real-LLM de edição de receita + gate | **pending** | **done (esta execução)** | RF-17, RF-18, RF-19, RF-20, RF-21, RF-23, RF-24 |

Cobertura de RFs: RF-01 a RF-27 — 100% (27/27), confirmada por `ai-spec check-spec-drift` no pré-voo e pela tabela "Cobertura de Requisitos" de `tasks.md`.

## O que foi executado nesta rodada

Apenas a tarefa 5.0 estava `pending`; as demais (1.0–4.0) já haviam sido concluídas e aprovadas em execuções anteriores (relatórios `1.0_execution_report.md`..`4.0_execution_report.md` já presentes). O orquestrador:

1. Rodou o pré-voo (`pre-execute-all-tasks.sh`) — OK, 5 tarefas validadas, sem drift de skills nem de spec.
2. Construiu o grafo de dependências a partir de `tasks.md` — única tarefa `ready`: 5.0 (dependências 1.0/2.0/3.0 já `done`).
3. Disparou subagent fresh (`task-executor` → skill `execute-task`) para 5.0.
4. Validou o retorno YAML (contrato estrito, evidência física do relatório, consistência de `tasks.md`).
5. Gerou `_orchestration_report.md` e este relatório de execução.

### Tarefa 5.0 — Golden real-LLM de edição de receita + gate

Criado `internal/agents/application/golden/cases_income_edit.go` com 9 casos golden cobrindo:
- Localização por termo/valor + correção (`na verdade foi`) — RF-17/RF-18/RF-21.
- Distinção explícita entre valor novo (`amountCents`) e critério de busca (`searchAmountCents`) — RF-21.
- Mudança de data reaproveitando a resolução determinística da tarefa 1.0 (`semana passada`, `dia N`) — RF-22, contrato verbatim mantido.
- Desambiguação com múltiplos candidatos e candidato único com nota de impacto antes da confirmação — RF-19, RF-20.
- Casos de erro: nenhum candidato compatível (sem alteração) e correção inválida (não persiste) — RF-24.

O mock `edit_entry` do harness real-LLM (`harness_realllm_test.go`) foi reescrito para refletir fielmente o schema de produção (`edit_entry.go:43-51`): `amountCents`/`searchAmountCents` tipados `integer`, distintos; contrato verbatim de `occurredAt` idêntico ao da tool real; 4 variantes de catálogo (`edit_entry`, `edit_entry_multiple_candidates`, `edit_entry_not_found`, `edit_entry_invalid_amount`) espelhando `NeedsConfirmation`/`ImpactNote` verbatim conforme `extractEditEntryVerbatim`.

Registrado em `registry.go` (`incomeEditCases` incorporado a `AllCases`).

## Evidência de Validação (reexecutada nesta rodada, escopo completo)

```
go build ./...                                                   -> OK
go vet ./internal/agents/...                                     -> OK
go test ./internal/agents/... -race -count=1                     -> ok em todos os pacotes,
    exceto 1 falha pré-existente fora de escopo (ver seção "Achado fora de escopo" abaixo)
```

Evidência específica da tarefa 5.0 (herdada do relatório do subagent, `5.0_execution_report.md`):

```
RUN_REAL_LLM=1 go test ./internal/agents/application/golden/ -tags integration \
  -run 'TestGoldenRealLLMSuite/TestGoldenSetGate/receita_edicao' -count=1 -v
  -> categoria=expense_income hits=27 total=27 ratio=1.0000 (9 casos novos × 3 repetições, 0 falso-sucesso)

RUN_REAL_LLM=1 go test ./internal/agents/application/golden/ -tags integration \
  -run 'TestGoldenRealLLMSuite/TestGoldenSetGate/(edicao|correcao)' -count=1 -v
  -> categoria=expense_income hits=39 total=39 ratio=1.0000 (9 casos novos + 4 pré-existentes, sem regressão)

go test ./internal/agents/application/golden/ -tags integration \
  -run TestGoldenToolSubsetsAreRegisteredInHarnessCatalog -count=1 -v -> ok (0 tools órfãs)

gofmt -l internal/agents/application/golden/ -> vazio
```

Gate ≥ 0,90 e 0 falso-sucesso: **atendido** (ratio 1.0000 em ambos os runs).

## Achado fora de escopo (não bloqueia o PRD)

`go test ./internal/agents/... -race` reportou 1 falha em `internal/agents/application/tools`:
`TestBuildQueryMonthToolPreviousResolvesToPriorMonth`. Investigado e confirmado:

- Não pertence ao escopo deste PRD (ferramenta `query_month`, consulta de mês — não é lançamento/edição de receita).
- Não foi tocado pelo diff de nenhuma das 5 tarefas (`git diff --stat` confirma zero alteração no arquivo de produção `query_month.go`; a única mudança em `financial_tools_test.go` desta rodada foi a adição dos 2 testes de contrato verbatim, sem relação com este teste).
- Reproduzido também em `git stash` (árvore de trabalho limpa) com o mesmo resultado — **falha pré-existente, não introduzida por esta execução**.
- Causa raiz: o teste calcula o mês esperado com `now.AddDate(0, -1, 0).Format("2006-01")`; em `time.AddDate` do Go, subtrair 1 mês do dia 31 produz uma data inválida (`31 de junho`) que é normalizada rolando para `1º de julho` — ou seja, o teste calcula erroneamente "mês anterior" como o mês corrente quando executado no dia 31. A implementação de produção (`DecideCompetence`/`Competence.Prev()`) não sofre desse defeito, pois opera sobre ano/mês puros, sem aritmética de dia-do-calendário. Isso só se manifesta porque a execução ocorreu no dia 31/07/2026.
- Não relacionado a nenhum RF deste PRD (RF-01..RF-27, todos escopados a `register_income`/`edit_entry`).

Registrado aqui para transparência e rastreabilidade; recomenda-se abertura de tarefa própria fora deste PRD para corrigir o helper de teste de `query_month`.

## Arquivos Alterados (acumulado das 5 tarefas, working tree)

- `internal/agents/application/workflows/parse_input_date_test.go` (novo)
- `internal/agents/application/workflows/write_shared.go`
- `internal/agents/application/workflows/transaction_write_workflow.go` / `_test.go`
- `internal/agents/application/tools/register_income.go`
- `internal/agents/application/tools/edit_entry.go`
- `internal/agents/application/tools/financial_tools_test.go`
- `internal/agents/application/messages/catalog.go` / `_test.go`
- `internal/agents/application/golden/cases_income_natural_language.go` (novo, tarefa 4.0)
- `internal/agents/application/golden/cases_income_edit.go` (novo, tarefa 5.0)
- `internal/agents/application/golden/registry.go`
- `internal/agents/application/golden/harness_realllm_test.go`

## Critérios de Aceite do PRD — Checklist Final

- [x] 100% de conformidade com o PRD (RF-01..RF-27 cobertos e comprovados por tarefa).
- [x] 0 desvios — nenhum requisito flexibilizado, reinterpretado ou simplificado.
- [x] 0 lacunas — todas as 5 tarefas `done` com evidência física.
- [x] 0 falso positivo — gates golden real-LLM com ratio 1.0000 e 0 falso-sucesso reportado nos relatórios de 4.0 e 5.0.
- [x] 0 pendências — `tasks.md` sem itens `pending`/`in_progress`/`blocked`/`needs_input`.
- [x] 0 ressalvas de escopo — fora de escopo (exclusão de receita, forma de pagamento, receita recorrente, múltiplos lançamentos) não foi implementado, conforme PRD.

## Próximos Passos

- Nenhum dentro do escopo deste PRD — implementação completa.
- Alterações permanecem não commitadas na working tree (política do projeto: commit somente sob pedido explícito do usuário).
- Sugestão (fora de escopo): abrir tarefa dedicada para corrigir `TestBuildQueryMonthToolPreviousResolvesToPriorMonth` (bug de `AddDate` em dia 31, achado incidental nesta execução).
