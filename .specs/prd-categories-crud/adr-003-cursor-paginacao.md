# ADR-003: Cursor opaco baseado em último ID + ordenação alfabética

## Metadados

- **Título:** Cursor opaco baseado em último ID + ordenação alfabética
- **Data:** 2026-06-09
- **Status:** Aceita
- **Decisores:** Time de engenharia MeControla
- **Relacionados:** PRD RF-14a, RT-09

## Contexto

O PRD exige paginação cursor-based para `GET /v1/category-dictionary`, com `page_size=50` default e máximo 200. O cursor deve ser opaco para o cliente.

## Decisão

O cursor é a codificação base64 do par `(term_normalized, id)` da última entrada da página atual, separados por `|`:

```
cursor = base64.StdEncoding.EncodeToString([]byte(lastTerm + "|" + lastID.String()))
```

A query usa ordenação por `term_normalized COLLATE "pt_BR", id` com cláusula `WHERE (term_normalized, id) > ($lastTerm, $lastID) LIMIT $pageSize`.

**Decodificação:**
```go
parts := strings.Split(string(decodedBytes), "|")
lastTerm := parts[0]
lastID, err := uuid.Parse(parts[1])
```

**Query:**
```sql
SELECT id, category_id, kind, term, signal_type, confidence, is_ambiguous
FROM mecontrola.category_dictionary
WHERE deprecated_at IS NULL
  AND kind = $1
  AND (term_normalized, id) > ($2, $3)
ORDER BY term_normalized COLLATE "pt_BR", id
LIMIT $4;
```

Quando o cursor é vazio (primeira página), a query omite a cláusula `>`.

## Alternativas Consideradas

| Alternativa | Vantagens | Desvantagens | Motivo de rejeição |
|---|---|---|---|
| Cursor base64 de `id` apenas | Cursor menor (apenas UUID) | Não permite `WHERE (term_normalized, id) >` sem lookup extra; ordenação pode falhar se id não for monotônico com term | Escolhida e depois refinada para incluir term_normalized |
| Offset com LIMIT | Simples de implementar; fácil de saltar páginas | Performance degrada linearmente; inconstante com inserções concorrentes | PRD exige cursor-based explicitamente |
| Timestamp-based cursor | Simples se houver `updated_at` indexado | Não é único; inserções em lote podem ter mesmo timestamp; requer índice composto | Não garante determinismo com seed editorial bulk |

## Consequências

### Benefícios Esperados

- Performance estável O(limit) independente da profundidade da página.
- Consistente com inserções: novas entradas no meio do alfabeto não causam skipped items nem duplicatas entre páginas.
- Cursor pequeno (24 bytes base64).

### Trade-offs e Custos

- Não permite navegação aleatória (saltar para página 10). Aceitável porque o único consumidor previsto é listagem completa ou scroll.
- Requer índice composto `(term_normalized, id)` para eficiência.

### Riscos e Mitigações

| Risco | Impacto | Mitigação |
|---|---|---|
| Cursor decodificado aponta para ID inexistente | Retorna página vazia com `next_cursor` nil | Cursor é apenas ponto de partida; `WHERE (term_normalized, id) >` funciona mesmo com gap |
| Ordenação PT-BR incorreta para caracteres especiais | Itens fora de ordem esperada | `ORDER BY term_normalized COLLATE "pt_BR", id` garante ordenação alfabética PT-BR correta |

## Plano de Implementação

1. Criar índice composto `(term_normalized, id)` na migration baseline.
2. Implementar `DictionaryRepository.List` com lógica de cursor.
3. Codificar/decodificar cursor no handler (camada de transporte).
4. Incluir `next_cursor` no DTO de output quando `has_more=true`.

## Monitoramento e Validação

- Teste de integração que percorre 3 páginas completas e valida continuidade sem gaps nem duplicatas.
- Teste que cursor inválido retorna página vazia ou primeira página (definir no handler).
