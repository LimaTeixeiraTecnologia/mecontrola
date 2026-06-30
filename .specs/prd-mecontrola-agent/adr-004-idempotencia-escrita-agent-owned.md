# Registro de Decisão Arquitetural (ADR-004)

## Metadados

- **Título:** Idempotência exatamente-uma-vez por intenção via ledger agent-owned
- **Data:** 2026-06-30
- **Status:** Aceita
- **Decisores:** Time de plataforma
- **Relacionados:** PRD (RF-38, D-19, D-22), techspec.md; `internal/platform/idempotency/middleware.go`; memória `feedback_agent_calls_modules_own_persistence`

## Contexto

Lançamentos exigem correção financeira: retries do agente, loops de tool-calling dentro de um mesmo `Run`, e reentregas do canal não podem duplicar. O middleware de idempotência atual **só persiste respostas 4xx** (`internal/platform/idempotency/middleware.go:136-154`): em sucesso (2xx) não grava, então uma segunda chamada cria novo registro. Os use cases de `transactions` geram `eventID := uuid.New()` por execução, sem aceitar chave de idempotência do cliente. O WhatsApp deduplica por `wamid` no dispatcher, mas isso não cobre múltiplas escritas dentro de um único inbound (D-22) nem re-execução do Run.

## Decisão

Introduzir um **ledger de idempotência agent-owned**: tabela `agents_write_ledger(user_id, wamid, item_seq, operation, resource_id, resource_kind, created_at)` com **unique `(wamid, item_seq, operation)`**. Um helper `IdempotentWrite` envolve toda tool de escrita: antes de chamar o use case, consulta o ledger por `(wamid, item_seq, operation)`; se existir, retorna o `resource_id` registrado como **replay** (`ToolOutcomeReplay`), sem segunda mutação; se não, executa o use case, grava o `resource_id` no ledger (mesma transação lógica quando possível) e retorna. `wamid` vem do inbound (`MessageID`); `item_seq` distingue múltiplos lançamentos de uma mesma mensagem (D-22); `operation` distingue create_transaction/create_card_purchase/edit/delete. O ledger é propriedade do módulo `agents` (não compartilha transação com outros módulos — apenas chama seus use cases via binding).

## Alternativas Consideradas

- **Corrigir o middleware para persistir 2xx** — Vantagem: resolve para todos os chamadores REST. Desvantagem: raio de impacto amplo, risco de regressão em billing/identity; fora do escopo do PRD. Rejeitada (registrada como risco residual).
- **Adicionar idempotency key aos use cases de transactions** — Vantagem: idempotência no produtor. Desvantagem: muda contrato de domínio de outro módulo; viola fronteira/escopo. Rejeitada.
- **Confiar só no dedup de `wamid` do WhatsApp** — não cobre loop de tool nem múltiplos itens por mensagem. Rejeitada (D-19 escolheu garantia explícita).

## Consequências

### Benefícios Esperados

- Exatamente-uma-vez por intenção sem tocar contratos de domínio; replay observável; suporta múltiplos lançamentos por mensagem.

### Trade-offs e Custos

- Tabela e consulta extra por escrita; o agente mantém persistência própria (alinhado à memória de design).

### Riscos e Mitigações

- **Janela entre use case e gravação do ledger** (crash no meio) → gravar o ledger na mesma transação do agent-owned quando o use case expõe o id de forma síncrona; em falha, o replay subsequente reconcilia por `wamid+item_seq`. Documentar semântica at-least-once→exactly-once via unique constraint.
- **Crescimento da tabela** → job de retenção análogo ao dedup do WhatsApp.

## Plano de Implementação

1. Migration `agents_write_ledger` + unique.
2. Repositório + `IdempotentWrite` helper.
3. Integrar nas tools de escrita; mapear replay para `ToolOutcomeReplay`.
4. Teste de concorrência (unique sob corrida) e replay.

## Monitoramento e Validação

- `agents_write_total{operation,outcome=created|replay}`; alerta se taxa de replay anômala.
- Teste de integração: dupla execução do mesmo inbound cria um único recurso.

## Impacto em Documentação e Operação

- Runbook: semântica de idempotência e job de retenção do ledger.

## Revisão Futura

- Reavaliar se o middleware global for corrigido para 2xx (poderia simplificar o ledger).
