# Tarefa 8.0: Fechar Matriz Production-Ready de Testes e Observabilidade

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Fechar a validacao production-ready da feature inteira, cobrindo testes unitarios, integracao, E2E deterministico, observabilidade e gates de Go. Esta tarefa deve provar cobertura integral de `RF-01` a `RF-35`, `RNF-01` a `RNF-05` e `CA-01` a `CA-23`.

<requirements>
RF-01, RF-02, RF-03, RF-04, RF-05, RF-06, RF-07, RF-08, RF-09, RF-10, RF-11, RF-12, RF-13, RF-14, RF-15, RF-16, RF-17, RF-18, RF-19, RF-20, RF-21, RF-22, RF-23, RF-24, RF-25, RF-26, RF-27, RF-28, RF-29, RF-30, RF-31, RF-32, RF-33, RF-34, RF-35.
RNF-01, RNF-02, RNF-03, RNF-04, RNF-05.
CA-01, CA-02, CA-03, CA-04, CA-05, CA-06, CA-07, CA-08, CA-09, CA-10, CA-11, CA-12, CA-13, CA-14, CA-15, CA-16, CA-17, CA-18, CA-19, CA-20, CA-21, CA-22, CA-23.
</requirements>

## Subtarefas

- [ ] 8.1 Criar matriz de testes que mapeia cada RF/RNF/CA para unit, integration ou E2E.
- [ ] 8.2 Rodar testes unitarios de `categories`, `transactions` e `agents`.
- [ ] 8.3 Rodar integration tests com Postgres para categorias, repositories, migrations, FKs e triggers.
- [ ] 8.4 Rodar E2E deterministico para despesa, receita, no match, multi-candidato, baixa evidencia, clarificacao, recurring template e manual canonical.
- [ ] 8.5 Validar que LLM/scorer/prompt nao aparece como autoridade de desbloqueio de escrita.
- [ ] 8.6 Implementar/validar logs, traces ou metricas de baixa cardinalidade definidos em `techspec.md`.
- [ ] 8.7 Rodar `go vet`, build/test proporcional e `golangci-lint run` no escopo quando disponivel.
- [ ] 8.8 Registrar evidencias de validacao no proprio task file ou em artefato local aprovado pelo padrao do projeto.

## Detalhes de Implementação

Seguir `techspec.md`, secoes "Abordagem de Testes", "Monitoramento e Observabilidade" e "Mapeamento RF -> Decisao -> Teste". Aplicar `mastra` nos cenarios agentivos e `go-implementation` em todo codigo Go. DMMF deve ser conferido explicitamente: estados fechados, smart constructors, workflow pipeline e erros discriminaveis.

## Critérios de Sucesso

- Nenhum RF, RNF ou CA fica sem teste ou justificativa objetiva de cobertura por teste superior.
- Banco rejeita bypass invalido mesmo quando o gate de aplicacao nao e executado.
- Agent nao persiste em no match, multi-candidato, baixa evidencia ou confirmacao ambigua.
- Manual canonical persiste evidencia deterministica completa.
- Observabilidade usa apenas labels permitidos e nao vaza IDs, termo buscado ou texto do usuario.
- Gates Go passam ou ausencia de ferramenta e registrada de forma objetiva.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — Valida fluxos agentivos, tools, workflow retomavel e garantia de que LLM/scorer nao autoriza escrita.

## Testes da Tarefa

- [ ] `go test -race -count=1 ./internal/categories/...`
- [ ] `go test -race -count=1 ./internal/transactions/...`
- [ ] `go test -race -count=1 ./internal/agents/...`
- [ ] `go test -tags=integration -count=1` nos pacotes de migrations, repositories e adapters quando habilitados.
- [ ] `go vet ./internal/categories/... ./internal/transactions/... ./internal/agents/...`
- [ ] `golangci-lint run ./internal/categories/... ./internal/transactions/... ./internal/agents/...` quando disponivel.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/categories`
- `internal/transactions`
- `internal/agents`
- `migrations/000001_initial_schema.up.sql`
- `migrations/000001_initial_schema.down.sql`
- `.specs/prd-contrato-categorias-transacoes-agentivas/tasks.md`
