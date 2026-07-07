# Decisao: Enums como TEXT + CHECK em vez de CREATE TYPE

## Contexto

A migration `000004_category_write_gate_schema.up.sql` define diversos campos enumerados (`category_kind`, `category_confidence`, `category_match_quality`, `category_signal_type`, `category_decision_source`) como `TEXT` com constraints `CHECK IN (...)`.

## Alternativa considerada

Usar `CREATE TYPE ... AS ENUM` ou `CREATE DOMAIN` para representar os enums no PostgreSQL.

## Decisao

Manter `TEXT` + `CHECK` por ora.

## Justificativa

1. **Compatibilidade com o codigo Go existente**: os campos sao mapeados para `string` no modelo Go. Mudar para `ENUM`/`DOMAIN` exigiria adapters de scan personalizados em todos os repositorios que leem/escrvem essas colunas.
2. **Evolucao gradual**: os valores permitidos ainda estao em estabilizacao. `CREATE TYPE` exige `ALTER TYPE ... ADD VALUE` para novos valores, o que e possivel mas adiciona complexidade operacional.
3. **CHECK IN garante integridade**: a constraint rejeita valores fora do dominio, preservando a invariante de dominio no banco.
4. **Custo de migracao**: alterar o tipo de colunas ja populadas requer reescrita da tabela e sincronizacao com o deploy do codigo.

## Riscos aceitos

- O tipo nao reflete semanticamente o dominio fechado no catalogo do PostgreSQL.
- Ferramentas de introspection podem nao reconhecer o enum automaticamente.

## Nota sobre category_outcome

O campo `category_outcome` esta restrito ao unico valor `'matched'`. Foi mantido porque:
- O dominio preve futuros outcomes (ex.: `'unmatched'`, `'manual'`) que ainda nao foram implementados.
- Remover a coluna agora exigiria reescrita da tabela e ajuste no codigo Go.
- A constraint `CHECK (category_outcome = 'matched')` documenta a invariante atual e pode ser relaxada futuramente sem alteracao de tipo.

## Condicoes de reversao

Reavaliar quando:
- Os valores do enum estiverem estaveis por pelo menos 6 meses.
- Houver adaptadores Go de scan para `ENUM`/`DOMAIN` padronizados no projeto.
- O volume de dados justificar a reescrita da tabela.

## Referencias

- `migrations/000004_category_write_gate_schema.up.sql`
- `internal/transactions/` (repositorios que leem/escrvem as colunas)
