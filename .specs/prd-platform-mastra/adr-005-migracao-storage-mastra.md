# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Migration `000003` — DROP `agent_*` + storage genérico inspirado no Mastra + `vector`
- **Data:** 2026-06-29
- **Status:** Aceita
- **Decisores:** Time de plataforma
- **Relacionados:** PRD (RF-35..RF-39), techspec, ADR-002, ADR-004

## Contexto

O módulo `internal/agent` será apagado; suas 7 tabelas (`agent_sessions`, `agent_decisions`, `agent_threads`, `agent_runs`, `agent_working_memory`, `agent_observations`, `agent_processed_events`, criadas em `000001`) ficam órfãs e carregam semântica de domínio (`intent_kind`, etc.). A plataforma precisa de um storage genérico, com chaves opacas, inspirado no storage do Mastra (threads, messages, resources/working memory, runs, snapshots, vetores, scorer results). As tabelas `workflow_runs`/`workflow_steps` (kernel) são preservadas. Migrations seguem golang-migrate sequencial `000NNN_*.{up,down}.sql` com par up/down.

## Decisão

Criar `migrations/000003_platform_mastra.{up,down}.sql`:

- **up:** `DROP TABLE` das 7 tabelas `agent_*` (ordem respeitando FKs); `CREATE EXTENSION IF NOT EXISTS vector`; `CREATE TABLE` de `platform_threads`, `platform_messages`, `platform_resources`, `platform_runs`, `platform_embeddings` (coluna `vector(1536)` + índice **HNSW**), `platform_scorer_results`. Todas com chaves opacas e sem coluna de semântica de domínio. `workflow_runs`/`workflow_steps` (kernel) **não são tocadas** (mantidas).
- **down (simétrico):** `DROP` das tabelas `platform_*` e da dependência de índice; `DROP EXTENSION` se criada por esta migration; **recriar as 7 tabelas `agent_*`** copiando o DDL exato de `000001` (reversibilidade real).

`workflow_runs`/`workflow_steps` permanecem intactas. `platform_runs` referencia o snapshot do kernel por `(workflow, correlation_key)`.

## Alternativas Consideradas

- **Renomear/ALTER as `agent_*` in place.** Vantagem: evita recriar. Desvantagem: FKs para `users`, colunas de domínio (`intent_kind`), risco de resíduo semântico na plataforma. Rejeitada (RF-37).
- **Manter `agent_*` e só adicionar novas.** Desvantagem: duplicação e dívida; contraria "desconsiderar agent totalmente". Rejeitada.
- **down não-reversível (drop sem recriar `agent_*`).** Desvantagem: viola padrão up/down simétrico do projeto. Rejeitada.

## Consequências

### Benefícios Esperados

- Schema limpo, genérico e alinhado ao Mastra; sem resíduo de domínio.
- Reversibilidade real (down recria estado anterior).

### Trade-offs e Custos

- `down` carrega o DDL completo das 7 tabelas (verboso, mas correto).
- Dependência operacional da extensão `vector`.

### Riscos e Mitigações

- **Risco:** dados de produção das `agent_*` perdidos no DROP. **Mitigação:** `internal/agent` descontinuado por decisão; backup/export pré-migração documentado no runbook; janela de migração controlada.
- **Risco:** `vector` ausente no ambiente. **Mitigação:** `IF NOT EXISTS`; pré-flight; falha explícita da migração se a extensão não puder ser criada.

## Plano de Implementação

1. Escrever `000003` up/down; copiar DDL `agent_*` de `000001` para o down.
2. Validar `migrate up`/`migrate down`/`up` idempotente em ambiente de teste.
3. Teste de integração: aplicar `000001..000003`, exercitar `platform_*` e pgvector; rodar `down` e verificar restauração.

## Monitoramento e Validação

- Verificação pós-migração: existência das tabelas `platform_*`, extensão `vector`, ausência das `agent_*`.
- Critério de sucesso: up/down reversíveis sem erro; integração verde.

## Impacto em Documentação e Operação

- Runbook de deploy: passo de backup, pré-flight pgvector, janela de migração.

## Revisão Futura

- Revisitar índice ANN e dimensionalidade conforme volume (alinhado a ADR-004).
