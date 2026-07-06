# Tarefa 7.0: Corrigir Decisao Agentiva, Tool e Workflow Retomavel

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Corrigir os fluxos agentivos que hoje podem aceitar candidato unico ou primeiro candidato sem provar outcome canonico. `RegisterEntry`, `classify_category` e `destructive_confirm_workflow` devem aplicar o mesmo criterio funcional de aceite, pedir clarificacao quando necessario e nunca usar LLM/scorer como autoridade de escrita.

<requirements>
RF-02, RF-03, RF-06, RF-07, RF-08, RF-12, RF-13, RF-14, RF-15, RF-16, RF-25, RF-26, RF-27, RF-31, RF-32.
RNF-01, RNF-02, RNF-05.
CA-01, CA-02, CA-03, CA-04, CA-05, CA-09, CA-10, CA-12, CA-14, CA-15.
</requirements>

## Subtarefas

- [ ] 7.1 Atualizar `RegisterEntry.classify` para aceitar apenas `Outcome=matched`, `len=1`, `!IsAmbiguous`, `Version>0`, root diferente de leaf e evidencia completa.
- [ ] 7.2 Garantir que bloqueios retornem clarificacao sem chamar writer.
- [ ] 7.3 Atualizar `classify_category` como tool explicativa com outcome, version, candidates ricos, `writeDecision` e reason.
- [ ] 7.4 Remover qualquer sugestao implicita de primeiro candidato quando outcome nao for aceito.
- [ ] 7.5 Atualizar `destructive_confirm_workflow` para derivar kind da direcao do draft.
- [ ] 7.6 Revalidar escolha por clarificacao via gate/resolve completo com source `user_selected_candidate`.
- [ ] 7.7 Garantir que texto livre dispare nova resolucao canonica e nao destrave persistencia sozinho.

## Detalhes de Implementação

Seguir `techspec.md`, secoes "Contratos de agents", "Fluxo de Dados" e "Abordagem de Testes". Aplicar `mastra`: tool como adapter fino, workflow duravel para confirmacao retomavel, estados fechados, sem branching solto em prompt, sem recriar primitivos de `internal/platform/workflow`.

## Critérios de Sucesso

- `RegisterEntry` nao chama writer para no match, multi-candidato, candidato unico ambiguo, version ausente, raiz sem folha ou baixa evidencia.
- `classify_category` explica candidatos e bloqueios, mas nao autoriza persistencia.
- `destructive_confirm_workflow` nao usa primeiro candidato e nao fixa `kind="expense"` para income.
- Clarificacao valida passa por revalidacao canonica antes do write.
- Scorer/LLM nunca desbloqueia escrita.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — Altera tool financeira, workflow retomavel de confirmacao destrutiva e consumidor agentivo real em `internal/agents`.

## Testes da Tarefa

- [ ] `go test -race -count=1 ./internal/agents/application/usecases/...`
- [ ] `go test -race -count=1 ./internal/agents/application/tools/...`
- [ ] `go test -race -count=1 ./internal/agents/application/workflows/...`
- [ ] Unit tests table-driven para no match, ambiguous, unico com outcome nao matched, `Version=0`, root=leaf, expense/income e erro operacional.
- [ ] Workflow tests: retomada ambigua nao persiste, texto livre reclassifica, escolha valida revalida, version drift bloqueia e income nao usa expense.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/usecases/register_entry.go`
- `internal/agents/application/usecases/register_entry_test.go`
- `internal/agents/application/tools/classify_category.go`
- `internal/agents/application/tools/classify_category_test.go`
- `internal/agents/application/workflows/destructive_confirm_workflow.go`
- `internal/agents/application/workflows/destructive_confirm_workflow_test.go`
