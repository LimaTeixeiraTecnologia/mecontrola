# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Corte imediato da superfície card-purchase com descarte de dados e refactor do agente
- **Data:** 2026-07-04
- **Status:** Aceita
- **Decisores:** Product owner (usuário), time de plataforma
- **Relacionados:** `prd.md` (RF-24, RF-24a), `techspec.md`, `adr-003-unified-transactions-schema.md`

## Contexto

A superfície `card-purchase` (5 rotas, 5 handlers, 5 use cases, DTOs, publisher, tabela
`transactions_card_purchases`) duplica o registro de compra no crédito. A unificação exige removê-la.
Duas restrições reais do ambiente:
- Produção tem **1 usuário e ledger vazio** (memória do projeto; verificável antes do release) — não
  há dados reais de compras no crédito a preservar.
- O consumidor `internal/agents` chama os use cases de card-purchase via binding + tools
  (`register_card_purchase`, `get_card_purchase`, `list_card_purchases`); removê-los sem substituto
  quebraria o agente em produção.

## Decisão

1. **Remoção imediata** da superfície card-purchase (rotas, handlers, use cases, DTOs, publisher).
2. **Descarte de dados**: `DROP TABLE transactions_card_purchases` sem backfill (seguro pelo ledger
   vazio). Confirmação obrigatória do estado de produção imediatamente antes do release.
3. **Refactor do agente no mesmo PR**: `register_expense` passa a aceitar `payment_method=credit_card`
   + `card_id` + `installments`, chamando o `CreateTransaction` unificado; as 3 tools de card-purchase
   e os 5 métodos correspondentes de binding/interface são removidos. Não há intervalo com agente
   quebrado.

Escopo: `internal/transactions/infrastructure/http`, `application/usecases`, `application/dtos`,
`migrations/000003`, `internal/agents/{application/tools,infrastructure/binding,application/interfaces}`.

## Alternativas Consideradas

- **Manter card_purchases legado read-only.** Vantagem: preserva histórico. Desvantagem: duas fontes
  de leitura, complexidade permanente; sem valor com ledger vazio. Rejeitada (PRD D-08).
- **Depreciar rotas com período de compatibilidade.** Vantagem: menos breaking. Desvantagem: mantém
  código morto e ambiguidade; produção tem 1 usuário controlado. Rejeitada (PRD RF-24).
- **Agente fora do escopo.** Rejeitada: deixaria agente quebrado no intervalo (risco de produção).

## Consequências

### Benefícios Esperados
- Elimina duplicidade e código morto; uma só fonte de verdade de escrita.
- Migration simples sem backfill (corte limpo).
- Agente permanece funcional.

### Trade-offs e Custos
- **Breaking change irreversível** (drop de tabela + rotas). Só aceitável pelo ledger vazio.
- PR maior por incluir o refactor do agente.

### Riscos e Mitigações
- Risco: produção não estar realmente vazia no momento do release → perda de dados. Mitigação:
  **gate pré-release** — `SELECT count(*) FROM mecontrola.transactions_card_purchases` deve ser 0;
  abortar release se >0. Rollback: migration `down` recria a tabela vazia + revert do PR.
- Risco: clientes externos das rotas `/card-purchases`. Mitigação: produção controlada (1 usuário),
  comunicação no changelog; e2e confirma 404 pós-corte.

## Plano de Implementação
1. Migration `000003` (ADR-003) com `DROP TABLE` ao final.
2. Remoção de rotas/handlers/use cases/DTOs/publisher de card-purchase.
3. Refactor do agente (`register_expense` + binding) e remoção das tools.
4. Gate pré-release (contagem 0) + validação real-LLM do agente.

## Monitoramento e Validação
- e2e: 404 nas rotas removidas; `register_expense` credit_card cria transação parcelada.
- Métrica: ausência de séries `transactions.card_purchase.*` no outbox pós-deploy.
- Critério de sucesso: 0 referências a `card_purchase` em produção (rotas, tabela, eventos).

## Impacto em Documentação e Operação
- Runbook `docs/runbooks/transactions.md`, dashboards e alertas: remover card-purchase.
- Changelog de release marcando breaking change.
- Onboarding do agente: nova assinatura de `register_expense`.

## Revisão Futura
- Reavaliar se o produto adotar múltiplos usuários com dados reais antes de qualquer corte análogo
  futuro (nesse cenário, descarte deixa de ser aceitável e exigiria migração de dados).
