# GitHub Actions no MeControla

## Introdução

GitHub Actions é a plataforma de automação nativa do GitHub. No repositório **MeControla**, ela orquestra todo o ciclo de vida do código em um único pipeline: desde a validação de um Pull Request até a publicação da imagem Docker e deploy na VPS.

Os workflows são definidos em arquivos YAML dentro de `.github/workflows/`. Cada workflow é composto por:

- **Triggers**: eventos que disparam a execução (push, pull_request, workflow_dispatch)
- **Jobs**: conjuntos de steps executados em um runner
- **Steps**: comandos individuais ou actions reutilizáveis
- **Actions**: blocos de automação mantidos pelo GitHub ou pela comunidade

---

## Workflows do projeto

| Workflow | Arquivo | Disparo | Objetivo |
|----------|---------|---------|----------|
| **CI/CD** | `.github/workflows/ci-cd.yml` | `push` e `pull_request` na `main`, `workflow_dispatch` | Validar, testar, buildar, publicar e deployar |
| **Auto-merge** | `.github/workflows/auto-merge.yml` | `pull_request` aberta pelo Dependabot | Aprovar e fazer merge automático de atualizações minor/patch |

---

## CI/CD — Pipeline unificado

### Triggers

```yaml
on:
  pull_request:
    branches: [main]
  push:
    branches: [main]
  workflow_dispatch:
    inputs:
      image_tag:
        description: "Tag da imagem a publicar na VPS"
        required: true
        type: string
```

O workflow roda em todo PR direcionado à `main`, em todo push na `main` e pode ser disparado manualmente para deploy de uma imagem específica.

### Estrutura do pipeline

```
setup
├── lint ────────────────────┐
├── unit ───────────────┐    │
├── integration         │    │
├── e2e (BDD)           │    │
├── security            │    │
├── governance          │    │
└── card-audit ─────────┘    │
              │              │
              └──────────────┘
                     ↓
              build-image  (apenas push main)
                     ↓
                deploy      (apenas push main, self-hosted, environment: staging)
                     ↓
                 smoke       (apenas push main, self-hosted, environment: staging)
```

### Jobs de validação (executam em PR e push)

| Job | Responsabilidade |
|-----|------------------|
| `setup` | Prepara o ambiente: checkout, instala Task e Go, cacheia dependências |
| `lint` | Executa `golangci-lint`, verifica formatação e regras PCI do módulo card |
| `unit` | Roda testes unitários com cobertura e faz upload do relatório |
| `integration` | Roda testes de integração com Docker/testcontainers |
| `e2e` | Roda testes BDD com Godog |
| `security` | Executa `govulncheck` para verificar vulnerabilidades |
| `governance` | Valida Taskfile, commits semânticos e spec do projeto |
| `card-audit` | Auditoria R0-R7 + anti-PCI no módulo card |

### Jobs de entrega (executam apenas em push na `main`)

| Job | Responsabilidade |
|-----|------------------|
| `coverage-comment` | Posta comentário de cobertura no PR (apenas em PR) |
| `build-image` | Build e push da imagem Docker para o GHCR |
| `deploy` | Executa o script `deployment/scripts/deploy.sh` na VPS |
| `smoke` | Roda `task auth:smoke` e `task onboarding:smoke` em staging |

### Deploy manual (`workflow_dispatch`)

Quando disparado manualmente, apenas os jobs `deploy-manual` e `smoke-manual` executam, usando a `image_tag` informada. Isso permite re-implantar uma imagem sem re-rodar toda a validação.

### Principais actions

| Action | Versão | Função |
|--------|--------|--------|
| `actions/checkout` | v6 | Clona o repositório |
| `actions/setup-go` | v6 | Instala o Go e cacheia módulos |
| `actions/cache` | v5 | Cacheia `.task/` e artefatos do golangci-lint |
| `actions/upload-artifact` | v7 | Salva relatórios de cobertura e metadados da imagem |
| `actions/download-artifact` | v8 | Recupera artefatos de outros jobs |
| `docker/setup-buildx-action` | v4 | Configura o Docker Buildx |
| `docker/login-action` | v4 | Autentica no GitHub Container Registry |
| `docker/build-push-action` | v6 | Build e push da imagem Docker |

### Variáveis de ambiente

```yaml
env:
  REGISTRY: ghcr.io
  IMAGE_NAME: ghcr.io/limateixeiratecnologia/mecontrola
```

---

## Auto-merge Dependabot

### Objetivo

Aprovar automaticamente PRs do Dependabot quando a atualização for **minor** ou **patch**, e comentar quando for **major** (exigindo revisão manual).

### Condição de execução

```yaml
if: github.actor == 'dependabot[bot]'
```

### Actions

- `actions/checkout@v6`
- `dependabot/fetch-metadata@v3`

---

## Por que usamos actions pinned por major version

As versões das actions são pinadas por major version (`@v6`, `@v7`, etc.) para:

1. **Receber correções de segurança e bugfixes** automaticamente dentro da major version.
2. **Evitar breaking changes** inesperados que poderiam ocorrer com tags flutuantes como `@main`.
3. **Manter compatibilidade** com o runtime Node.js suportado pelo GitHub Actions.

> Em ambientes com requisitos de segurança mais rígidos, é possível pinar pelo SHA completo da action (`uses: actions/checkout@<sha>`). No MeControla usamos major versions para balancear segurança e manutenibilidade.

---

## Segurança e permissões

### Permissões mínimas

Cada job recebe apenas as permissões necessárias:

```yaml
permissions:
  contents: read
```

Apenas jobs específicos recebem permissões extras:

- `coverage-comment`: `pull-requests: write`
- `build-image`: `packages: write`
- `deploy` / `smoke`: acesso a secrets via `environment: staging`

### Segredos

Segredos sensíveis (tokens, senhas, URLs de staging) são armazenados em `Secrets` do repositório e nunca aparecem no código:

```yaml
env:
  GHCR_TOKEN: ${{ secrets.GHCR_TOKEN }}
```

### Environments

Os jobs de deploy e smoke usam `environment: staging`. Isso permite:

- Requerer aprovação manual antes do deploy.
- Restringir secrets específicos ao environment.
- Auditar deploys na aba Environments do GitHub.

---

## Runners

| Tipo | Jobs | Motivo |
|------|------|--------|
| `ubuntu-latest` | setup, lint, unit, integration, e2e, security, governance, card-audit, coverage-comment, build-image | runners gerenciados pelo GitHub, isolados e rápidos |
| `[self-hosted, staging]` | deploy, smoke, deploy-manual, smoke-manual | acesso direto à infraestrutura de staging/VPS |

---

## Como interpretar os logs

1. Acesse a aba **Actions** do repositório.
2. Clique no workflow `CI/CD` executado.
3. Navegue pelo grafo de jobs.
4. Dentro de cada job, expanda os steps para ver os comandos e suas saídas.
5. Anotações de warning ou erro aparecem no topo da página do run.

### Comportamento esperado por evento

| Evento | Jobs que rodam |
|--------|----------------|
| `pull_request` | setup, lint, unit, integration, e2e, security, governance, card-audit, coverage-comment |
| `push` na `main` | todos os jobs de validação + build-image + deploy + smoke |
| `workflow_dispatch` | apenas deploy-manual + smoke-manual |

### Warnings comuns

| Warning | Causa | Solução |
|---------|-------|---------|
| `Node.js 20 is deprecated` | Action em versão antiga | Atualizar para a major version que usa Node 24 |
| `Cache miss` | Chave de cache não encontrada | Normal no primeiro run; próximos runs usam o cache |
| `.env obrigatório não encontrado` | Teste de integração não encontrou `.env` | Workflow cria `.env` no job `integration` antes dos testes |

---

## Atualizações recentes

- Unificação dos workflows `CI` e `CD` em um único arquivo `ci-cd.yml`.
- Migração das actions oficiais para versões com runtime Node 24.
- Substituição da action `arduino/setup-task@v2` por install manual do Task, eliminando dependência de action de terceiros sem suporte a Node 24.
- Correção do job `integration` para criar `.env` em vez de `.env.test`, alinhando com o loader de configuração da aplicação.
- Deploy e smoke tests integrados ao mesmo workflow, com `environment: staging` para controle de acesso.
