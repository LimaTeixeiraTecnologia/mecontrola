# ADR-002 — Rollout atômico single-deploy, sem dual-write

## Metadados

- **Título:** Migration + código + 12 callers atualizados no mesmo deploy
- **Data:** 2026-06-12
- **Status:** Aceita
- **Decisores:** Operador do mecontrola
- **Relacionados:** [PRD](prd.md) OBJ-07; [techspec](techspec.md) seção Sequenciamento

## Contexto

Mudar envelope de evento que atravessa 5 módulos pode ser feito de duas formas:
- **Dual-write**: producers gravam o campo novo + o legado em paralelo por uma janela; consumers preferem o novo mas aceitam o legado; eventualmente remove-se o legado.
- **Atômico**: migration + código + producers atualizados juntos; sem dual-write.

Dual-write parece seguro mas adiciona complexidade. Aqui não há "legado" — `user_id` está dentro do payload JSON e continuará lá (payload não é tocado). O que muda é a presença de uma coluna nova e campo struct novo, ambos opcionais (ADR-001).

## Decisão

**Single-deploy atômico:**
1. Mesmo PR ou PRs adjacentes sem janela contém: migration `000017`, alteração de `outbox.go`/`envelope.go`/`storage_postgres.go`, e atualização dos 12 callers.
2. Producers existentes continuam embedando `user_id` no payload JSON — **não remover** desse lugar (consumers que parseiam payload continuam funcionando).
3. Novo campo `AggregateUserID` é redundante com payload na v1; redundância é aceita como custo para single-deploy.

Sem dual-write. Sem feature flag. Sem fase intermediária.

## Alternativas Consideradas

1. **Dual-write com feature flag `OUTBOX_AGGREGATE_USER_ID_ENABLED`** — permite ativar/desativar runtime. **Rejeitada**: flag adiciona estado mutável; complica testes; nenhuma fase de rollback realista (revert do deploy é mais simples).
2. **Rollout em 3 PRs** (migration → código outbox → callers em PRs separados) — staged. **Rejeitada**: cada PR intermediário deixa o sistema em estado parcial sem benefício (sem caller usar, campo vazio em todo lugar).
3. **Manter user_id apenas no payload JSON, adicionar índice GIN** — sem mudança de schema/código. **Rejeitada**: índice GIN sobre JSONB para single field é caro vs index B-tree em coluna dedicada; queries por user_id ainda exigem JSON extraction.

## Consequências

### Benefícios Esperados

- Deploy único, sem fases intermediárias com estado inconsistente.
- Revert simétrico: revert do commit → migration `down` + código antigo.
- Sem código de feature flag a ser limpo depois.

### Trade-offs e Custos

- PR grande tocando 5 módulos. **Mitigação**: tasks decompostas em 8 itens executáveis em paralelo (tasks.md).
- Coordenação obrigatória: aplicar migration antes do deploy de código, ou deploy gera erro de "column doesn't exist". **Mitigação**: pipeline `migrate -> deploy` atômico no Taskfile do projeto.

### Riscos e Mitigações

- **R-01**: producer atualizado deploya antes da migration. **Mitigação**: `task migrate:up && task deploy` em sequência única; revert da migration disponível.
- **R-02**: revert parcial (código antigo + coluna nova) — coluna não preenchida, sem erro. Aceitável.
- **R-03**: revert completo + dump de registros pendentes precisa restaurar coluna. **Mitigação**: migration `down` apenas `DROP COLUMN`; dados em payload JSON intactos.

## Plano de Implementação

1. Aplicar migration em staging.
2. Deploy de código em staging.
3. Smoke: emitir 1 evento em cada módulo, verificar `aggregate_user_id` populado.
4. Aplicar em produção.

## Monitoramento e Validação

- Métrica `outbox_events_inserted_total{has_user_id}` em staging deve ter `true` ≥ 99%.
- Em produção: confirmar mesma proporção nas primeiras 24h.

## Impacto em Documentação e Operação

- Runbook de deploy: nota sobre ordem migrate-then-deploy.
- Nenhum playbook adicional necessário.

## Revisão Futura

Revisar se houver migração de outros campos do envelope outbox no futuro (e.g. `correlation_id`, `causation_id`) — mesma estratégia atômica.
