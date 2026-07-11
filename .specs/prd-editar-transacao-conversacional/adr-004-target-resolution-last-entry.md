# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Resolução determinística do alvo da edição via read tool `get_last_entry`/`list_recent_entries`
- **Data:** 2026-07-10
- **Status:** Aceita
- **Decisores:** Autor da techspec, aprovação do solicitante (múltipla escolha, recomendação aceita)
- **Relacionados:** PRD (D-02, RF-01..04), techspec.md, `.claude/rules/go-adapters.md`

## Contexto

A edição precisa identificar QUAL transação editar. O usuário usa linguagem natural: "edita o último lançamento" ou "edita o gasto de 30 no mercado". O ledger expõe `GetTransaction(id)`, `SearchTransactions(query, refMonth, limit)` e `ListMonthlyEntries(refMonth, cursor, limit)` (`interfaces/transactions_ledger.go`), mas não há uma capacidade de "pegar o último lançamento". Delegar a resolução de "último" ao LLM (listar e escolher) é frágil e não determinístico — risco de editar o alvo errado, ferindo robustez.

## Decisão

1. Adicionar uma read tool fina `get_last_entry` (ou `list_recent_entries`) que retorna o(s) lançamento(s) mais recente(s) do usuário com `id`, `version`, descrição, valor, categoria e data. O `exec` delega a uma nova porta de leitura no `transactionsLedgerAdapter` (`ListRecentEntries(ctx, limit)`), que chama um usecase/repo de leitura no ledger — adapter fino, sem regra.
2. Para "edita o gasto de 30 no mercado", reusar `search_transactions`; quando houver mais de uma correspondência, o agente apresenta lista numerada e aguarda escolha (desambiguação), reusando o padrão de candidatas já existente.
3. A `version` retornada alimenta `EditEntryCommand.TargetVersion` para o optimistic lock (ADR-003). O alvo deve pertencer ao usuário autenticado (ownership via `principalCtx` + cláusula `WHERE user_id` do repositório); alvo inexistente e alvo soft-deleted geram mensagens distintas (D-10).

## Alternativas Consideradas

- **Reusar apenas `search_transactions`/`list` e deixar o LLM escolher o "último":** menos código, mas resolução não determinística e dependente de ordem — risco de alvo errado. Rejeitada.
- **Exigir identificador/valor exato do usuário:** menos ambíguo, porém pouco natural no WhatsApp e distante dos padrões atuais. Rejeitada.

## Consequências

### Benefícios Esperados

- Resolução determinística de "último"; base sólida para desambiguação por atributos.
- `version` disponível na leitura, habilitando o optimistic lock sem round-trip extra.

### Trade-offs e Custos

- Nova tool + porta de leitura no ledger e possivelmente um método de repositório de leitura (ordenação por recência).

### Riscos e Mitigações

- Risco: ordenação por recência ambígua entre lançamentos do mesmo instante. Mitigação: desempate estável (ex.: `occurred_at`, `created_at`, `id`) no repositório de leitura.
- Rollback: remover a tool volta a resolução ao `search_transactions` (com perda de determinismo).

## Plano de Implementação

1. Porta `ListRecentEntries` no adapter + método de leitura no ledger.
2. Tool `get_last_entry` (schema fino) + registro em `module.go`.
3. Instruções do agente para desambiguação por atributos.
4. Testes de tool + golden `pending`/ownership.

## Monitoramento e Validação

- Run auditável cobre a leitura; sem métrica de alta cardinalidade.
- Sucesso: "edita o último lançamento" seleciona deterministicamente o alvo correto; ambiguidade gera lista.

## Impacto em Documentação e Operação

- Instruções do agente e catálogo de tools atualizados.

## Revisão Futura

- Avaliar unificar `get_last_entry` com `search_transactions` se o uso mostrar sobreposição.
