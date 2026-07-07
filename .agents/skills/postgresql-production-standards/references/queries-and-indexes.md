# Queries e Indices

## Fontes Oficiais
- Performance tips: https://www.postgresql.org/docs/current/performance-tips.html
- Indexes: https://www.postgresql.org/docs/current/indexes.html
- Tipos de indice: https://www.postgresql.org/docs/current/indexes-types.html
- Multicoluna: https://www.postgresql.org/docs/current/indexes-multicolumn.html
- Expressoes: https://www.postgresql.org/docs/current/indexes-expressional.html
- Parciais: https://www.postgresql.org/docs/current/indexes-partial.html
- Index-only scans: https://www.postgresql.org/docs/current/indexes-index-only-scans.html
- `EXPLAIN`: https://www.postgresql.org/docs/current/using-explain.html
- `CREATE INDEX`: https://www.postgresql.org/docs/current/sql-createindex.html

## Regras Mandatorias
- Criar indice apenas quando houver predicado, ordenacao, join ou padrao de acesso observavel que o justifique.
- Preferir o menor numero de indices capaz de sustentar os planos de acesso essenciais; cada indice adicional aumenta custo de escrita, `VACUUM` e armazenamento.
- Em B-tree multicoluna, alinhar a ordem das colunas ao padrao dominante de filtro e ordenacao observado.
- Usar indice parcial apenas quando o predicado for estavel e aparecer de forma consistente nas consultas relevantes.
- Usar indice por expressao apenas quando a expressao estiver presente na consulta e o custo de manutencao estiver justificado.
- Exigir `EXPLAIN` para tuning e `EXPLAIN ANALYZE` quando for seguro executar a consulta em ambiente representativo.
- Para paginacao em alto volume, preferir estrategias baseadas em chave estavel observavel quando `OFFSET` crescente se tornar custo dominante.

## Bloqueios Obrigatorios
- Bloquear recomendacao de indice sem SQL, predicados, `JOIN`, `ORDER BY`, cardinalidade aproximada ou plano.
- Bloquear remocao de indice sem evidencia de nao uso, redundancia ou impacto de manutencao.
- Bloquear tuning de query sem distinguir se o gargalo e I/O, cardinalidade, sort, hash, nested loop inadequado ou falta de estatisticas.

## Evidencia Minima
- SQL completo ou builder equivalente.
- `EXPLAIN` ou `EXPLAIN ANALYZE` quando o pedido for desempenho.
- Volume aproximado, seletividade esperada e padrao de acesso.
