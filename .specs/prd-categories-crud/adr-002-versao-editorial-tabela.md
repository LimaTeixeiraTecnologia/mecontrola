# ADR-002: Versão editorial monotônica em tabela dedicada

## Metadados

- **Título:** Versão editorial monotônica em tabela dedicada
- **Data:** 2026-06-09
- **Status:** Aceita
- **Decisores:** Time de engenharia MeControla
- **Relacionados:** PRD RF-18a, RF-36a, RT-09

## Contexto

O PRD exige que toda resposta de leitura exponha uma versão editorial monotônica `N` via header `ETag: "v<N>"` e campo `version` no corpo, com suporte a `If-None-Match: "v<N>"` → `304 Not Modified`. A versão deve incrementar a cada migration editorial aplicada com sucesso e nunca regredir.

## Decisão

Armazenar a versão editorial em uma tabela dedicada de uma única linha:

```sql
CREATE TABLE mecontrola.category_editorial_version (
    version     BIGINT      NOT NULL PRIMARY KEY DEFAULT 1,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

- Valor inicial: `1` (inserido pela migration baseline do módulo).
- Cada migration editorial de seed/dicionário incrementa `version` com `UPDATE mecontrola.category_editorial_version SET version = version + 1, updated_at = now()`.
- `VersionReader` lê essa linha em todo endpoint de leitura, inclusive antes de retornar erros (`404 not_found`, `422 invalid_query`, `422 invalid_kind`).
- Toda resposta de leitura (sucesso ou erro) inclui header `ETag: "v<N>"` e campo `version` no corpo JSON.
- Rollback editorial (deprecação + novo item) também incrementa a versão.

## Alternativas Consideradas

| Alternativa | Vantagens | Desvantagens | Motivo de rejeição |
|---|---|---|---|
| Coluna `version` em cada categoria/dicionário | Versionamento granular por item | ETag precisaria de `MAX(version)` a cada request; complexidade de schema; inconsistência se esquecer de atualizar uma linha | Não atende requisito de versão global monotônica simples |
| Trigger no INSERT/UPDATE das tabelas de seed | Automático, sem esquecimento | Rollback por `UPDATE` em `deprecated_at` dispara trigger; difícil de distinguir migration editorial de correção de bug | Menos explícito para revisão em PR; trigger esconde lógica |
| Hardcoded no código Go (`const currentVersion = 42`) | Zero query extra | Exige deploy para versionar; não permite hot-migration sem rebuild | Viola RF-36a que exige migration-only |

## Consequências

### Benefícios Esperados

- Single source of truth para versão editorial.
- Query simples e cacheável (`SELECT version FROM mecontrola.category_editorial_version`).
- Rastreabilidade: `updated_at` indica quando a última migration editorial foi aplicada.

### Trade-offs e Custos

- Uma query extra por endpoint de leitura. Mitigação: Postgres cacheia a linha em memória (1 linha, acesso frequente).
- Ponto de contenção teórico em escritas concorrentes de migrations. Mitigação: migrations editoriais rodam em janela controlada, serializadas pelo processo de migration.

### Riscos e Mitigações

| Risco | Impacto | Mitigação |
|---|---|---|
| Migration esquece de incrementar versão | Cache stale, consumidores não invalidam | Template de migration editorial inclui UPDATE obrigatório; teste de integração valida incremento |
| Versão overflow de BIGINT | Impossível na escala do projeto (4 migrations/mês) | BIGINT permite ~9 quintilhões |

## Plano de Implementação

1. Criar tabela na migration baseline do módulo categories.
2. Inserir linha inicial `version = 1`.
3. Implementar `VersionReader` em Postgres.
4. Nos handlers, ler versão, comparar com `If-None-Match`, retornar 304 quando igual.
5. Incluir `version` em todo DTO de output.

## Monitoramento e Validação

- Teste de integração que aplica migration editorial e valida `version` incrementado.
- Teste que `If-None-Match: "v1"` retorna 304 após baseline, e 200 após nova migration.
