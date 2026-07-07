# Baseline de Efetividade das Skills e Governanca

Este documento estabelece as metricas para medir o impacto das mudancas de governanca e das skills obrigatorias. A baseline inicial deve ser preenchida com dados reais a cada 30 dias.

## Metricas

### 1. Qualidade do Codigo

| Metrica | Definicao | Coleta | Baseline Inicial | Meta 30 dias | Meta 90 dias |
|---|---|---|---|---|---|
| Build quebrado em PR | PRs com `go build ./...` falhando | CI GitHub Actions | 0 | 0 | 0 |
| Vet falhando | PRs com `go vet ./...` falhando | CI GitHub Actions | 0 | 0 | 0 |
| Lint falhando | PRs com `golangci-lint run` falhando | CI GitHub Actions | 0 | 0 | 0 |
| Gate de governanca falhando | PRs com `task ci:*` falhando | CI GitHub Actions | 0 | 0 | 0 |
| `init()` em producao | Ocorrencias de `func init()` fora de testes | `task ci:init-prohibited` | 0 | 0 | 0 |
| Comentarios em producao | Ocorrencias de `//` fora de excecoes | `task ci:zero-comments` | 0 | 0 | 0 |

### 2. Processo de Desenvolvimento

| Metrica | Definicao | Coleta | Baseline Inicial | Meta 30 dias | Meta 90 dias |
|---|---|---|---|---|---|
| Tempo medio de ciclo de PR | Abertura do PR ate merge | GitHub API / gh pr list | A DEFINIR | -10% | -20% |
| Comentarios de revisao por PR | Quantidade de comentarios nao-bots | GitHub API / gh pr view | A DEFINIR | -15% | -25% |
| Rework pos-merge | PRs que geram follow-up em 7 dias | GitHub API / git log | A DEFINIR | -10% | -20% |
| Bundle de dominio validado | % de features com bundle `domain-modeling-production` validado | Auditoria manual | A DEFINIR | 100% | 100% |
| Bundle de pattern validado | % de features com bundle `design-patterns-mandatory` validado (quando aplicavel) | Auditoria manual | A DEFINIR | 100% | 100% |

### 3. Bugs por Categoria

| Metrica | Definicao | Coleta | Baseline Inicial | Meta 30 dias | Meta 90 dias |
|---|---|---|---|---|---|
| Bugs de dominio | Bugs causados por regra/invariante mal modelada | Sistema de tickets | A DEFINIR | -10% | -25% |
| Bugs de SQL/PostgreSQL | Bugs causados por query/migration/indice | Sistema de tickets | A DEFINIR | -10% | -25% |
| Bugs de deployment/ambiente | Bugs causados por compose/config/Docker | Sistema de tickets | A DEFINIR | -10% | -25% |
| Bugs de adaptador | SQL/regra de negocio em handler/consumer/job | Revisao de codigo | A DEFINIR | 0 | 0 |

### 4. Adocao das Skills

| Metrica | Definicao | Coleta | Baseline Inicial | Meta 30 dias | Meta 90 dias |
|---|---|---|---|---|---|
| `go-implementation` carregada | % de tarefas Go com evidencia de uso | Auditoria de sessoes | A DEFINIR | 100% | 100% |
| `domain-modeling-production` carregada | % de features de dominio com evidencia de uso | Auditoria de sessoes | A DEFINIR | 100% | 100% |
| `design-patterns-mandatory` carregada | % de escolhas de pattern com evidencia de uso | Auditoria de sessoes | A DEFINIR | 100% | 100% |
| `postgresql-production-standards` carregada | % de mudancas estruturais PostgreSQL com evidencia de uso | Auditoria de sessoes | A DEFINIR | 100% | 100% |
| `docker-postgres-production-stack` carregada | % de mudancas Docker Swarm + PostgreSQL com evidencia de uso | Auditoria de sessoes | A DEFINIR | 100% | 100% |

## Coleta Automatizada

Scripts sugeridos (a implementar quando houver acesso a API do GitHub):

```bash
# Tempo de ciclo e comentarios por PR
gh pr list --state merged --limit 100 --json number,createdAt,mergedAt,comments > pr_metrics.json

# Bugs por label
gh issue list --label "bug" --state all --limit 200 --json number,labels,createdAt > bug_metrics.json
```

## Revisao Periodica

- **Frequencia**: a cada 30 dias.
- **Responsavel**: tech lead ou owner de engenharia.
- **Acao**: atualizar este arquivo, identificar regressoes e ajustar `AGENTS.md`/`CLAUDE.md` quando necessario.
