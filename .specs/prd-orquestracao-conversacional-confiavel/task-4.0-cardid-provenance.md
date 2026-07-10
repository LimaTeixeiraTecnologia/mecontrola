# Tarefa 4.0: Proveniência determinística de cardId

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Garantir, por código determinístico, que todo `cardId` usado em escrita/consulta de fatura veio de
`resolve_card`/`list_cards` — nunca fabricado — via duas camadas: validação de existência na tool e um
`PostGuard` de cadeia.

<requirements>
- RF-16: para compra/consulta de fatura, `resolve_card` antes de
  `register_expense`/`create_recurrence`/`query_card_invoice`; usar só `cardId` de
  `resolve_card`/`list_cards`.
- RF-17: garantia determinística de proveniência do `cardId` por guarda/validação (não só prompt).
- RF-18: `resolve_card` com `found=false` → pedir escolha (não criar cartão automaticamente).
</requirements>

## Subtarefas

- [x] 4.1 Tools consumidoras de `cardId` (`register_expense`, `create_recurrence`, `query_card_invoice`):
  validar que o `cardId` resolve para um cartão real do `resourceId` (usecase de leitura); inexistente →
  erro de domínio limpo → `clarify`/fallback pedindo escolha.
- [x] 4.2 `guards/card_provenance.go` (`PostGuard`): se uma tool consumidora de cartão aparece em
  `Result.ToolCalls` sem `resolve_card`/`list_cards` antes na sequência → override para pedir escolha.
- [x] 4.3 Registrar o handler `card_provenance` na cadeia (3.0).

## Detalhes de Implementação

Ver `adr-003-cardid-provenance.md`. `ToolCallRecord{Tool, Outcome, Content}` (`ports.go:50`) dá nomes e
ordem das tools — suficiente para o guard sem depender de args. Tools leem identidade via
`agent.InboundIdentityFromContext(ctx)` (`identity_context.go:33`) e resolvem `resourceId → userID`. A
resolução/validação é do usecase; o guard só inspeciona a sequência (R-AGENT-WF-001.2).

## Critérios de Sucesso

- `cardId` fabricado/inexistente → clarify pedindo escolha, nunca lançamento na fatura errada.
- Tool consumidora sem `resolve_card`/`list_cards` prévio → pede escolha (guard).
- Caminho feliz (resolve → registro) inalterado.
- `go build/vet/test -race` verdes; zero comentários.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — altera tools financeiras e adiciona handler de cadeia no stack agentivo mecontrola.

## Testes da Tarefa

- [x] Testes unitários: `cardId` inexistente → clarify; consumidora sem resolve prévio → pede escolha;
  caminho feliz; `resolve_card found=false` → escolha.
- [x] Testes de integração: não aplicável (E2E de cartão no golden — 6.0).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/tools/register_expense.go`
- `internal/agents/application/tools/create_recurrence.go`
- `internal/agents/application/tools/query_card_invoice.go`
- `internal/agents/application/agents/guards/card_provenance.go` (novo)
