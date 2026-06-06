# DECISION BRIEF

## Problema
O sistema `mecontrola` (monolito modular Go, com módulos `internal/identity` e `internal/billing` já consolidados em E1/E2) ainda não possui taxonomia de categorias para classificar fluxo financeiro pessoal (receitas e despesas). Sem ela, ficam bloqueadas as features dependentes: lançamentos categorizados, orçamento por categoria, dashboards de gasto, metas e alertas. O usuário pediu CRUD completo com hierarquia categoria→subcategoria (ex.: "Custos Fixos" → "Aluguel"; "Conforto" → "Lavagem Automotiva"), seed inicial baseado em referências de mercado e personalização por usuário. O domínio é "fluxo financeiro pessoal" e cobre tanto receitas (entradas) quanto despesas (saídas).

## Objetivo
Entregar um novo bounded context `internal/categories` que ofereça:
- CRUD por usuário com hierarquia rígida em 2 níveis (categoria→subcategoria).
- Seed global compartilhado (read-only) com taxonomia padrão de mercado.
- Personalização: criação livre por usuário e clonagem editável das categorias de sistema.
- Soft-delete com proteção contra deleção de árvore com filhos ativos.
- Eventos `categories.v1.*` publicados via outbox com idempotência por `event_id`.
- Audit log dedicado registrando autor, momento e canal.
- Contrato estável para futuros consumidores (transactions, budget, bot WhatsApp).

Critério de sucesso preliminar: usuário do bot WhatsApp consegue (a) listar categorias do seed, (b) criar/editar/soft-deletar a própria categoria/subcategoria, (c) clonar e editar item do seed, (d) pesquisar por nome (fuzzy), com todos os eventos sendo publicados e auditáveis.

## Escopo Inicial
Inclui:
- Módulo `internal/categories` seguindo o "Padrão Obrigatório de Módulo" (`AGENTS.md`).
- Agregado `Category` com campos: `id`, `user_id` (nullable para sistema), `parent_id` (nullable self-ref), `name`, `kind` (`income`|`expense`), `description` (opcional), `created_at`, `updated_at`, `deleted_at`, `created_by`, `updated_by`, `cloned_from_category_id` (nullable).
- Migration de schema + migration de seed (`migrations/00XX_*.up.sql` + `00XX_seed_system_categories.up.sql`) com `ON CONFLICT DO NOTHING`.
- Use cases: `CreateCategory`, `UpdateCategory`, `SoftDeleteCategory`, `CloneSystemCategory`, `ListCategories`, `GetCategoryByID`, `SearchCategories` (fuzzy por nome).
- HTTP REST sob `/v1/categories` (POST, PATCH, DELETE, GET com filtros `q`, `kind`, `parent_id`, `include_system`).
- Outbox completo (publisher + idempotência por `event_id`) para `categories.v1.category_created`, `categories.v1.category_updated`, `categories.v1.category_deleted`, `categories.v1.category_cloned`.
- Tabela `category_audit_log` com colunas `id`, `category_id`, `actor_user_id`, `action`, `before`, `after`, `source` (`whatsapp`|`api`|`seed`), `occurred_at`.
- Wiring em `cmd/api`, `cmd/worker` e `internal/categories/module.go`.
- Testes unitários (domínio + use cases) e integration tests do repositório (`*_integration_test.go` Postgres real).

Exclui:
- Front-end gráfico próprio (canal MVP é exclusivamente WhatsApp).
- Cor/ícone/branding visual na entidade.
- Profundidade hierárquica > 2.
- Read-model materializado / projetor de árvore (reavaliar quando relatórios pesados exigirem).
- Reatribuição automática de lançamentos durante delete (não há módulo de lançamentos ainda).
- Versionamento completo do agregado (event sourcing).

## Restrições
- Stack Go conforme `go.mod`; sem `init()`, sem `panic` em produção, `context.Context` em toda fronteira de IO (R0–R7 da skill `go-implementation`).
- "Padrão Obrigatório de Módulo" (`AGENTS.md`): camadas `application/usecases`, `domain/{entities,valueobjects,repositories}`, `infrastructure/{http,repositories,messaging,jobs?}`, `module.go` para wiring.
- Outbox: `outbox.Publisher` com idempotência por `event_id` é obrigatório quando o módulo publica eventos (regra explícita em `AGENTS.md`).
- Migrações em `migrations/` (mesma esteira utilizada por billing e identity).
- Sem abstração de tempo (memory `feedback_no_time_abstraction`); usar `time.Now().UTC()` no ponto de uso.
- Sem `var _ Interface = (*Type)(nil)` (memory `feedback_no_interface_assertion`).
- Globais não exportados em camelCase idiomático (memory `feedback_no_underscore_global_prefix`).
- Refactors amplos exigem subagents paralelos (memory `feedback_subagents_orchestration`) — aplicar quando a wave de implementação cruzar `usecase/repo/handler/job/outbox` simultaneamente.

## Hipóteses
- H1 (confirmada): categorias cobrem receitas e despesas → coluna `kind` discriminadora.
- H2 (confirmada): hierarquia fixa em 2 níveis → validação no domínio.
- H3 (confirmada): entrega inclui soft-delete e auditoria → tabela `category_audit_log` + outbox.
- H4 (confirmada): feature de fundação que destrava orçamento/dashboards → contrato precisa ser estável.
- H5 (confirmada): segue Padrão Obrigatório de Módulo.
- H6 (confirmada): seed global compartilhado (read-only); customização via clone.
- H7 (confirmada): seed imutável pelo usuário; mas clone é editável.
- H8 (confirmada): audit log via tabela dedicada (não event sourcing completo).
- H9 (substituída por D5): em vez de exigir reatribuição, soft-delete bloqueia se houver filhos ativos; lançamentos vinculados permanecem apontando para categorias deletadas (sem módulo de lançamentos ainda).
- H10 (confirmada): API REST `/v1/categories`.

## Alternativas Avaliadas
### Alternativa 1 - Adjacency List (tabela única + parent_id) + validação no domínio
Resumo: Schema `categories(id, user_id, parent_id, name, kind, ...)`. `parent_id` é FK self-referente nullable. Domínio garante profundidade=2 e herança de `kind`. **Selecionada.**

Viabilidade:
- Técnica: alta — padrão clássico, suportado nativamente em Postgres.
- Operacional: alta — uma migração, um repositório, um handler, um stream de eventos.
- Financeira: alta — menor superfície de código e teste.

### Alternativa 2 - Duas tabelas (categories + subcategories)
Resumo: FK rígida `subcategory.category_id`. Schema enforce profundidade no banco.

Viabilidade:
- Técnica: alta, mas duplica recursos (dois agregados, dois CRUDs, dois conjuntos de testes).
- Operacional: média — mais migrations, mais handlers, mais eventos.
- Financeira: média — mais código, sem benefício proporcional dado profundidade fixa em 2.

### Alternativa 3 - Closure Table (suporta N níveis, limita 2 na app)
Resumo: Tabela auxiliar `category_paths(ancestor_id, descendant_id, depth)`. Suporta N níveis.

Viabilidade:
- Técnica: média — queries recursivas são fáceis, mas manutenção da tabela auxiliar exige triggers ou listeners.
- Operacional: média — overhead na escrita; mais consistência a manter.
- Financeira: média — overkill para profundidade=2 fixa.

### Alternativa 4 - Adjacency List + projetor de read-model category_tree_view
Resumo: Igual à Alternativa 1 no write-side; um consumer materializa `category_tree_view(user_id, root_category JSONB)`. Idêntico ao padrão de `billing.SubscriptionEventProjector`.

Viabilidade:
- Técnica: alta, mas adiciona consumer + tabela de projeção.
- Operacional: média — outro consumer para operar.
- Financeira: média — ganho marginal em cardinalidade baixa (~50 categorias por usuário).

## Trade-offs
- A (escolhida): aceito que a garantia de profundidade=2 vive no domínio, não no schema, em troca de simplicidade radical e CRUD único.
- Soft-delete bloqueando filhos: aceito 1 round-trip extra na UX em troca de prevenir delete acidental de árvore.
- Seed via migration SQL: aceito que editar seed exige nova migration em troca de versionamento via git.
- Clone em vez de override transparente: aceito mais código em troca de imutabilidade do seed.
- Outbox completo desde o início: aceito custo de tabela outbox + jobs em troca de contrato estável e padrão consistente com billing.
- Sem cor/ícone no MVP: aceito menor personalização visual em troca de aderência ao canal WhatsApp atual.
- Audit log na primeira onda: aceito prazo maior (~2-3 semanas) em troca de evitar refactor downstream.

## Riscos
- Risco: Editar seed via migration pode gerar conflito quando o seed evolui (ex.: renomeio em produção).
  Impacto: médio — usuários verem nomes antigos vs novos.
  Probabilidade: média.
  Mitigação: tratar seed como append-only (criar nova versão e marcar antiga como deprecated por flag) e documentar política de versionamento de seed no PRD.

- Risco: Listagem em árvore + filtros `q`/`kind` sem read-model pode degradar com crescimento (50+ categorias por usuário + sistema).
  Impacto: baixo no MVP, médio no médio prazo.
  Probabilidade: baixa.
  Mitigação: índices apropriados (`(user_id, kind, parent_id)`, `gin` para fuzzy via `pg_trgm`); reavaliar projeção (Alt D) quando p95 de listagem ultrapassar 50ms.

- Risco: Clone de categoria de sistema pode gerar drift se o seed for atualizado (cópia ficar desatualizada).
  Impacto: baixo — usuário mantém liberdade sobre a cópia.
  Probabilidade: média.
  Mitigação: armazenar `cloned_from_category_id`; opcional endpoint futuro de "diff/sync" (não no MVP).

- Risco: Bot WhatsApp pode interpretar mal categorias homônimas (ex.: "Mercado" sistema + "Supermercado" usuário).
  Impacto: médio — UX confusa no bot.
  Probabilidade: média.
  Mitigação: priorizar categorias do usuário sobre as do sistema em busca fuzzy; documentar regra de desempate no PRD.

- Risco: Auditoria com `before/after` JSON pode crescer ilimitadamente.
  Impacto: baixo no MVP, médio no longo prazo.
  Probabilidade: média.
  Mitigação: política de retenção (ex.: 12 meses) ou particionamento por mês (decidir no PRD).

- Risco: Idempotência de outbox depende de `event_id` único; falha de geração pode duplicar eventos.
  Impacto: médio.
  Probabilidade: baixa.
  Mitigação: usar `uuid v7` (ou KSUID) no domain event e UNIQUE constraint na tabela outbox (mesma estratégia já validada em billing).

## Custos
Estimativa relativa: média.

Drivers de custo:
- Implementação do agregado + repositório (~3-4 dias).
- HTTP handlers + middleware de autenticação (~2-3 dias).
- Outbox + idempotência (~2 dias, alinhado com billing).
- Audit log + canal de origem (~1-2 dias).
- Seed (migration + curadoria final dos nomes) (~1 dia).
- Endpoint de clone + testes (~1-2 dias).
- Testes unitários + integration tests com Postgres real (~3 dias).
- Wiring em `cmd/api` e `cmd/worker` + revisão (~1 dia).

## Impactos Operacionais
- Nova migração precisa ser executada antes do release (mesma esteira de `migrations/migrations_integration_test.go`).
- Novo consumer de outbox no `cmd/worker` (mesma esteira do `SubscriptionEventProjector` de identity).
- Bot WhatsApp precisa de cliente HTTP para `/v1/categories` (escopo do módulo do bot, não deste).
- Operação de rollback de seed exige nova migration (sem `down` destrutivo automático).
- Auditoria visível em logs/observabilidade através do canal `source`.

## Segurança
- Autenticação: API consome JWT do `internal/identity`; bot WhatsApp passa por gateway que injeta `user_id`.
- Autorização: usuário só pode ler/editar/deletar categorias próprias OU clonar de sistema. Listagem combina sistema + próprias.
- Categorias de sistema (`user_id IS NULL`) são imutáveis via API — guard explícito no use case.
- Audit log preserva intenção (action + before/after) e canal — pré-requisito para forensics de uso indevido.
- Sem dados sensíveis no agregado (apenas taxonomia).

## Observabilidade
- Métricas (Prometheus, mesmo padrão de billing):
  - `categories_create_total{kind, source}` (counter)
  - `categories_update_total{kind, source}` (counter)
  - `categories_delete_total{kind, source, outcome}` (counter; outcome = ok|blocked_has_children)
  - `categories_clone_total{kind}` (counter)
  - `categories_list_latency_seconds` (histogram)
  - `categories_search_latency_seconds` (histogram)
- Logs estruturados com `category_id`, `actor_user_id`, `source`, `event_id`.
- Tracing OTEL ao redor de use cases (mesmo wiring de billing).
- Eventos no outbox são fonte para análise de adoção downstream.

## Escalabilidade
- Cardinalidade esperada: ~30-50 categorias de sistema + ~10-30 por usuário ativo no estado estacionário.
- Listagem com `user_id` indexado + `(user_id, kind, parent_id)` composto deve responder <30ms em produção.
- Fuzzy search via `pg_trgm`/`ILIKE` é suficiente até ~10k registros. Acima disso, considerar índice GIN + score ranking.
- Gargalo não esperado no MVP. Alt D (projetor) é a porta de saída se algum relatório de longo prazo justificar.

## Alternativa Recomendada
**Alternativa 1 - Adjacency List (tabela única + parent_id) + validação no domínio.**

## Justificativa
- Maior pontuação no scorecard (40), com melhores notas em complexidade, tempo de entrega, custo e manutenibilidade.
- Profundidade=2 é restrição explícita (P1.4); enforcement no domínio é coerente com R5/R7 da skill `go-implementation`.
- Cardinalidade baixa elimina a vantagem prática de read-model materializado (Alt D).
- Schema simples mantém migrações, testes e wiring proporcionais ao valor de negócio.
- Consistente com `internal/billing` e `internal/identity` em DDD, outbox e migrations.
- Permite adicionar projeção (Alt D) no futuro sem migração disruptiva — apenas adicionar consumer + tabela de projeção.

## Decisões Pendentes
- Política de retenção do `category_audit_log` (a ser decidida no PRD: 12 meses default, particionado por mês?).
- Curadoria final dos nomes do seed (PT-BR, ordenação, descrições) — definir no PRD a partir das referências consultadas.
- Estratégia para versionar mudanças no seed (append-only com `deprecated_at` vs migration que renomeia in-place).
- Contrato exato do endpoint de busca fuzzy (`q` mínimo de caracteres, paginação, ranking).
- Como o bot WhatsApp se autentica vs API direta (afeta middleware) — pode estar fora do escopo deste módulo.

## Próximo Passo Recomendado
`create-prd` com objetivo de transformar este brief em PRD numerado contendo:
- Requisitos funcionais explícitos (FR-001 a FR-NNN) cobrindo CRUD, seed, clone, soft-delete, search, audit log e eventos.
- Schema completo do agregado e migrações.
- Especificação fechada do seed (lista PT-BR de categorias e subcategorias para income e expense).
- Contrato HTTP detalhado (request/response, status codes, validações).
- Plano de testes (unidade + integração).
- Critérios de aceitação por requisito.
