# Tarefa 7.0: Teste de regressão exaustivo dos intent.Kind

<critical>Ler o plano-fonte `docs/plans/2026_06_24_arquitetura_agente_mastra_workflows_bounded_contexts.md` (Item 6, "Refatorações") e o Passo 9 do exemplo antes de iniciar.</critical>

## Visão Geral

Hoje há 3 testes parciais e desconexos (`TestKindString`, `TestParseKind`, `TestNewKinds_StringAndParseRoundTrip`), nenhum cruzando o enum de `ParseIntentJSONSchema` com os kinds nem garantindo smart constructor. Um kind novo esquecido no schema (ou vice-versa) não quebra teste algum. Criar um teste mecânico exaustivo que blinda a paridade schema↔kind↔constructor e o round-trip `String()`↔`ParseKind()`.

<requirements>
- Um único teste exaustivo (package `intent` whitebox, padrão testify/suite — R-TESTING-001) que itera sobre TODOS os kinds derivados de fonte única.
- Asserções por kind: (a) round-trip `ParseKind(k.String())==k`; (b) `k.String() != "unknown"` para `k != KindUnknown`; (c) paridade bidirecional com o enum de `ParseIntentJSONSchema`; (d) smart constructor existe e produz o kind.
- Fonte única de kinds à prova de esquecimento (range `iota` de `KindUnknown` ao último) + teste que valida exaustividade do mapa de builders.
- Os 3 testes parciais consolidados ou mantidos sem redundância (decisão registrada).
</requirements>

## Subtarefas

- [ ] 7.1 Criar `internal/agent/domain/intent/intent_registry_test.go` (package `intent`, suite testify) com `allKinds()` derivado do range `iota` e `kindBuilders()` (map kind→builder).
- [ ] 7.2 Implementar os 5 métodos: round-trip, slug não-default, paridade bidirecional schema↔kind, todo kind tem builder, mapa de builders exaustivo.
- [ ] 7.3 Helper de extração do enum de `ParseIntentJSONSchema` que falha com mensagem clara (sem panic de type-assertion).
- [ ] 7.4 Consolidar/limpar sobreposição com `TestKindString`/`TestParseKind`/`TestNewKinds_*`.

## Detalhes de Implementação

Ver plano-fonte (Item 6). Âncoras: `internal/agent/domain/intent/intent.go` (consts `iota` :11-33), `intent_test.go`, `intent_new_kinds_test.go`, `internal/agent/application/prompting/prompts.go` (`ParseIntentJSONSchema` :155, enum :182). A asserção bidirecional (c) é a que fecha o gap — a unidirecional não pega slug órfão no schema. Sem dependência de outras tasks (toca só domínio/intent + leitura de prompting).

## Critérios de Sucesso

- O teste quebra se: um kind novo não estiver no schema; um slug do schema não tiver kind; um kind cair no `default:"unknown"`; um kind não tiver builder.
- `go test ./internal/agent/domain/intent/...` verde com a base atual (21 kinds).

## Definition of Done (DoD)

1. Teste exaustivo único presente, cobrindo (a)–(d) + exaustividade do mapa de builders.
2. `allKinds()` deriva de fonte única (range `iota`), não de lista manual divergível.
3. Paridade bidirecional schema↔kind implementada.
4. `go test ./internal/agent/domain/intent/...` e `go build ./...` verdes.

## Critérios de Aceite (gates executáveis)

```bash
cd /Users/jailtonjunior/Git/mecontrola

test -f internal/agent/domain/intent/intent_registry_test.go && echo "OK: arquivo presente" || echo "FAIL"

go test ./internal/agent/domain/intent/... -v -run 'KindRegistry|Kind'

# Prova de eficácia (manual/local): remover temporariamente um slug do enum em prompts.go
# deve fazer o teste de paridade FALHAR. Reverter após confirmar.

go build ./...
```

## Skills Necessárias

<!-- Tarefa exclusivamente de teste em Go de produção; sem skill processual extra além das auto-carregadas. go-implementation declarada por exigência do projeto (CLAUDE.md) e padrão R-TESTING-001. -->

- `go-implementation` — escrita de teste Go no padrão canônico testify/suite (R-TESTING-001) e validação da stack.

## Testes da Tarefa

- [ ] Testes unitários: o próprio teste exaustivo é o entregável; provar localmente que ele falha ao injetar divergência schema↔kind.
- [ ] Testes de integração: N/A (escopo de domínio puro).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`.</critical>

## Arquivos Relevantes
- `internal/agent/domain/intent/intent.go`
- `internal/agent/domain/intent/intent_registry_test.go` (novo)
- `internal/agent/domain/intent/intent_test.go`, `intent_new_kinds_test.go` (consolidar)
- `internal/agent/application/prompting/prompts.go` (leitura do enum)
