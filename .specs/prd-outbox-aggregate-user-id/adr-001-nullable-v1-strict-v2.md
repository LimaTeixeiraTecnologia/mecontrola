# ADR-001 — Coluna NULL na v1, validação obrigatória adiada para v2

## Metadados

- **Título:** `aggregate_user_id` aceita NULL na v1; tornar NOT NULL é decisão de v2 futura
- **Data:** 2026-06-12
- **Status:** Aceita
- **Decisores:** Operador do mecontrola
- **Relacionados:** [PRD](prd.md) RF-04, RF-08; [techspec](techspec.md) seção Storage

## Contexto

Adicionar `aggregate_user_id` como coluna em `outbox_events` cria a pergunta: NULL ou NOT NULL? E em paralelo: o construtor `outbox.NewEvent` deve **retornar erro** quando ausente, ou apenas logar e seguir?

NULL + opcional permite rollout single-deploy sem fase intermediária e mantém compat com registros antigos. NOT NULL + obrigatório exige coverage 100% dos callers antes de aplicar a migration — caso contrário, o app não consegue inserir eventos legítimos.

Existem 12 callers em 5 módulos. A chance de erro humano (PR futuro adiciona caller sem popular) é alta.

## Decisão

**v1 (este PRD):**
- Coluna `aggregate_user_id UUID NULL`.
- `outbox.NewEvent` aceita ausência. Quando vazio e `!isSystemEvent(type)`: log warn estruturado + métrica `outbox_events_inserted_total{has_user_id="false"}`. **Não retorna erro.**
- Gate de lint `task lint:outbox-user-id` falha CI em PRs futuros que adicionem callers sem `AggregateUserID`. Defesa em profundidade no CI substitui o que seria validação runtime no construtor.

**v2 (PRD futuro, sem prazo cravado):**
- `outbox.NewEvent` retorna `ErrInvalidAggregateUserID` quando vazio e `!isSystemEvent`.
- Migration `ALTER COLUMN ... SET NOT NULL` após confirmação que métrica `has_user_id="false"` = 0 por 30 dias.
- RLS Postgres pode usar `aggregate_user_id`.

## Alternativas Consideradas

1. **NOT NULL + erro em construtor na v1** — rigor máximo. **Rejeitada**: exige coverage 100% antes do deploy; se um caller for esquecido, app trava em production. Risco operacional alto sem ganho proporcional.
2. **NULL + erro em construtor na v1** — semi-rigor. **Rejeitada**: contradição (coluna aceita NULL, código não). Confunde reviewer.
3. **NOT NULL + DEFAULT '00000000-...'** — atende constraint mas vaza sentinel. **Rejeitada**: dificulta auditoria (quem é o "user zero"?) e produz falso positivo em queries.

## Consequências

### Benefícios Esperados

- Rollout single-deploy possível (migration + código + callers no mesmo PR ou em PRs próximos).
- Compat com registros antigos sem backfill.
- Gate de CI substitui validação runtime com diff zero de produção.

### Trade-offs e Custos

- Janela de exposição: caller que esquece `AggregateUserID` em PR futuro insere row com NULL silenciosamente até alguém olhar a métrica. **Mitigação**: gate de lint catch antes do merge; métrica + alerta operacional como segunda camada.
- Eventual migração para NOT NULL exige novo PRD e janela de deploy com cuidado.

### Riscos e Mitigações

- **R-01**: gate de lint tem bug e deixa passar. **Mitigação**: gate é simples (grep `outbox.EventInput{` sem `AggregateUserID:`); testar adversarialmente.
- **R-02**: métrica vira ruidosa por eventos de sistema legítimos sem dono. **Mitigação**: allowlist em ADR-004 silencia esses tipos.

## Plano de Implementação

1. Migration `000017` com coluna NULL.
2. `outbox.Event` + `EventInput` ganham campo. `NewEvent` valida UUID se presente, log warn se ausente.
3. Allowlist (ADR-004) inicial vazia ou mínima.
4. Gate de lint.
5. v2 quando métrica indicar coverage estável.

## Monitoramento e Validação

- Painel "Outbox Adoption" no Grafana: `has_user_id="true"` / total.
- Alerta `< 99%` por 10 min em estado estacionário.

## Impacto em Documentação e Operação

- Runbook `docs/runbooks/outbox.md` (se existir) ganha seção sobre o campo.
- README do módulo platform/outbox ganha nota.

## Revisão Futura

Reabrir como ADR de superseder quando:
- Métrica `has_user_id="true"` ≥ 99.99% por 30 dias.
- RLS Postgres entrar em pauta.
- Data sugerida: 2026-12-12 ou ao atingir critério de coverage.
