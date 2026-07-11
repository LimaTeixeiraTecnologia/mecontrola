# Review PRD — Onboarding sem Fricção até Primeiro Lançamento Financeiro

## Veredito

`APPROVED`

## Escopo Revisado

Revisão executada contra `.specs/prd-onboarding-sem-friccao-ate-primeiro-lancamento/` e contra o working tree atual em `internal/agents`, `internal/platform` e artefatos operacionais relacionados.

O ciclo não foi aprovado no primeiro passe. A aprovação final só foi registrada depois de remediar os achados, reexecutar golden real com LLM e passar os gates locais obrigatórios.

## Achados Remediados

| id | severidade | causa | remediação | status |
|---|---:|---|---|---|
| BUG-001 | major | RF-07 ainda permitia copy funcional com `cartão/cartões` em prompts, tools e respostas de fallback | copy de produção passou a usar `💳`; golden e testes passaram a exigir `💳`; `card_provenance` passou a tratar `resolve_card found=false` como clarificação determinística | fixed |
| BUG-002 | major | `rtk task lint:deadcode` falhava por wrapper morto `BuildPendingEntryWorkflow` | wrapper removido; chamadas de teste migradas para `BuildPendingEntryWorkflowWithObservability(..., nil)` | fixed |
| BUG-003 | major | Golden real reprovava receita com separador de milhar porque o LLM alterava `description` de `salário` para `Salário` | adicionado shortcut determinístico de `register_income` para receita com valor e descrição extraíveis, preservando o termo literal | fixed |
| BUG-004 | major | Golden real reprovava cadastro de `💳` com banco/apelido único e onboarding inicial sem `Oi` | adicionados shortcuts determinísticos para criação explícita de `💳` e primeira mensagem de onboarding | fixed |

## Matriz de Conformidade

| área | status | evidência |
|---|---|---|
| RF-01..RF-06 onboarding inicial/orçamento | atendido | workflow de onboarding, pre-guard inicial e testes/golden de onboarding passaram |
| RF-07..RF-15 linguagem e fluxo de `💳` | atendido | copy de produção usa `💳`; `card_provenance` cobre ausência e `found=false`; golden `card` passou 18/18 |
| RF-16..RF-19 pix/débito não exigem `💳` | atendido | `card_provenance` só bloqueia `credit_card`; golden e integração de pending-entry passaram |
| RF-20..RF-23 receita BRL com separador de milhar | atendido | golden real `expense_income` passou 27/27; shortcut preserva `description=salário` |
| RF-24..RF-29 confirmação, idempotência e retomada | atendido | guards de relay/falso sucesso, workflows e integrações de agents passaram |
| RF-30..RF-34 arquitetura/observabilidade | atendido | sem novo canal externo; `internal/platform` preservado; build/vet/race/lint passaram |
| RF-35..RF-39 testes/golden/E2E | atendido | golden determinístico, golden real, race suite e `agents:integration` passaram |
| DoD TechSpec | atendido | `rtk task ci:pipeline` passou com código 0 |

## Resultado Golden Real

`RUN_REAL_LLM=1 task test:golden:gate` passou usando o provider real configurado localmente.

Categorias finais:

| categoria | resultado |
|---|---:|
| `expense_income` | 27/27 |
| `query` | 21/21 |
| `card` | 18/18 |
| `budget` | 18/18 |
| `recurrence` | 9/9 |
| `onboarding` | 6/6 |
| `pending` | 6/6 |
| `confirmation` | 3/3 |
| `follow_up` | 3/3 |
| `ambiguity` | 3/3 |
| `whatsapp_format` | 3/3 |
| `no_internal_terms` | 3/3 |
| `tool_error` | 1/1 |

## Validações Executadas

| comando | resultado |
|---|---|
| `rtk go test -race -count=1 ./internal/agents/application/agents/guards ./internal/agents/application/agents ./internal/agents/application/tools ./internal/agents/application/golden` | passou; 350 testes |
| `rtk sh -c '... RUN_REAL_LLM=1 task test:golden:gate'` | passou; golden real 100% por categoria |
| `rtk go build ./internal/platform/... ./internal/agents/...` | passou |
| `rtk go vet ./internal/platform/... ./internal/agents/...` | passou |
| `rtk golangci-lint run ./internal/agents/...` | passou; sem issues |
| `rtk task agents:integration` | passou |
| `rtk go test -race -count=1 ./internal/platform/... ./internal/agents/...` | passou; 1934 testes em 74 packages |
| `rtk task lint:deadcode` | passou |
| `rtk task ci:pipeline` | passou; código 0 |

## Arquivos de Evidência

- `internal/agents/application/agents/guards/card_provenance.go`
- `internal/agents/application/agents/guards/create_card_shortcut.go`
- `internal/agents/application/agents/guards/onboarding_initial.go`
- `internal/agents/application/agents/guards/register_income_shortcut.go`
- `internal/agents/application/agents/mecontrola_agent.go`
- `internal/agents/application/golden/cases_card.go`
- `internal/agents/application/golden/cases_expense_income.go`
- `internal/agents/application/golden/cases_onboarding.go`
- `internal/agents/application/workflows/onboarding_workflow.go`
- `internal/agents/application/workflows/pending_entry_workflow.go`

## Bugs Canônicos para Bugfix

```json
[]
```

## Conclusão

Não restam achados acionáveis, bugs canônicos pendentes, lacunas de RF, lacunas de DoD ou falhas de validação local para a PRD alvo.
