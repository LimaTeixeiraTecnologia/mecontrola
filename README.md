# MeControla

[![CI](https://github.com/LimaTeixeiraTecnologia/mecontrola/actions/workflows/ci.yml/badge.svg)](https://github.com/LimaTeixeiraTecnologia/mecontrola/actions/workflows/ci.yml)
![Signed Image](https://img.shields.io/badge/image-signed%20cosign-brightgreen)
![SBOM Available](https://img.shields.io/badge/SBOM-SPDX--JSON-blue)
![Governance](https://img.shields.io/badge/governance-ai--spec-purple)

Monolito modular em Go para fluxos financeiros conversacionais via WhatsApp, com bootstrap separado para HTTP server, worker e migrações.

## Estado atual do projeto

- **Arquitetura:** monolito modular com bounded contexts em `internal/`
- **Módulos ativos no bootstrap:** `identity`, `billing`, `onboarding`, `categories`, `card`, `budgets` e `platform`
- **Entrypoints principais:** `cmd/server`, `cmd/worker` e `cmd/migrate`
- **Mensageria interna:** outbox transacional + dispatcher no worker
- **HTTP inbound:** Chi
- **Deploy alvo:** VPS com Docker Compose, GHCR, cosign e Caddy

## Stack

| Componente | Versão / detalhe atual |
|---|---|
| Go | `1.26.4` |
| devkit-go | `v0.5.0` |
| Router HTTP | `github.com/go-chi/chi/v5 v5.3.0` |
| CLI | Cobra |
| Banco | PostgreSQL 16 (`postgres:16-alpine`) |
| Observabilidade local | `grafana/otel-lgtm:0.7.5` |
| Proxy de produção | Caddy 2 |
| Automação local | Task `3.51.1` |
| Registro de imagem | `ghcr.io/limateixeiratecnologia/mecontrola` |
| Supply chain | Trivy + cosign + SBOM SPDX-JSON |

## Módulos e responsabilidades

| Módulo | Responsabilidade atual |
|---|---|
| `internal/identity` | usuários, principal/auth, projeções de assinatura, webhook WhatsApp inbound e housekeeping de `auth_events` |
| `internal/billing` | webhook Kiwify, reconciliação de assinaturas, grace period, housekeeping de eventos e publicação de eventos de assinatura |
| `internal/onboarding` | checkout/magic token, ativação via WhatsApp, outreach, expiração de tokens e limpeza de mensagens Meta processadas |
| `internal/categories` | catálogo de categorias e dicionário de categorias com busca HTTP |
| `internal/card` | CRUD de cartões, listagem, consulta e cálculo de fatura por competência |
| `internal/budgets` | orçamentos mensais, recorrência, despesas, resumo mensal, alertas e jobs de retenção/reprocessamento |
| `internal/platform` | eventos, outbox, worker manager, HTTP client compartilhado, idempotência e capacidades transversais |

## Rotas HTTP atuais

| Método | Rota | Origem |
|---|---|---|
| `GET` | `/health` | health endpoint do servidor |
| `GET` | `/ready` | readiness com checagem de banco |
| `POST` | `/api/v1/identity/users/` | identity |
| `GET` | `/api/v1/whatsapp/verify` | webhook Meta |
| `POST` | `/api/v1/whatsapp/inbound` | webhook Meta inbound |
| `POST` | `/api/v1/billing/webhooks/kiwify` | webhook Kiwify |
| `POST` | `/api/v1/onboarding/checkout` | onboarding |
| `GET` | `/api/v1/onboarding/tokens/{token}/state` | onboarding |
| `GET` | `/api/v1/categories/` | categories |
| `GET` | `/api/v1/categories/{id}` | categories |
| `GET` | `/api/v1/category-dictionary/` | categories |
| `GET` | `/api/v1/category-dictionary/search` | categories |
| `POST` | `/api/v1/cards/` | card |
| `GET` | `/api/v1/cards/` | card |
| `GET` | `/api/v1/cards/{id}/` | card |
| `PUT` | `/api/v1/cards/{id}/` | card |
| `DELETE` | `/api/v1/cards/{id}/` | card |
| `GET` | `/api/v1/cards/{id}/invoices` | card |
| `POST` | `/api/v1/budgets/` | budgets |
| `POST` | `/api/v1/budgets/recurrence` | budgets |
| `GET` | `/api/v1/budgets/alerts` | budgets |
| `POST` | `/api/v1/budgets/expenses` | budgets |
| `PATCH` | `/api/v1/budgets/expenses/{id}` | budgets |
| `DELETE` | `/api/v1/budgets/expenses/{id}` | budgets |
| `POST` | `/api/v1/budgets/{competence}/activate` | budgets |
| `DELETE` | `/api/v1/budgets/{competence}` | budgets |
| `GET` | `/api/v1/budgets/{competence}/summary` | budgets |

Consulte também:

- [docs/postman/README.md](docs/postman/README.md)
- [internal/categories/openapi.yaml](internal/categories/openapi.yaml)
- [internal/budgets/openapi.yaml](internal/budgets/openapi.yaml)

## Estrutura do repositório

```text
cmd/
  main.go
  migrate/
  server/
  worker/
configs/
deployment/
  caddy/
  compose/
  docker/
  grafana/
  promtail/
  runbooks/
  scripts/
  telemetry/
docs/
  adrs/
  discoveries/
  epics/
  grafana/
  postman/
  refactors/
  runbooks/
  runs/
internal/
  billing/
  budgets/
  card/
  categories/
  identity/
  onboarding/
  platform/
migrations/
taskfiles/
```

## Ambiente local

### Pré-requisitos

| Ferramenta | Obrigatório |
|---|---|
| Docker Engine + Compose v2 | sim |
| Task `3.51.1` | sim |
| Go `1.26.4` | sim |
| `golangci-lint` | para `task lint:run` |
| `govulncheck` + `trivy` | para `task security:vulncheck` |

### Setup

```sh
git clone https://github.com/LimaTeixeiraTecnologia/mecontrola.git
cd mecontrola
cp .env.example .env
task setup
```

### Variáveis mínimas para bootstrap local

Hoje o bootstrap de `server` e `worker` instancia `identity`, `billing`, `onboarding`, `categories`, `card` e `budgets` logo na subida. Por isso, além do banco, o `.env` precisa ter valores válidos para os pontos abaixo.

Use valores de desenvolvimento, por exemplo:

```env
DB_PASSWORD=mecontrola_local_password

ONBOARDING_TOKEN_ENCRYPTION_KEY=0123456789abcdef0123456789abcdef
META_PHONE_NUMBER_ID=local-phone-number-id
META_ACCESS_TOKEN=local-meta-access-token
META_APP_SECRET=local-meta-app-secret
META_VERIFY_TOKEN=local-meta-verify-token

KIWIFY_PRODUCT_ID_MONTHLY=local-monthly
KIWIFY_PRODUCT_ID_QUARTERLY=local-quarterly
KIWIFY_PRODUCT_ID_ANNUAL=local-annual
KIWIFY_WEBHOOK_SECRET=local-kiwify-webhook-secret
```

Se quiser exercitar fluxos reais, complete também as credenciais de Meta Cloud API e Kiwify no `.env`.

### Subir o ambiente

O `compose` base aponta por padrão para `ghcr.io/limateixeiratecnologia/mecontrola:latest`. Se você não tiver acesso de pull ao GHCR privado, use a imagem local.

Construindo imagem local:

```sh
task build:docker:build IMAGE_TAG=dev
IMAGE_NAME=mecontrola IMAGE_TAG=dev task local:seed
```

Se você tiver acesso ao GHCR e quiser usar a imagem remota publicada:

```sh
task local:seed
```

Isso sobe:

- `postgres`
- `otel-lgtm`
- executa `migrate`
- sobe `server`
- sobe `worker`

Resultado validado localmente em `2026-06-11`:

- `task build:docker:build IMAGE_TAG=dev` construiu a imagem local `mecontrola:dev`
- `IMAGE_NAME=mecontrola IMAGE_TAG=dev task local:seed` subiu `postgres`, `otel-lgtm`, `server` e `worker`
- o passo final de `migrate` aplicou as migrations com sucesso em banco limpo
- uma segunda execução explícita de `migrate` terminou com sucesso, sem reaplicar a migration inicial
- o fluxo local foi ajustado para não subir `migrate` implicitamente dentro de `task local:up`, evitando corrida com `task local:seed`
- o endpoint OTLP local foi alinhado para `grpc` em formato `host:port`, sem `http://`
- a tabela de versionamento do `golang-migrate` ficou fixada em `public.schema_migrations`, evitando divergência de schema entre a primeira execução e os reruns
- `task local:up` passou a subir o ambiente na ordem `infra -> migrate -> app`, evitando crash de bootstrap em banco vazio

### Troubleshooting do bootstrap local

Se `task local:seed` falhar no `migrate` com erro parecido com:

```text
relation "outbox_events" already exists
```

no estado atual do repositório, o cenário mais provável é volume local do Postgres carregando resíduos de execuções antigas, anteriores à correção do fluxo de migrations. A causa raiz original foi eliminada:

- `task local:up` não sobe mais o serviço `migrate` implicitamente
- o `cmd/migrate` usa tabela de controle fixa em `public.schema_migrations`
- o runtime não executa startup migrations paralelas no bootstrap local atual

Para recriar o ambiente do zero:

```sh
task local:reset
IMAGE_NAME=mecontrola IMAGE_TAG=dev task local:seed
```

Se você quiser manter os dados locais, não rode `local:reset`; primeiro inspecione o estado atual das migrations e do schema antes de aplicar nova remediation.

Observações:

- `task local:up` e os bootstraps de `ngrok` sobem apenas `postgres`, `otel-lgtm`, `server`, `worker` e, quando aplicável, `caddy`
- `task local:up`, `task local:seed` e os bootstraps de `ngrok` executam `migrate` antes de iniciar a app
- para OTLP `grpc`, use endpoint local em formato `localhost:4317` ou `otel-lgtm:4317`, sem prefixo `http://`

### Endpoints locais

| Serviço | Endereço |
|---|---|
| API | `http://localhost:8080` |
| Health | `http://localhost:8080/health` |
| Ready | `http://localhost:8080/ready` |
| Grafana local | `http://localhost:3000` |
| OTLP gRPC | `localhost:4317` |
| OTLP HTTP | `localhost:4318` |
| PostgreSQL | `localhost:5432` |

Credenciais padrão do LGTM local:

```text
admin / admin@dev
```

### Comandos locais úteis

```sh
task local:up
task local:down
task local:logs
task local:ps
task local:reset
task --list-all
```

### Webhooks locais com ngrok

Use `ngrok` apenas para desenvolvimento local e homologação manual. Ele não faz parte do deployment publicado na VPS.

#### Passo 1. Confirmar pré-requisitos

Você precisa ter:

- `.env` criado na raiz do projeto
- Docker Engine + Docker Compose v2
- `curl`
- `ngrok` instalado e autenticado localmente

Para validar tudo de uma vez:

```sh
task ngrok:check
```

Se o comando falhar:

- copie `.env.example` para `.env` caso o arquivo ainda não exista
- confirme que `ngrok config check` passa localmente
- confirme que `docker compose version` funciona no terminal

#### Passo 2. Garantir variáveis mínimas no `.env`

O ambiente local precisa subir `server`, `worker`, `postgres` e, opcionalmente, `caddy`. Antes de abrir o túnel, confirme pelo menos os valores mínimos já descritos neste README.

Se você for receber callbacks reais de Meta ou Kiwify, substitua os valores locais pelos segredos e IDs reais do ambiente que será testado.

#### Passo 3. Escolher como expor a aplicação

Opção mais simples, expondo o `server` direto em `127.0.0.1:8080`:

```sh
IMAGE_NAME=mecontrola IMAGE_TAG=dev task ngrok:server
```

Essa opção:

- valida pré-requisitos
- sobe `postgres`, `server`, `worker` e `otel-lgtm`
- espera o `health` responder em `http://127.0.0.1:8080/health`
- abre o túnel `ngrok` em foreground

Opção mais próxima da borda de produção, passando pelo `caddy` local:

```sh
IMAGE_NAME=mecontrola IMAGE_TAG=dev task ngrok:caddy
```

Essa opção:

- valida pré-requisitos
- sobe o mesmo ambiente local com `--profile proxy`
- espera o `health` responder em `http://127.0.0.1:80/health`
- abre o túnel `ngrok` apontando para o `caddy`

Quando usar cada uma:

- `task ngrok:server`: menor atrito para testar callbacks e webhooks locais.
- `task ngrok:caddy`: útil para validar o tráfego passando pela mesma borda reversa usada em produção.

#### Passo 4. Manter o túnel aberto

Os comandos `task ngrok:server` e `task ngrok:caddy` deixam o `ngrok` rodando em foreground. Não feche esse terminal enquanto o provedor externo precisar acessar sua máquina.

Se o processo for interrompido:

- a URL pública muda quando um novo túnel for criado
- você precisará atualizar novamente a URL cadastrada na Meta ou na Kiwify

#### Passo 5. Descobrir as URLs públicas geradas

Em outro terminal, descubra as URLs públicas montadas para os webhooks:

```sh
task ngrok:urls
```

Saídas esperadas:

- `https://<host-ngrok>/api/v1/whatsapp/verify`
- `https://<host-ngrok>/api/v1/whatsapp/inbound`
- `https://<host-ngrok>/api/v1/billing/webhooks/kiwify`

O comando lê a API local do `ngrok` em `127.0.0.1:4040`. Se ele falhar, o túnel ainda não foi iniciado ou não está acessível.

#### Passo 6. Cadastrar a URL no provedor externo

Para Meta WhatsApp, use as URLs do passo anterior nos endpoints do webhook:

- verificação: `https://<host-ngrok>/api/v1/whatsapp/verify`
- inbound: `https://<host-ngrok>/api/v1/whatsapp/inbound`

Para Kiwify, use:

- `https://<host-ngrok>/api/v1/billing/webhooks/kiwify`

Antes de apontar o provedor para a URL do `ngrok`, confirme localmente que o endpoint responde:

```sh
curl -i http://127.0.0.1:8080/health
curl -i http://127.0.0.1:8080/ready
```

Se estiver usando `task ngrok:caddy`, você também pode validar:

```sh
curl -i http://127.0.0.1/health
```

#### Passo 7. Usar domínio reservado do ngrok, se necessário

Se você tiver domínio reservado no ngrok, pode sobrescrever em tempo de execução:

```sh
IMAGE_NAME=mecontrola IMAGE_TAG=dev task ngrok:server NGROK_DOMAIN=mecontrola-dev.ngrok.app
```

O mesmo vale para a opção com proxy:

```sh
IMAGE_NAME=mecontrola IMAGE_TAG=dev task ngrok:caddy NGROK_DOMAIN=mecontrola-dev.ngrok.app
```

#### Passo 8. Encerrar o túnel e o ambiente local

Para encerrar o `ngrok`, volte ao terminal onde a tarefa está rodando e use `Ctrl+C`.

Para desligar os containers locais:

```sh
task local:down
```

Se quiser relembrar essas instruções rapidamente no terminal:

```sh
task ngrok:stop:tips
```

## CLI da aplicação

```sh
go run ./cmd --help
```

Subcomandos atuais:

```text
mecontrola server
mecontrola worker
mecontrola migrate
mecontrola migrate-down --steps 1
```

## Desenvolvimento

### Rodar localmente fora do Docker Compose

```sh
task build
./bin/mecontrola server
./bin/mecontrola worker
./bin/mecontrola migrate
```

### Testes

```sh
task test:unit
task test:integration
task test:coverage
task test:coverage:identity
task card:test
task card:integration
```

### Lint, segurança e validação rápida

```sh
task lint:run
task lint:fmt
task lint:fmt:check
task lint:pci
task security:vulncheck
task check
```

### Mocks, benchmarks e auditorias

```sh
task mocks
task bench:outbox
task card:audit
```

### Smokes disponíveis

```sh
task auth:smoke
task onboarding:smoke
```

### Load tests disponíveis

```sh
task loadtest:card
task loadtest:card:mixed
task loadtest:card:setup
task loadtest:card:teardown
```

## Worker atual

O `cmd/worker` monta um `worker.Manager` com jobs e handlers dos módulos. Hoje ele inclui:

- dispatcher, reaper e housekeeping do outbox
- housekeeping de `auth_events`
- reconciliação de billing
- housekeeping de eventos Kiwify
- expiração de grace period
- outreach de onboarding
- expiração de tokens de onboarding
- limpeza de mensagens Meta já processadas
- reaper de rascunhos abandonados em budgets
- reaper de eventos pendentes em budgets
- purge de retenção em budgets

## Build e imagem Docker

### Binário

```sh
task build
task build:build:all
```

### Imagem

Build local:

```sh
task build:docker:build IMAGE_TAG=dev
```

Build para tag versionada:

```sh
SHA=$(git rev-parse --short HEAD)
task build:docker:build IMAGE_TAG=${SHA}
task security:image-scan IMAGE_SHA=${SHA}
task security:sbom IMAGE_SHA=${SHA}
```

## CI/CD atual

### CI

O workflow `.github/workflows/ci.yml` executa:

- lint
- formatação
- testes unitários
- testes de integração
- `govulncheck` + `trivy fs`
- governança (`ai-spec`, conventional commits, validação do Taskfile)
- `auth:smoke` em `main`

### CD

O workflow `.github/workflows/cd.yml` faz:

1. build e push da imagem para GHCR
2. scan Trivy da imagem
3. geração de SBOM
4. assinatura e attestations com cosign
5. deploy para VPS via `deployment/scripts/deploy.sh`

Em `workflow_dispatch`, o deploy aceita `image_tag` explícita.

## Deploy em produção

Os arquivos de compose atuais são:

- `deployment/compose/compose.yml`
- `deployment/compose/compose.prod.yml`

O script operacional atual é:

```sh
bash deployment/scripts/deploy.sh <image-tag>
```

Fluxo resumido do deploy:

```text
git push / workflow_dispatch
  -> build e push GHCR
  -> trivy image
  -> sbom
  -> cosign sign + attest
  -> SSH na VPS
  -> docker compose pull
  -> migrate
  -> up -d server worker
  -> smoke em /health
```

Em produção, `server` e `worker` rodam com:

- `read_only: true`
- `tmpfs` para `/tmp`
- `cap_drop: [ALL]`
- `no-new-privileges`
- `user 65532:65532`

## Segurança

### Verificar assinatura da imagem

```sh
cosign verify \
  --certificate-identity-regexp '^https://github\.com/LimaTeixeiraTecnologia/mecontrola/' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  ghcr.io/limateixeiratecnologia/mecontrola:<sha>
```

### Scan local

```sh
task security:vulncheck
task security:scan
```

Para reporte de vulnerabilidades, consulte [SECURITY.md](SECURITY.md).

## Governança

As regras operacionais do repositório estão em:

- [AGENTS.md](AGENTS.md)
- [CLAUDE.md](CLAUDE.md)
- [GEMINI.md](GEMINI.md)

As automações de desenvolvimento usam:

- [Taskfile.yml](Taskfile.yml)
- [taskfiles/](taskfiles/)
- [migrations/](migrations/)
- [deployment/](deployment/)
