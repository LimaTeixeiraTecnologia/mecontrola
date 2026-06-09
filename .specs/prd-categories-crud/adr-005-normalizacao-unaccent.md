# ADR-005: Normalização accent-insensitive via coluna gerada PostgreSQL

## Metadados

- **Título:** Normalização accent-insensitive via coluna gerada PostgreSQL
- **Data:** 2026-06-09
- **Status:** Aceita
- **Decisores:** Time de engenharia MeControla
- **Relacionados:** PRD RF-20, RT-09

## Contexto

O PRD exige que busca e unicidade editorial sejam case-insensitive e accent-insensitive. A implementação deve usar coluna gerada `term_normalized GENERATED ALWAYS AS (lower(unaccent(term))) STORED` com índice B-tree.

## Decisão

Implementar a normalização exclusivamente no banco via coluna gerada:

```sql
term_normalized TEXT GENERATED ALWAYS AS (lower(unaccent(term))) STORED
```

- A aplicação Go não computa normalização própria.
- Toda comparação de igualdade usa `term_normalized = lower(unaccent($1))` no servidor.
- Não há paridade Go/SQL para manter.

## Alternativas Consideradas

| Alternativa | Vantagens | Desvantagens | Motivo de rejeição |
|---|---|---|---|
| Normalização em Go antes de enviar para Postgres | Controle total; não depende de extensão | Paridade Go/SQL obrigatória; divergência de locale; código extra | PRD RF-20 manda explicitamente que a aplicação não compute normalização própria |
| Usar `citext` | Case-insensitive nativo | Não resolve acentos; comportamento de locale dependente do servidor | Não atende accent-insensitive |
| Usar `pg_trgm` ou full-text search | Fuzzy matching; busca parcial | Viola RF-25 (sem fuzzy) e RF-26 (correspondência exata do termo completo) | Fora do escopo do MVP |

## Consequências

### Benefícios Esperados

- Sempre sincronizado com `term`; impossível de desnormalizar.
- Índice B-tree eficiente para igualdade e range.
- Sem lógica duplicada entre Go e SQL.

### Trade-offs e Custos

- Extensão `unaccent` é obrigatória no Postgres; deve estar habilitada antes da migration de seed.
- `GENERATED ALWAYS` ocupa espaço em disco (armazena valor derivado). Com ~5k entradas, impacto é negligenciável.

### Riscos e Mitigações

| Risco | Impacto | Mitigação |
|---|---|---|
| `unaccent` não instalado em ambiente de produção | Migration falha | Migration de habilitação roda antes; runbook de deploy lista extensão como pré-requisito; CI usa mesma imagem Postgres |
| `unaccent` trata algum caractere PT-BR de forma inesperada | Busca não encontra termo válido | Testes de integração cobrem termos com acentos, cedilha, til |

## Plano de Implementação

1. Migration habilita `CREATE EXTENSION IF NOT EXISTS unaccent;`.
2. Migration cria tabela `category_dictionary` com `term_normalized GENERATED ALWAYS`.
3. Índice único parcial em `(kind, category_id, term_normalized)`.
4. Queries de busca usam `lower(unaccent($1))` sem computação em Go.

## Monitoramento e Validação

- Teste de integração que insere "Água", busca por "agua", "água", "AGUA" e encontra o mesmo registro.
- Teste de unicidade que tenta inserir "água" e "agua" para a mesma subcategoria e falha com violação de índice único.
