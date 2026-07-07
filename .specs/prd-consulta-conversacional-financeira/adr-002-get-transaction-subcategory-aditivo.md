# Registro de Decisão Arquitetural (ADR-002)

## Metadados

- **Título:** Extensão aditiva única de `get_transaction` — expor `subcategoryNameSnapshot`
- **Data:** 2026-07-07
- **Status:** Aceita
- **Decisores:** Autor da techspec, dono do módulo `internal/agents`
- **Relacionados:** PRD spec-version 3 (RF-06, RF-35, D-03, D-09), techspec desta pasta, `.claude/rules/go-adapters.md` (R-ADAPTER-001.2)

## Contexto

O exemplo C5 do PRD promete "Categoria: *Custo Fixo > Supermercado*" (categoria **>** subcategoria).
O domínio `interfaces.Entry` já carrega `CategoryNameSnapshot` **e** `SubcategoryNameSnapshot`
(`interfaces/types.go:181-198`), mas a projeção da tool `get_transaction` mapeia apenas o primeiro,
descartando a subcategoria. D-03 (RF-35) proíbe alterar tools; logo, sem uma exceção, C5 não pode ser
fiel ao próprio exemplo do PRD.

## Decisão

Abrir uma **exceção aditiva única** a D-03: adicionar o campo `SubcategoryNameSnapshot` a
`GetTransactionOutput`, incluí-lo no schema JSON estrito (`properties` + `required`) e mapear
`entry.SubcategoryNameSnapshot` no `exec`. Mudança **puramente aditiva**: nenhum campo removido,
nenhuma lógica de domínio, branching ou SQL; o adapter permanece fino (R-ADAPTER-001.2). Nenhuma outra
tool, assinatura de use case, binding ou `module.go` é tocada.

## Alternativas Consideradas

- **Honrar D-03 estrito**: C5 exibe só a categoria raiz. Rejeitada pelo usuário — deixa a entrega
  menos rica que o US e cria gap UX↔PRD.
- **Nova tool `get_transaction_detail`**: viola D-03 (tool nova) e duplica `get_transaction`.
- **Derivar subcategoria via `classify_category`/`list_categories`**: `classify_category` vai de
  termo→categoria (não id→nome) e adicionaria chamadas; não resolve o caso e piora latência.

## Consequências

### Benefícios Esperados

- C5 fiel ao PRD sem introduzir lógica; dado já existe no domínio (só exposição).
- Superfície mínima: 3 pontos no mesmo arquivo + 1 assert de teste.

### Trade-offs e Custos

- Amplia D-03 em exatamente um campo; documentado como exceção fechada (D-09) para não virar
  precedente de mudanças de tool "por conveniência".

### Riscos e Mitigações

- **Risco:** schema `Strict:true` exige o novo campo em `required`; um retorno sem o campo quebraria a
  decodificação. **Mitigação:** o `exec` sempre preenche o campo (string vazia quando não houver
  subcategoria); teste cobre com e sem subcategoria.

## Plano de Implementação

1. Adicionar campo à struct de saída.
2. Adicionar propriedade ao schema e a `required`.
3. Mapear no `exec`.
4. Estender `TestGetTransactionTool_Success` para asseverar o campo (com e sem valor).

## Monitoramento e Validação

- `go test -race ./internal/agents/application/tools/...` verde.
- C5 no harness real-LLM renderiza `Categoria > Subcategoria` quando há subcategoria.

## Impacto em Documentação e Operação

- Nenhum consumidor externo da tool além do agente; sem contrato público afetado.

## Revisão Futura

- Se novas consultas exigirem mais campos de `Entry`, avaliar expor um subconjunto coeso de uma vez,
  em vez de exceções pontuais sucessivas.
