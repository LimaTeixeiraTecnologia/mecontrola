# Tarefa 5.0: Remover RenderSystemPrompt morto e consolidar DefaultRegistry na registry canônica

<critical>Ler o plano-fonte `docs/plans/2026_06_24_arquitetura_agente_mastra_workflows_bounded_contexts.md` (Item 1) antes de iniciar.</critical>

## Visão Geral

`RenderSystemPrompt()` é código morto (só referenciado em testes). `DefaultRegistry()` lista 9 tools, enquanto `routableKinds()`/`buildRegistry()` cobrem 21 kinds, e `warnMissingToolBindings` só rastreia 6 — três listas desalinhadas. Eliminar o código morto e a duplicação, fazendo a registry canônica (`buildRegistry`/`routableKinds`) ser a fonte única (R-AGENT-WF-001.1).

<requirements>
- `RenderSystemPrompt` removido de produção e de teste; constantes `toolSystemHeader`/`toolSystemFooter` removidas se ficarem órfãs.
- `warnMissingToolBindings` deixa de depender de lista hardcoded paralela: introspecta a registry real (opção (a) recomendada — eliminar `DefaultRegistry`) ou mantém `DefaultRegistry` derivado de `routableKinds()` com cobertura total (sem `tracked==false` silencioso).
- Decisão (a)/(b) registrada no plano-fonte ou no relatório da task.
- Zero comentários em Go de produção (R-ADAPTER-001.1).
</requirements>

## Subtarefas

- [ ] 5.1 Remover `RenderSystemPrompt` (`registry.go:89`) e `TestRenderSystemPrompt` (`registry_test.go`); remover `toolSystemHeader`/`toolSystemFooter` se órfãos.
- [ ] 5.2 Implementar a opção (a): eliminar `DefaultRegistry` e reescrever `warnMissingToolBindings` (`intent_router.go:222`) para iterar a registry canônica.
- [ ] 5.3 Atualizar/realocar `TestDefaultRegistry` (`registry_test.go:114`) para derivar o esperado de `routableKinds()` em vez do literal `9` — ou removê-lo se `DefaultRegistry` sair.
- [ ] 5.4 Adicionar teste de paridade que falha quando o conjunto de specs ≠ conjunto de kinds roteáveis (anti-reincidência da divergência).

## Detalhes de Implementação

Ver plano-fonte (Item 1). Âncoras: `internal/agent/application/tools/registry.go` (`RenderSystemPrompt` :89, `DefaultRegistry` :106, consts :20-28), `internal/agent/application/services/intent_router.go` (`warnMissingToolBindings` :222, mapa `bindings` :230-237), `agent_workflows.go` (`routableKinds`/`buildRegistry`). Pré-condição: Task 4.0 (estabilizou `buildRegistry`/registro de tools).

## Critérios de Sucesso

- Nenhuma referência a `RenderSystemPrompt`.
- `warnMissingToolBindings` não usa lista paralela parcial; nenhum spec cai em `tracked==false`.
- Teste de paridade garante que a divergência não reincide.
- Build + suites do agent verdes.

## Definition of Done (DoD)

1. `RenderSystemPrompt` (e consts órfãs) removidos de produção e teste.
2. Fonte única de roteamento por kind = registry canônica; `DefaultRegistry` eliminado (opção a) ou alinhado e testado (opção b).
3. Teste de paridade specs↔kinds presente e verde.
4. Build + suites verdes.

## Critérios de Aceite (gates executáveis)

```bash
cd /Users/jailtonjunior/Git/mecontrola

grep -rn "RenderSystemPrompt" internal/ --include="*.go" && echo "FAIL: RenderSystemPrompt ainda existe" || echo "OK"

# opção (a): DefaultRegistry eliminado
grep -rn "DefaultRegistry" internal/ --include="*.go" && echo "REVISAR: DefaultRegistry remanescente (ok só se opção b com teste de paridade)" || echo "OK (opção a)"

grep -rn "toolSystemHeader\|toolSystemFooter" internal/agent/application/tools/ --include="*.go" \
  && echo "REVISAR: const órfã" || echo "OK"

grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" "^[[:space:]]*//" \
  internal/agent/application/tools/ | grep -Ev "(//go:|//nolint:|// Code generated)" \
  && echo "FAIL: comentários proibidos" || echo "OK"

go build ./... && go test ./internal/agent/...
```

## Skills Necessárias

- `go-implementation` — remoção de código morto e refatoração de Go de produção (Etapas 1–5 + checklist R0–R7).
- `mastra` — registry de tools/kinds e roteamento canônico (R-AGENT-WF-001.1).

## Testes da Tarefa

- [ ] Testes unitários: `warnMissingToolBindings` cobre todos os bindings esperados; teste de paridade specs↔kinds.
- [ ] Testes de integração: roteamento dos 21 kinds permanece resolvendo (não regride).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`.</critical>

## Arquivos Relevantes
- `internal/agent/application/tools/registry.go`, `registry_test.go`
- `internal/agent/application/services/intent_router.go`
- `internal/agent/application/services/agent_workflows.go` (`routableKinds`/`buildRegistry`)
