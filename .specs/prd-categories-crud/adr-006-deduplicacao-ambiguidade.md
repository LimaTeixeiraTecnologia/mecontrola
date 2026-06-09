# ADR-006: Deduplicação por precedência editorial e ambiguidade estrita

## Metadados

- **Título:** Deduplicação por precedência editorial e ambiguidade estrita
- **Data:** 2026-06-09
- **Status:** Aceita
- **Decisores:** Time de engenharia MeControla
- **Relacionados:** PRD RF-27, RF-21, RF-22, RF-23

## Contexto

O PRD exige que a busca do dicionário retorne no máximo três candidatos, deduplicados por `category_id`, com precedência editorial definida e ambiguidade estrita quando múltiplas subcategorias distintas casam com o mesmo `q`.

## Decisão

A lógica de resolução de candidatos fica em um domain service stateless `CandidateResolver`, aplicado pelo use case `SearchDictionary` após o repositório retornar todas as entradas ativas que casam com `q`.

### Algoritmo

1. **Filtrar** entradas com `deprecated_at IS NULL` e `kind = $kind`.
2. **Agrupar** por `category_id`.
3. **Para cada grupo**, escolher a entrada de maior precedência editorial:
   - Ordem: `canonical_name (5) > alias (4) > phrase (3) > merchant (2) > segment (1)`.
   - Empate no mesmo `signal_type`: resolver por caminho alfabético PT-BR (`root_name > subcategory_name`).
4. **Deduplicar**: manter apenas a entrada vencedora de cada grupo.
5. **Ordenar** os grupos vencedores pela mesma precedência + caminho alfabético.
6. **Limitar** a 3 candidatos.
7. **Regra de ambiguidade estrita**: se o número de grupos vencedores distintos for > 1, **todos** os candidatos retornados recebem `is_ambiguous=true` na resposta, independente da confiança individual da entrada vencedora.
8. **Has_more**: `true` quando existe ao menos um `category_id` ativo correspondente a `q` que foi descartado pelo limite de 3.

### Exemplo de saída

- `q=uber`, `kind=expense`:
  - Encontra `merchant` em `Transporte por Aplicativo Recorrente` e `Transporte de Lazer`.
  - Ambos são `merchant` (mesmo signal_type).
  - Ordenação por caminho alfabético: `Prazeres > Transporte de Lazer` vem antes de `Custo Fixo > Transporte por Aplicativo Recorrente`? Não — alfabético por path completo: "Custo Fixo > Transporte por Aplicativo Recorrente" vs "Prazeres > Transporte de Lazer". "Custo Fixo" < "Prazeres", então Custo Fixo vem primeiro.
  - Como há 2 grupos, ambos recebem `is_ambiguous=true`.

## Alternativas Consideradas

| Alternativa | Vantagens | Desvantagens | Motivo de rejeição |
|---|---|---|---|
| Deduplicação e ordenação no SQL | Performance potencialmente melhor | Query extremamente complexa com window functions e CASE; difícil de manter e testar | Legibilidade e testabilidade são prioritárias na volumetria-alvo (~5k entradas) |
| Ambiguidade apenas quando entrada individual é ambígua | Mais granular | Viola RF-27 explicitamente: "coexistência de subcategorias distintas é, por si só, ambiguidade" | Contradiz requisito fechado do PRD |
| Fuzzy matching + ranking | Recupera mais candidatos | Viola RF-25 (sem fuzzy) e RF-26 (exata completa) | Fora do escopo |

## Consequências

### Benefícios Esperados

- Regra centralizada em service de domínio puro, testável sem infraestrutura.
- Comportamento determinístico e auditável.
- Proteção contra falso positivo: nunca promove termo ambíguo a decisão final.

### Trade-offs e Custos

- Aplicação recebe todas as entradas casadas do Postgres e filtra em memória. Na volumetria-alvo (máximo ~12 entradas por subcategoria, ~400 subcategorias), o volume é insignificante. Se o dataset crescer além do MVP, a deduplicação pode ser movida para SQL.

### Riscos e Mitigações

| Risco | Impacto | Mitigação |
|---|---|---|
| Merchant com dezenas de subcategorias gera lista grande em memória | Degradação de performance | Limitar query do repositório a um número seguro (ex: 100 entradas); volumetria-alvo torna isso improvável |
| Precedência editorial mudar no futuro | Mudança de comportamento sem mudança de código | Precedência é constante no código; mudança exige PR e teste de regressão |

## Plano de Implementação

1. Implementar `CandidateResolver` em `domain/services/candidate_resolver.go`.
2. Unit tests com table-driven suite cobrindo todos os cenários de RF-27.
3. Integrar no use case `SearchDictionary`.
4. Testes de integração cobrindo CC-B1 a CC-B5.

## Monitoramento e Validação

- Métrica `category_dictionary_search_total` com label `outcome=ambiguous` monitora frequência de ambiguidade.
- Teste negativo para cada termo listado em RF-34 (ex: `compra`, `pix`, `boleto`) garantindo que, se existirem, retornam `is_ambiguous=true` ou `no_match`.
