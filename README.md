# MeControla

[![CI](https://github.com/LimaTeixeiraTecnologia/mecontrola/actions/workflows/ci.yml/badge.svg)](https://github.com/LimaTeixeiraTecnologia/mecontrola/actions/workflows/ci.yml)
[![CD](https://github.com/LimaTeixeiraTecnologia/mecontrola/actions/workflows/cd.yml/badge.svg)](https://github.com/LimaTeixeiraTecnologia/mecontrola/actions/workflows/cd.yml)
[![Image Signed](https://img.shields.io/badge/image-cosign%20keyless-blue)](https://github.com/LimaTeixeiraTecnologia/mecontrola/actions/workflows/cd.yml)
[![SBOM](https://img.shields.io/badge/SBOM-SPDX--JSON-green)](https://github.com/LimaTeixeiraTecnologia/mecontrola/actions/workflows/cd.yml)
[![Governance](https://img.shields.io/badge/governance-AGENTS.md-orange)](./AGENTS.md)

Monolito modular em Go para fluxos financeiros conversacionais via WhatsApp.

## Uso rápido

Se você quer só clonar, subir e validar:

```bash
cp .env.example .env
task setup
task local:up
```

Depois acesse:

| Recurso | Endereço |
|---|---|
| API | `http://localhost:8080` |
| Health principal | `http://localhost:8080/health` |
| Health detalhado | `http://localhost:8080/healthz` |
| Readiness | `http://localhost:8080/readyz` |
| Liveness | `http://localhost:8080/livez` |
| OpenAPI local | `http://localhost:8080/__docs` |
| Catálogo OpenAPI JSON | `http://localhost:8080/__docs/openapi/index.json` |
| Grafana local | `http://localhost:3000` |

`/__docs` só é exposto quando `ENVIRONMENT=local`.

## Escolha seu modo de execução

| Objetivo | Caminho recomendado | Quando usar |
|---|---|---|
| Subir tudo rápido | `task local:up` | Dia a dia com app em containers |
| Rodar app fora do Docker | `task local:infra` + `task migrate:up` + `task run` | Debug, `go run`, VS Code |
| Paridade com produção | `task build:docker:build IMAGE_TAG=local` + `task swarm:local:up IMAGE_TAG=local` | Validar comportamento em Swarm |

## Índice

- [Visão geral do projeto](#visão-geral-do-projeto)
- [Pré-requisitos](#pré-requisitos)
- [Configuração inicial](#configuração-inicial)
- [Rodando localmente](#rodando-localmente)
- [Configuração e secrets](#configuração-e-secrets)
- [Comandos do dia a dia](#comandos-do-dia-a-dia)
- [Zerar o banco e reaplicar migrations](#zerar-o-banco-e-reaplicar-migrations) (local Cenário A/C · VPS Cenário B)
- [Debug no VS Code](#debug-no-vs-code)
- [Webhooks locais com ngrok](#webhooks-locais-com-ngrok)
- [Deploy e operação](#deploy-e-operação)
- [Acesso remoto](#acesso-remoto)
- [CI/CD](#cicd)
- [Documentação complementar](#documentação-complementar)
- [Contribuição](#contribuição)
- [Governance](#governance)

## Visão geral do projeto

### Arquitetura

- Monolito modular em Go.
- Bounded contexts em `internal/`.
- Fluxo arquitetural permitido: `infrastructure -> application -> domain`.
- Plataforma compartilhada em `internal/platform/`.

### Módulos principais

| Módulo | Responsabilidade |
|---|---|
| `internal/agents` | Runtime de agentes, workflow/tool calling, OpenRouter, memória e dispatch via WhatsApp |
| `internal/billing` | Webhook Kiwify, reconciliação, grace period e housekeeping de cobrança |
| `internal/budgets` | Orçamentos, despesas por categoria, recorrência, resumo e jobs de retenção |
| `internal/card` | CRUD de cartões, listagem paginada e fatura por competência |
| `internal/categories` | Catálogo de categorias e dicionário com busca HTTP |
| `internal/identity` | Usuários, principal/auth, gateway HMAC e housekeeping de auth events |
| `internal/onboarding` | Ativação, magic token, outreach e integração WhatsApp/Meta |
| `internal/transactions` | Transações financeiras, idempotência, recorrência e resumo mensal |
| `internal/platform` | Workflow kernel, agent runtime, outbox, worker manager, observabilidade, HTTP client, memória e integrações transversais |
| `internal/bootstrap` | Wiring de módulos e bootstrap da aplicação |

### Entrypoints

O binário `mecontrola` expõe estes subcomandos:

```bash
mecontrola server
mecontrola worker
mecontrola migrate
mecontrola migrate-down
```

Resumo:

| Comando | Uso |
|---|---|
| `mecontrola server` | Sobe o servidor HTTP |
| `mecontrola worker` | Sobe o worker de background |
| `mecontrola migrate` | Aplica migrations pendentes e sai |
| `mecontrola migrate-down` | Reverte migrations; default é 1 step, use `--steps -1` para reset total |

### Stack

| Componente | Valor atual |
|---|---|
| Linguagem | Go `1.26.4` |
| Banco | PostgreSQL `16` |
| Pooler | pgBouncer `edoburu/pgbouncer:v1.25.2-p0` |
| Migrações | `golang-migrate v4.19.1` |
| HTTP | `go-chi/chi v5.3.0` |
| Observabilidade | OpenTelemetry `v1.44.0` + `grafana/otel-lgtm:0.7.5` |
| Orquestração local/prod | Docker Compose + Docker Swarm single-node |
| Registro de imagem | `ghcr.io/limateixeiratecnologia/mecontrola` |

## Pré-requisitos

### Obrigatórios

| Ferramenta | Versão | Observação |
|---|---|---|
| Docker Engine + Compose v2 | Docker `24+` | Infra local e Swarm |
| Go | `1.26.4+` | Declarado em `go.mod` |
| Task | `3.51.1` | Runner principal |

### Recomendados para desenvolvimento

| Ferramenta | Uso |
|---|---|
| `golangci-lint` | Lint estático; `task setup` instala a versão pinada em `.tools/bin` |
| `mockery` | Geração de mocks |
| `govulncheck` | Auditoria de vulnerabilidades |
| `ngrok` | Webhooks locais |

### Necessários para produção e operação

| Ferramenta | Uso |
|---|---|
| `sops` | Edição de `deployment/config/prod.secrets.env` |
| `age` / `age-keygen` | Criptografia de secrets |
| `trivy` | Scan de imagem e SBOM |
| `cosign` | Assinatura keyless |
| `gitsign` | Assinatura keyless de commits |

Depois de instalar o básico:

```bash
task setup
```

`task setup` instala hooks, provisiona o `golangci-lint` pinado e configura `gitsign`.

## Configuração inicial

### 1. Criar `.env`

```bash
cp .env.example .env
```

### 2. O que você precisa ajustar no começo

Para desenvolvimento local sem integrações reais, normalmente basta começar com o `.env.example`.

Grupos importantes:

| Grupo | Variáveis |
|---|---|
| Execução local | `ENVIRONMENT`, `APP_MODE`, `PORT`, `WORKER_HEALTH_ADDR` |
| Banco | `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME` |
| Observabilidade local | `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_LGTM_ADMIN_USER`, `OTEL_LGTM_ADMIN_PASSWORD`, `LOG_LEVEL`, `LOG_FORMAT` |
| Agent/LLM | `OPENROUTER_API_KEY`, `AGENT_LLM_PRIMARY_MODEL`, `AGENT_LLM_EMBED_MODEL` |
| Meta/WhatsApp | `META_*`, `WHATSAPP_*`, `WA_MSG_*` |
| Kiwify | `KIWIFY_*` |
| Onboarding/Auth | `ONBOARDING_TOKEN_ENCRYPTION_KEY`, `IDENTITY_GATEWAY_SHARED_SECRET_CURRENT` |
| Email | `EMAIL_PROVIDER`, `SMTP_*` ou `RESEND_*` |

### 3. Regras práticas para o `.env`

- Para subir localmente, mantenha `ENVIRONMENT=local`.
- Em `production`, placeholders `CHANGE_ME_*` falham na validação de config.
- Os docs OpenAPI locais em `/__docs` só aparecem com `ENVIRONMENT=local`.
- O arquivo de referência completo é `.env.example`.

## Rodando localmente

### Opção 1. Ambiente completo com Docker Compose

Caminho recomendado para uso diário.

```bash
task local:up
```

O que esse comando faz:

1. sobe `postgres` e `otel-lgtm`
2. executa `migrate`
3. sobe `server` e `worker`

Validação:

| Verificação | Resultado esperado |
|---|---|
| `task local:ps` | containers `postgres`, `server`, `worker` e `otel-lgtm` no ar |
| `http://localhost:8080/health` | `200 OK` |
| `http://localhost:8080/healthz` | `200 OK` |
| `http://localhost:8080/readyz` | `200 OK` |
| `http://localhost:3000` | tela do Grafana |

Observações:

- o profile proxy com Caddy fica desligado por padrão no local
- o worker expõe health internamente em `:8081`, mas essa porta não é publicada no host no fluxo local padrão

Comandos úteis:

```bash
task local:logs
task local:down
task local:destroy
task local:db:restart
```

### Opção 2. Só a infra + aplicação fora do Docker

Use quando quiser `go run`, Delve ou debug no VS Code.

```bash
task local:infra
task migrate:up
task run
```

`task run` compila e executa o `server`.

Se você também quiser o worker fora do Docker, abra outro terminal:

```bash
go run ./cmd worker
```

Validação da infra:

| Recurso | Endereço |
|---|---|
| PostgreSQL | `localhost:5432` |
| Grafana | `http://localhost:3000` |
| OTLP gRPC | `localhost:4317` |
| OTLP HTTP | `localhost:4318` |

### Opção 3. Paridade local com produção via Swarm

Use para validar a stack canônica de produção.

Primeiro, gere a imagem local:

```bash
task build:docker:build IMAGE_TAG=local
```

Depois:

```bash
task swarm:local:config
task swarm:local:up IMAGE_TAG=local
```

Validação:

```bash
task swarm:local:ps
task swarm:local:logs
```

Observações:

- Esse fluxo usa `deployment/compose/compose.swarm.yml`.
- Ele é voltado para paridade de runtime, não para desenvolvimento rápido.

### Serviço opcional: Mailpit

`mailpit` existe no compose local, mas não sobe com `task local:infra` nem `task local:up`.

Se você precisar da UI SMTP local:

```bash
docker compose --env-file .env \
  -f deployment/compose/compose.yml \
  -f deployment/compose/compose.local.yml \
  up -d mailpit
```

Depois acesse `http://localhost:8025`.

## Configuração e secrets

### Local

O arquivo local é:

```text
.env
```

Fonte de referência:

```text
.env.example
```

### Produção

Produção usa dois arquivos versionados:

| Arquivo | Conteúdo | Commitado |
|---|---|---|
| `deployment/config/prod.env` | Configuração não secreta | Sim |
| `deployment/config/prod.secrets.env` | Secrets criptografados com SOPS + age | Sim |

Pontos importantes:

- Não existe `.env` persistente como fonte canônica na VPS.
- Em produção a aplicação lê secrets de `/run/secrets/<NOME>`.
- A chave privada `age` não deve ser commitada.

### Config UI

Para editar `prod.env` e `prod.secrets.env` via navegador:

```bash
task local:configui:hash-password
export CONFIG_UI_PASSWORD_HASH='<hash bcrypt>'
export SOPS_AGE_KEY="$(cat key.txt)"
task local:configui:run
```

Depois acesse `http://localhost:8080`.

Comportamento real do `configui`:

- bind padrão em `127.0.0.1:8080`
- autenticação básica `admin` + senha bcrypt
- procura chave age em `SOPS_AGE_KEY_FILE`, `key.txt`, `.sops/age/key.txt` ou `~/.config/sops/age/key.txt`

Documentação detalhada:

- `deployment/runbooks/configui.md`

## Comandos do dia a dia

Rode `task --list-all` para a lista completa. Os comandos abaixo são os mais úteis.

### Setup e build

| Comando | Uso |
|---|---|
| `task setup` | Prepara a máquina local |
| `task build:build` | Compila `bin/mecontrola` |
| `task build:build:all` | Cross-compile para múltiplas plataformas |
| `task build:docker:build IMAGE_TAG=<tag>` | Gera a imagem Docker |
| `task run` | Build + run do `server` local |

### Desenvolvimento local

| Comando | Uso |
|---|---|
| `task local:infra` | Sobe só `postgres` + `otel-lgtm` |
| `task local:up` | Sobe infra + migrations + app |
| `task local:ps` | Status dos containers |
| `task local:logs` | Logs dos containers |
| `task local:down` | Para os containers |
| `task local:destroy` | Remove containers e volumes locais |
| `task migrate:up` | Aplica migrations |
| `task migrate:down` | Reverte todas as migrations |

### Testes

| Comando | Uso |
|---|---|
| `task test:unit` | Unitários com `-race` e cobertura |
| `task test:integration` | Integração com Docker/testcontainers |
| `task test:all` | Unitários + integração |
| `task test:coverage` | Relatório HTML em `coverage/coverage.html` |
| `task test:e2e` | Suite E2E com Godog |
| `RUN_REAL_LLM=1 task test:conformance:real` | Conformidade do agent com OpenRouter real |
| `task agents:integration` | Integração do módulo `internal/agents` |
| `task card:test` | Unitários do módulo card |
| `task card:integration` | Integração do módulo card |

### Lint, quality e segurança

| Comando | Uso |
|---|---|
| `task lint:run` | Linter + gates obrigatórios |
| `task lint:fix` | Correções automáticas |
| `task lint:fmt` | `gofmt` + `goimports` |
| `task lint:fmt:check` | Falha se houver drift de formatação |
| `task lint:tidy` | Valida `go.mod` e `go.sum` |
| `task lint:pci` | Gate anti-PCI |
| `task lint:user-isolation` | Gate `user_id` em repositórios per-user |
| `task lint:auth-bypass` | Gate `RequireGatewayAuth` |
| `task lint:outbox-user-id` | Gate `AggregateUserID` |
| `task lint:deadcode` | Gate de código morto em `internal/agents` |
| `task card:lint` | Lint do módulo card |
| `task card:audit` | Auditoria R0-R7 do módulo card |
| `task security:vulncheck` | Vulnerabilidades Go |
| `task security:scan` | `vulncheck` + audit |
| `task check` | `lint:run` + `test:unit` + `security:vulncheck` |
| `task ci:fast` | Gate rápido para PR |
| `task ci:pipeline` | Pipeline local completa |

### Ngrok e webhooks

| Comando | Uso |
|---|---|
| `task ngrok:check` | Valida pré-requisitos |
| `task ngrok:server` | Sobe ambiente e abre túnel para `127.0.0.1:8080` |
| `task ngrok:caddy` | Sobe ambiente com Caddy e abre túnel para `127.0.0.1:80` |
| `task ngrok:urls` | Imprime URLs públicas dos webhooks |
| `task ngrok:stop:tips` | Mostra como encerrar túnel e containers |

## Zerar o banco e reaplicar migrations

Use quando precisar começar do zero — banco limpo, todas as migrations reaplicadas.

### Cenário A — via Docker Compose (fluxo padrão)

1. Parar os containers:

```bash
task local:down
```

2. Remover containers e volumes (apaga todos os dados do postgres):

```bash
task local:destroy
```

Confirme com `y` quando solicitado.

3. Subir só a infra (postgres + otel-lgtm):

```bash
task local:infra
```

4. Aguardar o postgres ficar pronto:

```bash
docker compose --env-file .env \
  -f deployment/compose/compose.yml \
  -f deployment/compose/compose.local.yml \
  exec postgres pg_isready -U mecontrola
```

Repita até retornar `mecontrola:5432 - accepting connections`.

5. Aplicar todas as migrations:

```bash
go run ./cmd migrate
```

Validação:

```bash
docker compose --env-file .env \
  -f deployment/compose/compose.yml \
  -f deployment/compose/compose.local.yml \
  exec postgres psql -U mecontrola -d mecontrola_db \
  -c "SELECT version FROM schema_migrations ORDER BY version;" \
  -c "\dt mecontrola.*"
```

Deve mostrar `version = 2` e listar as 51 tabelas do schema `mecontrola`.

### Cenário B — VPS (produção via SSH)

Usado para recriar o schema em produção sem parar o Postgres.

1. Escalar server e worker para 0 (interrompe tráfego de leitura/escrita):

```bash
ssh root@187.77.45.48 "docker service scale \
  mecontrola_server-1=0 \
  mecontrola_server-2=0 \
  mecontrola_worker-1=0 \
  mecontrola_worker-2=0"
```

2. Dropar e recriar o schema `mecontrola` (destrói todas as tabelas):

```bash
ssh root@187.77.45.48 "
  PGCONTAINER=\$(docker ps --filter name=mecontrola_postgres \
    --format '{{.Names}}' | grep -v exporter | head -1)
  docker exec \$PGCONTAINER psql -U mecontrola -d mecontrola_db \
    -c 'DROP SCHEMA mecontrola CASCADE;' \
    -c 'CREATE SCHEMA mecontrola;'
"
```

3. Executar o serviço de migrate do Swarm:

```bash
ssh root@187.77.45.48 "docker service update --force mecontrola_migrate"
```

> **Nota:** o `--force` inicia um novo task com todas as secrets do Swarm (`DB_PASSWORD`, `META_*`, etc.) já configuradas no serviço. O container de migrate sai com exit 0 após aplicar as migrations; o Swarm tentará reiniciá-lo e falhará (secrets `META_*` indisponíveis no retry), mas as migrations **já foram aplicadas**. Confirme com o passo 5.

4. Validar schema e reverter o migrate para o estado estável:

```bash
ssh root@187.77.45.48 "docker service rollback mecontrola_migrate"
```

5. Confirmar que as migrations foram aplicadas:

```bash
ssh root@187.77.45.48 "
  PGCONTAINER=\$(docker ps --filter name=mecontrola_postgres \
    --format '{{.Names}}' | grep -v exporter | head -1)
  docker exec \$PGCONTAINER psql -U mecontrola -d mecontrola_db \
    -c 'SELECT version FROM schema_migrations ORDER BY version;' \
    -c \"SELECT COUNT(*) AS total_tables FROM information_schema.tables WHERE table_schema = 'mecontrola';\"
"
```

Resultado esperado: `version = 2`, `total_tables = 51`.

6. Escalar server e worker de volta:

```bash
ssh root@187.77.45.48 "docker service scale \
  mecontrola_server-1=1 \
  mecontrola_server-2=1 \
  mecontrola_worker-1=1 \
  mecontrola_worker-2=1"
```

7. Validar health:

```bash
curl -sf http://187.77.45.48/health
```

### Cenário C — dropar e recriar o banco sem destruir volumes (local)

Use quando quiser zerar apenas os dados sem remover os volumes Docker.

1. Parar os containers:

```bash
task local:down
```

2. Subir só a infra:

```bash
task local:infra
```

3. Dropar e recriar o banco:

```bash
docker compose --env-file .env \
  -f deployment/compose/compose.yml \
  -f deployment/compose/compose.local.yml \
  exec postgres psql -U mecontrola \
  -c "DROP DATABASE IF EXISTS mecontrola_db;" \
  -c "CREATE DATABASE mecontrola_db;"
```

4. Aplicar todas as migrations:

```bash
go run ./cmd migrate
```

> **Nota:** a migration `000001` exige a extensão `vector` (pgvector). Certifique-se de que `POSTGRES_IMAGE` no `.env` aponte para uma imagem com pgvector instalado, como `pgvector/pgvector:pg16`.

## Debug no VS Code

O repositório já traz `.vscode/launch.json` e `.vscode/tasks.json`.

Configurações disponíveis:

| Configuração | Uso |
|---|---|
| `server` | Debug do servidor HTTP |
| `worker` | Debug do worker |
| `migrate` | Debug das migrations |
| `Test: current file` | Debug do arquivo de teste atual |
| `Test: current package (run only selected test)` | Debug de teste selecionado |
| `Test: integration suite` | Debug com tag `integration` |
| `server (attach to PID)` | Attach em processo existente |
| `Stack: server + worker (debug both)` | Debug conjunto de server + worker |

Como usar:

1. garanta que `.env` exista
2. abra o VS Code
3. selecione uma configuração
4. pressione `F5`

Observações confirmadas no workspace:

- as configs `server`, `worker` e `migrate` usam `program: ${workspaceFolder}/cmd`
- o `preLaunchTask` padrão chama `task local:infra`
- as configs injetam `DB_HOST=localhost`

## Webhooks locais com ngrok

Fluxo recomendado:

```bash
task ngrok:server
```

Em outro terminal:

```bash
task ngrok:urls
```

O comando imprime:

- webhook de verificação Meta
- webhook inbound Meta/WhatsApp
- webhook Kiwify

Quando terminar:

```bash
task local:down
```

## Deploy e operação

### Caminho padrão de produção

O caminho padrão é:

```bash
task swarm:prod:deploy:full IMAGE_TAG=<tag>
```

Pré-requisitos:

- acesso SSH à VPS
- `deployment/config/prod.env` ajustado
- `deployment/config/prod.secrets.env` criptografado e válido
- `AGE_PRIVATE_KEY`, `SOPS_AGE_KEY` ou `SOPS_AGE_KEY_FILE` definidos

Exemplo:

```bash
export AGE_PRIVATE_KEY="$(cat key.txt)"
task swarm:prod:deploy:full IMAGE_TAG=$(git rev-parse --short HEAD)
```

### Deploy local direto para a VPS

Quando você quiser buildar localmente e transferir a imagem sem depender do GHCR:

```bash
export AGE_PRIVATE_KEY="$(cat key.txt)"
task swarm:prod:deploy:full:local IMAGE_TAG=$(git rev-parse --short HEAD)
```

Ou, via script:

```bash
bash deployment/scripts/deploy-local.sh
```

### Operações Swarm úteis

| Comando | Uso |
|---|---|
| `task swarm:local:config` | Valida o compose Swarm local |
| `task swarm:local:up IMAGE_TAG=<tag>` | Sobe stack Swarm local |
| `task swarm:local:ps` | Lista services do Swarm local |
| `task swarm:local:logs` | Logs do Swarm local |
| `task swarm:local:rm` | Remove stack local |
| `task swarm:prod:sync` | Sincroniza código com a VPS |
| `task swarm:prod:secrets` | Atualiza Docker secrets |
| `task swarm:prod:migrate` | Executa migrations na VPS |
| `task swarm:prod:deploy IMAGE_TAG=<tag>` | Deploy Swarm usando `SECRETS_ENV_FILE` já descriptografado |
| `task swarm:prod:ps` | Lista services na VPS |
| `task swarm:prod:health` | Verifica health checks na VPS |
| `task swarm:prod:rollback PREVIOUS_TAG=<tag>` | Rollback manual usando `SECRETS_ENV_FILE` |
| `task swarm:prod:prune` | Limpeza remota |

### Backup, restore e alertas

| Comando | Uso |
|---|---|
| `task swarm:prod:pgbackrest:check` | Verifica o pgBackRest |
| `task swarm:prod:pgbackrest:backup TYPE=full` | Backup full |
| `task swarm:prod:pgbackrest:backup TYPE=diff` | Backup diferencial |
| `task swarm:prod:pgbackrest:backup TYPE=incr` | Backup incremental |
| `task swarm:prod:pgbackrest:info` | Lista backups |
| `task swarm:prod:alert:test` | Testa alerta Telegram/Grafana |

Para procedimentos críticos e destrutivos, use os runbooks:

- `deployment/runbooks/deploy.md`
- `deployment/runbooks/rollback.md`
- `deployment/runbooks/restore-pitr.md`
- `deployment/runbooks/restore-vps.md`
- `deployment/runbooks/rotate-secret.md`

### Observabilidade em produção

No Swarm canônico:

- `caddy` publica `80` e `443`
- `otel-lgtm` publica `3000`
- `pg-tunnel` publica `127.0.0.1:15432`

Isso significa:

- API pública via Caddy
- Grafana disponível na porta `3000` da VPS
- acesso ao PostgreSQL por túnel local na porta `15432` da VPS

## Acesso remoto

### SSH na VPS

```bash
ssh root@187.77.45.48
cd /opt/mecontrola
```

### Banco de dados

Abra um túnel SSH para o `pg-tunnel` publicado pelo Swarm:

```bash
ssh -N -L 15432:127.0.0.1:15432 root@187.77.45.48
```

Depois conecte seu cliente PostgreSQL com:

| Campo | Valor |
|---|---|
| Host | `localhost` |
| Porta | `15432` |
| Database | `mecontrola_db` |
| User | `mecontrola` |
| Password | valor atual de `DB_PASSWORD` |
| SSL | `disable` |

### Grafana

```bash
ssh -N -L 3001:127.0.0.1:3000 root@187.77.45.48
```

Depois acesse:

```text
http://localhost:3001
```

Use:

| Campo | Valor |
|---|---|
| User | `admin` |
| Password | valor atual de `OTEL_LGTM_ADMIN_PASSWORD` |

## CI/CD

Workflows principais:

| Workflow | Arquivo | Objetivo |
|---|---|---|
| CI | `.github/workflows/ci.yml` | Lint, testes, security e build em PR/merge group |
| CD | `.github/workflows/cd.yml` | Build de imagem, scan, assinatura e deploy em `main` |
| E2E manual | `.github/workflows/e2e.yml` | Suite BDD com Godog |
| Dependabot | `.github/workflows/auto-merge.yml` | Atualizações automáticas e auto-merge controlado |

Resumo do pipeline:

- quality gates
- unit
- integration
- vulncheck
- build
- build/push de imagem
- scan com Trivy
- assinatura com cosign
- deploy Swarm
- healthcheck

## Documentação complementar

### Operação

| Assunto | Arquivo |
|---|---|
| Config UI | `deployment/runbooks/configui.md` |
| Deploy manual/completo | `deployment/runbooks/deploy.md` |
| Rollback | `deployment/runbooks/rollback.md` |
| Restore PITR | `deployment/runbooks/restore-pitr.md` |
| Restore completo da VPS | `deployment/runbooks/restore-vps.md` |
| Rotação de secrets | `deployment/runbooks/rotate-secret.md` |
| Alertas e observabilidade | `deployment/runbooks/alerts-testing.md` |

### Infra e observabilidade

| Assunto | Arquivo |
|---|---|
| Dashboards Grafana | `deployment/dashboards/README.md` |
| Terraform para backups AWS | `deployment/terraform/README.md` |

### API, arquitetura e specs

| Assunto | Local |
|---|---|
| Regras do repositório | `AGENTS.md` |
| PRDs e techspecs | `.specs/` |
| Arquitetura textual | `docs/diagrams/architecture.md` |
| Diagramas C4 | `docs/diagrams/` |
| Coleção Postman | `docs/postman/` |
| Skills externas pinadas | `skills-lock.json` |

## Contribuição

Fluxo mínimo esperado:

1. alinhe escopo em issue ou contexto equivalente para mudanças maiores
2. rode `task setup` ao clonar
3. use Conventional Commits
4. rode `task check` antes de abrir PR
5. não flexibilize regras de arquitetura ou governança

## Governance

Referências canônicas:

| Artefato | Papel |
|---|---|
| `AGENTS.md` | Fonte canônica de regras, arquitetura e skills obrigatórias |
| `.claude/rules/` | Regras transversais e ADRs operacionais |
| `.specs/` | PRDs, techspecs e material de execução |
| `deployment/runbooks/` | Procedimentos operacionais oficiais |

Se o README e o código divergirem, o working tree atual e as regras de `AGENTS.md` prevalecem.
