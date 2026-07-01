# Execucao — auditoria de unificacao de migrations PostgreSQL

## Adendo — 2026-07-01 14:05 UTC

- verificacao remota executada via SSH em `root@187.77.45.48`
- evidencia encontrada no host alvo:
  - `mecontrola.schema_migrations` existia
  - tabelas de negocio como `mecontrola.users` nao existiam mais
  - o schema efetivo da VPS estava vazio de dados e sem baseline estrutural aplicado
- impacto na decisao:
  - o bloqueio anterior deixava de valer para essa VPS especifica
  - a premissa de zero impacto passou a ser comprovavel para esse ambiente em `2026-07-01`
- desfecho desta rodada:
  - unificacao implementada no repositorio como baseline unico em `migrations/000001_initial_schema.*`
  - removidas as migrations intermediarias `000002` a `000007`
  - validacoes locais executadas com sucesso para `cmd/migrate` e `migrations`

## 1. Executive summary

- decisao final: bloquear a unificacao destrutiva do historico de migrations
- motivo: a premissa "nenhum usuario impactado" nao esta comprovada no estado atual do repositorio e ha evidencia local contraria em 2026-06-29
- status: `BLOCKED`

Em `2026-06-29`, o repositorio registrou uma tentativa anterior de unificacao que foi revertida apos verificacao de dados reais em producao em `docs/runs/2026-06-29-unificar-migrations.md`. O estado atual tambem mostra uso ativo de tabelas introduzidas depois do baseline inicial, inclusive `platform_*` e `agents_write_ledger`, alem de testes e repositorios que dependem do schema incremental atual.

Conclusao operacional: sem evidencia nova e externa ao repositorio provando que o ambiente alvo e descartavel ou sem dados ativos, a unificacao por squash ou rewrite do historico nao atende o requisito de zero impacto.

## 2. Inventario das migrations atuais

| migration | finalidade | objetos principais | dependencias | risco |
| --- | --- | --- | --- | --- |
| `000001_initial_schema` | baseline estrutural do schema `mecontrola` | cria tabelas de negocio, tabelas operacionais, funcoes, extensoes `unaccent`, `pgcrypto`, `pg_trgm`, indices e constraints | schema `mecontrola`, bookkeeping em `mecontrola.schema_migrations` via `cmd/migrate/migrate.go` | alto |
| `000002_seed_reference_data` | seeds de referencia e editorial para billing e categorias | popula `billing_plans`, `categories`, `category_dictionary`, atualiza `category_editorial_version` | depende de `000001` | medio |
| `000003_platform_mastra` | forward migration do modelo antigo `agent_*` para `platform_*` | drop `agent_*`, create extension `vector`, cria `platform_threads`, `platform_resources`, `platform_messages`, `platform_runs`, `platform_embeddings`, `platform_scorer_results` | depende de `000001` e preservacao de dados existentes | alto |
| `000004_drop_onboarding_sessions` | remove tabela descontinuada | drop `onboarding_sessions` e indice associado | depende de `000001` | medio |
| `000005_activation_journey` | evolucao do onboarding/ativacao | adiciona colunas em `onboarding_tokens`, cria `onboarding_activation_nomatch_throttle` e `onboarding_welcome_processed` | depende de `000001` | medio |
| `000006_agents_write_ledger` | ledger de idempotencia de escrita do agente | cria `agents_write_ledger` e indice por usuario/data | depende de `000001` | medio |
| `000007_transactions_origin_ref` | idempotencia/origem em transacoes | adiciona colunas e unique indexes em `transactions` e `transactions_card_purchases` | depende de `000001` e uso atual de transacoes | medio |

Observacoes obrigatorias:

- `taskfiles/migrate.yml` aplica migrations com `go run ./cmd/... migrate`.
- `cmd/migrate/migrate.go` fixa `SchemaName: "mecontrola"` para `schema_migrations`.
- `migrations/migrations_integration_test.go` valida o schema incremental atual e ja assume `onboarding_sessions` ausente e `agents_write_ledger` presente.
- `scripts/verify-go-mod.sh` referenciado pela skill Go nao existe no workspace atual; ausencia registrada, sem substituicao inventada.

## 3. Mapa do schema final atual

| tabela/grupo | origem | proposito | evidencia de uso | status |
| --- | --- | --- | --- | --- |
| `outbox_events` | `000001` | outbox transacional | testes e codigo em `internal/onboarding`, `internal/card`, `internal/transactions` | ativa |
| `users`, `user_identities`, `auth_events`, `identity_*` | `000001` | identidade e autenticacao | repositorios e testes em `internal/identity` e `migrations/migrations_integration_test.go` | ativa |
| `billing_*` | `000001` + `000002` | billing, eventos, seeds de planos | testes de migrations e modulos de billing | ativa |
| `categories`, `category_dictionary`, `category_editorial_version` | `000001` + `000002` | catalogo editorial e classificacao | repositorios, handlers e testes em `internal/categories` | ativa |
| `cards` | `000001` | cartoes | repositorios, jobs e e2e em `internal/card` | ativa |
| `budgets*`, `budget_alerts_sent` | `000001` | orcamentos, alertas e drafts | jobs, repositorios e testes em `internal/budgets` | ativa |
| `transactions*` | `000001` + `000007` | transacoes, compras, faturas, recorrencia e sumario | repositorios e e2e em `internal/transactions` | ativa |
| `channel_processed_messages` | `000001` | deduplicacao de mensagens inbound | testes de migrations e stack worker | ativa |
| `onboarding_tokens` | `000001` + `000005` | jornada de ativacao | modulo onboarding e testes de migrations | ativa |
| `onboarding_activation_nomatch_throttle`, `onboarding_welcome_processed` | `000005` | controle operacional da ativacao | `migrations/migrations_integration_test.go` e fluxo de onboarding | ativa |
| `platform_threads`, `platform_resources`, `platform_messages`, `platform_runs`, `platform_embeddings`, `platform_scorer_results` | `000003` | substrato de agente/memoria/scoring | repositorios e testes em `internal/platform/agent`, `internal/platform/memory`, `internal/platform/scorer` | ativa |
| `agents_write_ledger` | `000006` | ledger de idempotencia de escrita do agente | repositorio em `internal/agents/infrastructure/persistence/write_ledger_repository.go`, e2e e plano dedicado em `docs/plans/2026-07-01-eliminar-janela-idempotencia-write-ledger.md` | ativa |
| `onboarding_sessions` | `000001`, removida por `000004` | sessao antiga de onboarding | migrations e reviews historicos confirmam remocao | removida por migration |
| `agent_*` | `000001`, removidas por `000003` | substrato antigo de agentes | `000003_platform_mastra.up.sql` remove explicitamente e o codigo atual usa `platform_*` | removida por migration |

## 4. Analise formal de tabelas obsoletas

### `onboarding_sessions`

- evidencias a favor da remocao:
  - `000004_drop_onboarding_sessions.up.sql` faz `DROP TABLE`
  - `migrations/migrations_integration_test.go` valida que a tabela esta ausente no schema final
- evidencias contra:
  - documentacao historica ainda menciona a tabela em pontos nao canonicamente atualizados
- conclusao:
  - nao e candidata nova; ja foi removida de forma explicita
- decisao final:
  - manter somente o historico incremental atual

### `agent_sessions`, `agent_decisions`, `agent_working_memory`, `agent_observations`, `agent_threads`, `agent_runs`, `agent_processed_events`

- evidencias a favor da remocao:
  - `000003_platform_mastra.up.sql` remove explicitamente as tabelas antigas
  - codigo atual usa `platform_*` em vez de `agent_*`
- evidencias contra:
  - `000003_platform_mastra.down.sql` ainda precisa recria-las para rollback historico
- conclusao:
  - obsolescencia funcional comprovada no schema final, mas rollback historico ainda depende do DDL de down
- decisao final:
  - nao apagar a trilha historica das migrations que as descrevem

### Demais tabelas do schema final

- evidencias a favor da remocao:
  - nenhuma prova cumulativa suficiente no working tree
- evidencias contra:
  - uso ativo em codigo, testes, bootstrap ou constraints do schema
- conclusao:
  - nao elegiveis para remocao
- decisao final:
  - bloquear qualquer remocao adicional

## 5. Estrategia recomendada de unificacao

- abordagem escolhida: bloquear squash/destruicao do historico; permitir apenas, em trabalho futuro e separado, um baseline novo para fresh install se houver aprovacao operacional explicita
- por que e a mais segura:
  - preserva upgrades existentes
  - evita reescrever historico ja conhecido como sensivel a dados reais
  - respeita a regra da skill de persistencia: nao rodar migrations destrutivas automaticamente em producao
- como evita impacto em usuarios:
  - nao altera `000001`..`000007`
  - nao troca o caminho de upgrade de bancos existentes
- como preserva integridade, rastreabilidade e rollback:
  - mantem cadeia historica auditavel
  - preserva `down.sql` existentes onde ja ha rollback definido

## 6. Plano exato de implementacao

Implementacao executada nesta rodada:

- criado este relatorio de execucao e bloqueio em `docs/database/`
- nenhuma migration, entrypoint Go, teste ou task foi alterado

Implementacao permitida apenas se houver nova aprovacao baseada em evidencia externa:

1. comprovar no ambiente alvo que nao ha usuarios/dados afetados
2. desenhar baseline novo apenas para fresh install
3. manter `000001`..`000007` como caminho de upgrade legado
4. adicionar testes que provem fresh install via baseline novo e upgrade via cadeia historica

Arquivos que seriam afetados somente nesse cenario futuro:

- `migrations/`
- `migrations/migrations_integration_test.go`
- eventualmente `cmd/migrate/migrate.go` se for necessario suportar dois caminhos de bootstrap sem ambiguidade

## 7. Validacao obrigatoria

Comandos nao executados nesta rodada porque o resultado seguro foi bloqueio documental, sem mudanca em codigo ou migrations:

- `gofmt -w <arquivos alterados, se houver Go>`
- `go test -race -count=1 ./migrations/...`
- `go test -race -count=1 ./cmd/migrate/...`
- `go build ./cmd/...`
- `go vet ./cmd/... ./migrations/...`

Motivo da nao execucao:

- a implementacao aprovada terminou antes da fase de alteracao de codigo
- nenhuma mudanca em Go ou SQL versionado foi aplicada

## 8. Referencias oficiais do PostgreSQL utilizadas

- `DROP TABLE` — https://www.postgresql.org/docs/current/sql-droptable.html
  - influencia: reforca que remocao de tabelas e operacao destrutiva; nao aceitavel sem prova de impacto zero
- `ALTER TABLE` — https://www.postgresql.org/docs/current/sql-altertable.html
  - influencia: reforca que alteracoes estruturais podem adquirir lock forte; relevante para evitar rewrite agressivo em ambiente com usuarios
- `CREATE EXTENSION` — https://www.postgresql.org/docs/current/sql-createextension.html
  - influencia: baseline final precisa preservar o comportamento de extensoes como `vector`, `pgcrypto`, `pg_trgm` e `unaccent`

## 9. Riscos residuais

- riscos aceitos:
  - documentacao historica fora de `AGENTS.md` e migrations pode continuar citando tabelas ja removidas
- riscos bloqueantes:
  - ausencia de prova atual de impacto zero no ambiente alvo
  - historico local de `2026-06-29` mostra que a premissa ja falhou para producao
  - rewrite do historico pode quebrar upgrade de bancos existentes e invalida rollback historico
- premissas que ainda precisariam de confirmacao externa:
  - versao atual aplicada em `mecontrola.schema_migrations` no ambiente alvo
  - cardinalidade real das tabelas de negocio e de plataforma
  - existencia ou nao de bancos descartaveis separados para fresh install
