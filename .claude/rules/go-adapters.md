# Adaptadores Go — Regras de Camada

- Rule ID: R-ADAPTER-001
- Severidade: hard
- Escopo: `internal/<modulo>/infrastructure/http/server/handlers/`,
  `internal/<modulo>/infrastructure/messaging/database/consumers/`,
  `internal/<modulo>/infrastructure/messaging/database/producers/`,
  `internal/<modulo>/infrastructure/jobs/handlers/`

## Objetivo

Garantir que adaptadores de entrada e saida permanecam finos, sem logica de negocio, e que a
politica de zero comentarios em codigo Go seja inegociavel e verificavel.

## R-ADAPTER-001.1 — Zero Comentarios em Codigo Go [HARD]

Nenhum arquivo `.go` de producao pode conter comentarios de linha (`//`) ou bloco (`/* */`),
com excecao unica e explicita de:

- Arquivos gerados por ferramenta com cabecalho `// Code generated` na linha 1 (ex: `mocks/`
  gerados por mockery, `.pb.go`).
- Diretivas de compilador: `//go:build`, `//go:generate`, `//go:embed`, `//nolint:` com
  justificativa na mesma linha.

Comentarios de pacote, inline e bloco sao proibidos em todos os demais arquivos Go — incluindo
`internal/`, `configs/`, `cmd/` e quaisquer novos pacotes.

Gate de verificacao (deve retornar vazio antes de merge — aplica-se a arquivos de producao, exclui `_test.go`):

```bash
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*.pb.go" --exclude="*_test.go" \
  "^[[:space:]]*//" internal/ configs/ cmd/ \
  | grep -Ev "(//go:|//nolint:|// Code generated)" \
  && echo "FAIL: comentarios proibidos encontrados" && exit 1 \
  || true
```

## R-ADAPTER-001.2 — Adaptadores Finos: handler -> usecase [HARD]

Os quatro diretorios de adapter abaixo sao portas de entrada (inbound) ou adapters outbound e
devem permanecer finos sem excecao:

| Tipo | Caminho canonico |
|------|-----------------|
| HTTP Handler | `internal/<modulo>/infrastructure/http/server/handlers/` |
| Consumer (DB outbox) | `internal/<modulo>/infrastructure/messaging/database/consumers/` |
| Job Handler | `internal/<modulo>/infrastructure/jobs/handlers/` |
| Producer (DB outbox) | `internal/<modulo>/infrastructure/messaging/database/producers/` |

Fluxo obrigatorio: `adapter -> usecase -> (repository | service | client)`

Proibido em qualquer arquivo nos quatro caminhos acima:

1. Regra ou calculo de negocio (ex: calcular janela de expiracao, decidir status de dominio).
2. Query SQL direta (`QueryContext`, `ExecContext`, `db.Query`, `tx.Exec`, `db.Exec`).
3. Branching sobre estado de dominio — comparar campos de entidade para decidir comportamento.
4. Injetar `RepositoryFactory`, `database.DBTX`, `manager.Manager` ou service de dominio
   diretamente quando o use case pode receber essa responsabilidade.
5. Chamar repositorio ou client externo sem passar pelo use case correspondente.

Permitido em producers:

- Serializar payload ja decidido pela camada `application`.
- Publicar evento via `outbox.Publisher` ou equivalente.
- Nao decidir trigger, payload semantico ou branching de dominio.

Gate de verificacao (deve retornar vazio antes de merge — exclui `_test.go` pois integration tests usam SQL diretamente em fixtures):

```bash
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "QueryContext\|ExecContext\|db\.Query\|tx\.Exec\|db\.Exec" \
  internal/*/infrastructure/http/server/handlers/ \
  internal/*/infrastructure/messaging/database/consumers/ \
  internal/*/infrastructure/messaging/database/producers/ \
  internal/*/infrastructure/jobs/handlers/ \
  && echo "FAIL: SQL direto em adapter" && exit 1 \
  || true
```

## R-ADAPTER-001.3 — Matriz de Referencias go-implementation por Tipo de Adapter [HARD]

Ao implementar qualquer arquivo nos quatro caminhos de adapter, carregar apenas as referencias da
coluna correspondente. Nunca carregar mais de 4 simultaneas; se conflito, priorizar as 3 mais
criticas e registrar as demais como contexto nao carregado.

| Tipo de Adapter | Referencias obrigatorias | Referencias sob demanda |
|-----------------|--------------------------|------------------------|
| HTTP Handler | `architecture.md`, `api.md` | `observability.md` (se span/tracer), `security.md` (se auth/middleware), `examples-infrastructure.md` (se lifecycle/shutdown) |
| Consumer | `architecture.md`, `messaging.md` | `observability.md` (se span/tracer), `examples-infrastructure.md` (se lifecycle), `testing.md` (se novo consumer com suite) |
| Job Handler | `architecture.md` | `graceful-lifecycle.md` (se shutdown/drain), `observability.md` (se span/tracer), `examples-infrastructure.md` (se lifecycle) |
| Producer | `architecture.md`, `messaging.md` | `observability.md` (se span/tracer), `persistence.md` (se outbox com tx explicita) |

Regra de carregamento dos exemplos concretos:

- `examples-domain-flow.md`: apenas se a tarefa cobrir fluxo end-to-end (dominio + adapter).
- `examples-testing.md`: apenas se a tarefa incluir novo teste com suite/mockery.
- `examples-infrastructure.md`: apenas se a tarefa envolver graceful shutdown, paginacao ou lifecycle.

Proibido: carregar `patterns-structural.md` para qualquer adapter — Factory, Adapter, Decorator
ja estao inline no SKILL.md.

## Proibido (R-ADAPTER-001 global)

- Aprovar PR que adicione comentario de codigo fora das excecoes listadas em R-ADAPTER-001.1.
- Aprovar PR que adicione logica de negocio, SQL direto ou branching de dominio nos quatro
  caminhos de adapter.
- Flexibilizar estas regras por diferenca de ferramenta, conveniencia ou deadline.
